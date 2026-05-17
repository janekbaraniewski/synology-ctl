package views

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/janbaraniewski/synology-ctl/internal/dsm"
	"github.com/janbaraniewski/synology-ctl/internal/tui"
)

// Services is the system-service list with control actions.
type Services struct {
	ctx     Ctx
	svcs    []dsm.Service
	err     error
	cursor  int
	flash   string
}

type servicesMsg struct {
	S   []dsm.Service
	Err error
}
type svcActionMsg struct {
	ID, Action string
	Err        error
}

func NewServices(c Ctx) tui.View { return &Services{ctx: c} }

func (s *Services) Name() string                   { return "services" }
func (s *Services) Title() string                  { return "Services" }
func (s *Services) Icon() string                   { return "⌬" }
func (s *Services) RefreshInterval() time.Duration { return 20 * time.Second }
func (s *Services) Bindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start")),
		key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop")),
		key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "restart")),
	}
}

func (s *Services) Init() tea.Cmd { return s.fetch() }

func (s *Services) fetch() tea.Cmd {
	c := s.ctx.Client
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) ([]dsm.Service, error) { return c.Services(ctx) },
		func(v []dsm.Service, err error) tea.Msg { return servicesMsg{S: v, Err: err} },
	)
}

func (s *Services) act(id, action string) tea.Cmd {
	c := s.ctx.Client
	return tui.Fetch(20*time.Second,
		func(ctx context.Context) (struct{}, error) { return struct{}{}, c.ServiceControl(ctx, id, action) },
		func(_ struct{}, err error) tea.Msg { return svcActionMsg{ID: id, Action: action, Err: err} },
	)
}

func (s *Services) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	switch m := msg.(type) {
	case tui.TickMsg:
		return s, s.fetch()
	case servicesMsg:
		s.svcs, s.err = m.S, m.Err
		return s, nil
	case svcActionMsg:
		if m.Err != nil {
			s.flash = m.Action + " failed: " + m.Err.Error()
		} else {
			s.flash = m.Action + " ok"
		}
		return s, s.fetch()
	case tea.KeyMsg:
		switch m.String() {
		case "r":
			return s, s.fetch()
		case "j", "down":
			if s.cursor < len(s.svcs)-1 {
				s.cursor++
			}
		case "k", "up":
			if s.cursor > 0 {
				s.cursor--
			}
		case "s":
			if id := s.selID(); id != "" {
				return s, s.act(id, "start")
			}
		case "x":
			if id := s.selID(); id != "" {
				return s, s.act(id, "stop")
			}
		case "R":
			if id := s.selID(); id != "" {
				return s, s.act(id, "restart")
			}
		}
	}
	return s, nil
}

func (s *Services) selID() string {
	if s.cursor < 0 || s.cursor >= len(s.svcs) {
		return ""
	}
	return s.svcs[s.cursor].ID
}

func (s *Services) Render(width, height int) string {
	t := s.ctx.Theme
	if s.svcs == nil && s.err == nil {
		return Card(t, width, " ⌬  Services ", "\n  Loading…\n", true)
	}
	if s.err != nil && s.svcs == nil {
		return Card(t, width, " ⌬  Services ", "\n"+errLine(t, s.err)+"\n", true)
	}
	cols := []Column{
		{Header: "ID", Width: 20},
		{Header: "DISPLAY NAME", Width: 0},
		{Header: "ENABLED", Width: 10, Align: lipgloss.Center},
		{Header: "STATUS", Width: 12, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0, len(s.svcs))
	for _, sv := range s.svcs {
		enabled := "no"
		if sv.Enabled {
			enabled = "yes"
		}
		status := "stopped"
		if sv.Running {
			status = "running"
		}
		rows = append(rows, []Cell{
			Plain(sv.ID),
			Plain(sv.DisplayName),
			Plain(enabled),
			Styled(" "+status+" ", t.HealthStyle(status)),
		})
	}
	body := "\n" + Table(t, width-4, height-5, cols, rows, s.cursor) + "\n"
	if s.flash != "" {
		body += lipgloss.NewStyle().Foreground(t.Muted).Render("  "+s.flash) + "\n"
	}
	return Card(t, width, " ⌬  Services — [s]tart [x]stop [R]estart ", body, true)
}
