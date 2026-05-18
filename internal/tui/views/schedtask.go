package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/janbaraniewski/synology-ctl/internal/dsm"
	"github.com/janbaraniewski/synology-ctl/internal/tui"
)

// SchedTasksView lists DSM's Task Scheduler entries (scripts, S.M.A.R.T.
// tests, reboots…). 60s refresh — schedule changes are rare and we don't
// need to follow them tick-by-tick.

type schedTasksMsg struct {
	T   []dsm.ScheduledTask
	Err error
}

type SchedTasksView struct {
	ctx Ctx

	tasks    []dsm.ScheduledTask
	tasksErr error

	cursor int
	filter Filter
	loaded bool

	detail *dsm.ScheduledTask
}

func NewSchedTasks(c Ctx) tui.View { return &SchedTasksView{ctx: c} }

func (v *SchedTasksView) Name() string                   { return "tasks" }
func (v *SchedTasksView) Title() string                  { return "Scheduled Tasks" }
func (v *SchedTasksView) Icon() string                   { return "◷" }
func (v *SchedTasksView) RefreshInterval() time.Duration { return 60 * time.Second }
func (v *SchedTasksView) Bindings() []key.Binding        { return BaseBindings() }

func (v *SchedTasksView) Init() tea.Cmd { return v.fetch() }

func (v *SchedTasksView) fetch() tea.Cmd {
	c := v.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(10*time.Second,
		func(ctx context.Context) ([]dsm.ScheduledTask, error) { return c.ScheduledTasks(ctx) },
		func(x []dsm.ScheduledTask, err error) tea.Msg { return schedTasksMsg{T: x, Err: err} },
	)
}

func (v *SchedTasksView) filtered() []dsm.ScheduledTask {
	if v.filter.Value() == "" {
		return v.tasks
	}
	out := make([]dsm.ScheduledTask, 0, len(v.tasks))
	for _, t := range v.tasks {
		if MatchesAll(v.filter.Value(), t.Name, t.Type, t.Owner, t.Repeat, t.Action, t.LastRunResult) {
			out = append(out, t)
		}
	}
	return out
}

func (v *SchedTasksView) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	if v.detail != nil {
		if km, ok := msg.(tea.KeyMsg); ok && (km.String() == "esc" || km.String() == "q") {
			v.detail = nil
		}
		return v, nil
	}
	if v.filter.IsActive() {
		if v.filter.Update(msg) {
			return v, nil
		}
	}
	switch m := msg.(type) {
	case tui.TickMsg:
		return v, v.fetch()
	case schedTasksMsg:
		v.tasks, v.tasksErr = m.T, m.Err
		v.loaded = true
		v.clampCursor()
	case tea.KeyMsg:
		switch m.String() {
		case "j", "down":
			rows := v.filtered()
			if v.cursor < len(rows)-1 {
				v.cursor++
			}
		case "k", "up":
			if v.cursor > 0 {
				v.cursor--
			}
		case "g":
			v.cursor = 0
		case "G":
			v.cursor = max(len(v.filtered())-1, 0)
		case "/":
			v.filter.Open()
		case "esc":
			if v.filter.Value() != "" {
				v.filter.Clear()
				v.cursor = 0
			}
		case "r":
			return v, v.fetch()
		case "enter":
			rows := v.filtered()
			if v.cursor >= 0 && v.cursor < len(rows) {
				t := rows[v.cursor]
				v.detail = &t
			}
		}
	}
	return v, nil
}

