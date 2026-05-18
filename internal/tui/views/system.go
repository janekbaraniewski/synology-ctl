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

// System is the box-identity view: model / serial / DSM version / CPU /
// RAM / uptime / temperature / load averages. Houses reboot (`B`) and
// shutdown (`S`) actions, each behind a confirm modal — there is no
// "easy on" for the NAS once it powers off.
type System struct {
	ctx Ctx

	info *dsm.SystemInfo
	util *dsm.Utilization

	infoErr error
	utilErr error

	confirm *Confirm
	flash   string
}

// NewSystem constructs the system view.
func NewSystem(c Ctx) tui.View { return &System{ctx: c, confirm: NewConfirm(c.Theme)} }

func (s *System) Name() string                   { return "system" }
func (s *System) Title() string                  { return "System" }
func (s *System) Icon() string                   { return "⌂" }
func (s *System) RefreshInterval() time.Duration { return 15 * time.Second }
func (s *System) Bindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("B"), key.WithHelp("B", "reboot (confirm)")),
		key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "shutdown (confirm)")),
	}
}

func (s *System) Init() tea.Cmd { return tea.Batch(s.fetchInfo(), s.fetchUtil()) }

func (s *System) fetchInfo() tea.Cmd {
	c := s.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) (*dsm.SystemInfo, error) { return c.SystemInfo(ctx) },
		func(v *dsm.SystemInfo, err error) tea.Msg { return sysViewInfoMsg{I: v, Err: err} },
	)
}

func (s *System) fetchUtil() tea.Cmd {
	c := s.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(5*time.Second,
		func(ctx context.Context) (*dsm.Utilization, error) { return c.Utilization(ctx) },
		func(u *dsm.Utilization, err error) tea.Msg { return sysViewUtilMsg{U: u, Err: err} },
	)
}

type systemActionMsg struct {
	Action string
	Err    error
}

func (s *System) issue(action string) tea.Cmd {
	c := s.ctx.Client
	return tui.Fetch(30*time.Second,
		func(ctx context.Context) (struct{}, error) {
			switch action {
			case "reboot":
				return struct{}{}, c.Reboot(ctx)
			case "shutdown":
				return struct{}{}, c.Shutdown(ctx)
			}
			return struct{}{}, nil
		},
		func(_ struct{}, err error) tea.Msg { return systemActionMsg{Action: action, Err: err} },
	)
}

func (s *System) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	if handled, cmd := s.confirm.Update(msg); handled {
		return s, cmd
	}
	switch m := msg.(type) {
	case ConfirmedMsg:
		switch m.Token {
		case "reboot":
			s.flash = "issuing reboot…"
			return s, s.issue("reboot")
		case "shutdown":
			s.flash = "issuing shutdown…"
			return s, s.issue("shutdown")
		}
	case CancelledMsg:
		s.flash = "cancelled"
		return s, nil
	case systemActionMsg:
		if m.Err != nil {
			s.flash = m.Action + " failed: " + m.Err.Error()
		} else {
			s.flash = m.Action + " accepted — the box should obey shortly"
		}
		return s, nil
	case tui.TickMsg:
		return s, tea.Batch(s.fetchInfo(), s.fetchUtil())
	case sysViewInfoMsg:
		s.info, s.infoErr = m.I, m.Err
	case sysViewUtilMsg:
		s.util, s.utilErr = m.U, m.Err
	case tea.KeyMsg:
		switch m.String() {
		case "r":
			return s, tea.Batch(s.fetchInfo(), s.fetchUtil())
		case "B":
			s.confirm.Ask("reboot", "Reboot the NAS?",
				"The NAS will be unreachable for a few minutes.")
		case "S":
			s.confirm.Ask("shutdown", "Shut down the NAS?",
				"The NAS will power off. You'll need physical access (or WoL) to bring it back.")
		}
	}
	return s, nil
}

