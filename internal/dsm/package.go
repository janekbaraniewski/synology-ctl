package dsm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
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
	ID                   string          `json:"id"`
	Package              string          `json:"package,omitempty"`
	Name                 string          `json:"name,omitempty"`
	DSMAppName           string          `json:"dsm_app_name,omitempty"`
	Description          string          `json:"desc,omitempty"`
	Version              string          `json:"version,omitempty"`
	Maintainer           string          `json:"maintainer,omitempty"`
	Beta                 bool            `json:"beta,omitempty"`
	DownloadURL          string          `json:"link,omitempty"`
	MD5                  string          `json:"md5,omitempty"`
	Source               string          `json:"source,omitempty"`
	Type                 int             `json:"type,omitempty"`
	Start                flexBool        `json:"start,omitempty"`
	QStart               flexBool        `json:"qstart,omitempty"`
	Depsers              json.RawMessage `json:"depsers,omitempty"`
	Deppkgs              json.RawMessage `json:"deppkgs,omitempty"`
	ConflictPkgs         json.RawMessage `json:"conflictpkgs,omitempty"`
	BreakPkgs            json.RawMessage `json:"breakpkgs,omitempty"`
	ReplacePkgs          json.RawMessage `json:"replacepkgs,omitempty"`
	InstallType          string          `json:"install_type,omitempty"`
	InstallOnColdStorage flexBool        `json:"install_on_cold_storage,omitempty"`
	Size                 int64           `json:"size,omitempty"`
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
	// Keep the list narrow, but include the fields Package Center sends
	// back into Installation.check / get_queue / install. Without this
	// metadata some DSM 7 builds reject installs even though the package
	// is visible in the catalog.
	params.Set("additional", `["beta","desc","maintainer","version","size","link","md5","source","type","start","qstart","deppkgs","depsers","conflictpkgs","breakpkgs","replacepkgs","install_type","install_on_cold_storage"]`)

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
// CheckCodesign + CheckDsm are retained as caller intent from the old
// start/status/end flow. Modern DSM's queue-based Package Center flow
// does not expose a safe bypass knob here; PackageInstall follows the
// same validation path as the DSM web UI.
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

// PackageInstall orchestrates the DSM 7 Package Center install flow:
//
//  1. `check` validates environment/dependencies and returns volume data.
//  2. `get_queue` expands the selected package into the actual install
//     queue, including dependencies.
//  3. `install` starts each queued operation.
//  4. SYNO.Core.Package.list is polled until the requested package
//     appears in a non-transient state.
//
// Older examples of this API use `start/status/end`, but DSM 7.0.1
// advertises Package.Installation while returning method 103 for those
// calls. The queue flow below mirrors Package Center's own JavaScript.
func (c *Client) PackageInstall(ctx context.Context, pkg ServerPackage, catalog []ServerPackage, opts InstallOpts) error {
	id := pkg.Identifier()
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

	if checkedVolume, err := c.packageInstallCheck(ctx, pkg, volume); err != nil {
		return err
	} else if checkedVolume != "" {
		volume = checkedVolume
	}

	queue, err := c.packageInstallQueue(ctx, pkg)
	if err != nil {
		return err
	}
	if len(queue) == 0 {
		queue = []packageInstallQueueItem{{Pkg: id, Beta: pkg.Beta}}
	}

	for _, item := range queue {
		if item.Pkg == "" {
			continue
		}
		meta := pkg
		if item.Pkg != id || item.Beta != pkg.Beta {
			if found, ok := findServerPackage(catalog, item.Pkg, item.Beta); ok {
				meta = found
			} else {
				meta = ServerPackage{ID: item.Pkg, Package: item.Pkg, Beta: item.Beta}
			}
		}
		installVolume := volume
		if item.Volume != "" {
			installVolume = item.Volume
		}
		if err := c.packageInstallStart(ctx, meta, installVolume); err != nil {
			return fmt.Errorf("package install %s: %w", item.Pkg, err)
		}
	}

	return c.waitPackageInstalled(ctx, id, deadline, poll)
}

