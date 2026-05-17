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

// Users lists local DSM accounts.
type Users struct {
	listBase
	ctx     Ctx
	users   []dsm.User
	err     error
	detail2 *dsm.User
}

type usersMsg struct {
	U   []dsm.User
	Err error
}

func NewUsers(c Ctx) tui.View {
	u := &Users{ctx: c}
	u.initBase(c)
	return u
}

func (u *Users) Name() string                   { return "users" }
func (u *Users) Title() string                  { return "Users" }
func (u *Users) Icon() string                   { return "◐" }
func (u *Users) RefreshInterval() time.Duration { return 60 * time.Second }
func (u *Users) Bindings() []key.Binding        { return BaseBindings() }
func (u *Users) Init() tea.Cmd                  { return u.fetch() }

func (u *Users) fetch() tea.Cmd {
	c := u.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) ([]dsm.User, error) { return c.Users(ctx) },
		func(v []dsm.User, err error) tea.Msg { return usersMsg{U: v, Err: err} },
	)
}

func (u *Users) visible() []dsm.User {
	if u.FilterValue() == "" {
		return u.users
	}
	out := make([]dsm.User, 0)
	for _, x := range u.users {
		if u.FilterMatch(x.Name, x.Description, x.Email) {
			out = append(out, x)
		}
	}
	return out
}

func (u *Users) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	if u.detail2 != nil {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc", "q":
				u.detail2 = nil
				return u, nil
			}
		}
		return u, nil
	}
	rows := u.visible()
	if cmd, handled := u.HandleKey(msg, len(rows)); handled {
		return u, cmd
	}
	if u.IsEnter(msg) && len(rows) > 0 {
		picked := rows[u.Cursor()]
		u.detail2 = &picked
		return u, nil
	}
	switch m := msg.(type) {
	case tui.TickMsg:
		return u, u.fetch()
	case usersMsg:
		u.users, u.err = m.U, m.Err
		u.ClampCursor(len(u.visible()))
		return u, nil
	case tea.KeyMsg:
		if m.String() == "r" {
			return u, u.fetch()
		}
	}
	return u, nil
}

func (u *Users) Render(width, height int) string {
	t := u.ctx.Theme
	if u.detail2 != nil {
		return renderUserDetail(t, width, *u.detail2)
	}
	_ = height
	if u.users == nil && u.err == nil {
		return Card(t, width, " ◐  Users ", "\n  Loading…\n", true)
	}
	if u.err != nil && u.users == nil {
		return Card(t, width, " ◐  Users ", "\n"+errLine(t, u.err)+"\n", true)
	}
	cols := []Column{
		{Header: "NAME", Width: 22},
		{Header: "UID", Width: 8, Align: lipgloss.Right},
		{Header: "DESCRIPTION", Width: 0},
		{Header: "EMAIL", Width: 28},
		{Header: "STATUS", Width: 12, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0)
	for _, x := range u.visible() {
		status := x.Expired
		if status == "" {
			status = "normal"
		}
		rows = append(rows, []Cell{
			Plain(x.Name),
			Plain(itoaInt(x.UID)),
			Plain(x.Description),
			Plain(x.Email),
			Styled(status, t.HealthStyle(status)),
		})
	}
	footerH := 1
	if f := u.FilterFooter(t); f != "" {
		footerH = 2
	}
	body := "\n" + Table(t, width-4, height-3-footerH, cols, rows, u.Cursor()) + "\n"
	if f := u.FilterFooter(t); f != "" {
		body += f + "\n"
	}
	return Card(t, width, " ◐  Users — ⏎ details · / filter ", body, true)
}

func itoaInt(i int) string {
	if i == 0 {
		return "0"
	}
	const digits = "0123456789"
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = digits[i%10]
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
