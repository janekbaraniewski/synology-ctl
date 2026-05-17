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
	ctx    Ctx
	logs   []dsm.LogEntry
	total  int
	err    error
	cursor int
	offset int
	source string // "system" | "connection"
}

type logsMsg struct {
	L     []dsm.LogEntry
	Total int
	Err   error
}

func NewLogs(c Ctx) tui.View { return &Logs{ctx: c, source: "system"} }

func (l *Logs) Name() string                   { return "logs" }
func (l *Logs) Title() string                  { return "Logs" }
func (l *Logs) Icon() string                   { return "≡" }
func (l *Logs) RefreshInterval() time.Duration { return 20 * time.Second }
func (l *Logs) Bindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next page")),
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev page")),
		key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle source")),
	}
}

func (l *Logs) Init() tea.Cmd { return l.fetch() }

func (l *Logs) fetch() tea.Cmd {
	c := l.ctx.Client
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

func (l *Logs) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	switch m := msg.(type) {
	case tui.TickMsg:
		return l, l.fetch()
	case logsMsg:
		l.logs, l.err = m.L, m.Err
		l.total = m.Total
		return l, nil
	case tea.KeyMsg:
		switch m.String() {
		case "r":
			return l, l.fetch()
		case "j", "down":
			if l.cursor < len(l.logs)-1 {
				l.cursor++
			}
		case "k", "up":
			if l.cursor > 0 {
				l.cursor--
			}
		case "n":
			if l.offset+100 < l.total {
				l.offset += 100
				l.cursor = 0
				return l, l.fetch()
			}
		case "p":
			if l.offset >= 100 {
				l.offset -= 100
				l.cursor = 0
				return l, l.fetch()
			}
		case "t":
			if l.source == "system" {
				l.source = "connection"
			} else {
				l.source = "system"
			}
			l.offset = 0
			l.cursor = 0
			return l, l.fetch()
		}
	}
	return l, nil
}

func (l *Logs) Render(width, height int) string {
	t := l.ctx.Theme
	title := " ≡  Logs · " + l.source + " — [n]ext [p]rev [t]oggle "
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
	rows := make([][]Cell, 0, len(l.logs))
	for _, e := range l.logs {
		level := strings.ToLower(e.Level)
		levelStyle := t.SubtleChip()
		switch level {
		case "err", "error":
			levelStyle = t.Chip(t.Error)
		case "warn", "warning":
			levelStyle = t.Chip(t.Warn)
		case "info":
			levelStyle = t.Chip(t.Info)
		}
		event := e.Event
		if e.Descr != "" {
			event = e.Descr
		}
		rows = append(rows, []Cell{
			Plain(e.Time),
			Styled(" "+level+" ", levelStyle),
			Plain(e.User),
			Plain(e.IP),
			Plain(event),
		})
	}
	body := "\n" + Table(t, width-4, height-5, cols, rows, l.cursor) + "\n"
	footer := lipgloss.NewStyle().Foreground(t.Muted).Render(
		" page " + itoaInt(l.offset/100+1) + " · " + itoaInt(l.total) + " entries total",
	)
	body += footer + "\n"
	return Card(t, width, title, body, true)
}
