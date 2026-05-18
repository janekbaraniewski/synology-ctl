package dsm

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// DDNSProvider is one entry from SYNO.Core.DDNS.Provider.list — a
// supported Dynamic DNS provider DSM knows how to update (Synology
// itself, No-IP, Dyn, Cloudflare via a module, …).
type DDNSProvider struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name,omitempty"`
	URL         string   `json:"url,omitempty"`
	IsCustom    flexBool `json:"is_custom,omitempty"`
	Builtin     flexBool `json:"builtin,omitempty"`
	Description string   `json:"desc,omitempty"`
}

// DDNSProviders lists Dynamic DNS providers DSM supports via
// SYNO.Core.DDNS.Provider "list" v1. Returns an empty slice (and nil
// error) when the API is not advertised.
func (c *Client) DDNSProviders(ctx context.Context) ([]DDNSProvider, error) {
	const api = "SYNO.Core.DDNS.Provider"
	if !c.Supports(api) {
		return []DDNSProvider{}, nil
	}
	var resp struct {
		Providers []DDNSProvider `json:"providers"`
		Total     int            `json:"total"`
	}
	if err := c.Call(ctx, api, 1, "list", nil, &resp); err != nil {
		return nil, err
	}
	// Older DSM 6.x firmware wraps as "items".
	if len(resp.Providers) == 0 {
		var alt struct {
			Providers []DDNSProvider `json:"items"`
		}
		if err := c.Call(ctx, api, 1, "list", nil, &alt); err == nil && len(alt.Providers) > 0 {
			return alt.Providers, nil
		}
	}
	return resp.Providers, nil
}

// DDNSRecord is one entry from SYNO.Core.DDNS.Record.list — a configured
// Dynamic DNS hostname binding, including its last-known external IP and
// last update status.
type DDNSRecord struct {
	ID              int      `json:"id"`
	Hostname        string   `json:"hostname"`
	Provider        string   `json:"provider,omitempty"`
	Username        string   `json:"username,omitempty"`
	Enable          flexBool `json:"enable,omitempty"`
	ExternalIPv4    string   `json:"external_address,omitempty"`
	ExternalIPv6    string   `json:"external_address_ipv6,omitempty"`
	LastUpdated     int64    `json:"last_update_time,omitempty"` // epoch seconds
	Status          string   `json:"status,omitempty"`
	HeartbeatEnable flexBool `json:"heartbeat_enable,omitempty"`
}

// DDNSRecords lists configured Dynamic DNS hostnames via
// SYNO.Core.DDNS.Record "list" v1. The function also consults
// SYNO.Core.DDNS.ExtIP for the current external address(es) and fills
// the ExternalIPv4 / ExternalIPv6 fields when DSM omits them from the
// per-record payload (common on DSM 7.0 — DSM 7.2 inlines them).
// Returns an empty slice (and nil error) when the Record API is not
// advertised.
func (c *Client) DDNSRecords(ctx context.Context) ([]DDNSRecord, error) {
	const api = "SYNO.Core.DDNS.Record"
	if !c.Supports(api) {
		return []DDNSRecord{}, nil
	}
	var resp struct {
		Records []DDNSRecord `json:"records"`
		Total   int          `json:"total"`
	}
	if err := c.Call(ctx, api, 1, "list", nil, &resp); err != nil {
		return nil, err
	}

	// Optional enrichment: SYNO.Core.DDNS.ExtIP returns the current
	// external IPv4/IPv6 the box sees. We use it only to fill blanks.
	if c.Supports("SYNO.Core.DDNS.ExtIP") {
		var ext struct {
			IPs []struct {
				IPv4 string `json:"ipv4,omitempty"`
				IPv6 string `json:"ipv6,omitempty"`
			} `json:"ips"`
			IPv4 string `json:"ipv4,omitempty"`
			IPv6 string `json:"ipv6,omitempty"`
		}
		if err := c.Call(ctx, "SYNO.Core.DDNS.ExtIP", 1, "list", nil, &ext); err == nil {
			v4 := ext.IPv4
			v6 := ext.IPv6
			if v4 == "" && len(ext.IPs) > 0 {
				v4 = ext.IPs[0].IPv4
			}
			if v6 == "" && len(ext.IPs) > 0 {
				v6 = ext.IPs[0].IPv6
			}
			for i := range resp.Records {
				if resp.Records[i].ExternalIPv4 == "" {
					resp.Records[i].ExternalIPv4 = v4
				}
				if resp.Records[i].ExternalIPv6 == "" {
					resp.Records[i].ExternalIPv6 = v6
				}
			}
		}
	}
	return resp.Records, nil
}

