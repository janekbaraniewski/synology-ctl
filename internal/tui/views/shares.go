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
	listBase
	ctx     Ctx
	shares  []dsm.Share
	err     error
	detail2 *dsm.Share
}

type sharesMsg struct {
	S   []dsm.Share
	Err error
}

func NewShares(c Ctx) tui.View {
	s := &Shares{ctx: c}
	s.initBase(c)
	return s
}

func (s *Shares) Name() string                   { return "shares" }
func (s *Shares) Title() string                  { return "Shares" }
func (s *Shares) Icon() string                   { return "▦" }
func (s *Shares) RefreshInterval() time.Duration { return 30 * time.Second }
func (s *Shares) Bindings() []key.Binding        { return BaseBindings() }
func (s *Shares) Init() tea.Cmd                  { return s.fetch() }

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

func (s *Shares) visible() []dsm.Share {
	if s.FilterValue() == "" {
		return s.shares
	}
	out := make([]dsm.Share, 0, len(s.shares))
	for _, sh := range s.shares {
		if s.FilterMatch(sh.Name, sh.Path, sh.Desc) {
			out = append(out, sh)
		}
	}
	return out
}

func (s *Shares) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	if s.detail2 != nil {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc", "q":
				s.detail2 = nil
				return s, nil
			}
		}
		return s, nil
	}
	rows := s.visible()
	if cmd, handled := s.HandleKey(msg, len(rows)); handled {
		return s, cmd
	}
	if s.IsEnter(msg) && len(rows) > 0 {
		picked := rows[s.Cursor()]
		s.detail2 = &picked
		return s, nil
	}
	switch m := msg.(type) {
	case tui.TickMsg:
		return s, s.fetch()
	case sharesMsg:
		s.shares, s.err = m.S, m.Err
		s.ClampCursor(len(s.visible()))
		return s, nil
	case tea.KeyMsg:
		if m.String() == "r" {
			return s, s.fetch()
		}
	}
	return s, nil
}

func (s *Shares) Render(width, height int) string {
	t := s.ctx.Theme
	if s.detail2 != nil {
		return renderShareDetail(t, width, height, *s.detail2)
	}
	if s.shares == nil && s.err == nil {
		return Card(t, width, " ▦  Shares ", "\n  Loading…\n", true)
	}
	if s.err != nil && s.shares == nil {
		return Card(t, width, " ▦  Shares ", "\n"+errLine(t, s.err)+"\n", true)
	}
	cols := []Column{
		{Header: "NAME", Width: 22},
		{Header: "PATH", Width: 0},
		{Header: "ENC", Width: 6, Align: lipgloss.Center},
		{Header: "RECYCLE", Width: 9, Align: lipgloss.Center},
		{Header: "HIDDEN", Width: 8, Align: lipgloss.Center},
		{Header: "QUOTA", Width: 16, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0)
	for _, sh := range s.visible() {
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
		enc := "—"
		if sh.IsEncrypted() {
			enc = "yes"
		}
		rows = append(rows, []Cell{
			Plain(sh.Name),
			Plain(sh.Path),
			Plain(enc),
			Plain(recycle),
			Plain(hidden),
			Plain(quota),
		})
	}
	footerH := 1
	if f := s.FilterFooter(t); f != "" {
		footerH = 2
	}
	body := "\n" + Table(t, width-4, height-3-footerH, cols, rows, s.Cursor()) + "\n"
	if f := s.FilterFooter(t); f != "" {
		body += f + "\n"
	}
	return Card(t, width, " ▦  Shares — ⏎ details · / filter ", body, true)
}
