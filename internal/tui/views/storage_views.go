package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/janbaraniewski/synology-ctl/internal/dsm"
	"github.com/janbaraniewski/synology-ctl/internal/tui"
)

// storageBase shares state + fetch logic across the volume/disk/pool views,
// since they all hang off a single SYNO.Core.Storage call.
type storageBase struct {
	ctx      Ctx
	storage  *dsm.Storage
	err      error
	cursor   int
	width    int
	height   int
}

func (b *storageBase) fetch() tea.Cmd {
	c := b.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) (*dsm.Storage, error) { return c.Storage(ctx) },
		func(s *dsm.Storage, err error) tea.Msg { return storageMsg{S: s, Err: err} },
	)
}

func (b *storageBase) handleKey(m tea.KeyMsg, rowCount int) {
	switch m.String() {
	case "j", "down":
		if b.cursor < rowCount-1 {
			b.cursor++
		}
	case "k", "up":
		if b.cursor > 0 {
			b.cursor--
		}
	case "g":
		b.cursor = 0
	case "G":
		b.cursor = rowCount - 1
		if b.cursor < 0 {
			b.cursor = 0
		}
	}
}

// ─────────────────────────── Volumes ───────────────────────────

// Volumes lists logical filesystems with usage and health.
type Volumes struct{ storageBase }

func NewVolumes(c Ctx) tui.View { return &Volumes{storageBase: storageBase{ctx: c}} }

func (v *Volumes) Name() string                   { return "volumes" }
func (v *Volumes) Title() string                  { return "Volumes" }
func (v *Volumes) Icon() string                   { return "▮" }
func (v *Volumes) RefreshInterval() time.Duration { return 15 * time.Second }
func (v *Volumes) Bindings() []key.Binding        { return nil }
func (v *Volumes) Init() tea.Cmd                  { return v.fetch() }

func (v *Volumes) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	switch m := msg.(type) {
	case tui.TickMsg:
		return v, v.fetch()
	case storageMsg:
		v.storage, v.err = m.S, m.Err
		return v, nil
	case tea.KeyMsg:
		if m.String() == "r" {
			return v, v.fetch()
		}
		if v.storage != nil {
			v.handleKey(m, len(v.storage.Volumes))
		}
	}
	return v, nil
}

func (v *Volumes) Render(width, height int) string {
	v.width, v.height = width, height
	t := v.ctx.Theme
	if v.storage == nil && v.err == nil {
		return Card(t, width, " ▮  Volumes ", "\n  Loading…\n", true)
	}
	if v.err != nil && v.storage == nil {
		return Card(t, width, " ▮  Volumes ", "\n"+errLine(t, v.err)+"\n", true)
	}
	cols := []Column{
		{Header: "VOLUME", Width: 16, Align: lipgloss.Left},
		{Header: "FS", Width: 8},
		{Header: "RAID", Width: 14},
		{Header: "", Width: 28, Align: lipgloss.Left}, // gauge
		{Header: "USED", Width: 18, Align: lipgloss.Right},
		{Header: "FREE", Width: 12, Align: lipgloss.Right},
		{Header: "STATUS", Width: 0, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0, len(v.storage.Volumes))
	for _, vol := range v.storage.Volumes {
		total := ParseSizeString(vol.Size.Total)
		used := ParseSizeString(vol.Size.Used)
		ratio := 0.0
		if total > 0 {
			ratio = float64(used) / float64(total)
		}
		name := vol.DisplayName
		if name == "" {
			name = vol.ID
		}
		rows = append(rows, []Cell{
			Plain(name),
			Plain(strings.ToUpper(vol.FSType)),
			Plain(vol.RaidType),
			Plain(Gauge(t, 22, ratio) + fmt.Sprintf(" %4.1f%%", ratio*100)),
			Plain(HumanBytes(used) + " / " + HumanBytes(total)),
			Plain(HumanBytes(total - used)),
			Styled(" "+vol.Status+" ", t.HealthStyle(vol.Status)),
		})
	}
	body := "\n" + Table(t, width-4, height-4, cols, rows, v.cursor) + "\n"
	return Card(t, width, " ▮  Volumes ", body, true)
}

// ─────────────────────────── Disks ───────────────────────────

// Disks lists physical drives with health, temperature and bay/slot info.
type Disks struct{ storageBase }

func NewDisks(c Ctx) tui.View { return &Disks{storageBase: storageBase{ctx: c}} }

func (d *Disks) Name() string                   { return "disks" }
func (d *Disks) Title() string                  { return "Disks" }
func (d *Disks) Icon() string                   { return "●" }
func (d *Disks) RefreshInterval() time.Duration { return 15 * time.Second }
func (d *Disks) Bindings() []key.Binding        { return nil }
func (d *Disks) Init() tea.Cmd                  { return d.fetch() }

func (d *Disks) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	switch m := msg.(type) {
	case tui.TickMsg:
		return d, d.fetch()
	case storageMsg:
		d.storage, d.err = m.S, m.Err
		return d, nil
	case tea.KeyMsg:
		if m.String() == "r" {
			return d, d.fetch()
		}
		if d.storage != nil {
			d.handleKey(m, len(d.storage.Disks))
		}
	}
	return d, nil
}

func (d *Disks) Render(width, height int) string {
	d.width, d.height = width, height
	t := d.ctx.Theme
	if d.storage == nil && d.err == nil {
		return Card(t, width, " ●  Disks ", "\n  Loading…\n", true)
	}
	if d.err != nil && d.storage == nil {
		return Card(t, width, " ●  Disks ", "\n"+errLine(t, d.err)+"\n", true)
	}
	cols := []Column{
		{Header: "BAY", Width: 6, Align: lipgloss.Left},
		{Header: "MODEL", Width: 28},
		{Header: "TYPE", Width: 10},
		{Header: "CAPACITY", Width: 12, Align: lipgloss.Right},
		{Header: "TEMP", Width: 8, Align: lipgloss.Right},
		{Header: "SMART", Width: 10, Align: lipgloss.Center},
		{Header: "STATUS", Width: 0, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0, len(d.storage.Disks))
	for _, dk := range d.storage.Disks {
		bay := trimDev(dk.ID)
		if dk.Container.Str != "" {
			bay = dk.Container.Str
		}
		cap := ParseSizeString(dk.Capacity)
		smartStyle := t.HealthStyle(dk.Smart.Status)
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
			Styled(" "+smart+" ", smartStyle),
			Styled(" "+dk.Status+" ", t.HealthStyle(dk.Status)),
		})
	}
	body := "\n" + Table(t, width-4, height-4, cols, rows, d.cursor) + "\n"
	return Card(t, width, " ●  Disks ", body, true)
}

func errLine(t tui.Theme, err error) string {
	return lipgloss.NewStyle().Foreground(t.Error).Render("  " + err.Error())
}