type packageInstallQueueItem struct {
	Pkg    string `json:"pkg"`
	Beta   bool   `json:"beta"`
	Volume string `json:"volume,omitempty"`
}

func (c *Client) packageInstallCheck(ctx context.Context, pkg ServerPackage, fallbackVolume string) (string, error) {
	id := pkg.Identifier()
	params := url.Values{}
	params.Set("id", id)
	params.Set("blupgrade", "false")
	params.Set("blCheckDep", "false")
	if pkg.Version != "" {
		params.Set("ver", pkg.Version)
	}
	if pkg.Size > 0 {
		params.Set("size", strconv.FormatInt(pkg.Size, 10))
	}
	if pkg.InstallType != "" {
		params.Set("install_type", pkg.InstallType)
	}
	if pkg.InstallOnColdStorage.Bool() {
		params.Set("install_on_cold_storage", "true")
	}
	setRawJSON(params, "depsers", pkg.Depsers)
	setRawJSON(params, "deppkgs", pkg.Deppkgs)
	setRawJSON(params, "conflictpkgs", pkg.ConflictPkgs)
	setRawJSON(params, "breakpkgs", pkg.BreakPkgs)
	setRawJSON(params, "replacepkgs", pkg.ReplacePkgs)

	var resp struct {
		IsOccupied flexBool `json:"is_occupied,omitempty"`
		VolumeList []struct {
			MountPoint string `json:"mount_point,omitempty"`
			Path       string `json:"path,omitempty"`
			VolumePath string `json:"volume_path,omitempty"`
		} `json:"volume_list,omitempty"`
	}
	if err := c.Call(ctx, "SYNO.Core.Package.Installation", 2, "check", params, &resp); err != nil {
		// DSM 7.0.x can return generic parameter/version errors here
		// unless the catalog row includes the exact dependency payload
		// Package Center cached from its i18n bundle. Keep this step
		// advisory; get_queue/install still performs the real gate.
		if e, ok := err.(*Error); ok && (e.Code == 103 || e.Code == 104 || e.Code == 114 || e.Code == 120) {
			return fallbackVolume, nil
		}
		return "", fmt.Errorf("package install check: %w", err)
	}
	if resp.IsOccupied.Bool() {
		return "", fmt.Errorf("package install check: Package Center is busy")
	}
	if fallbackVolume != "" {
		return fallbackVolume, nil
	}
	for _, v := range resp.VolumeList {
		if v.MountPoint != "" {
			return v.MountPoint, nil
		}
		if v.VolumePath != "" {
			return v.VolumePath, nil
		}
		if v.Path != "" {
			return v.Path, nil
		}
	}
	return fallbackVolume, nil
}

func (c *Client) packageInstallQueue(ctx context.Context, pkg ServerPackage) ([]packageInstallQueueItem, error) {
	id := pkg.Identifier()
	req := []struct {
		Pkg       string `json:"pkg"`
		Operation string `json:"operation"`
		Version   string `json:"version,omitempty"`
		Beta      bool   `json:"beta"`
	}{{
		Pkg:       id,
		Operation: "install",
		Version:   pkg.Version,
		Beta:      pkg.Beta,
	}}
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("package install queue: encode request: %w", err)
	}
	params := url.Values{}
	params.Set("pkgs", string(b))

	var resp struct {
		Queue            []packageInstallQueueItem `json:"queue"`
		NonExistPkgs     []json.RawMessage         `json:"non_exist_pkgs"`
		ConflictedPkgs   []json.RawMessage         `json:"conflicted_pkgs"`
		BrokenPkgs       []json.RawMessage         `json:"broken_pkgs"`
		PausedPkgs       []json.RawMessage         `json:"paused_pkgs"`
		CausePausingPkgs []json.RawMessage         `json:"cause_pausing_pkgs"`
	}
	if err := c.Call(ctx, "SYNO.Core.Package.Installation", 1, "get_queue", params, &resp); err != nil {
		return nil, fmt.Errorf("package install queue: %w", err)
	}
	if len(resp.NonExistPkgs) > 0 {
		return nil, fmt.Errorf("package install queue: not found: %s", rawList(resp.NonExistPkgs))
	}
	if len(resp.ConflictedPkgs) > 0 {
		return nil, fmt.Errorf("package install queue: conflicts: %s", rawList(resp.ConflictedPkgs))
	}
	if len(resp.BrokenPkgs) > 0 {
		return nil, fmt.Errorf("package install queue: broken packages: %s", rawList(resp.BrokenPkgs))
	}
	if len(resp.PausedPkgs) > 0 || len(resp.CausePausingPkgs) > 0 {
		return nil, fmt.Errorf("package install queue: paused packages: %s%s", rawList(resp.PausedPkgs), rawList(resp.CausePausingPkgs))
	}
	return resp.Queue, nil
}

