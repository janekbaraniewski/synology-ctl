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

// Shares is the shared-folders view.
type Shares struct {
	ctx    Ctx
	shares []dsm.Share
	err    error
	cursor int
}

type sharesMsg struct {
	S   []dsm.Share
	Err error
}

func NewShares(c Ctx) tui.View { return &Shares{ctx: c} }

func (s *Shares) Name() string                   { return "shares" }
func (s *Shares) Title() string                  { return "Shares" }
func (s *Shares) Icon() string                   { return "▦" }
func (s *Shares) RefreshInterval() time.Duration { return 30 * time.Second }
func (s *Shares) Bindings() []key.Binding        { return nil }

func (s *Shares) Init() tea.Cmd { return s.fetch() }

func (s *Shares) fetch() tea.Cmd {
	c := s.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) ([]dsm.Share, error) { return c.Shares(ctx) },
		func(v []dsm.Share, err error) tea.Msg { return sharesMsg{S: v, Err: err} },
	)
}

func (s *Shares) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	switch m := msg.(type) {
	case tui.TickMsg:
		return s, s.fetch()
	case sharesMsg:
		s.shares, s.err = m.S, m.Err
		return s, nil
	case tea.KeyMsg:
		switch m.String() {
		case "r":
			return s, s.fetch()
		case "j", "down":
			if s.cursor < len(s.shares)-1 {
				s.cursor++
			}
		case "k", "up":
			if s.cursor > 0 {
				s.cursor--
			}
		case "g":
			s.cursor = 0
		case "G":
			s.cursor = len(s.shares) - 1
		}
	}
	return s, nil
}

func (s *Shares) Render(width, height int) string {
	t := s.ctx.Theme
	if s.shares == nil && s.err == nil {
		return Card(t, width, " ▦  Shares ", "\n  Loading…\n", true)
	}
	if s.err != nil && s.shares == nil {
		return Card(t, width, " ▦  Shares ", "\n"+errLine(t, s.err)+"\n", true)
	}
	cols := []Column{
		{Header: "NAME", Width: 20},
		{Header: "PATH", Width: 0},
		{Header: "RECYCLE", Width: 9, Align: lipgloss.Center},
		{Header: "HIDDEN", Width: 8, Align: lipgloss.Center},
		{Header: "QUOTA USED", Width: 14, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0, len(s.shares))
	for _, sh := range s.shares {
		quota := "—"
		if sh.ShareQuota > 0 {
			quota = HumanBytes(uint64(sh.ShareQuotaUsed)*1024*1024) + " / " + HumanBytes(uint64(sh.ShareQuota)*1024*1024)
		}
		recycle := "—"
		if sh.EnableRecycle {
			recycle = "yes"
		}
		hidden := "—"
		if sh.Hidden {
			hidden = "yes"
		}
		rows = append(rows, []Cell{
			Plain(sh.Name),
			Plain(sh.Path),
			Plain(recycle),
			Plain(hidden),
			Plain(quota),
		})
	}
	body := "\n" + Table(t, width-4, height-4, cols, rows, s.cursor) + "\n"
	return Card(t, width, " ▦  Shares ", body, true)
}
