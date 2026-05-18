package dsm

import (
	"context"
)

// QuickConnectStatus is SYNO.Core.QuickConnect "get" — DSM's "punch a
// reverse tunnel through Synology's relay so the box is reachable
// without router-side port-forwarding" feature. We surface the
// minimum the TUI needs to give a "configured / not configured" verdict.
type QuickConnectStatus struct {
	Enabled        bool   `json:"enabled"`
	QuickConnectID string `json:"quickconnect_id,omitempty"`
	IsRouterCompat bool   `json:"is_router_compat,omitempty"`
	RelayEnabled   bool   `json:"relay_enabled,omitempty"`
}

// QuickConnectStatus returns the current QuickConnect configuration via
// SYNO.Core.QuickConnect "get" v1.
//
// Returns nil (and nil error) when the API is not advertised. DSM
// units that have explicitly opted out of QuickConnect can still
// advertise the API and return enabled=false — we keep that
// distinction so the view shows "off" rather than "unsupported".
func (c *Client) QuickConnectStatus(ctx context.Context) (*QuickConnectStatus, error) {
	const api = "SYNO.Core.QuickConnect"
	if !c.Supports(api) {
		return nil, nil
	}
	// DSM 7 wraps the answer in `data.quickconnect`; older builds
	// inline the fields. Accept both shapes.
	var raw struct {
		Enabled        flexBool `json:"enabled"`
		QuickConnectID string   `json:"quickconnect_id,omitempty"`
		ID             string   `json:"id,omitempty"`
		IsRouterCompat flexBool `json:"is_router_compat,omitempty"`
		Relay          flexBool `json:"relay,omitempty"`
		RelayEnabled   flexBool `json:"relay_enabled,omitempty"`
		QuickConnect   struct {
			Enabled        flexBool `json:"enabled"`
			QuickConnectID string   `json:"quickconnect_id,omitempty"`
			ID             string   `json:"id,omitempty"`
			IsRouterCompat flexBool `json:"is_router_compat,omitempty"`
			RelayEnabled   flexBool `json:"relay_enabled,omitempty"`
		} `json:"quickconnect,omitempty"`
	}
	if err := c.Call(ctx, api, 1, "get", nil, &raw); err != nil {
		return nil, err
	}
	out := &QuickConnectStatus{
		Enabled:        raw.Enabled.Bool() || raw.QuickConnect.Enabled.Bool(),
		QuickConnectID: coalesceStr(raw.QuickConnectID, raw.ID, raw.QuickConnect.QuickConnectID, raw.QuickConnect.ID),
		IsRouterCompat: raw.IsRouterCompat.Bool() || raw.QuickConnect.IsRouterCompat.Bool(),
		RelayEnabled:   raw.RelayEnabled.Bool() || raw.Relay.Bool() || raw.QuickConnect.RelayEnabled.Bool(),
	}
	return out, nil
}

// PortForwardingMapping is one row of a UPnP-mediated port-forwarding
// table: a single internal-port → external-port translation tied to a
// transport protocol and (when DSM knows) the service it was set up
// for. Service is free-form ("HTTPS", "Plex", "DSM"), not a wire
// identifier — DSM uses it only as a display string.
type PortForwardingMapping struct {
	Protocol     string `json:"protocol"`      // "tcp" / "udp"
	InternalPort int    `json:"internal_port"` // port on the NAS
	ExternalPort int    `json:"external_port"` // port on the router
	Service      string `json:"service,omitempty"`
}

// RouterPortForwarding is SYNO.Core.PortForwarding "list" — DSM's view
// of the router's UPnP port mappings, including the ones DSM itself
// has asked the router to install. The Enabled flag reports whether
// DSM's UPnP client is even attempting to manage the router; on
// networks where the router doesn't speak UPnP this comes back false
// with an empty mappings list.
type RouterPortForwarding struct {
	Enabled  bool                    `json:"enabled"`
	Mappings []PortForwardingMapping `json:"mappings"`
}

// PortForwarding returns DSM's router-port-forwarding view via
// SYNO.Core.PortForwarding "list" v1.
//
// Returns a non-nil struct with Enabled=false and an empty mappings
// slice when the router doesn't expose UPnP — the empty case is
// common and not an error, so we don't escalate it. Returns nil (and
// nil error) when the API is not advertised at all.
func (c *Client) PortForwarding(ctx context.Context) (*RouterPortForwarding, error) {
	const api = "SYNO.Core.PortForwarding"
	if !c.Supports(api) {
		return nil, nil
	}
	var raw struct {
		Enabled  flexBool `json:"enabled"`
		Mappings []struct {
			Protocol     string `json:"protocol,omitempty"`
			InternalPort int    `json:"internal_port,omitempty"`
			ExternalPort int    `json:"external_port,omitempty"`
			Service      string `json:"service,omitempty"`
			// DSM occasionally uses alt key names for the same fields;
			// accept them so we don't bail on a quirky firmware.
			PortInternal int    `json:"port_internal,omitempty"`
			PortExternal int    `json:"port_external,omitempty"`
			Desc         string `json:"desc,omitempty"`
		} `json:"mappings"`
	}
	if err := c.Call(ctx, api, 1, "list", nil, &raw); err != nil {
		// A failure here is typically "UPnP unavailable upstream",
		// which is normal — we degrade to an empty, disabled struct
		// rather than failing the whole view.
		return &RouterPortForwarding{Enabled: false, Mappings: []PortForwardingMapping{}}, nil
	}
	out := &RouterPortForwarding{
		Enabled:  raw.Enabled.Bool(),
		Mappings: make([]PortForwardingMapping, 0, len(raw.Mappings)),
	}
	for _, m := range raw.Mappings {
		mapping := PortForwardingMapping{
			Protocol:     m.Protocol,
			InternalPort: m.InternalPort,
			ExternalPort: m.ExternalPort,
			Service:      coalesceStr(m.Service, m.Desc),
		}
		if mapping.InternalPort == 0 {
			mapping.InternalPort = m.PortInternal
		}
		if mapping.ExternalPort == 0 {
			mapping.ExternalPort = m.PortExternal
		}
		out.Mappings = append(out.Mappings, mapping)
	}
	return out, nil
}
