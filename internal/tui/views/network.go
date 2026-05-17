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

// Network shows interface state, addressing and link speed.
type Network struct {
	ctx    Ctx
	ifs    []dsm.NetworkInterface
	err    error
	cursor int
}

type netMsg struct {
	I   []dsm.NetworkInterface
	Err error
}

func NewNetwork(c Ctx) tui.View { return &Network{ctx: c} }

func (n *Network) Name() string                   { return "network" }
func (n *Network) Title() string                  { return "Network" }
func (n *Network) Icon() string                   { return "⇄" }
func (n *Network) RefreshInterval() time.Duration { return 30 * time.Second }
func (n *Network) Bindings() []key.Binding        { return nil }

func (n *Network) Init() tea.Cmd { return n.fetch() }

func (n *Network) fetch() tea.Cmd {
	c := n.ctx.Client
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) ([]dsm.NetworkInterface, error) { return c.NetworkInterfaces(ctx) },
		func(v []dsm.NetworkInterface, err error) tea.Msg { return netMsg{I: v, Err: err} },
	)
}

func (n *Network) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	switch m := msg.(type) {
	case tui.TickMsg:
		return n, n.fetch()
	case netMsg:
		n.ifs, n.err = m.I, m.Err
		return n, nil
	case tea.KeyMsg:
		switch m.String() {
		case "r":
			return n, n.fetch()
		case "j", "down":
			if n.cursor < len(n.ifs)-1 {
				n.cursor++
			}
		case "k", "up":
			if n.cursor > 0 {
				n.cursor--
			}
		}
	}
	return n, nil
}

func (n *Network) Render(width, height int) string {
	t := n.ctx.Theme
	if n.ifs == nil && n.err == nil {
		return Card(t, width, " ⇄  Network ", "\n  Loading…\n", true)
	}
	if n.err != nil && n.ifs == nil {
		return Card(t, width, " ⇄  Network ", "\n"+errLine(t, n.err)+"\n", true)
	}
	cols := []Column{
		{Header: "INTERFACE", Width: 14},
		{Header: "MAC", Width: 19},
		{Header: "IPv4", Width: 18},
		{Header: "GATEWAY", Width: 16},
		{Header: "SPEED", Width: 12},
		{Header: "MTU", Width: 6, Align: lipgloss.Right},
		{Header: "STATUS", Width: 0, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0, len(n.ifs))
	for _, ni := range n.ifs {
		ip := ni.IPv4Address
		if ip != "" && ni.IPv4Mask != "" {
			ip += "/" + ni.IPv4Mask
		}
		rows = append(rows, []Cell{
			Plain(ni.IFName),
			Plain(ni.MAC),
			Plain(ip),
			Plain(ni.IPv4Gateway),
			Plain(ni.LinkSpeed),
			Plain(itoaInt(ni.MTU)),
			Styled(" "+ni.Status+" ", t.HealthStyle(ni.Status)),
		})
	}
	body := "\n" + Table(t, width-4, height-4, cols, rows, n.cursor) + "\n"
	return Card(t, width, " ⇄  Network ", body, true)
}
