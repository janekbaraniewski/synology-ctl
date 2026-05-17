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
	listBase
	ctx     Ctx
	ifs     []dsm.NetworkInterface
	err     error
	detail2 *dsm.NetworkInterface
}

type netMsg struct {
	I   []dsm.NetworkInterface
	Err error
}

func NewNetwork(c Ctx) tui.View {
	n := &Network{ctx: c}
	n.initBase(c)
	return n
}

func (n *Network) Name() string                   { return "network" }
func (n *Network) Title() string                  { return "Network" }
func (n *Network) Icon() string                   { return "⇄" }
func (n *Network) RefreshInterval() time.Duration { return 30 * time.Second }
func (n *Network) Bindings() []key.Binding        { return BaseBindings() }
func (n *Network) Init() tea.Cmd                  { return n.fetch() }

func (n *Network) fetch() tea.Cmd {
	c := n.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) ([]dsm.NetworkInterface, error) { return c.NetworkInterfaces(ctx) },
		func(v []dsm.NetworkInterface, err error) tea.Msg { return netMsg{I: v, Err: err} },
	)
}

func (n *Network) visible() []dsm.NetworkInterface {
	if n.FilterValue() == "" {
		return n.ifs
	}
	out := make([]dsm.NetworkInterface, 0)
	for _, ni := range n.ifs {
		if n.FilterMatch(ni.IFName, ni.Type, ni.IP, ni.Gateway, ni.MAC, ni.Status) {
			out = append(out, ni)
		}
	}
	return out
}

func (n *Network) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	if n.detail2 != nil {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc", "q":
				n.detail2 = nil
				return n, nil
			}
		}
		return n, nil
	}
	rows := n.visible()
	if cmd, handled := n.HandleKey(msg, len(rows)); handled {
		return n, cmd
	}
	if n.IsEnter(msg) && len(rows) > 0 {
		picked := rows[n.Cursor()]
		n.detail2 = &picked
		return n, nil
	}
	switch m := msg.(type) {
	case tui.TickMsg:
		return n, n.fetch()
	case netMsg:
		n.ifs, n.err = m.I, m.Err
		n.ClampCursor(len(n.visible()))
		return n, nil
	case tea.KeyMsg:
		if m.String() == "r" {
			return n, n.fetch()
		}
	}
	return n, nil
}

func (n *Network) Render(width, height int) string {
	t := n.ctx.Theme
	if n.detail2 != nil {
		return renderNetworkDetail(t, width, *n.detail2)
	}
	_ = height
	if n.ifs == nil && n.err == nil {
		return Card(t, width, " ⇄  Network ", "\n  Loading…\n", true)
	}
	if n.err != nil && n.ifs == nil {
		return Card(t, width, " ⇄  Network ", "\n"+errLine(t, n.err)+"\n", true)
	}
	cols := []Column{
		{Header: "INTERFACE", Width: 12},
		{Header: "TYPE", Width: 10},
		{Header: "IPv4", Width: 22},
		{Header: "GATEWAY", Width: 16},
		{Header: "SPEED", Width: 14, Align: lipgloss.Right},
		{Header: "STATUS", Width: 0, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0)
	for _, ni := range n.visible() {
		ip := ni.IP
		if ip != "" && ni.Mask != "" {
			ip += "/" + ni.Mask
		}
		speed := "—"
		if ni.Speed > 0 {
			speed = itoaInt(ni.Speed) + " Mbit/s"
		}
		rows = append(rows, []Cell{
			Plain(ni.IFName),
			Plain(ni.Type),
			Plain(ip),
			Plain(ni.Gateway),
			Plain(speed),
			Styled(ni.Status, t.HealthStyle(ni.Status)),
		})
	}
	footerH := 1
	if f := n.FilterFooter(t); f != "" {
		footerH = 2
	}
	body := "\n" + Table(t, width-4, height-3-footerH, cols, rows, n.Cursor()) + "\n"
	if f := n.FilterFooter(t); f != "" {
		body += f + "\n"
	}
	return Card(t, width, " ⇄  Network — ⏎ details · / filter ", body, true)
}
