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

// Packages lists installed DSM packages with start/stop actions.
type Packages struct {
	ctx     Ctx
	pkgs    []dsm.Package
	err     error
	cursor  int
	pending map[string]string // id → action in flight (for the UI hint)
	flash   string
}

type packagesMsg struct {
	P   []dsm.Package
	Err error
}
type pkgActionMsg struct {
	ID, Action string
	Err        error
}

func NewPackages(c Ctx) tui.View {
	return &Packages{ctx: c, pending: map[string]string{}}
}

func (p *Packages) Name() string                   { return "packages" }
func (p *Packages) Title() string                  { return "Packages" }
func (p *Packages) Icon() string                   { return "▣" }
func (p *Packages) RefreshInterval() time.Duration { return 20 * time.Second }
func (p *Packages) Bindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start")),
		key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop")),
		key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "restart")),
	}
}

func (p *Packages) Init() tea.Cmd { return p.fetch() }

func (p *Packages) fetch() tea.Cmd {
	c := p.ctx.Client
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) ([]dsm.Package, error) { return c.Packages(ctx) },
		func(v []dsm.Package, err error) tea.Msg { return packagesMsg{P: v, Err: err} },
	)
}

func (p *Packages) act(id, action string) tea.Cmd {
	c := p.ctx.Client
	p.pending[id] = action
	return tui.Fetch(20*time.Second,
		func(ctx context.Context) (struct{}, error) { return struct{}{}, c.PackageControl(ctx, id, action) },
		func(_ struct{}, err error) tea.Msg { return pkgActionMsg{ID: id, Action: action, Err: err} },
	)
}

func (p *Packages) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	switch m := msg.(type) {
	case tui.TickMsg:
		return p, p.fetch()
	case packagesMsg:
		p.pkgs, p.err = m.P, m.Err
		return p, nil
	case pkgActionMsg:
		delete(p.pending, m.ID)
		if m.Err != nil {
			p.flash = m.Action + " failed: " + m.Err.Error()
		} else {
			p.flash = m.Action + " ok"
		}
		return p, p.fetch()
	case tea.KeyMsg:
		switch m.String() {
		case "r":
			return p, p.fetch()
		case "j", "down":
			if p.cursor < len(p.pkgs)-1 {
				p.cursor++
			}
		case "k", "up":
			if p.cursor > 0 {
				p.cursor--
			}
		case "s":
			if id := p.selectedID(); id != "" {
				return p, p.act(id, "start")
			}
		case "x":
			if id := p.selectedID(); id != "" {
				return p, p.act(id, "stop")
			}
		case "R":
			if id := p.selectedID(); id != "" {
				return p, p.act(id, "restart")
			}
		}
	}
	return p, nil
}

func (p *Packages) selectedID() string {
	if p.cursor < 0 || p.cursor >= len(p.pkgs) {
		return ""
	}
	return p.pkgs[p.cursor].ID
}

func (p *Packages) Render(width, height int) string {
	t := p.ctx.Theme
	if p.pkgs == nil && p.err == nil {
		return Card(t, width, " ▣  Packages ", "\n  Loading…\n", true)
	}
	if p.err != nil && p.pkgs == nil {
		return Card(t, width, " ▣  Packages ", "\n"+errLine(t, p.err)+"\n", true)
	}
	cols := []Column{
		{Header: "NAME", Width: 28},
		{Header: "VERSION", Width: 16},
		{Header: "MAINTAINER", Width: 0},
		{Header: "STATUS", Width: 12, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0, len(p.pkgs))
	for _, pk := range p.pkgs {
		status := pk.Status
		if act, ok := p.pending[pk.ID]; ok {
			status = act + "…"
		}
		rows = append(rows, []Cell{
			Plain(pk.Name),
			Plain(pk.Version),
			Plain(pk.Maintainer),
			Styled(" "+status+" ", t.HealthStyle(status)),
		})
	}
	body := "\n" + Table(t, width-4, height-5, cols, rows, p.cursor) + "\n"
	if p.flash != "" {
		body += lipgloss.NewStyle().Foreground(t.Muted).Render("  "+p.flash) + "\n"
	}
	title := " ▣  Packages — [s]tart [x]stop [R]estart "
	return Card(t, width, title, body, true)
}
