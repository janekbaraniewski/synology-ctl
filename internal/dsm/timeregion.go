package dsm

import (
	"context"
	"time"
)

// TimeRegion is DSM's Control Panel → Regional Options flattened into a
// single struct. Fields are filled best-effort across multiple endpoints
// because DSM splits region settings across three different APIs that
// each landed in a different firmware era — and DSM 7's Region.Language
// API isn't always advertised.
type TimeRegion struct {
	TimeZone     string `json:"time_zone,omitempty"`
	TimeZoneDesc string `json:"time_zone_desc,omitempty"`
	NTPEnabled   bool   `json:"ntp_enabled,omitempty"`
	NTPServer    string `json:"ntp_server,omitempty"`
	CurrentTime  string `json:"current_time,omitempty"` // DSM format: "2026-05-17 22:31:18"
	AutoDST      bool   `json:"auto_dst,omitempty"`
	TimeFormat   string `json:"time_format,omitempty"` // "12" / "24"
	DateFormat   string `json:"date_format,omitempty"` // DSM-style: "yyyy-MM-dd"
}

// TimeRegion fetches DSM's Time + Region settings. The call walks
// three endpoints in priority order, each filling whatever fields it
// knows about while leaving the others alone:
//
//  1. SYNO.Core.System.info — always available (we already use it for
//     the System view). Provides time_zone, ntp_server, enabled_ntp,
//     and the current system clock.
//  2. SYNO.Core.Region.NTP "get" — DSM 7's dedicated NTP-config API.
//     Adds the auto-DST flag the System.info call doesn't carry.
//  3. SYNO.Core.Region.Language "get" — provides the user's locale-
//     driven time_format / date_format preferences.
//
// Any 102 ("API does not exist") on the secondary endpoints is
// swallowed — DSM 6.x firmware in particular only ships path #1, and
// returning a partially-filled struct is more useful than failing.
func (c *Client) TimeRegion(ctx context.Context) (*TimeRegion, error) {
	out := &TimeRegion{}

	// — path 1: SYNO.Core.System.info —
	// This is the load-bearing call: anything past it is enrichment.
	// We re-call System.info here (rather than asking the caller to
	// pass one in) so the Time/Region view doesn't have a hard
	// data dependency on the System view's refresh cadence.
	if si, err := c.SystemInfo(ctx); err == nil && si != nil {
		out.TimeZone = si.TimeZone
		out.TimeZoneDesc = si.TimeZoneDesc
		out.NTPEnabled = si.NTPEnabled
		out.NTPServer = si.NTPServer
		out.CurrentTime = si.SystemTime
	}

	// — path 2: SYNO.Core.Region.NTP "get" —
	// Some DSM 7 builds advertise the API only when NTP is enabled, so
	// 102 is treated as "fall through" rather than a hard error.
	type ntpConf struct {
		Enabled flexBool `json:"enabled,omitempty"`
		Server  string   `json:"server,omitempty"`
		Servers []string `json:"servers,omitempty"`
		AutoDST flexBool `json:"auto_dst,omitempty"`
	}
	var ntp ntpConf
	if err := c.Call(ctx, "SYNO.Core.Region.NTP", 1, "get", nil, &ntp); err == nil {
		if !out.NTPEnabled {
			out.NTPEnabled = ntp.Enabled.Bool()
		}
		if out.NTPServer == "" {
			out.NTPServer = ntp.Server
			if out.NTPServer == "" && len(ntp.Servers) > 0 {
				out.NTPServer = ntp.Servers[0]
			}
		}
		out.AutoDST = ntp.AutoDST.Bool()
	}

	// — path 3: SYNO.Core.Region.Language "get" —
	// Provides the locale-driven display formats.
	type langConf struct {
		TimeFormat string `json:"time_format,omitempty"`
		DateFormat string `json:"date_format,omitempty"`
		Locale     string `json:"locale,omitempty"`
		Timezone   string `json:"timezone,omitempty"`
	}
	var lang langConf
	if err := c.Call(ctx, "SYNO.Core.Region.Language", 1, "get", nil, &lang); err == nil {
		if lang.TimeFormat != "" {
			out.TimeFormat = lang.TimeFormat
		}
		if lang.DateFormat != "" {
			out.DateFormat = lang.DateFormat
		}
		// DSM occasionally surfaces a more canonical timezone string
		// here than System.info does (e.g. "Europe/Warsaw" vs
		// "Warsaw"). Prefer the Language API's value when both are
		// present.
		if lang.Timezone != "" {
			out.TimeZone = lang.Timezone
		}
	}

	// Sensible defaults for fields nobody surfaced. 24h is the
	// world-majority format and what DSM ships pre-configured for
	// non-US locales; "yyyy-MM-dd" is ISO and matches what System.info
	// returns in its `time` field.
	if out.TimeFormat == "" {
		out.TimeFormat = "24"
	}
	if out.DateFormat == "" {
		out.DateFormat = "yyyy-MM-dd"
	}
	if out.CurrentTime == "" {
		out.CurrentTime = time.Now().Format("2006-01-02 15:04:05")
	}

	return out, nil
}
