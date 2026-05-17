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

// Dashboard is the live overview shown on startup. Every two seconds it
// fetches utilization + storage and re-renders the gauges, sparklines,
// volume bars and disk strip.
type Dashboard struct {
	ctx Ctx

	util      *dsm.Utilization
	storage   *dsm.Storage
	utilErr   error
	storeErr  error
	lastTick  time.Time

	// Ring buffers for sparklines. Each holds the latest ~120 samples.
	cpuHist  []float64
	memHist  []float64
	rxHist   []float64
	txHist   []float64
	diskHist []float64

	width, height int
}

const histSize = 120

// NewDashboard constructs the view.
func NewDashboard(c Ctx) tui.View { return &Dashboard{ctx: c} }

// View interface

func (d *Dashboard) Name() string                   { return "dashboard" }
func (d *Dashboard) Title() string                  { return "Dashboard" }
func (d *Dashboard) Icon() string                   { return "◆" }
func (d *Dashboard) RefreshInterval() time.Duration { return 2 * time.Second }

func (d *Dashboard) Bindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "force refresh")),
	}
}

type utilMsg struct {
	U   *dsm.Utilization
	Err error
}
type storageMsg struct {
	S   *dsm.Storage
	Err error
}

func (d *Dashboard) Init() tea.Cmd {
	return tea.Batch(d.fetchUtil(), d.fetchStorage())
}

func (d *Dashboard) fetchUtil() tea.Cmd {
	c := d.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(5*time.Second,
		func(ctx context.Context) (*dsm.Utilization, error) { return c.Utilization(ctx) },
		func(u *dsm.Utilization, err error) tea.Msg { return utilMsg{U: u, Err: err} },
	)
}

func (d *Dashboard) fetchStorage() tea.Cmd {
	c := d.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) (*dsm.Storage, error) { return c.Storage(ctx) },
		func(s *dsm.Storage, err error) tea.Msg { return storageMsg{S: s, Err: err} },
	)
}

func (d *Dashboard) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	switch m := msg.(type) {
	case tui.TickMsg:
		d.lastTick = m.At
		return d, tea.Batch(d.fetchUtil(), d.fetchStorage())
	case utilMsg:
		d.util, d.utilErr = m.U, m.Err
		d.sampleHistory()
		return d, nil
	case storageMsg:
		d.storage, d.storeErr = m.S, m.Err
		return d, nil
	case tea.KeyMsg:
		if m.String() == "r" {
			return d, tea.Batch(d.fetchUtil(), d.fetchStorage())
		}
	}
	return d, nil
}

func (d *Dashboard) sampleHistory() {
	if d.util == nil {
		return
	}
	cpu := float64(d.util.CPU.UserLoad + d.util.CPU.SystemLoad + d.util.CPU.OtherLoad)
	d.cpuHist = pushHistory(d.cpuHist, cpu)
	d.memHist = pushHistory(d.memHist, float64(d.util.Memory.RealUsage))
	var rx, tx int64
	for _, n := range d.util.Network {
		if n.Device == "total" {
			rx, tx = n.Rx, n.Tx
			break
		}
	}
	if rx == 0 && tx == 0 && len(d.util.Network) > 0 {
		for _, n := range d.util.Network {
			rx += n.Rx
			tx += n.Tx
		}
	}
	d.rxHist = pushHistory(d.rxHist, float64(rx))
	d.txHist = pushHistory(d.txHist, float64(tx))
	d.diskHist = pushHistory(d.diskHist, float64(d.util.Disk.Total.Util))
}

func pushHistory(buf []float64, v float64) []float64 {
	buf = append(buf, v)
	if len(buf) > histSize {
		buf = buf[len(buf)-histSize:]
	}
	return buf
}

func (d *Dashboard) Render(width, height int) string {
	d.width, d.height = width, height
	t := d.ctx.Theme

	// Bail with friendly message until first data arrives.
	if d.util == nil && d.utilErr == nil {
		return Card(t, width, " ◆  Loading dashboard… ", "\n  Connecting to "+placeholderHost(d.ctx)+"\n", true)
	}
	if d.utilErr != nil && d.util == nil {
		return Card(t, width, " ◆  Dashboard ",
			lipgloss.NewStyle().Foreground(t.Error).Render("\nFailed to fetch utilization:\n  "+d.utilErr.Error()+"\n"),
			true)
	}

	// Layout:
	//   [system summary card, full width]
	//   [cpu] [ram] [net] [disk]   ← 4-up
	//   [volumes card, full width]
	//   [disks card,   full width]
	colW := (width - 3) / 4 // 3 gaps between 4 cards
	if colW < 18 {
		colW = (width - 1) / 2
	}

	rows := []string{
		d.renderSystemSummary(width),
		d.renderMetricRow(colW),
		d.renderVolumes(width),
		d.renderDisks(width),
	}
	return strings.Join(rows, "\n")
}

