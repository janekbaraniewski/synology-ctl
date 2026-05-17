package views

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/janbaraniewski/synology-ctl/internal/dsm"
	"github.com/janbaraniewski/synology-ctl/internal/tui"
)

// storageState is shared by both Volumes and Disks since both views hang
// off a single SYNO.Storage.CGI.Storage.load_info call.
type storageState struct {
	storage *dsm.Storage
	err     error
}

func (s *storageState) fetch(ctx Ctx) tea.Cmd {
	c := ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) (*dsm.Storage, error) { return c.Storage(ctx) },
		func(v *dsm.Storage, err error) tea.Msg { return storageMsg{S: v, Err: err} },
	)
}

func errLine(t tui.Theme, err error) string {
	return lipgloss.NewStyle().Foreground(t.Error).Render("  " + err.Error())
}

// ─────────────────────────── Volumes ───────────────────────────

// Volumes lists logical filesystems with usage and health.
type Volumes struct {
	listBase
	ctx Ctx
	storageState

	// detailVol is non-zero when the user has drilled in. We render a
	// dedicated, charted detail screen for volumes instead of the
	// generic JSON inspector other views use.
	detailVol     *dsm.Volume
	detailRawJSON []byte
	rawMode       bool // J toggles raw JSON inside detail
}

// NewVolumes constructs the view.
func NewVolumes(c Ctx) tui.View {
	v := &Volumes{ctx: c}
	v.initBase(c)
	return v
}

func (v *Volumes) Name() string                   { return "volumes" }
func (v *Volumes) Title() string                  { return "Volumes" }
func (v *Volumes) Icon() string                   { return "▮" }
func (v *Volumes) RefreshInterval() time.Duration { return 15 * time.Second }
func (v *Volumes) Bindings() []key.Binding        { return BaseBindings() }
func (v *Volumes) Init() tea.Cmd                  { return v.fetch(v.ctx) }

func (v *Volumes) visible() []dsm.Volume {
	if v.storage == nil {
		return nil
	}
	if v.FilterValue() == "" {
		return v.storage.Volumes
	}
	out := make([]dsm.Volume, 0, len(v.storage.Volumes))
	for _, vol := range v.storage.Volumes {
		if v.FilterMatch(vol.ID, vol.VolPath, vol.FSType, vol.RaidType, vol.Status, vol.Desc) {
			out = append(out, vol)
		}
	}
	return out
}

func (v *Volumes) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	// Detail screen has its own key handling: esc closes, J toggles raw JSON.
	if v.detailVol != nil {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc", "q":
				v.detailVol = nil
				v.rawMode = false
				return v, nil
			case "J":
				v.rawMode = !v.rawMode
				return v, nil
			}
		}
		switch m := msg.(type) {
		case tui.TickMsg:
			return v, v.fetch(v.ctx)
		case storageMsg:
			v.storage, v.err = m.S, m.Err
			// Refresh the snapshot inside the detail screen too so live
			// values update without leaving the drill-down.
			if v.storage != nil && v.detailVol != nil {
				for _, vol := range v.storage.Volumes {
					if vol.ID == v.detailVol.ID {
						vv := vol
						v.detailVol = &vv
						break
					}
				}
			}
			return v, nil
		}
		return v, nil
	}

	rows := v.visible()
	if cmd, handled := v.HandleKey(msg, len(rows)); handled {
		return v, cmd
	}
	if v.IsEnter(msg) && len(rows) > 0 {
		picked := rows[v.Cursor()]
		v.detailVol = &picked
		if v.storage != nil {
			v.detailRawJSON = locateVolumeRawJSON(v.storage.Raw, picked.ID)
		}
		return v, nil
	}
	switch m := msg.(type) {
	case tui.TickMsg:
		return v, v.fetch(v.ctx)
	case storageMsg:
		v.storage, v.err = m.S, m.Err
		v.ClampCursor(len(v.visible()))
		return v, nil
	case tea.KeyMsg:
		if m.String() == "r" {
			return v, v.fetch(v.ctx)
		}
	}
	return v, nil
}

// locateVolumeRawJSON pulls a single volume's raw JSON out of the
// aggregated load_info payload, so the detail view can render fields we
// haven't modelled explicitly without re-issuing the call.
func locateVolumeRawJSON(blob []byte, id string) []byte {
	type wrap struct {
		Volumes []map[string]any `json:"volumes"`
	}
	var w wrap
	if err := json.Unmarshal(blob, &w); err != nil {
		return nil
	}
	for _, v := range w.Volumes {
		if v["id"] == id {
			b, _ := json.Marshal(v)
			return b
		}
	}
	return nil
}