func (v *SchedTasksView) clampCursor() {
	n := len(v.filtered())
	if v.cursor >= n {
		v.cursor = n - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}

func (v *SchedTasksView) Render(width, height int) string {
	t := v.ctx.Theme
	if v.detail != nil {
		return renderSchedTaskDetail(t, width, *v.detail)
	}

	if v.loaded && len(v.tasks) == 0 && v.tasksErr == nil {
		return fitOrScroll(emptyStateCard(t, width,
			"◷  Task Scheduler",
			"No scheduled tasks returned by DSM.",
			"Add a task in Control Panel → Task Scheduler to see it here."), height)
	}

	tasks := v.filtered()
	var parts []string
	parts = append(parts, sectionHeader(t, width, "Scheduled tasks", len(tasks), v.tasksErr))
	if !v.loaded {
		parts = append(parts, "  "+muted(t, "loading…"))
	} else if len(tasks) == 0 {
		parts = append(parts, "  "+muted(t, "(none matching)"))
	}
	for i, tk := range tasks {
		parts = append(parts, v.renderRow(tk, i == v.cursor))
	}
	parts = append(parts, "")
	parts = append(parts, lipgloss.NewStyle().Foreground(t.Muted).Render(
		"  ↑/↓ move · ⏎ details · / filter · esc clear · r refresh"))
	if fr := v.filter.Render(t); fr != "" {
		parts = append(parts, fr)
	}
	return fitOrScroll(strings.Join(parts, "\n"), height)
}

func (v *SchedTasksView) renderRow(tk dsm.ScheduledTask, highlight bool) string {
	t := v.ctx.Theme
	mu := lipgloss.NewStyle().Foreground(t.Muted)
	text := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	state := "disabled"
	if tk.Enable.Bool() {
		state = "enabled"
	}
	next := "—"
	if tk.NextTriggerTime > 0 {
		next = time.Unix(tk.NextTriggerTime, 0).Format("2006-01-02 15:04")
	}
	return lipgloss.JoinHorizontal(lipgloss.Center,
		caretGlyph(t, highlight), " ",
		padRight(text.Render(clipTo(tk.Name, 28)), 28), " ",
		padRight(mu.Render(tk.Type), 12), " ",
		padRight(mu.Render(next), 18), " ",
		padRight(mu.Render(tk.Owner), 14), " ",
		t.HealthStyle(state).Render(state),
	)
}

// Inspect implements tui.Inspector — scheduled tasks carry just enough
// detail (repeat schedule, last result) to benefit from a side pane
// without forcing the user into the full overlay.
func (v *SchedTasksView) Inspect(width, height int) string {
	_ = height
	t := v.ctx.Theme
	tasks := v.filtered()
	if v.cursor < 0 || v.cursor >= len(tasks) {
		return muted(t, "  (no selection)")
	}
	return renderSchedTaskInspect(t, width, tasks[v.cursor])
}

func renderSchedTaskDetail(t tui.Theme, width int, tk dsm.ScheduledTask) string {
	if width < 60 {
		width = 60
	}
	state := "disabled"
	if tk.Enable.Bool() {
		state = "enabled"
	}
	next, last := "—", "—"
	if tk.NextTriggerTime > 0 {
		next = time.Unix(tk.NextTriggerTime, 0).Format("2006-01-02 15:04")
	}
	if tk.LastRunTime > 0 {
		last = time.Unix(tk.LastRunTime, 0).Format("2006-01-02 15:04")
	}
	repeatTime := "—"
	if tk.RepeatHour > 0 || tk.RepeatMin > 0 {
		repeatTime = fmt.Sprintf("%02d:%02d", tk.RepeatHour, tk.RepeatMin)
	}
	parts := []string{
		hero(t, width, "◷", tk.Name, state, tk.Type),
		propsCard(t, width, " Properties ", [][2]string{
			{"ID", fmt.Sprintf("%d", tk.ID)},
			{"Name", tk.Name},
			{"Type", tk.Type},
			{"Owner", tk.Owner},
			{"Owner UID", fmt.Sprintf("%d", tk.OwnerUID)},
			{"Repeat", tk.Repeat},
			{"Time", repeatTime},
			{"Next trigger", next},
			{"Last run", last},
			{"Last result", tk.LastRunResult},
			{"Action", tk.Action},
		}),
	}
	chip := func(label string, on bool) string {
		st := t.HealthStyle("disabled")
		if on {
			st = t.HealthStyle("enabled")
		}
		return st.Render(" " + label + " ")
	}
	parts = append(parts, chipsCard(t, width, " Flags ", []string{
		chip("enabled", tk.Enable.Bool()),
		chip("runnable", tk.CanRun.Bool()),
		chip("editable", tk.CanEdit.Bool()),
	}))
	parts = append(parts, noteCard(t, width, "  esc to go back · run-now / edit aren't wired up yet"))
	return strings.Join(parts, "\n")
}

func renderSchedTaskInspect(t tui.Theme, width int, tk dsm.ScheduledTask) string {
	_ = width
	state := "disabled"
	if tk.Enable.Bool() {
		state = "enabled"
	}
	next, last := "—", "—"
	if tk.NextTriggerTime > 0 {
		next = time.Unix(tk.NextTriggerTime, 0).Format("2006-01-02 15:04")
	}
	if tk.LastRunTime > 0 {
		last = time.Unix(tk.LastRunTime, 0).Format("2006-01-02 15:04")
	}
	return strings.Join([]string{
		t.Title().Render(" Scheduled task "),
		lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render("  " + tk.Name),
		lipgloss.NewStyle().Foreground(t.Muted).Render("  " + tk.Type),
		"  " + t.HealthStyle(state).Render(state),
		"",
		muted(t, "  Owner       ") + tk.Owner,
		muted(t, "  Repeat      ") + tk.Repeat,
		muted(t, "  Next        ") + next,
		muted(t, "  Last run    ") + last,
		muted(t, "  Last result ") + tk.LastRunResult,
		"",
		muted(t, "  Action      ") + clipTo(tk.Action, 38),
	}, "\n")
}
