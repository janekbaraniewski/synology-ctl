package dsm

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// FirewallConf mirrors SYNO.Core.Network.Firewall.Conf "get" — the
// global firewall state (enabled / disabled, currently bound profile,
// notification preferences).
type FirewallConf struct {
	Enable          flexBool `json:"enable"`
	ProfileName     string   `json:"profile_name,omitempty"`
	ProfileID       int      `json:"profile_id,omitempty"`
	NotifyDeny      flexBool `json:"notify_deny,omitempty"`
	LogDeny         flexBool `json:"log_deny,omitempty"`
	GeoDBVersion    string   `json:"geo_db_version,omitempty"`
	DefaultPolicy   string   `json:"default_policy,omitempty"` // "allow" / "deny"
	AdapterStatuses []struct {
		Adapter string   `json:"adapter"`
		Enabled flexBool `json:"enable"`
	} `json:"adapters,omitempty"`
}

// FirewallStatus returns the global firewall configuration via
// SYNO.Core.Network.Firewall.Conf "get" v1. Returns nil (and nil error)
// when the API is not advertised.
func (c *Client) FirewallStatus(ctx context.Context) (*FirewallConf, error) {
	const api = "SYNO.Core.Network.Firewall.Conf"
	if !c.Supports(api) {
		return nil, nil
	}
	var out FirewallConf
	if err := c.Call(ctx, api, 1, "get", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FirewallProfile is one entry from SYNO.Core.Network.Firewall.Profile.list
// — a named ruleset that can be bound to one or more network adapters.
type FirewallProfile struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"desc,omitempty"`
	RuleCount   int      `json:"rule_count,omitempty"`
	InUse       flexBool `json:"in_use,omitempty"`
	IsDefault   flexBool `json:"is_default,omitempty"`
}

// FirewallProfiles lists firewall profiles via
// SYNO.Core.Network.Firewall.Profile "list" v1. Returns an empty slice
// (and nil error) when the API is not advertised.
func (c *Client) FirewallProfiles(ctx context.Context) ([]FirewallProfile, error) {
	const api = "SYNO.Core.Network.Firewall.Profile"
	if !c.Supports(api) {
		return []FirewallProfile{}, nil
	}
	var resp struct {
		Profiles []FirewallProfile `json:"profiles"`
		Total    int               `json:"total"`
	}
	if err := c.Call(ctx, api, 1, "list", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Profiles, nil
}

// FirewallRule is one entry from SYNO.Core.Network.Firewall.Rules.list —
// a single ordered rule within a profile. ip_type / src_type values
// follow DSM: "all", "ip", "range", "subnet". port_dst is a free-form
// string ("80", "1024-65535", "tcp/22").
type FirewallRule struct {
	RuleID    int      `json:"rule_id"`
	ProfileID int      `json:"profile_id,omitempty"`
	Order     int      `json:"order,omitempty"`
	Enable    flexBool `json:"enable,omitempty"`
	Policy    string   `json:"policy,omitempty"` // "accept" / "drop"
	Protocol  string   `json:"protocol,omitempty"`
	PortDst   string   `json:"port_dst,omitempty"`
	SrcType   string   `json:"src_type,omitempty"`
	SrcIP     string   `json:"src_ip,omitempty"`
	SrcSubnet string   `json:"src_subnet,omitempty"`
	SrcGeo    []string `json:"src_geo,omitempty"`
	Adapter   string   `json:"adapter,omitempty"`
	Comment   string   `json:"comment,omitempty"`
}

// FirewallRules lists the ordered firewall rules for the given profile
// via SYNO.Core.Network.Firewall.Rules "list" v1. Pass an empty string
// for the active profile. Returns an empty slice (and nil error) when
// the API is not advertised.
func (c *Client) FirewallRules(ctx context.Context, profile string) ([]FirewallRule, error) {
	const api = "SYNO.Core.Network.Firewall.Rules"
	if !c.Supports(api) {
		return []FirewallRule{}, nil
	}
	params := url.Values{}
	if profile != "" {
		params.Set("profile", profile)
	}
	var resp struct {
		Rules []FirewallRule `json:"rules"`
		Total int            `json:"total"`
	}
	if err := c.Call(ctx, api, 1, "list", params, &resp); err != nil {
		return nil, err
	}
	return resp.Rules, nil
}

// NewFirewallRule bundles the inputs needed to insert a firewall rule.
// Name is a human label DSM stores as the rule's comment. Action is
// either "allow" or "deny" — we translate to DSM's "accept" / "drop"
// policy keys on the wire. Protocol is "tcp", "udp", "all", or "icmp".
// Source is a free-form string: an IP, CIDR, or one of DSM's well-known
// sentinels ("any", "all"). DestPort is a port number or DSM range
// ("443", "1024-65535", or "all"). Profile names the firewall profile
// the rule is appended to.
type NewFirewallRule struct {
	Name     string
	Action   string // "allow" / "deny"
	Protocol string // "tcp" / "udp" / "all" / "icmp"
	Source   string
	DestPort string
	Profile  string
}

// firewallPolicy maps the user-facing action verb onto DSM's policy
// key. Anything outside the allow/deny dyad is passed through verbatim
// so callers who already know the wire form can use it.
func firewallPolicy(action string) string {
	switch action {
	case "allow":
		return "accept"
	case "deny":
		return "drop"
	}
	return action
}

// CreateFirewallRule inserts a firewall rule into the given profile via
// SYNO.Core.Network.Firewall.Rules v1 `add`. The new rule is appended
// to the profile's ordered list — DSM evaluates rules top-to-bottom,
// so a freshly-added rule lands just above the implicit default rule.
// We don't expose reordering yet (delete + recreate is the v1
// workflow, as documented in the TUI).
func (c *Client) CreateFirewallRule(ctx context.Context, profile string, r NewFirewallRule) error {
	if profile == "" {
		return fmt.Errorf("dsm: firewall profile is required")
	}
	if r.Action == "" {
		return fmt.Errorf("dsm: firewall action is required")
	}
	if r.Protocol == "" {
		return fmt.Errorf("dsm: firewall protocol is required")
	}
	params := url.Values{}
	params.Set("profile", profile)
	params.Set("policy", firewallPolicy(r.Action))
	params.Set("protocol", r.Protocol)
	params.Set("port_dst", r.DestPort)
	// DSM's src_type accepts "all", "ip", "subnet", or "geo:CC". The
	// TUI passes the source string verbatim; an empty / "any" / "all"
	// value maps to the "all sources" sentinel.
	src := r.Source
	switch src {
	case "", "any", "all":
		params.Set("src_type", "all")
	default:
		params.Set("src_type", "ip")
		params.Set("src_ip", src)
	}
	params.Set("comment", r.Name)
	params.Set("enable", "true")
	return c.Call(ctx, "SYNO.Core.Network.Firewall.Rules", 1, "add", params, nil)
}

// DeleteFirewallRule removes a firewall rule from the profile by its
// integer rule id. Endpoint is SYNO.Core.Network.Firewall.Rules v1
// `delete`. DSM accepts a JSON-array id list even for a single rule;
// we wrap the lone id here so callers don't have to think about the
// quoting.
func (c *Client) DeleteFirewallRule(ctx context.Context, profile string, id int) error {
	if profile == "" {
		return fmt.Errorf("dsm: firewall profile is required")
	}
	if id <= 0 {
		return fmt.Errorf("dsm: firewall rule id is required")
	}
	params := url.Values{}
	params.Set("profile", profile)
	params.Set("rule_id", "["+strconv.Itoa(id)+"]")
	return c.Call(ctx, "SYNO.Core.Network.Firewall.Rules", 1, "delete", params, nil)
}

// SetFirewallRuleEnabled toggles whether a firewall rule participates
// in policy evaluation. Endpoint is SYNO.Core.Network.Firewall.Rules
// v1 `set` — DSM tolerates a sparse update (profile + rule_id +
// enable) on this path, which is what lets us treat enable/disable as
// a one-shot instead of round-tripping the whole rule definition.
func (c *Client) SetFirewallRuleEnabled(ctx context.Context, profile string, id int, enabled bool) error {
	if profile == "" {
		return fmt.Errorf("dsm: firewall profile is required")
	}
	if id <= 0 {
		return fmt.Errorf("dsm: firewall rule id is required")
	}
	params := url.Values{}
	params.Set("profile", profile)
	params.Set("rule_id", strconv.Itoa(id))
	params.Set("enable", strconv.FormatBool(enabled))
	return c.Call(ctx, "SYNO.Core.Network.Firewall.Rules", 1, "set", params, nil)
}
