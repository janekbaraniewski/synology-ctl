package dsm

import (
	"context"
)

// PowerSchedule is one entry from SYNO.Core.Hardware.PowerSchedule.list
// — a single recurring rule for the box to power itself on, off, or
// restart at a fixed wall-clock time on specific days of the week.
//
// Day is the lowercase three-letter weekday name DSM uses on the wire
// ("sun", "mon", … "sat"). Hour is 0-23, Minute is 0-59. Action is one
// of "power-on" / "power-off" / "restart" — DSM uses different keys
// internally; the converter helpers below map both directions.
type PowerSchedule struct {
	Day     string `json:"day"`
	Hour    int    `json:"hour"`
	Minute  int    `json:"minute"`
	Action  string `json:"action"`
	Enabled bool   `json:"enabled"`
}

// PowerSchedule fetches DSM's recurring power-schedule entries via
// SYNO.Core.Hardware.PowerSchedule "list" v1.
//
// DSM has shipped two slightly different wire shapes for this API:
//   - DSM 6.x and early DSM 7 returned `tasks` with `power_on_*` and
//     `power_off_*` fields per row.
//   - DSM 7.2+ returns `schedules` with explicit day/hour/minute/action
//     fields, which is much easier to consume.
//
// We try the modern shape first and fall back to massaging the legacy
// shape when the structured fields come back empty. Returns an empty
// slice (and nil error) when the API is not advertised at all.
func (c *Client) PowerSchedule(ctx context.Context) ([]PowerSchedule, error) {
	const api = "SYNO.Core.Hardware.PowerSchedule"
	if !c.Supports(api) {
		return []PowerSchedule{}, nil
	}

	// Modern shape (DSM 7.2+).
	var modern struct {
		Schedules []PowerSchedule `json:"schedules"`
		Total     int             `json:"total"`
	}
	if err := c.Call(ctx, api, 1, "list", nil, &modern); err == nil && len(modern.Schedules) > 0 {
		return modern.Schedules, nil
	}

	// Legacy shape (DSM 6.x / DSM 7.0). Each row carries both
	// power-on and power-off times in the same record; we explode
	// into two PowerSchedule entries when both halves are filled.
	type legacyRow struct {
		ID           int      `json:"id"`
		Enabled      flexBool `json:"enabled"`
		Weekdays     string   `json:"weekdays,omitempty"` // comma-joined: "sun,mon,…"
		PowerOnHour  int      `json:"power_on_hour"`
		PowerOnMin   int      `json:"power_on_minute"`
		PowerOffHour int      `json:"power_off_hour"`
		PowerOffMin  int      `json:"power_off_minute"`
		Action       string   `json:"action,omitempty"`
	}
	var legacy struct {
		Tasks []legacyRow `json:"tasks"`
		Total int         `json:"total"`
	}
	if err := c.Call(ctx, api, 1, "list", nil, &legacy); err != nil {
		// Both shapes failed — propagate the modern-shape error path,
		// but never invent a fake one. Empty slice is the right answer
		// for "API gave us nothing".
		return []PowerSchedule{}, nil
	}

	out := make([]PowerSchedule, 0, 2*len(legacy.Tasks))
	for _, r := range legacy.Tasks {
		days := splitWeekdays(r.Weekdays)
		if len(days) == 0 {
			days = []string{"daily"}
		}
		for _, day := range days {
			if r.PowerOnHour != 0 || r.PowerOnMin != 0 {
				out = append(out, PowerSchedule{
					Day:     day,
					Hour:    r.PowerOnHour,
					Minute:  r.PowerOnMin,
					Action:  "power-on",
					Enabled: r.Enabled.Bool(),
				})
			}
			if r.PowerOffHour != 0 || r.PowerOffMin != 0 {
				out = append(out, PowerSchedule{
					Day:     day,
					Hour:    r.PowerOffHour,
					Minute:  r.PowerOffMin,
					Action:  "power-off",
					Enabled: r.Enabled.Bool(),
				})
			}
		}
	}
	return out, nil
}

// splitWeekdays parses DSM's comma-joined weekday strings ("sun,mon")
// into a slice. Returns nil for the empty input so callers can detect
// "no day info" and fall back to a sensible default.
func splitWeekdays(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}

// WakeOnLANConf is SYNO.Core.Hardware.WOL "get" — whether Wake-on-LAN
// is armed, and on which adapter's MAC. The MAC is included so the TUI
// can show the user the exact target without forcing them to cross-
// reference the network view.
type WakeOnLANConf struct {
	Enabled    bool   `json:"enabled"`
	MACAddress string `json:"mac_address,omitempty"`
}

// WakeOnLANConf returns the current Wake-on-LAN configuration via
// SYNO.Core.Hardware.WOL "get" v1.
//
// The API has been stable across DSM 6 + 7. We still tolerate two
// payload shapes: DSM 7 returns `{ "enable": true, "mac": "…" }` at
// the top of `data`, while a couple of 7.0 builds wrapped the
// adapter info into a `wol` sub-object. We try both.
//
// Returns nil (and nil error) when the API is not advertised.
func (c *Client) WakeOnLANConf(ctx context.Context) (*WakeOnLANConf, error) {
	const api = "SYNO.Core.Hardware.WOL"
	if !c.Supports(api) {
		return nil, nil
	}

	// DSM 7-style flat shape, plus the per-adapter wrapped form.
	var raw struct {
		Enable flexBool `json:"enable"`
		MAC    string   `json:"mac,omitempty"`
		WOL    struct {
			Enable flexBool `json:"enable"`
			MAC    string   `json:"mac,omitempty"`
		} `json:"wol,omitempty"`
		Adapters []struct {
			Adapter string   `json:"adapter"`
			Enable  flexBool `json:"enable"`
			MAC     string   `json:"mac,omitempty"`
		} `json:"adapters,omitempty"`
	}
	if err := c.Call(ctx, api, 1, "get", nil, &raw); err != nil {
		return nil, err
	}
	out := &WakeOnLANConf{
		Enabled:    raw.Enable.Bool() || raw.WOL.Enable.Bool(),
		MACAddress: raw.MAC,
	}
	if out.MACAddress == "" {
		out.MACAddress = raw.WOL.MAC
	}
	if out.MACAddress == "" && len(raw.Adapters) > 0 {
		// Prefer the first adapter that has WOL armed; otherwise just
		// take the first row so the user sees *something* identifying
		// the binding.
		for _, a := range raw.Adapters {
			if a.Enable.Bool() {
				out.Enabled = true
				out.MACAddress = a.MAC
				break
			}
		}
		if out.MACAddress == "" {
			out.MACAddress = raw.Adapters[0].MAC
		}
	}
	return out, nil
}