func (c *Client) packageInstallStart(ctx context.Context, pkg ServerPackage, volume string) error {
	id := pkg.Identifier()
	params := url.Values{}
	params.Set("name", id)
	params.Set("blqinst", "true")
	params.Set("volume_path", volume)
	params.Set("is_syno", strconv.FormatBool(pkg.isSynologySource()))
	params.Set("beta", strconv.FormatBool(pkg.Beta))
	params.Set("installrunpackage", strconv.FormatBool(pkg.Start.Bool() || pkg.QStart.Bool()))

	if !pkg.isSynologySource() && pkg.DownloadURL != "" {
		params.Set("url", pkg.DownloadURL)
		params.Set("operation", "install")
		if pkg.MD5 != "" {
			params.Set("checksum", pkg.MD5)
		}
		if pkg.Size > 0 {
			params.Set("filesize", strconv.FormatInt(pkg.Size, 10))
		}
		if pkg.Type != 0 {
			params.Set("type", strconv.Itoa(pkg.Type))
		}
	}

	var resp struct {
		Progress float64 `json:"progress,omitempty"`
		Error    string  `json:"error,omitempty"`
		Code     int     `json:"code,omitempty"`
	}
	if err := c.Call(ctx, "SYNO.Core.Package.Installation", 1, "install", params, &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("%s", resp.Error)
	}
	if resp.Code != 0 {
		return fmt.Errorf("DSM returned install code %d", resp.Code)
	}
	if resp.Progress < 0 {
		return fmt.Errorf("DSM rejected install request")
	}
	return nil
}

func (c *Client) waitPackageInstalled(ctx context.Context, id string, timeout, poll time.Duration) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var lastErr error
	for {
		pkgs, err := c.Packages(deadlineCtx)
		if err != nil {
			lastErr = err
		} else {
			for _, p := range pkgs {
				if p.ID != id {
					continue
				}
				switch strings.ToLower(p.Status) {
				case "installing", "downloading", "queueing", "upgrading", "repairing", "loading":
				case "broken":
					return fmt.Errorf("package install: %s installed but is broken", id)
				default:
					return nil
				}
			}
		}

		select {
		case <-deadlineCtx.Done():
			if lastErr != nil {
				return fmt.Errorf("package install: timed out after %s waiting for %s (last package list error: %v)", timeout, id, lastErr)
			}
			return fmt.Errorf("package install: timed out after %s waiting for %s", timeout, id)
		case <-time.After(poll):
		}
	}
}

func (p ServerPackage) isSynologySource() bool {
	switch strings.ToLower(p.Source) {
	case "", "syno", "synology":
		return true
	default:
		return false
	}
}

func setRawJSON(params url.Values, key string, raw json.RawMessage) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	params.Set(key, string(raw))
}

func rawList(items []json.RawMessage) string {
	if len(items) == 0 {
		return ""
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s := strings.TrimSpace(string(item))
		if unquoted, err := strconv.Unquote(s); err == nil {
			s = unquoted
		}
		out = append(out, s)
	}
	return strings.Join(out, ", ")
}

func findServerPackage(catalog []ServerPackage, id string, beta bool) (ServerPackage, bool) {
	for _, p := range catalog {
		if p.Identifier() == id && p.Beta == beta {
			return p, true
		}
	}
	for _, p := range catalog {
		if p.Identifier() == id {
			return p, true
		}
	}
	return ServerPackage{}, false
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
