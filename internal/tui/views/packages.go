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
	listBase
	ctx     Ctx
	pkgs    []dsm.Package
	err     error
	pending map[string]string // id → action in flight
	flash   string
	detail2 *dsm.Package
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
	p := &Packages{ctx: c, pending: map[string]string{}}
	p.initBase(c)
	return p
}

func (p *Packages) Name() string                   { return "packages" }
func (p *Packages) Title() string                  { return "Packages" }
func (p *Packages) Icon() string                   { return "▣" }
func (p *Packages) RefreshInterval() time.Duration { return 20 * time.Second }
func (p *Packages) Bindings() []key.Binding {
	return append(BaseBindings(),
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start")),
		key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop")),
		key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "restart")),
	)
}
func (p *Packages) Init() tea.Cmd { return p.fetch() }

func (p *Packages) fetch() tea.Cmd {
	c := p.ctx.Client
	if c == nil {
		return nil
	}
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

func (p *Packages) visible() []dsm.Package {
	if p.FilterValue() == "" {
		return p.pkgs
	}
	out := make([]dsm.Package, 0)
	for _, pk := range p.pkgs {
		if p.FilterMatch(pk.ID, pk.Name, pk.Maintainer, pk.Status, pk.Version) {
			out = append(out, pk)
		}
	}
	return out
}

func (p *Packages) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	if p.detail2 != nil {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc", "q":
				p.detail2 = nil
				return p, nil
			}
		}
		return p, nil
	}
	rows := p.visible()
	if cmd, handled := p.HandleKey(msg, len(rows)); handled {
		return p, cmd
	}
	if p.IsEnter(msg) && len(rows) > 0 {
		picked := rows[p.Cursor()]
		p.detail2 = &picked
		return p, nil
	}
	switch m := msg.(type) {
	case tui.TickMsg:
		return p, p.fetch()
	case packagesMsg:
		p.pkgs, p.err = m.P, m.Err
		p.ClampCursor(len(p.visible()))
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
	rows := p.visible()
	if p.Cursor() < 0 || p.Cursor() >= len(rows) {
		return ""
	}
	return rows[p.Cursor()].ID
}

func (p *Packages) Render(width, height int) string {
	t := p.ctx.Theme
	if p.detail2 != nil {
		return renderPackageDetail(t, width, *p.detail2)
	}
	_ = height
	if p.pkgs == nil && p.err == nil {
		return Card(t, width, " ▣  Packages ", "\n  Loading…\n", true)
	}
	if p.err != nil && p.pkgs == nil {
		return Card(t, width, " ▣  Packages ", "\n"+errLine(t, p.err)+"\n", true)
	}
	cols := []Column{
		{Header: "NAME", Width: 28},
		{Header: "VERSION", Width: 18},
		{Header: "MAINTAINER", Width: 0},
		{Header: "STATUS", Width: 14, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0)
	for _, pk := range p.visible() {
		status := pk.Status
		if act, ok := p.pending[pk.ID]; ok {
			status = act + "…"
		}
		rows = append(rows, []Cell{
			Plain(pk.Name),
			Plain(pk.Version),
			Plain(pk.Maintainer),
			Styled(status, t.HealthStyle(status)),
		})
	}
	footerH := 2
	if f := p.FilterFooter(t); f != "" {
		footerH = 3
	}
	body := "\n" + Table(t, width-4, height-3-footerH, cols, rows, p.Cursor()) + "\n"
	if p.flash != "" {
		body += lipgloss.NewStyle().Foreground(t.Muted).Render("  "+p.flash) + "\n"
	}
	if f := p.FilterFooter(t); f != "" {
		body += f + "\n"
	}
	return Card(t, width, " ▣  Packages — ⏎ details · / filter · [s]tart [x]stop [R]estart ", body, true)
}