func (d *Dashboard) renderSystemSummary(width int) string {
	t := d.ctx.Theme
	var model, dsmVer, temp, uptime, serial string
	if d.util != nil {
		// best-effort placeholder — system info is owned by the App top bar
	}
	// We don't have direct SystemInfo here (the App owns it). We surface
	// a short status row instead, with a chip for connection state.
	chip := t.HealthStyle("connected").Render(" connected ")
	if d.utilErr != nil {
		chip = t.HealthStyle("error").Render(" error ")
	}
	title := " ◆  Live overview "
	body := fmt.Sprintf("\n%s   %s\n",
		chip,
		lipgloss.NewStyle().Foreground(t.Muted).Render(
			fmt.Sprintf("Last sample %s · sampling every 2s", relativeTime(d.lastTick)),
		),
	)
	_ = model
	_ = dsmVer
	_ = temp
	_ = uptime
	_ = serial
	return Card(t, width, title, body, false)
}

func (d *Dashboard) renderMetricRow(colW int) string {
	t := d.ctx.Theme
	if d.util == nil {
		return ""
	}

	cpu := d.util.CPU.UserLoad + d.util.CPU.SystemLoad + d.util.CPU.OtherLoad
	mem := d.util.Memory.RealUsage
	totalMem := d.util.Memory.TotalReal // KB
	usedMem := totalMem - d.util.Memory.AvailReal

	var rx, tx int64
	for _, n := range d.util.Network {
		if n.Device == "total" {
			rx, tx = n.Rx, n.Tx
		}
	}
	diskUtil := d.util.Disk.Total.Util

	innerW := colW - 4 // card padding + border

	cpuCard := Card(t, colW, " CPU ",
		fmt.Sprintf("\n%s %3d%%\n%s\n",
			Gauge(t, innerW, float64(cpu)/100),
			cpu,
			Sparkline(t, innerW, d.cpuHist)),
		false)

	ramCard := Card(t, colW, " Memory ",
		fmt.Sprintf("\n%s %3d%%\n%s · %s\n",
			Gauge(t, innerW, float64(mem)/100),
			mem,
			HumanBytes(uint64(usedMem)*1024),
			HumanBytes(uint64(totalMem)*1024)),
		false)

	netCard := Card(t, colW, " Network ",
		fmt.Sprintf("\n %s ↓ %s\n %s ↑ %s\n",
			Sparkline(t, innerW-12, d.rxHist), HumanRate(rx),
			Sparkline(t, innerW-12, d.txHist), HumanRate(tx)),
		false)

	diskCard := Card(t, colW, " Disk I/O ",
		fmt.Sprintf("\n%s %3d%%\n%s\n",
			Gauge(t, innerW, float64(diskUtil)/100),
			diskUtil,
			Sparkline(t, innerW, d.diskHist)),
		false)

	return lipgloss.JoinHorizontal(lipgloss.Top, cpuCard, " ", ramCard, " ", netCard, " ", diskCard)
}

func (d *Dashboard) renderVolumes(width int) string {
	t := d.ctx.Theme
	if d.storage == nil {
		return Card(t, width, " Volumes ", "\n  …loading\n", false)
	}
	if len(d.storage.Volumes) == 0 {
		return Card(t, width, " Volumes ", "\n  No volumes reported.\n", false)
	}
	var lines []string
	for _, v := range d.storage.Volumes {
		total := ParseSizeString(v.Size.Total)
		used := ParseSizeString(v.Size.Used)
		ratio := 0.0
		if total > 0 {
			ratio = float64(used) / float64(total)
		}
		name := v.VolPath
		if name == "" {
			name = v.ID
		}
		status := t.HealthStyle(v.Status).Render(" " + v.Status + " ")
		bar := Gauge(t, 32, ratio)
		line := fmt.Sprintf("  %-12s  %s  %6.1f%%  %s / %s   %s   %s",
			Wrap(name, 12),
			bar,
			ratio*100,
			HumanBytes(used),
			HumanBytes(total),
			lipgloss.NewStyle().Foreground(t.Muted).Render(strings.ToUpper(v.FSType)+" "+v.RaidType),
			status,
		)
		lines = append(lines, line)
	}
	body := "\n" + strings.Join(lines, "\n") + "\n"
	return Card(t, width, " Volumes ", body, false)
}

func (d *Dashboard) renderDisks(width int) string {
	t := d.ctx.Theme
	if d.storage == nil {
		return ""
	}
	if len(d.storage.Disks) == 0 {
		return Card(t, width, " Disks ", "\n  No disks reported.\n", false)
	}
	var chips []string
	for _, dk := range d.storage.Disks {
		label := fmt.Sprintf("%s · %s · %d°C", trimDev(dk.ID), dk.Model, dk.Temperature)
		chips = append(chips, t.HealthStyle(dk.Status).Render(" "+label+" "))
	}
	body := "\n  " + strings.Join(chips, "  ") + "\n"
	return Card(t, width, " Disks ", body, false)
}

func trimDev(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func placeholderHost(c Ctx) string {
	if c.Client == nil {
		return "<no client>"
	}
	return c.Client.Host()
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < 2*time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return t.Format("15:04:05")
	}
}
