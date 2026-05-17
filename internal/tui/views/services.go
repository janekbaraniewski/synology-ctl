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

// Services lists DSM services with enable/disable controls.
type Services struct {
	listBase
	ctx   Ctx
	svcs  []dsm.Service
	err   error
	flash string
}

type servicesMsg struct {
	S   []dsm.Service
	Err error
}
type svcActionMsg struct {
	ID, Action string
	Err        error
}

func NewServices(c Ctx) tui.View {
	s := &Services{ctx: c}
	s.initBase(c)
	return s
}

func (s *Services) Name() string                   { return "services" }
func (s *Services) Title() string                  { return "Services" }
func (s *Services) Icon() string                   { return "⌬" }
func (s *Services) RefreshInterval() time.Duration { return 20 * time.Second }
func (s *Services) Bindings() []key.Binding {
	return append(BaseBindings(),
		key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "enable")),
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "disable")),
	)
}
func (s *Services) Init() tea.Cmd { return s.fetch() }

func (s *Services) fetch() tea.Cmd {
	c := s.ctx.Client
	if c == nil {
		return nil
	}
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

func (s *Services) visible() []dsm.Service {
	if s.FilterValue() == "" {
		return s.svcs
	}
	out := make([]dsm.Service, 0)
	for _, sv := range s.svcs {
		if s.FilterMatch(sv.ID, sv.DisplayName(), sv.EnableStatus) {
			out = append(out, sv)
		}
	}
	return out
}

func (s *Services) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	rows := s.visible()
	if cmd, handled := s.HandleKey(msg, len(rows)); handled {
		return s, cmd
	}
	if s.IsEnter(msg) && len(rows) > 0 {
		s.ShowDetail("Service "+rows[s.Cursor()].ID, rows[s.Cursor()])
		return s, nil
	}
	switch m := msg.(type) {
	case tui.TickMsg:
		return s, s.fetch()
	case servicesMsg:
		s.svcs, s.err = m.S, m.Err
		s.ClampCursor(len(s.visible()))
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
		case "e":
			if id := s.selID(); id != "" {
				return s, s.act(id, "enable")
			}
		case "d":
			if id := s.selID(); id != "" {
				return s, s.act(id, "disable")
			}
		}
	}
	return s, nil
}

func (s *Services) selID() string {
	rows := s.visible()
	if s.Cursor() < 0 || s.Cursor() >= len(rows) {
		return ""
	}
	return rows[s.Cursor()].ID
}

func (s *Services) Render(width, height int) string {
	t := s.ctx.Theme
	if s.DetailVisible() {
		return s.RenderDetail(width, height)
	}
	if s.svcs == nil && s.err == nil {
		return Card(t, width, " ⌬  Services ", "\n  Loading…\n", true)
	}
	if s.err != nil && s.svcs == nil {
		return Card(t, width, " ⌬  Services ", "\n"+errLine(t, s.err)+"\n", true)
	}
	cols := []Column{
		{Header: "ID", Width: 32},
		{Header: "DISPLAY NAME", Width: 0},
		{Header: "STATE", Width: 14, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0)
	for _, sv := range s.visible() {
		state := sv.EnableStatus
		switch state {
		case "enabled":
			state = "enabled"
		case "static":
			state = "always-on"
		case "disabled":
			state = "disabled"
		}
		rows = append(rows, []Cell{
			Plain(sv.ID),
			Plain(sv.DisplayName()),
			Styled(state, t.HealthStyle(state)),
		})
	}
	footerH := 2
	if f := s.FilterFooter(t); f != "" {
		footerH = 3
	}
	body := "\n" + Table(t, width-4, height-3-footerH, cols, rows, s.Cursor()) + "\n"
	if s.flash != "" {
		body += lipgloss.NewStyle().Foreground(t.Muted).Render("  "+s.flash) + "\n"
	}
	if f := s.FilterFooter(t); f != "" {
		body += f + "\n"
	}
	return Card(t, width, " ⌬  Services — ⏎ details · / filter · [e]nable [d]isable ", body, true)
}
