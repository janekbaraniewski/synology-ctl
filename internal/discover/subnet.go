package discover

import (
	"context"
	"net"
	"sync"
	"time"
)

// ScanSubnets sweeps each selected interface's local subnet for DSM
// responders. This is the generic VPN-discovery path: WireGuard,
// OpenVPN, IPSec, IPVS — anything that routes a subnet small enough to
// sweep — surfaces via this path.
//
// Tailscale is the exception: each peer lives at a /32 inside
// 100.64.0.0/10, so subnet-scanning would mean probing four million
// addresses. Tailscale uses ScanTailnet() which enumerates peers via
// the CLI instead.
//
// We cap the prefix at /22 (1024 hosts) so we never blanket-scan a
// large network. mDNS already covers /24 LANs; this is about VPN
// subnets that don't carry multicast.
func ScanSubnets(ctx context.Context, timeout time.Duration, ifaces []*net.Interface) ([]Device, error) {
	const maxHosts = 1024
	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	targets := subnetTargets(ifaces, maxHosts)
	if len(targets) == 0 {
		return nil, nil
	}

	type result struct {
		d  Device
		ok bool
	}
	out := make(chan result, len(targets))
	sem := make(chan struct{}, 32)
	var wg sync.WaitGroup
	for _, ip := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(addr string) {
			defer wg.Done()
			defer func() { <-sem }()
			if d, ok := probeAddr(scanCtx, addr); ok {
				out <- result{d, true}
			}
		}(ip)
	}
	go func() { wg.Wait(); close(out) }()

	var devices []Device
	for r := range out {
		devices = append(devices, r.d)
	}
	return devices, nil
}

// subnetTargets enumerates the host addresses to probe across the
// given interfaces. We deliberately skip:
//   - link-local addresses
//   - 100.64.0.0/10 (CGNAT — Tailscale lives here; CLI enumeration is
//     handled elsewhere)
//   - prefixes shorter than /22 (too many hosts)
//   - the interface's own address
//   - network + broadcast addresses
func subnetTargets(ifaces []*net.Interface, maxHosts int) []string {
	if len(ifaces) == 0 {
		// Default to every up, non-loopback interface.
		all, _ := net.Interfaces()
		for i := range all {
			ni := &all[i]
			if ni.Flags&net.FlagUp == 0 || ni.Flags&net.FlagLoopback != 0 {
				continue
			}
			ifaces = append(ifaces, ni)
		}
	}

	seen := map[string]bool{}
	var targets []string
	for _, ni := range ifaces {
		addrs, err := ni.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}
			ones, bits := ipnet.Mask.Size()
			if ones == 0 || bits == 0 {
				continue
			}
			// Skip CGNAT (Tailscale) — handled by ScanTailnet.
			if inCGNAT(ipnet.IP) {
				continue
			}
			if ones < 22 { // >1024 hosts
				continue
			}
			hosts := hostsIn(ipnet, maxHosts)
			for _, h := range hosts {
				if h.Equal(ipnet.IP) {
					continue
				}
				s := h.String()
				if seen[s] {
					continue
				}
				seen[s] = true
				targets = append(targets, s)
			}
		}
	}
	return targets
}

func inCGNAT(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127
	}
	return false
}

// hostsIn yields every usable host inside the IPv4 network, capped to
// `max`. Network and broadcast addresses are excluded.
func hostsIn(ipnet *net.IPNet, max int) []net.IP {
	ip4 := ipnet.IP.To4()
	if ip4 == nil {
		return nil
	}
	mask := ipnet.Mask
	if len(mask) != 4 {
		return nil
	}
	start := make(net.IP, 4)
	end := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		start[i] = ip4[i] & mask[i]
		end[i] = start[i] | ^mask[i]
	}
	var out []net.IP
	// Skip network address (start) and broadcast (end).
	cur := make(net.IP, 4)
	copy(cur, start)
	incIP(cur)
	for {
		if compareIP(cur, end) >= 0 {
			break
		}
		host := make(net.IP, 4)
		copy(host, cur)
		out = append(out, host)
		if len(out) >= max {
			break
		}
		incIP(cur)
	}
	return out
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] != 0 {
			break
		}
	}
}

func compareIP(a, b net.IP) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// probeAddr runs the DSM probe against a bare IP, trying https first.
func probeAddr(ctx context.Context, ip string) (Device, bool) {
	candidates := []struct {
		scheme string
		port   int
	}{{"https", 5001}, {"http", 5000}}
	for _, c := range candidates {
		if !isDSMAt(ctx, c.scheme, ip, c.port) {
			continue
		}
		return Device{
			Hostname: ip, // best we have without rDNS
			Vendor:   "Synology",
			Port:     c.port,
			Secure:   c.scheme == "https",
			IPv4:     []net.IP{net.ParseIP(ip)},
		}, true
	}
	return Device{}, false
}
