package dsm

import (
	"context"
	"time"
)

// DSMUpdateStatus describes the state of DSM firmware updates: what's
// installed today, what (if anything) is waiting upstream, and whether
// the box is allowed to apply patches on its own.
//
// The shape unifies what DSM exposes across two separate APIs:
// SYNO.Core.Upgrade.Server (which knows about the catalog — name,
// release-notes URL, channel) and SYNO.Core.Upgrade (which knows about
// the locally-cached state — download progress, current version). We
// surface a single struct so the view doesn't have to know which call
// each field came from.
type DSMUpdateStatus struct {
	CurrentVersion    string `json:"current_version,omitempty"`
	AvailableVersion  string `json:"available_version,omitempty"`
	UpdateAvailable   bool   `json:"update_available,omitempty"`
	LastCheck         int64  `json:"last_check,omitempty"` // epoch seconds
	AutoUpdateEnabled bool   `json:"auto_update_enabled,omitempty"`
	ReleaseNotesURL   string `json:"release_notes_url,omitempty"`
	UpdateChannel     string `json:"update_channel,omitempty"` // "stable" / "beta"
}

// DSMUpdate returns the upgrade status from DSM's Control Panel →
// Update & Restore surface.
//
// Wire order:
//  1. SYNO.Core.Upgrade.Server v1 `check` — the rich path. Returns the
//     catalog entry for the next version when one exists, including the
//     release-notes URL and the channel name. On DSM 7.0/7.1 firmware
//     this endpoint sometimes 102s (api-not-supported) instead of just
//     returning "no update". We treat 102 as "fall through, don't fail".
//  2. SYNO.Core.Upgrade v1 `download_status` — the minimal path. Older
//     boxes expose only this; it tells us the current version and
//     whether a download is queued, with none of the catalog metadata.
//
// Either call is best-effort. We return whatever we can stitch together,
// with the empty-zero defaults filling in for fields neither call
// surfaced. Callers (the TUI) should treat zero values as "unknown"
// rather than "definitely off".
func (c *Client) DSMUpdate(ctx context.Context) (*DSMUpdateStatus, error) {
	out := &DSMUpdateStatus{}

	// — primary: SYNO.Core.Upgrade.Server.check —
	type serverCheck struct {
		Current struct {
			Version string `json:"version"`
		} `json:"current"`
		Update struct {
			Available    flexBool `json:"available"`
			Version      string   `json:"version"`
			ReleaseNotes string   `json:"release_notes,omitempty"`
			ReleaseURL   string   `json:"release_link,omitempty"`
		} `json:"update"`
		ReleaseNotesURL string   `json:"release_notes_url,omitempty"`
		LastCheck       int64    `json:"last_check,omitempty"`
		AutoUpdate      flexBool `json:"auto_update,omitempty"`
		Channel         string   `json:"channel,omitempty"`
	}
	var sc serverCheck
	serverErr := c.Call(ctx, "SYNO.Core.Upgrade.Server", 1, "check", nil, &sc)
	if serverErr == nil {
		out.CurrentVersion = sc.Current.Version
		out.AvailableVersion = sc.Update.Version
		out.UpdateAvailable = sc.Update.Available.Bool()
		out.LastCheck = sc.LastCheck
		out.AutoUpdateEnabled = sc.AutoUpdate.Bool()
		out.UpdateChannel = sc.Channel
		// DSM is inconsistent about which key holds the release-notes
		// link; prefer the structured one, fall back to whichever is
		// non-empty.
		switch {
		case sc.Update.ReleaseURL != "":
			out.ReleaseNotesURL = sc.Update.ReleaseURL
		case sc.ReleaseNotesURL != "":
			out.ReleaseNotesURL = sc.ReleaseNotesURL
		}
	} else if !is102(serverErr) {
		// A real error (network blip, auth issue) on the primary path —
		// surface it so the view can flag the user.
		return nil, serverErr
	}

	// — fallback: SYNO.Core.Upgrade.download_status —
	// This call is cheap; we always run it to fill blanks left by the
	// primary path (and to provide *any* result on firmware where the
	// primary 102s out).
	type dlStatus struct {
		Version string `json:"version,omitempty"`
		Status  string `json:"status,omitempty"`
		Update  struct {
			Available flexBool `json:"available,omitempty"`
			Version   string   `json:"version,omitempty"`
		} `json:"update,omitempty"`
	}
	var dl dlStatus
	if err := c.Call(ctx, "SYNO.Core.Upgrade", 1, "download_status", nil, &dl); err == nil {
		if out.CurrentVersion == "" {
			out.CurrentVersion = dl.Version
		}
		if out.AvailableVersion == "" {
			out.AvailableVersion = dl.Update.Version
		}
		if !out.UpdateAvailable {
			out.UpdateAvailable = dl.Update.Available.Bool()
		}
	}

	// Last resort for the current version — SystemInfo always has it,
	// it's the only field we can guarantee. We don't fire SystemInfo
	// proactively (the view calls it elsewhere) but we don't want a
	// completely empty card either.
	if out.CurrentVersion == "" {
		if si, err := c.SystemInfo(ctx); err == nil && si != nil {
			out.CurrentVersion = coalesceStr(si.DSMVersion, si.Version)
		}
	}

	// Default LastCheck to "just now" if the API didn't tell us — DSM 7
	// runs the check on demand when this endpoint is hit, so the
	// timestamp on a successful primary call is effectively "now". Keep
	// it 0 when both calls failed so the view shows "—" instead of
	// lying.
	if out.LastCheck == 0 && serverErr == nil {
		out.LastCheck = time.Now().Unix()
	}

	return out, nil
}

// is102 reports whether err is a typed DSM error with code 102 ("the
// requested API does not exist"). The Upgrade.Server endpoint has been
// known to return 102 on firmware that doesn't carry the upstream
// catalog client; we treat that as "fall through, try the next path"
// rather than a hard failure.
func is102(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*Error); ok {
		return e.Code == 102
	}
	return false
}

// coalesceStr returns the first non-empty argument. Lives here so dsm
// doesn't have to depend on views for a one-line helper.
func coalesceStr(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}
