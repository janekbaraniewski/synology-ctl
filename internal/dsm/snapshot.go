package dsm

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// Snapshot is one row from SYNO.Core.Share.Snapshot v2 `list`.
//
// DSM exposes Btrfs snapshots per shared folder. The list call is
// read-only and (on the DSM 7.x firmware we calibrated against)
// does *not* require OTP step-up — only the mutating calls do.
// That matters for the TUI: a fresh OTP modal would be jarring
// just to browse, and we let the user open the share, see the
// existing snapshots, and only get prompted when they actually
// hit `c` (create) or `D` (delete).
type Snapshot struct {
	// Name is DSM's filesystem-level snapshot identifier
	// (typically the ISO-ish "GMT+02-2024.05.18-14.30.21" format).
	// All other operations key off this string.
	Name string `json:"name"`

	// Time is epoch seconds — DSM's `time` field, surfaced as int64
	// because the field arrives as a JSON number on every firmware
	// we've seen.
	Time int64 `json:"time,omitempty"`

	// Description is the human-supplied note from `create`. May be empty.
	Description string `json:"desc,omitempty"`

	// Locked snapshots can't be deleted without unlocking first.
	Locked bool `json:"lock,omitempty"`

	// Schedule snapshots come from the DSM scheduler vs. on-demand.
	Schedule bool `json:"schedule_snapshot,omitempty"`
}

// Snapshots lists the snapshots taken for a share. Endpoint is
// SYNO.Core.Share.Snapshot v2 `list`. share is the bare share name
// (e.g. "photos", not "/volume1/photos").
//
// Listing is OTP-light on the firmware we've tested — only the
// mutating create/delete calls demand inline OTP. We therefore
// route it through the normal Call path rather than CallWithOTP;
// if a future DSM tightens this, the TUI's existing IsOTPStepupRequired
// detection will kick in once we swap this to CallWithOTP.
func (c *Client) Snapshots(ctx context.Context, share string) ([]Snapshot, error) {
	if share == "" {
		return nil, fmt.Errorf("dsm: share name is required")
	}
	params := url.Values{}
	params.Set("name", share)
	params.Set("offset", "0")
	params.Set("limit", "-1")
	params.Set("additional", `["desc","lock","schedule_snapshot"]`)
	var resp struct {
		Snapshots []Snapshot `json:"snapshots"`
		Total     int        `json:"total"`
	}
	if err := c.Call(ctx, "SYNO.Core.Share.Snapshot", 2, "list", params, &resp); err != nil {
		return nil, err
	}
	return resp.Snapshots, nil
}

// CreateSnapshot takes a new snapshot of a share. Endpoint is
// SYNO.Core.Share.Snapshot v2 `create`. The DSM firmware we
// calibrated against demands a fresh OTP code for every snapshot
// mutation — pass the user-entered code through `otp`. When the
// returned error is *OTPRequiredError, the TUI should pop the
// OTP modal and re-issue with the captured code.
//
// desc is the human-readable note attached to the snapshot
// (may be empty). DSM will silently truncate at ~200 chars.
func (c *Client) CreateSnapshot(ctx context.Context, share, desc, otp string) error {
	if share == "" {
		return fmt.Errorf("dsm: share name is required")
	}
	params := url.Values{}
	params.Set("name", share)
	params.Set("desc", desc)
	// "false" by default — locked snapshots need a separate unlock
	// step before they can be deleted, which we don't expose yet.
	params.Set("lock", strconv.FormatBool(false))
	return c.CallWithOTP(ctx, "SYNO.Core.Share.Snapshot", 2, "create", params, otp, nil)
}

// DeleteSnapshot removes a snapshot by name. Endpoint is
// SYNO.Core.Share.Snapshot v2 `delete`. As with create, DSM
// requires a fresh OTP — callers should be prepared to receive
// *OTPRequiredError and re-issue with the user-supplied code.
//
// snapshotName is the bare Name field from a Snapshot returned
// by Snapshots() — DSM's own GMT-prefixed identifier, not a
// user-friendly label.
func (c *Client) DeleteSnapshot(ctx context.Context, share, snapshotName, otp string) error {
	if share == "" {
		return fmt.Errorf("dsm: share name is required")
	}
	if snapshotName == "" {
		return fmt.Errorf("dsm: snapshot name is required")
	}
	params := url.Values{}
	params.Set("name", share)
	// DSM 7 wants the snapshot list as a JSON array even for a
	// single entry; older firmware accepted bare strings, but the
	// array form is forward-compatible.
	params.Set("snapshots", `["`+snapshotName+`"]`)
	return c.CallWithOTP(ctx, "SYNO.Core.Share.Snapshot", 2, "delete", params, otp, nil)
}
