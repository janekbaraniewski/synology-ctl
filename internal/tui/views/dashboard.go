package views

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/janbaraniewski/synology-ctl/internal/dsm"
	"github.com/janbaraniewski/synology-ctl/internal/tui"
)

// Dashboard is the live overview. Every two seconds it samples
// utilization, processes and storage, and renders:
//
//	┌─ System info ─────────────────────────────────────────────┐
//	│   model · serial · DSM · uptime · temperature             │
//	├─ Metrics row ─────────────────────────────────────────────┤
//	│   CPU      Memory      Network      Disk I/O              │
//	│   (gauges and sparklines, dense)                          │
//	├─ Volumes ─────────────────────────────────────────────────┤
//	│   per-volume usage bars                                   │
//	├─ Disks ───────────────────────────────────────────────────┤
//	│   per-drive strip with temperature                        │
//	├─ Top processes / Alerts ──────────────────────────────────┤
//	│   (two-column: top CPU users · recent warn/err log entries)│
//	└────────────────────────────────────────────────────────────┘
type Dashboard struct {
	ctx Ctx

	util     *dsm.Utilization
	storage  *dsm.Storage
	procs    []dsm.Process
	logs     []dsm.LogEntry
	utilErr  error
	storeErr error
	procErr  error
	logErr   error

	lastTick time.Time

	cpuHist  []float64
	memHist  []float64
	rxHist   []float64
	txHist   []float64
	diskHist []float64
}

const dashHist = 120

// NewDashboard constructs the view.
func NewDashboard(c Ctx) tui.View { return &Dashboard{ctx: c} }

