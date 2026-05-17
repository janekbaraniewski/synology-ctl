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
	ctx    Ctx
	users  []dsm.User
	err    error
	cursor int
}

type usersMsg struct {
	U   []dsm.User
	Err error
}

func NewUsers(c Ctx) tui.View { return &Users{ctx: c} }

func (u *Users) Name() string                   { return "users" }
func (u *Users) Title() string                  { return "Users" }
func (u *Users) Icon() string                   { return "◐" }
func (u *Users) RefreshInterval() time.Duration { return 60 * time.Second }
func (u *Users) Bindings() []key.Binding        { return nil }

func (u *Users) Init() tea.Cmd { return u.fetch() }

func (u *Users) fetch() tea.Cmd {
	c := u.ctx.Client
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) ([]dsm.User, error) { return c.Users(ctx) },
		func(v []dsm.User, err error) tea.Msg { return usersMsg{U: v, Err: err} },
	)
}

func (u *Users) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	switch m := msg.(type) {
	case tui.TickMsg:
		return u, u.fetch()
	case usersMsg:
		u.users, u.err = m.U, m.Err
		return u, nil
	case tea.KeyMsg:
		switch m.String() {
		case "r":
			return u, u.fetch()
		case "j", "down":
			if u.cursor < len(u.users)-1 {
				u.cursor++
			}
		case "k", "up":
			if u.cursor > 0 {
				u.cursor--
			}
		}
	}
	return u, nil
}

func (u *Users) Render(width, height int) string {
	t := u.ctx.Theme
	if u.users == nil && u.err == nil {
		return Card(t, width, " ◐  Users ", "\n  Loading…\n", true)
	}
	if u.err != nil && u.users == nil {
		return Card(t, width, " ◐  Users ", "\n"+errLine(t, u.err)+"\n", true)
	}
	cols := []Column{
		{Header: "NAME", Width: 20},
		{Header: "UID", Width: 8, Align: lipgloss.Right},
		{Header: "DESCRIPTION", Width: 0},
		{Header: "EMAIL", Width: 28},
		{Header: "STATUS", Width: 12, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0, len(u.users))
	for _, x := range u.users {
		status := x.Expired
		if status == "" {
			status = "normal"
		}
		rows = append(rows, []Cell{
			Plain(x.Name),
			Plain(itoaInt(x.UID)),
			Plain(x.Description),
			Plain(x.Email),
			Styled(" "+status+" ", t.HealthStyle(status)),
		})
	}
	body := "\n" + Table(t, width-4, height-4, cols, rows, u.cursor) + "\n"
	return Card(t, width, " ◐  Users ", body, true)
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
