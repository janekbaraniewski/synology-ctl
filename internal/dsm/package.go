package dsm

import (
	"context"
	"net/url"
)

// Package describes an installed DSM application package.
type Package struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Version            string `json:"version"`
	Status             string `json:"status"`             // "running", "stop", "broken", …
	StatusCode         int    `json:"status_code"`
	StartStopRestartable bool `json:"start_stop_restartable,omitempty"`
	CtlUninstall       bool   `json:"ctl_uninstall,omitempty"`
	Maintainer         string `json:"maintainer,omitempty"`
	Description        string `json:"description,omitempty"`
	Beta               bool   `json:"beta,omitempty"`
	Distributor        string `json:"distributor,omitempty"`
	InstallType        string `json:"install_type,omitempty"`
	Timestamp          int64  `json:"timestamp,omitempty"`
}

// Packages returns installed packages with optional extra fields.
func (c *Client) Packages(ctx context.Context) ([]Package, error) {
	params := url.Values{}
	params.Set("additional", `["description","maintainer","distributor","status","beta","install_type","startable","stoppable","installing","ctl_uninstall","start_stop_restartable","timestamp"]`)
	var resp struct {
		Packages []Package `json:"packages"`
		Total    int       `json:"total"`
	}
	if err := c.Call(ctx, "SYNO.Core.Package", 2, "list", params, &resp); err != nil {
		return nil, err
	}
	return resp.Packages, nil
}

// PackageControl starts, stops, or restarts a package by id.
// action ∈ {"start","stop","restart"}.
func (c *Client) PackageControl(ctx context.Context, id, action string) error {
	params := url.Values{}
	params.Set("id", id)
	api := "SYNO.Core.Package." + capitalize(action) // not directly used; method below routes correctly
	_ = api
	switch action {
	case "start":
		return c.Call(ctx, "SYNO.Core.Package.Control", 1, "start", params, nil)
	case "stop":
		return c.Call(ctx, "SYNO.Core.Package.Control", 1, "stop", params, nil)
	case "restart":
		// DSM does not expose a single restart; chain stop+start.
		if err := c.Call(ctx, "SYNO.Core.Package.Control", 1, "stop", params, nil); err != nil {
			return err
		}
		return c.Call(ctx, "SYNO.Core.Package.Control", 1, "start", params, nil)
	}
	return nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 32
	}
	return string(b)
}
