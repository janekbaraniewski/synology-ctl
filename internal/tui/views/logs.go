package views

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/janbaraniewski/synology-ctl/internal/dsm"
	"github.com/janbaraniewski/synology-ctl/internal/tui"
)

// Logs is a paginated system log view with severity colouring.
type Logs struct {
	listBase
	ctx    Ctx
	logs   []dsm.LogEntry
	total  int
	err    error
	offset int
	source string
}

type logsMsg struct {
	L     []dsm.LogEntry
	Total int
	Err   error
}

func NewLogs(c Ctx) tui.View {
	l := &Logs{ctx: c, source: "system"}
	l.initBase(c)
	return l
}

func (l *Logs) Name() string                   { return "logs" }
func (l *Logs) Title() string                  { return "Logs" }
func (l *Logs) Icon() string                   { return "≡" }
func (l *Logs) RefreshInterval() time.Duration { return 20 * time.Second }
func (l *Logs) Bindings() []key.Binding {
	return append(BaseBindings(),
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next page")),
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev page")),
		key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle source")),
	)
}
func (l *Logs) Init() tea.Cmd { return l.fetch() }

func (l *Logs) fetch() tea.Cmd {
	c := l.ctx.Client
	if c == nil {
		return nil
	}
	q := dsm.LogQuery{Source: l.source, Offset: l.offset, Limit: 100}
	return tui.Fetch(10*time.Second,
		func(ctx context.Context) ([]dsm.LogEntry, error) {
			items, total, err := c.Logs(ctx, q)
			l.total = total
			return items, err
		},
		func(v []dsm.LogEntry, err error) tea.Msg { return logsMsg{L: v, Total: l.total, Err: err} },
	)
}

func (l *Logs) visible() []dsm.LogEntry {
	if l.FilterValue() == "" {
		return l.logs
	}
	out := make([]dsm.LogEntry, 0)
	for _, e := range l.logs {
		if l.FilterMatch(e.Time, e.Level, e.User, e.IP, e.Event, e.Descr) {
			out = append(out, e)
		}
	}
	return out
}

func (l *Logs) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	rows := l.visible()
	if cmd, handled := l.HandleKey(msg, len(rows)); handled {
		return l, cmd
	}
	if l.IsEnter(msg) && len(rows) > 0 {
		l.ShowDetail("Log entry "+rows[l.Cursor()].Time, rows[l.Cursor()])
		return l, nil
	}
	switch m := msg.(type) {
	case tui.TickMsg:
		return l, l.fetch()
	case logsMsg:
		l.logs, l.err = m.L, m.Err
		l.total = m.Total
		l.ClampCursor(len(l.visible()))
		return l, nil
	case tea.KeyMsg:
		switch m.String() {
		case "r":
			return l, l.fetch()
		case "n":
			if l.offset+100 < l.total {
				l.offset += 100
				l.ResetCursor()
				return l, l.fetch()
			}
		case "p":
			if l.offset >= 100 {
				l.offset -= 100
				l.ResetCursor()
				return l, l.fetch()
			}
		case "t":
			if l.source == "system" {
				l.source = "connection"
			} else {
				l.source = "system"
			}
			l.offset = 0
			l.ResetCursor()
			return l, l.fetch()
		}
	}
	return l, nil
}

func (l *Logs) Render(width, height int) string {
	t := l.ctx.Theme
	title := " ≡  Logs · " + l.source + " — ⏎ details · / filter · [n]ext [p]rev [t]oggle "
	if l.DetailVisible() {
		return l.RenderDetail(width, height)
	}
	if l.logs == nil && l.err == nil {
		return Card(t, width, title, "\n  Loading…\n", true)
	}
	if l.err != nil && l.logs == nil {
		return Card(t, width, title, "\n"+errLine(t, l.err)+"\n", true)
	}
	cols := []Column{
		{Header: "TIME", Width: 20},
		{Header: "LEVEL", Width: 8, Align: lipgloss.Center},
		{Header: "USER", Width: 14},
		{Header: "IP", Width: 16},
		{Header: "EVENT", Width: 0},
	}
	rows := make([][]Cell, 0)
	for _, e := range l.visible() {
		level := strings.ToLower(e.Level)
		levelStyle := lipgloss.NewStyle().Foreground(t.Muted)
		switch level {
		case "err", "error":
			levelStyle = lipgloss.NewStyle().Foreground(t.Error).Bold(true)
		case "warn", "warning":
			levelStyle = lipgloss.NewStyle().Foreground(t.Warn).Bold(true)
		case "info":
			levelStyle = lipgloss.NewStyle().Foreground(t.Info)
		}
		event := e.Event
		if e.Descr != "" {
			event = e.Descr
		}
		rows = append(rows, []Cell{
			Plain(e.Time),
			Styled(level, levelStyle),
			Plain(e.User),
			Plain(e.IP),
			Plain(event),
		})
	}
	footerH := 2
	if f := l.FilterFooter(t); f != "" {
		footerH = 3
	}
	body := "\n" + Table(t, width-4, height-3-footerH, cols, rows, l.Cursor()) + "\n"
	body += lipgloss.NewStyle().Foreground(t.Muted).Render(
		" page "+itoaInt(l.offset/100+1)+" · "+itoaInt(l.total)+" entries total") + "\n"
	if f := l.FilterFooter(t); f != "" {
		body += f + "\n"
	}
	return Card(t, width, title, body, true)
}
