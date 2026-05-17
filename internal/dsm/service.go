package dsm

import (
	"context"
	"net/url"
)

// Service is a system-level service (SMB, AFP, SSH, …) reported by DSM.
type Service struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Enabled     bool   `json:"enable_status"`
	Running     bool   `json:"status"`
}

// Services returns the system service list.
func (c *Client) Services(ctx context.Context) ([]Service, error) {
	params := url.Values{}
	params.Set("offset", "0")
	params.Set("limit", "-1")
	params.Set("additional", `["enable_status","status","display_name"]`)
	var resp struct {
		Service []Service `json:"service"`
		Total   int       `json:"total"`
	}
	if err := c.Call(ctx, "SYNO.Core.Service", 1, "list", params, &resp); err != nil {
		return nil, err
	}
	return resp.Service, nil
}

// ServiceControl issues a control action against a service.
// action ∈ {"start","stop","restart"}.
func (c *Client) ServiceControl(ctx context.Context, id, action string) error {
	params := url.Values{}
	params.Set("services", `["`+id+`"]`)
	return c.Call(ctx, "SYNO.Core.Service.Control", 1, action, params, nil)
}