// NewDDNSRecord bundles the inputs needed to register a Dynamic DNS
// hostname with DSM. Provider is the machine key from DDNSProviders()
// (e.g. "Synology"); Hostname is the FQDN to keep updated; Username
// and Password are the provider-side credentials DSM stores so it can
// post updates on behalf of the user. None of these may be empty.
type NewDDNSRecord struct {
	Provider string
	Hostname string
	Username string
	Password string
}

// CreateDDNSRecord registers a Dynamic DNS hostname via
// SYNO.Core.DDNS.Record v1 `set`. DSM overloads `set` for both create
// and update on this API — passing an unknown hostname inserts a new
// record, while passing one that already exists replaces the stored
// credentials. The TUI only exposes the insert direction; updating
// would require an explicit edit form we've deferred.
func (c *Client) CreateDDNSRecord(ctx context.Context, r NewDDNSRecord) error {
	if r.Provider == "" {
		return fmt.Errorf("dsm: ddns provider is required")
	}
	if r.Hostname == "" {
		return fmt.Errorf("dsm: ddns hostname is required")
	}
	if r.Username == "" {
		return fmt.Errorf("dsm: ddns username is required")
	}
	if r.Password == "" {
		return fmt.Errorf("dsm: ddns password is required")
	}
	params := url.Values{}
	params.Set("provider", r.Provider)
	params.Set("hostname", r.Hostname)
	params.Set("username", r.Username)
	params.Set("password", r.Password)
	// DSM defaults the record to enabled on insert; we send the flag
	// explicitly so the behaviour doesn't drift with firmware changes.
	params.Set("enable", "true")
	return c.Call(ctx, "SYNO.Core.DDNS.Record", 1, "set", params, nil)
}

// DeleteDDNSRecord removes a Dynamic DNS record via
// SYNO.Core.DDNS.Record v1 `delete`. DSM keys this call by hostname
// (not the integer ID surfaced in DDNSRecord) — the hostname uniquely
// identifies the binding from the user's point of view, and reusing
// it here means callers don't need to round-trip an ID they may not
// have on hand.
func (c *Client) DeleteDDNSRecord(ctx context.Context, hostname string) error {
	if hostname == "" {
		return fmt.Errorf("dsm: ddns hostname is required")
	}
	params := url.Values{}
	params.Set("hostname", hostname)
	return c.Call(ctx, "SYNO.Core.DDNS.Record", 1, "delete", params, nil)
}

// SetDDNSRecordEnabled flips the enable flag on an existing DDNS record
// without touching credentials. Endpoint is SYNO.Core.DDNS.Record v1
// `set` — DSM tolerates the partial form (hostname + enable) on this
// path, which is how we keep enable/disable out of the full edit
// workflow.
func (c *Client) SetDDNSRecordEnabled(ctx context.Context, hostname string, enabled bool) error {
	if hostname == "" {
		return fmt.Errorf("dsm: ddns hostname is required")
	}
	params := url.Values{}
	params.Set("hostname", hostname)
	params.Set("enable", strconv.FormatBool(enabled))
	return c.Call(ctx, "SYNO.Core.DDNS.Record", 1, "set", params, nil)
}
