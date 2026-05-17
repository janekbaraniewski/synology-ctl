package dsm

import (
	"context"
	"net/url"
)

// NetworkInterface is a single NIC reported by DSM.
type NetworkInterface struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	IFName      string   `json:"ifname"`        // eth0, bond0, ovs_eth0
	Type        string   `json:"type"`          // lan, bond, …
	UseDHCP     bool     `json:"use_dhcp,omitempty"`
	MAC         string   `json:"mac,omitempty"`
	MTU         int      `json:"mtu,omitempty"`
	IPv4Address string   `json:"ip,omitempty"`
	IPv4Mask    string   `json:"mask,omitempty"`
	IPv4Gateway string   `json:"gateway,omitempty"`
	IPv6        []struct {
		Address string `json:"address"`
		Prefix  int    `json:"prefix_length"`
		Scope   string `json:"scope"`
	} `json:"ipv6,omitempty"`
	Status      string   `json:"status"` // "connected", "disconnected"
	LinkSpeed   string   `json:"link_speed,omitempty"`
	Duplex      string   `json:"duplex,omitempty"`
	IsDefault   bool     `json:"default,omitempty"`
}

// NetworkInterfaces lists network interfaces.
func (c *Client) NetworkInterfaces(ctx context.Context) ([]NetworkInterface, error) {
	params := url.Values{}
	params.Set("additional", `["type","ip","mask","gateway","mac","status","link_speed","duplex","mtu","ipv6","use_dhcp"]`)
	var resp struct {
		Interfaces []NetworkInterface `json:"interfaces"`
		Total      int                `json:"total"`
	}
	if err := c.Call(ctx, "SYNO.Core.Network.Interface", 1, "list", params, &resp); err != nil {
		return nil, err
	}
	return resp.Interfaces, nil
}

// DNSConfig captures DNS resolver settings.
type DNSConfig struct {
	NameServers []string `json:"nameservers,omitempty"`
	Search      []string `json:"search,omitempty"`
}

// Hostname returns the configured hostname.
func (c *Client) Hostname(ctx context.Context) (string, error) {
	var resp struct {
		Hostname string `json:"hostname"`
	}
	if err := c.Call(ctx, "SYNO.Core.Network", 1, "get", nil, &resp); err != nil {
		return "", err
	}
	return resp.Hostname, nil
}
