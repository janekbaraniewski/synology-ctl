package dsm

import (
	"context"
	"net/url"
	"time"
)

// Info populates the API path table used to route subsequent calls. It is
// safe to call multiple times; results are cached for an hour.
func (c *Client) Info(ctx context.Context) error {
	c.mu.RLock()
	fresh := time.Since(c.infoDoneAt) < time.Hour && len(c.apiPaths) > 0
	c.mu.RUnlock()
	if fresh {
		return nil
	}
	params := url.Values{}
	params.Set("query", "all")
	var out map[string]apiInfo
	if err := c.Call(ctx, "SYNO.API.Info", 1, "query", params, &out); err != nil {
		return err
	}
	c.mu.Lock()
	c.apiPaths = out
	c.infoDoneAt = time.Now()
	c.mu.Unlock()
	return nil
}

// SupportedAPIs returns the names of APIs the server advertises, sorted.
func (c *Client) SupportedAPIs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, 0, len(c.apiPaths))
	for k := range c.apiPaths {
		out = append(out, k)
	}
	return out
}

// Supports reports whether the device advertises the given API.
func (c *Client) Supports(api string) bool {
	c.mu.RLock()
	_, ok := c.apiPaths[api]
	c.mu.RUnlock()
	return ok
}

// APIDescriptor is the public shape of an entry in the SYNO.API.Info table.
type APIDescriptor struct {
	Path       string
	MinVersion int
	MaxVersion int
}

// APIInfo returns the descriptor for an API; ok is false if the device
// doesn't advertise it (or Info() hasn't been called yet).
func (c *Client) APIInfo(api string) (APIDescriptor, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	a, ok := c.apiPaths[api]
	if !ok {
		return APIDescriptor{}, false
	}
	return APIDescriptor{Path: a.Path, MinVersion: a.MinVersion, MaxVersion: a.MaxVersion}, true
}
