package dsm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Package describes an installed DSM application package. Fields
// outside id/name/version/timestamp arrive nested under `additional`;
// UnmarshalJSON flattens them.
type Package struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	Timestamp    int64  `json:"timestamp,omitempty"`
	Status       string `json:"status,omitempty"` // running / stop / broken
	Maintainer   string `json:"maintainer,omitempty"`
	Description  string `json:"description,omitempty"`
	Beta         bool   `json:"beta,omitempty"`
	CtlUninstall bool   `json:"ctl_uninstall,omitempty"`
}

func (p *Package) UnmarshalJSON(b []byte) error {
	type alias Package
	var raw struct {
		alias
		Additional struct {
			Status       string `json:"status"`
			StatusOrigin string `json:"status_origin"`
			Maintainer   string `json:"maintainer"`
			Description  string `json:"description"`
			Beta         bool   `json:"beta"`
			CtlUninstall bool   `json:"ctl_uninstall"`
		} `json:"additional"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*p = Package(raw.alias)
	if p.Status == "" {
		p.Status = raw.Additional.Status
	}
	if p.Maintainer == "" {
		p.Maintainer = raw.Additional.Maintainer
	}
	if p.Description == "" {
		p.Description = raw.Additional.Description
	}
	if !p.Beta {
		p.Beta = raw.Additional.Beta
	}
	if !p.CtlUninstall {
		p.CtlUninstall = raw.Additional.CtlUninstall
	}
	return nil
}

// Packages returns installed packages with status + metadata.
func (c *Client) Packages(ctx context.Context) ([]Package, error) {
	params := url.Values{}
	params.Set("additional", `["status","ctl_uninstall","beta","maintainer","description"]`)
	var resp struct {
		Packages []Package `json:"packages"`
		Total    int       `json:"total"`
	}
	if err := c.Call(ctx, "SYNO.Core.Package", 2, "list", params, &resp); err == nil {
		return resp.Packages, nil
	}
	// Older firmware: drop the additional set and retry on v1.
	if err := c.Call(ctx, "SYNO.Core.Package", 1, "list", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Packages, nil
}

// PackageUninstall removes an installed package via
// SYNO.Core.Package.Uninstallation v1 `uninstall`. The endpoint
// rejects packages that ship with `ctl_uninstall=false` — callers
// should check the Package.CtlUninstall flag before exposing the
// action to users.
func (c *Client) PackageUninstall(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("dsm: package id is required")
	}
	params := url.Values{}
	params.Set("id", id)
	return c.Call(ctx, "SYNO.Core.Package.Uninstallation", 1, "uninstall", params, nil)
}

// ServerPackage describes a package available for install from
// Synology's package repository. The shape is intentionally minimal:
// DSM returns dozens of marketing fields (icons, screenshots,
// changelog entries) that the TUI doesn't need to render the
// "install which one?" picker.
//
// Field-name drift across firmwares: DSM 7.0.1's catalogue response
// uses "package" for the identifier, not "id"; the human-readable
// name doesn't ship in this payload at all (the web UI fetches it
// from a separate i18n strings bundle). To stay tolerant, we accept
// both shapes for the id and fall back to the id when there's no
// name. Use DisplayName() rather than reading Name directly.
type ServerPackage struct {
	ID          string `json:"id"`
	Package     string `json:"package,omitempty"`
	Name        string `json:"name,omitempty"`
	DSMAppName  string `json:"dsm_app_name,omitempty"`
	Description string `json:"desc,omitempty"`
	Version     string `json:"version,omitempty"`
	Maintainer  string `json:"maintainer,omitempty"`
	Beta        bool   `json:"beta,omitempty"`
	DownloadURL string `json:"link,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

// Identifier returns the catalogue-internal id, preferring `id` and
// falling back to `package` when DSM 7.0.x is on the line.
func (p ServerPackage) Identifier() string {
	if p.ID != "" {
		return p.ID
	}
	return p.Package
}

// DisplayName returns the human-readable label, in priority order:
// the i18n-resolved Name, the additional.dsm_app_name, then the
// identifier as a last resort so the row is never blank.
func (p ServerPackage) DisplayName() string {
	if p.Name != "" {
		return p.Name
	}
	if p.DSMAppName != "" {
		return p.DSMAppName
	}
	return p.Identifier()
}

// PackageServerList returns the catalog of packages available to
// install from DSM's configured package source(s). Endpoint is
// SYNO.Core.Package.Server `list`.
//
// DSM 7.x is fussy: the web UI uses v2 with `blqinst=true` and a
// minimal `additional` field list. Without those, some firmwares
// return an empty catalogue silently (the call succeeds but reports
// 0 packages). We try v2 first — that's what the modern web UI
// uses — and fall back to v1 if the DSM advertises it but rejects
// the v2 shape.
//
// The call is also genuinely slow — DSM walks the upstream Synology
// package index over the public internet. Callers should budget a
// long context (90–180s on a slow NAS / residential line).
//
// Response shape varies by firmware: the catalogue lives under
// `packages` on modern DSM and `list` on DSM 7.0.1. We try both.
func (c *Client) PackageServerList(ctx context.Context) ([]ServerPackage, error) {
	params := url.Values{}
	params.Set("blqinst", "true")
	params.Set("lang", "enu")
	// Minimal additional set — the TUI doesn't render thumbnails,
	// changelogs, dependency graphs, or the dozen marketing fields
	// the web UI fetches. Asking for less keeps the upstream walk
	// quick(er).
	params.Set("additional", `["beta","desc","maintainer","version","size","link"]`)

	var resp struct {
		Packages []ServerPackage `json:"packages"`
		List     []ServerPackage `json:"list"`
		Total    int             `json:"total"`
	}

	// Try v2 first — what the web UI uses on modern DSM. If the
	// device only advertises v1 (older firmware), DSM returns
	// error 104 ("version not supported") and we retry on v1.
	v2err := c.Call(ctx, "SYNO.Core.Package.Server", 2, "list", params, &resp)
	if v2err == nil && (len(resp.Packages) > 0 || len(resp.List) > 0) {
		return firstNonEmpty(resp.Packages, resp.List), nil
	}
	if v2err != nil {
		if e, ok := v2err.(*Error); ok && e.Code != 104 {
			return nil, v2err
		}
	}

	// v1 fallback. Keep the same params — v1 ignores the lang
	// field harmlessly.
	if err := c.Call(ctx, "SYNO.Core.Package.Server", 1, "list", params, &resp); err != nil {
		return nil, err
	}
	return firstNonEmpty(resp.Packages, resp.List), nil
}

// firstNonEmpty returns the first slice that has content, or the empty
// second slice if both are empty.
func firstNonEmpty(a, b []ServerPackage) []ServerPackage {
	if len(a) > 0 {
		return a
	}
	return b
}

// InstallOpts holds tweakables for PackageInstall.
//
// VolumePath is the destination volume (DSM 7 wants an explicit
// `/volume1` rather than "auto"). When empty we fall back to
// `/volume1`, which exists on every supported model.
//
// CheckCodesign + CheckDsm let the caller skip Synology's signature
// and firmware-compat gates — leave both true unless you're
// installing an unsigned community build, in which case set them to
// false and accept the risk.
//
// PollInterval / Timeout control the install-status loop in
// PackageInstall.
type InstallOpts struct {
	VolumePath     string
	CheckCodesign  bool
	CheckDsm       bool
	BetaIfNoStable bool
	PollInterval   time.Duration
	Timeout        time.Duration
}

// PackageInstall orchestrates the multi-step install flow against
// SYNO.Core.Package.Installation. The DSM install pipeline is:
//
//  1. `start` — kicks off the download+verify+install and returns a
//     task id. The call accepts the package id and a handful of
//     install-time toggles (target volume, signature checks, etc).
//  2. `status` — polled in a loop until `finished: true`. The
//     response surfaces stage strings ("downloading",
//     "installing", "post_install") plus a numeric progress.
//  3. `end` — releases the task. Skipping this leaves the slot
//     reserved on some firmware, which makes the next install
//     return a "busy" error.
//
// The flow is encapsulated here so the TUI just calls
// PackageInstall(id, opts) — but the steps are individually
// documented because anyone wiring a chunked-progress callback
// later will need to reuse `start` + `status` directly.
func (c *Client) PackageInstall(ctx context.Context, id string, opts InstallOpts) error {
	if id == "" {
		return fmt.Errorf("dsm: package id is required")
	}
	volume := opts.VolumePath
	if volume == "" {
		volume = "/volume1"
	}
	poll := opts.PollInterval
	if poll <= 0 {
		poll = 2 * time.Second
	}
	deadline := opts.Timeout
	if deadline <= 0 {
		deadline = 10 * time.Minute
	}

	// 1. Start the install task. DSM returns {"taskid":"..."}.
	startParams := url.Values{}
	startParams.Set("id", id)
	startParams.Set("volume_path", volume)
	startParams.Set("check_codesign", strconv.FormatBool(opts.CheckCodesign))
	startParams.Set("check_dsm", strconv.FormatBool(opts.CheckDsm))
	if opts.BetaIfNoStable {
		startParams.Set("blqinst", "true")
	}
	var startResp struct {
		TaskID string `json:"taskid"`
	}
	if err := c.Call(ctx, "SYNO.Core.Package.Installation", 1, "start", startParams, &startResp); err != nil {
		return fmt.Errorf("package install start: %w", err)
	}
	if startResp.TaskID == "" {
		return fmt.Errorf("package install: empty task id from DSM")
	}

	// 2. Poll status until finished or the deadline elapses.
	deadlineCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	for {
		statusParams := url.Values{}
		statusParams.Set("taskid", startResp.TaskID)
		var statusResp struct {
			Finished bool   `json:"finished"`
			Stage    string `json:"stage,omitempty"`
			Status   string `json:"status,omitempty"`
			Error    string `json:"error,omitempty"`
		}
		if err := c.Call(deadlineCtx, "SYNO.Core.Package.Installation", 1, "status", statusParams, &statusResp); err != nil {
			return fmt.Errorf("package install status: %w", err)
		}
		if statusResp.Error != "" {
			return fmt.Errorf("package install failed: %s", statusResp.Error)
		}
		if statusResp.Finished {
			break
		}
		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf("package install: timed out after %s (last stage: %q)", deadline, statusResp.Stage)
		case <-time.After(poll):
		}
	}

	// 3. End the task to release the slot. Best-effort: the install
	//    itself succeeded by the time we get here.
	endParams := url.Values{}
	endParams.Set("taskid", startResp.TaskID)
	_ = c.Call(ctx, "SYNO.Core.Package.Installation", 1, "end", endParams, nil)
	return nil
}

// PackageControl starts, stops, or restarts a package by id.
func (c *Client) PackageControl(ctx context.Context, id, action string) error {
	params := url.Values{}
	params.Set("id", id)
	switch action {
	case "start":
		return c.Call(ctx, "SYNO.Core.Package.Control", 1, "start", params, nil)
	case "stop":
		return c.Call(ctx, "SYNO.Core.Package.Control", 1, "stop", params, nil)
	case "restart":
		if err := c.Call(ctx, "SYNO.Core.Package.Control", 1, "stop", params, nil); err != nil {
			return err
		}
		return c.Call(ctx, "SYNO.Core.Package.Control", 1, "start", params, nil)
	}
	return nil
}
