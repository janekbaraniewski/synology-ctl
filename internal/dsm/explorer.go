package dsm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// APIEntry is the explorer-friendly shape of a SYNO.API.Info row.
type APIEntry struct {
	Name string
	Path string
	Min  int
	Max  int
}

// APIList returns every API the device advertises, sorted by name.
// Info() should have been called at least once; the slice will be empty
// otherwise.
func (c *Client) APIList() []APIEntry {
	c.mu.RLock()
	out := make([]APIEntry, 0, len(c.apiPaths))
	for name, info := range c.apiPaths {
		out = append(out, APIEntry{
			Name: name,
			Path: info.Path,
			Min:  info.MinVersion,
			Max:  info.MaxVersion,
		})
	}
	c.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// CallRaw issues an arbitrary DSM request and returns the raw `data`
// field of the JSON envelope. On a DSM-reported failure it returns a
// typed *Error so callers can show the code in the same UI the rest of
// the app uses.
//
// This is the universal-explorer's escape hatch: we don't decode into a
// typed Go struct, we just hand the JSON back for pretty-printing.
func (c *Client) CallRaw(ctx context.Context, api string, version int, method string, params url.Values) (json.RawMessage, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("api", api)
	params.Set("version", strconv.Itoa(version))
	params.Set("method", method)
	if sid := c.SID(); sid != "" {
		params.Set("_sid", sid)
	}

	endpoint := *c.baseURL
	endpoint.Path = c.pathFor(api)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if tok := c.token(); tok != "" {
		req.Header.Set("X-SYNO-TOKEN", tok)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dsm: %s.%s: %w", api, method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("dsm: read body: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("dsm: %s.%s: http %d: %s", api, method, resp.StatusCode, truncate(string(body), 200))
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		// Some endpoints (rare) don't wrap in the envelope. Hand the raw
		// body back so the user can still inspect it.
		return json.RawMessage(body), nil
	}
	if !env.Success {
		code := 0
		if env.Error != nil {
			code = env.Error.Code
		}
		return nil, &Error{Code: code, API: api, Method: method}
	}
	if len(env.Data) == 0 {
		return json.RawMessage("null"), nil
	}
	return env.Data, nil
}