func (v *Volumes) Render(width, height int) string {
	t := v.ctx.Theme
	// Custom drill-down takes over the canvas when open.
	if v.detailVol != nil {
		if v.rawMode {
			// Reuse the generic JSON inspector when the user wants the
			// raw payload — same data, no curation.
			v.ShowDetail("Volume "+v.detailVol.VolPath+" (raw)", json.RawMessage(v.detailRawJSON))
			s := v.RenderDetail(width, height)
			v.detail.Hide() // we own the visibility; the inspector is a one-shot render
			return s
		}
		pools := []dsm.StoragePool{}
		disks := []dsm.Disk{}
		if v.storage != nil {
			pools = v.storage.StoragePools
			disks = v.storage.Disks
		}
		return renderVolumeDetail(t, width, height, *v.detailVol, pools, disks, v.detailRawJSON)
	}
	if v.storage == nil && v.err == nil {
		return Card(t, width, " ▮  Volumes ", "\n  Loading…\n", true)
	}
	if v.err != nil && v.storage == nil {
		return Card(t, width, " ▮  Volumes ", "\n"+errLine(t, v.err)+"\n", true)
	}
	cols := []Column{
		{Header: "VOLUME", Width: 14},
		{Header: "FS", Width: 8},
		{Header: "RAID", Width: 14},
		{Header: "", Width: 30}, // gauge
		{Header: "USED", Width: 22, Align: lipgloss.Right},
		{Header: "FREE", Width: 12, Align: lipgloss.Right},
		{Header: "STATUS", Width: 0, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0)
	for _, vol := range v.visible() {
		total := ParseSizeString(vol.Size.Total)
		used := ParseSizeString(vol.Size.Used)
		ratio := 0.0
		if total > 0 {
			ratio = float64(used) / float64(total)
		}
		name := vol.VolPath
		if name == "" {
			name = vol.ID
		}
		var free uint64
		if total > used {
			free = total - used
		}
		rows = append(rows, []Cell{
			Plain(name),
			Plain(strings.ToUpper(vol.FSType)),
			Plain(vol.RaidType),
			Plain(Gauge(t, 22, ratio) + fmt.Sprintf(" %4.1f%%", ratio*100)),
			Plain(HumanBytes(used) + " / " + HumanBytes(total)),
			Plain(HumanBytes(free)),
			Styled(vol.Status, t.HealthStyle(vol.Status)),
		})
	}
	footerH := 1
	if f := v.FilterFooter(t); f != "" {
		footerH = 2
	}
	body := "\n" + Table(t, width-4, height-3-footerH, cols, rows, v.Cursor()) + "\n"
	if f := v.FilterFooter(t); f != "" {
		body += f + "\n"
	}
	return Card(t, width, " ▮  Volumes — ⏎ details · / filter ", body, true)
}

// ─────────────────────────── Disks ───────────────────────────

// Disks lists physical drives with health, temperature and bay/slot info.
type Disks struct {
	listBase
	ctx Ctx
	storageState
}

func NewDisks(c Ctx) tui.View {
	d := &Disks{ctx: c}
	d.initBase(c)
	return d
}

func (d *Disks) Name() string                   { return "disks" }
func (d *Disks) Title() string                  { return "Disks" }
func (d *Disks) Icon() string                   { return "●" }
func (d *Disks) RefreshInterval() time.Duration { return 15 * time.Second }
func (d *Disks) Bindings() []key.Binding        { return BaseBindings() }
func (d *Disks) Init() tea.Cmd                  { return d.fetch(d.ctx) }

func (d *Disks) visible() []dsm.Disk {
	if d.storage == nil {
		return nil
	}
	if d.FilterValue() == "" {
		return d.storage.Disks
	}
	out := make([]dsm.Disk, 0)
	for _, dk := range d.storage.Disks {
		if d.FilterMatch(dk.ID, dk.Model, dk.Vendor, dk.Status, dk.DiskType, dk.Container.Str) {
			out = append(out, dk)
		}
	}
	return out
}

func (d *Disks) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	rows := d.visible()
	if cmd, handled := d.HandleKey(msg, len(rows)); handled {
		return d, cmd
	}
	if d.IsEnter(msg) && len(rows) > 0 {
		d.ShowDetail("Disk "+trimDev(rows[d.Cursor()].ID), rows[d.Cursor()])
		return d, nil
	}
	switch m := msg.(type) {
	case tui.TickMsg:
		return d, d.fetch(d.ctx)
	case storageMsg:
		d.storage, d.err = m.S, m.Err
		d.ClampCursor(len(d.visible()))
		return d, nil
	case tea.KeyMsg:
		if m.String() == "r" {
			return d, d.fetch(d.ctx)
		}
	}
	return d, nil
}

func (d *Disks) Render(width, height int) string {
	t := d.ctx.Theme
	if d.DetailVisible() {
		return d.RenderDetail(width, height)
	}
	if d.storage == nil && d.err == nil {
		return Card(t, width, " ●  Disks ", "\n  Loading…\n", true)
	}
	if d.err != nil && d.storage == nil {
		return Card(t, width, " ●  Disks ", "\n"+errLine(t, d.err)+"\n", true)
	}
	cols := []Column{
		{Header: "BAY", Width: 8},
		{Header: "MODEL", Width: 30},
		{Header: "TYPE", Width: 10},
		{Header: "CAPACITY", Width: 12, Align: lipgloss.Right},
		{Header: "TEMP", Width: 8, Align: lipgloss.Right},
		{Header: "SMART", Width: 10, Align: lipgloss.Center},
		{Header: "STATUS", Width: 0, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0)
	for _, dk := range d.visible() {
		bay := trimDev(dk.ID)
		if dk.Container.Str != "" {
			bay = dk.Container.Str
		}
		cap := ParseSizeString(dk.Capacity)
		smart := dk.Smart.Status
		if smart == "" {
			smart = "—"
		}
		rows = append(rows, []Cell{
			Plain(bay),
			Plain(strings.TrimSpace(dk.Vendor + " " + dk.Model)),
			Plain(dk.DiskType),
			Plain(HumanBytes(cap)),
			Plain(fmt.Sprintf("%d°C", dk.Temperature)),
			Styled(smart, t.HealthStyle(smart)),
			Styled(dk.Status, t.HealthStyle(dk.Status)),
		})
	}
	footerH := 1
	if f := d.FilterFooter(t); f != "" {
		footerH = 2
	}
	body := "\n" + Table(t, width-4, height-3-footerH, cols, rows, d.Cursor()) + "\n"
	if f := d.FilterFooter(t); f != "" {
		body += f + "\n"
	}
	return Card(t, width, " ●  Disks — ⏎ details · / filter ", body, true)
}