func (s *System) Render(width, height int) string {
	t := s.ctx.Theme
	if s.confirm.Open() {
		return s.confirm.Render(width, height)
	}
	if s.info == nil {
		return Card(t, width, " ⌂  Loading system info… ", "\n  reaching out to DSM…\n", true)
	}

	muted := lipgloss.NewStyle().Foreground(t.Muted)
	text := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	pair := func(k, v string) string {
		if v == "" {
			v = "—"
		}
		return muted.Render(k+":") + " " + text.Render(v)
	}

	uptime := HumanDurationFromDSMUptime(s.info.UptimeSeconds).String()
	tempColored := lipgloss.NewStyle().Foreground(tempColor(t, s.info.Temperature)).Bold(true).
		Render(fmt.Sprintf("%d°C", s.info.Temperature))

	rows := [][]string{
		{pair("Model", s.info.Model), pair("Serial", s.info.Serial), pair("DSM", coalesce(s.info.DSMVersion, s.info.Version))},
		{
			pair("CPU", strings.TrimSpace(s.info.CPUVendor+" "+s.info.CPUFamily) + " · " + s.info.CPUCores + " cores"),
			pair("RAM", fmt.Sprintf("%d MB", s.info.RAMTotalMB)),
			pair("Uptime", uptime),
		},
	}
	if s.util != nil {
		rows = append(rows, []string{
			pair("Load 1m", fmt.Sprintf("%d%%", s.util.CPU.OneMinLoad)),
			pair("Load 5m", fmt.Sprintf("%d%%", s.util.CPU.FiveMinLoad)),
			pair("Load 15m", fmt.Sprintf("%d%%", s.util.CPU.FifteenMinLoad)),
		})
		rows = append(rows, []string{
			pair("Memory", fmt.Sprintf("%d%% · %s used", s.util.Memory.RealUsage,
				HumanBytes(uint64(s.util.Memory.TotalReal-s.util.Memory.AvailReal)*1024))),
			pair("Buffer/cache", HumanBytes(uint64(s.util.Memory.Buffer+s.util.Memory.Cached)*1024)),
			pair("Swap", fmt.Sprintf("%d%%", s.util.Memory.SwapUsage)),
		})
	}
	rows = append(rows, []string{muted.Render("Temperature:") + " " + tempColored})

	body := t.Title().Render(" Identity ") + "\n"
	for _, row := range rows {
		body += strings.Join(row, "   ") + "\n"
	}

	parts := []string{
		t.Card(false).Width(width - 2).Render(body),
		"",
		t.Title().Render(" Power "),
		lipgloss.NewStyle().Foreground(t.Muted).Render(
			"  Press " + t.Chip(t.Error).Render(" B ") + " to reboot, " +
				t.Chip(t.Warn).Render(" S ") + " to shut down."),
	}
	if s.infoErr != nil {
		parts = append(parts, errLine(t, s.infoErr))
	}
	if s.utilErr != nil {
		parts = append(parts, errLine(t, s.utilErr))
	}
	if s.flash != "" {
		parts = append(parts, "", lipgloss.NewStyle().Foreground(t.Muted).Render("  "+s.flash))
	}
	return fitOrScroll(strings.Join(parts, "\n"), height)
}

func (s *System) Inspect(width, height int) string {
	if s.info == nil {
		return ""
	}
	t := s.ctx.Theme
	muted := lipgloss.NewStyle().Foreground(t.Muted)
	text := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	tempStyle := lipgloss.NewStyle().Foreground(tempColor(t, s.info.Temperature)).Bold(true)
	uptime := HumanDurationFromDSMUptime(s.info.UptimeSeconds).String()
	parts := []string{
		t.Title().Render(" " + coalesce(s.info.Model, "NAS") + " "),
		"",
		muted.Render("Serial:    ") + text.Render(s.info.Serial),
		muted.Render("DSM:       ") + text.Render(coalesce(s.info.DSMVersion, s.info.Version)),
		muted.Render("Uptime:    ") + text.Render(uptime),
		muted.Render("Temp:      ") + tempStyle.Render(fmt.Sprintf("%d°C", s.info.Temperature)),
	}
	if s.util != nil {
		parts = append(parts,
			"",
			muted.Render("CPU 1m:    ")+text.Render(fmt.Sprintf("%d%%", s.util.CPU.OneMinLoad)),
			muted.Render("CPU 5m:    ")+text.Render(fmt.Sprintf("%d%%", s.util.CPU.FiveMinLoad)),
			muted.Render("CPU 15m:   ")+text.Render(fmt.Sprintf("%d%%", s.util.CPU.FifteenMinLoad)),
			muted.Render("Memory:    ")+text.Render(fmt.Sprintf("%d%%", s.util.Memory.RealUsage)),
		)
	}
	_ = width
	_ = height
	return strings.Join(parts, "\n")
}