func (d *Dashboard) Name() string                   { return "dashboard" }
func (d *Dashboard) Title() string                  { return "Dashboard" }
func (d *Dashboard) Icon() string                   { return "◆" }
func (d *Dashboard) RefreshInterval() time.Duration { return 2 * time.Second }
func (d *Dashboard) Bindings() []key.Binding {
	return []key.Binding{key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh"))}
}

type procsMsg struct {
	P   []dsm.Process
	Err error
}
type recentLogsMsg struct {
	L   []dsm.LogEntry
	Err error
}

func (d *Dashboard) Init() tea.Cmd {
	return tea.Batch(d.fetchUtil(), d.fetchStorage(), d.fetchProcs(), d.fetchRecentLogs())
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

func (d *Dashboard) fetchProcs() tea.Cmd {
	c := d.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(5*time.Second,
		func(ctx context.Context) ([]dsm.Process, error) { return c.Processes(ctx) },
		func(p []dsm.Process, err error) tea.Msg { return procsMsg{P: p, Err: err} },
	)
}

func (d *Dashboard) fetchRecentLogs() tea.Cmd {
	c := d.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(5*time.Second,
		func(ctx context.Context) ([]dsm.LogEntry, error) {
			items, _, err := c.Logs(ctx, dsm.LogQuery{Source: "system", Limit: 20})
			return items, err
		},
		func(l []dsm.LogEntry, err error) tea.Msg { return recentLogsMsg{L: l, Err: err} },
	)
}

type utilMsg struct {
	U   *dsm.Utilization
	Err error
}
type storageMsg struct {
	S   *dsm.Storage
	Err error
}

func (d *Dashboard) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	switch m := msg.(type) {
	case tui.TickMsg:
		d.lastTick = m.At
		return d, tea.Batch(d.fetchUtil(), d.fetchStorage(), d.fetchProcs(), d.fetchRecentLogs())
	case utilMsg:
		d.util, d.utilErr = m.U, m.Err
		d.sampleHistory()
		return d, nil
	case storageMsg:
		d.storage, d.storeErr = m.S, m.Err
		return d, nil
	case procsMsg:
		d.procs, d.procErr = m.P, m.Err
		return d, nil
	case recentLogsMsg:
		d.logs, d.logErr = m.L, m.Err
		return d, nil
	case tea.KeyMsg:
		if m.String() == "r" {
			return d, tea.Batch(d.fetchUtil(), d.fetchStorage(), d.fetchProcs(), d.fetchRecentLogs())
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
	d.rxHist = pushHistory(d.rxHist, float64(rx))
	d.txHist = pushHistory(d.txHist, float64(tx))
	d.diskHist = pushHistory(d.diskHist, float64(d.util.Disk.Total.Util))
}

func pushHistory(buf []float64, v float64) []float64 {
	buf = append(buf, v)
	if len(buf) > dashHist {
		buf = buf[len(buf)-dashHist:]
	}
	return buf
}

func (d *Dashboard) Render(width, height int) string {
	t := d.ctx.Theme
	if d.util == nil && d.utilErr == nil {
		return Card(t, width, " ◆  Loading dashboard… ", "\n  fetching first sample…\n", true)
	}

	// Reserve fixed-height sections at the top; let the bottom row stretch.
	metricsH := 7
	volumesH := d.volumesHeight()
	disksH := 5
	bottomH := height - metricsH - volumesH - disksH - 1
	if bottomH < 8 {
		bottomH = 8
	}

	rows := []string{
		d.renderMetricsRow(width, metricsH),
		d.renderVolumes(width, volumesH),
		d.renderDisks(width, disksH),
		d.renderBottomRow(width, bottomH),
	}
	return strings.Join(rows, "\n")
}

func (d *Dashboard) volumesHeight() int {
	if d.storage == nil {
		return 4
	}
	rows := len(d.storage.Volumes)
	if rows < 1 {
		rows = 1
	}
	return rows + 3 // border + title + spacing
}

func (d *Dashboard) renderMetricsRow(width, height int) string {
	t := d.ctx.Theme
	colW := (width - 3) / 4
	if colW < 22 {
		colW = (width - 1) / 2
	}

	var cpu, mem int
	var memUsedKB, memTotalKB int
	var rx, tx int64
	var diskUtil int
	if d.util != nil {
		cpu = d.util.CPU.UserLoad + d.util.CPU.SystemLoad + d.util.CPU.OtherLoad
		mem = d.util.Memory.RealUsage
		memTotalKB = d.util.Memory.TotalReal
		memUsedKB = memTotalKB - d.util.Memory.AvailReal
		for _, n := range d.util.Network {
			if n.Device == "total" {
				rx, tx = n.Rx, n.Tx
			}
		}
		diskUtil = d.util.Disk.Total.Util
	}

	cpuCard := d.metricCard(colW, " CPU ",
		fmt.Sprintf("%3d%%", cpu), float64(cpu)/100, d.cpuHist, "")
	memCard := d.metricCard(colW, " Memory ",
		fmt.Sprintf("%3d%%", mem), float64(mem)/100, d.memHist,
		fmt.Sprintf("%s / %s", HumanBytes(uint64(memUsedKB)*1024), HumanBytes(uint64(memTotalKB)*1024)))
	netCard := d.networkMetricCard(colW, rx, tx)
	diskCard := d.metricCard(colW, " Disk I/O ",
		fmt.Sprintf("%3d%%", diskUtil), float64(diskUtil)/100, d.diskHist, "")
	_ = t
	_ = height
	return lipgloss.JoinHorizontal(lipgloss.Top, cpuCard, " ", memCard, " ", netCard, " ", diskCard)
}

func (d *Dashboard) metricCard(width int, title, big string, ratio float64, hist []float64, subtitle string) string {
	t := d.ctx.Theme
	innerW := width - 4
	if innerW < 12 {
		innerW = 12
	}
	titleStyle := t.Title().Render(title)
	bigStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(big)
	bar := Gauge(t, innerW, ratio)
	spark := Sparkline(t, innerW, hist)
	sub := lipgloss.NewStyle().Foreground(t.Muted).Render(subtitle)
	body := titleStyle + "\n" + bigStyle + "  " + bar + "\n" + spark + "\n" + sub
	return t.Card(false).Width(width - 2).Render(body)
}

func (d *Dashboard) networkMetricCard(width int, rx, tx int64) string {
	t := d.ctx.Theme
	innerW := width - 4
	if innerW < 12 {
		innerW = 12
	}
	rxSpark := Sparkline(t, innerW-12, d.rxHist)
	txSpark := Sparkline(t, innerW-12, d.txHist)
	rxLabel := lipgloss.NewStyle().Foreground(t.Success).Bold(true).Render(fmt.Sprintf("↓ %s", HumanRate(rx)))
	txLabel := lipgloss.NewStyle().Foreground(t.Info).Bold(true).Render(fmt.Sprintf("↑ %s", HumanRate(tx)))
	title := t.Title().Render(" Network ")
	body := title + "\n" +
		padRight(rxLabel, 12) + " " + rxSpark + "\n" +
		padRight(txLabel, 12) + " " + txSpark + "\n" +
		lipgloss.NewStyle().Foreground(t.Muted).Render(" sample · 2s")
	return t.Card(false).Width(width - 2).Render(body)
}

func (d *Dashboard) renderVolumes(width, height int) string {
	t := d.ctx.Theme
	title := t.Title().Render(" Volumes ")
	if d.storage == nil {
		if d.storeErr != nil {
			return t.Card(false).Width(width - 2).Render(title + "\n" + errLine(t, d.storeErr))
		}
		return t.Card(false).Width(width - 2).Render(title + "\n  …loading")
	}
	if len(d.storage.Volumes) == 0 {
		return t.Card(false).Width(width - 2).Render(title + "\n  no volumes reported")
	}
	var lines []string
	barWidth := width - 60
	if barWidth < 16 {
		barWidth = 16
	}
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
		bar := Gauge(t, barWidth, ratio)
		status := t.HealthStyle(v.Status).Render(" " + v.Status + " ")
		muted := lipgloss.NewStyle().Foreground(t.Muted)
		text := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
		lines = append(lines, fmt.Sprintf("  %s  %s  %s   %s   %s",
			text.Render(padRight(name, 10)),
			bar,
			text.Render(fmt.Sprintf("%5.1f%%", ratio*100)),
			muted.Render(fmt.Sprintf("%s / %s", HumanBytes(used), HumanBytes(total))),
			status,
		))
	}
	body := title + "\n" + strings.Join(lines, "\n")
	_ = height
	return t.Card(false).Width(width - 2).Render(body)
}

func (d *Dashboard) renderDisks(width, height int) string {
	t := d.ctx.Theme
	title := t.Title().Render(" Disks ")
	if d.storage == nil {
		return t.Card(false).Width(width - 2).Render(title + "\n  …loading")
	}
	if len(d.storage.Disks) == 0 {
		return t.Card(false).Width(width - 2).Render(title + "\n  no disks reported")
	}
	var chips []string
	for _, dk := range d.storage.Disks {
		bay := trimDev(dk.ID)
		if dk.Container.Str != "" && len(d.storage.Disks) > 4 {
			bay = dk.Container.Str
		}
		muted := lipgloss.NewStyle().Foreground(t.Muted)
		health := t.HealthStyle(dk.Status).Render(dk.Status)
		text := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
		chips = append(chips,
			fmt.Sprintf("%s %s %s  %s",
				text.Render(bay),
				muted.Render(strings.TrimSpace(dk.Vendor+" "+dk.Model)),
				lipgloss.NewStyle().Foreground(tempColor(t, dk.Temperature)).Bold(true).Render(fmt.Sprintf("%d°C", dk.Temperature)),
				health,
			))
	}
	body := title + "\n  " + strings.Join(chips, "    ")
	_ = height
	return t.Card(false).Width(width - 2).Render(body)
}

func tempColor(t tui.Theme, c int) lipgloss.AdaptiveColor {
	switch {
	case c >= 50:
		return t.Error
	case c >= 40:
		return t.Warn
	default:
		return t.Success
	}
}

func (d *Dashboard) renderBottomRow(width, height int) string {
	colW := (width - 1) / 2
	left := d.renderTopProcesses(colW, height)
	right := d.renderRecentAlerts(width-colW-1, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (d *Dashboard) renderTopProcesses(width, height int) string {
	t := d.ctx.Theme
	title := t.Title().Render(" Top processes (by CPU) ")
	if d.procErr != nil {
		return t.Card(false).Width(width - 2).Height(height).Render(title + "\n" + errLine(t, d.procErr))
	}
	if len(d.procs) == 0 {
		return t.Card(false).Width(width - 2).Height(height).Render(title + "\n  …")
	}
	sorted := make([]dsm.Process, len(d.procs))
	copy(sorted, d.procs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].CPU > sorted[j].CPU })
	n := 10
	if n > len(sorted) {
		n = len(sorted)
	}
	maxCPU := 1
	for _, p := range sorted[:n] {
		if p.CPU > maxCPU {
			maxCPU = p.CPU
		}
	}
	muted := lipgloss.NewStyle().Foreground(t.Muted)
	text := lipgloss.NewStyle().Foreground(t.Text)
	accent := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	header := muted.Render(fmt.Sprintf("  %-7s  %-32s %s %s", "PID", "COMMAND", "CPU", "  MEM"))
	rows := []string{header}
	for _, p := range sorted[:n] {
		bar := Gauge(t, 10, float64(p.CPU)/float64(maxCPU))
		rows = append(rows, fmt.Sprintf("  %-7d  %-32s %s %s",
			p.PID,
			text.Render(clipTo(p.Command, 32)),
			bar,
			accent.Render(HumanBytes(uint64(p.Mem)*1024)),
		))
	}
	body := title + "\n" + strings.Join(rows, "\n")
	return t.Card(false).Width(width - 2).Height(height).Render(body)
}

func (d *Dashboard) renderRecentAlerts(width, height int) string {
	t := d.ctx.Theme
	title := t.Title().Render(" Recent activity ")
	if d.logErr != nil {
		return t.Card(false).Width(width - 2).Height(height).Render(title + "\n" + errLine(t, d.logErr))
	}
	if len(d.logs) == 0 {
		return t.Card(false).Width(width - 2).Height(height).Render(title + "\n  …")
	}
	muted := lipgloss.NewStyle().Foreground(t.Muted)
	text := lipgloss.NewStyle().Foreground(t.Text)
	var rows []string
	maxRows := height - 4
	if maxRows < 1 {
		maxRows = 1
	}
	for i, e := range d.logs {
		if i >= maxRows {
			break
		}
		level := strings.ToLower(e.Level)
		var levelChip string
		switch level {
		case "err", "error":
			levelChip = lipgloss.NewStyle().Foreground(t.Error).Bold(true).Render("✗")
		case "warn", "warning":
			levelChip = lipgloss.NewStyle().Foreground(t.Warn).Bold(true).Render("⚠")
		default:
			levelChip = lipgloss.NewStyle().Foreground(t.Info).Render("•")
		}
		event := e.Event
		if e.Descr != "" {
			event = e.Descr
		}
		rows = append(rows, fmt.Sprintf("  %s  %s  %s",
			levelChip,
			muted.Render(padRight(e.Time, 18)),
			text.Render(clipTo(event, width-30)),
		))
	}
	body := title + "\n" + strings.Join(rows, "\n")
	return t.Card(false).Width(width - 2).Height(height).Render(body)
}

func trimDev(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
