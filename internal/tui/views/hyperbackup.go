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

// HyperBackupView lists Hyper Backup tasks. Refresh is 60s — backup
// state moves on a human cadence, so spamming the API would be silly.

type backupTasksMsg struct {
	T   []dsm.BackupTask
	Err error
}

type HyperBackupView struct {
	ctx Ctx

	tasks   []dsm.BackupTask
	tasksErr error

	cursor int
	filter Filter
	loaded bool

	detail *dsm.BackupTask
}

func NewHyperBackup(c Ctx) tui.View { return &HyperBackupView{ctx: c} }

func (v *HyperBackupView) Name() string                   { return "backup" }
func (v *HyperBackupView) Title() string                  { return "Hyper Backup" }
func (v *HyperBackupView) Icon() string                   { return "⏏" }
func (v *HyperBackupView) RefreshInterval() time.Duration { return 60 * time.Second }
func (v *HyperBackupView) Bindings() []key.Binding        { return BaseBindings() }

func (v *HyperBackupView) Init() tea.Cmd { return v.fetchTasks() }

func (v *HyperBackupView) fetchTasks() tea.Cmd {
	c := v.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(15*time.Second,
		func(ctx context.Context) ([]dsm.BackupTask, error) { return c.BackupTasks(ctx) },
		func(x []dsm.BackupTask, err error) tea.Msg { return backupTasksMsg{T: x, Err: err} },
	)
}

func (v *HyperBackupView) filtered() []dsm.BackupTask {
	if v.filter.Value() == "" {
		return v.tasks
	}
	out := make([]dsm.BackupTask, 0, len(v.tasks))
	for _, t := range v.tasks {
		if MatchesAll(v.filter.Value(), t.Name, t.Type, t.RepoTarget, t.RepoHost, t.RepoPath, t.Status, t.LastStatus) {
			out = append(out, t)
		}
	}
	return out
}

func (v *HyperBackupView) Update(msg tea.Msg) (tui.View, tea.Cmd) {
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
		return v, v.fetchTasks()
	case backupTasksMsg:
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
			return v, v.fetchTasks()
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

func (v *HyperBackupView) clampCursor() {
	n := len(v.filtered())
	if v.cursor >= n {
		v.cursor = n - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}

func (v *HyperBackupView) Render(width, height int) string {
	t := v.ctx.Theme
	if v.detail != nil {
		return renderBackupTaskDetail(t, width, *v.detail)
	}

	if v.loaded && len(v.tasks) == 0 && v.tasksErr == nil {
		return fitOrScroll(emptyStateCard(t, width,
			"⏏  Hyper Backup",
			"Hyper Backup is not installed, or no backup tasks have been configured.",
			"Install Hyper Backup and add a task to see backup state, repo targets, and last-run results here."), height)
	}

	tasks := v.filtered()
	var parts []string
	parts = append(parts, sectionHeader(t, width, "Backup tasks", len(tasks), v.tasksErr))
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

func (v *HyperBackupView) renderRow(tk dsm.BackupTask, highlight bool) string {
	t := v.ctx.Theme
	mu := lipgloss.NewStyle().Foreground(t.Muted)
	text := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	lastRun := "—"
	if tk.LastRun > 0 {
		lastRun = time.Unix(tk.LastRun, 0).Format("2006-01-02 15:04")
	}
	repo := tk.RepoTarget
	if tk.RepoHost != "" {
		repo += " · " + tk.RepoHost
	}
	status := tk.LastStatus
	if status == "" {
		status = tk.Status
	}
	if status == "" {
		status = tk.State
	}
	if status == "" {
		status = "—"
	}
	return lipgloss.JoinHorizontal(lipgloss.Center,
		caretGlyph(t, highlight), " ",
		padRight(text.Render(tk.Name), 22), " ",
		padRight(mu.Render(clipTo(repo, 24)), 24), " ",
		padRight(mu.Render(lastRun), 18), " ",
		padLeft(mu.Render(HumanBytes(uint64(tk.TotalSize))), 10), " ",
		t.HealthStyle(status).Render(status),
	)
}

func renderBackupTaskDetail(t tui.Theme, width int, tk dsm.BackupTask) string {
	if width < 60 {
		width = 60
	}
	status := tk.LastStatus
	if status == "" {
		status = tk.Status
	}
	if status == "" {
		status = "unknown"
	}
	lastRun, nextRun := "—", "—"
	if tk.LastRun > 0 {
		lastRun = time.Unix(tk.LastRun, 0).Format("2006-01-02 15:04")
	}
	if tk.NextRun > 0 {
		nextRun = time.Unix(tk.NextRun, 0).Format("2006-01-02 15:04")
	}
	duration := "—"
	if tk.LastDuration > 0 {
		duration = (time.Duration(tk.LastDuration) * time.Second).String()
	}
	parts := []string{
		hero(t, width, "⏏", tk.Name, status, tk.Type),
		propsCard(t, width, " Properties ", [][2]string{
			{"Task ID", fmt.Sprintf("%d", tk.TaskID)},
			{"Name", tk.Name},
			{"Type", tk.Type},
			{"Repository", tk.RepoTarget},
			{"Repo host", tk.RepoHost},
			{"Repo path", tk.RepoPath},
			{"Schedule", tk.Schedule},
			{"Last run", lastRun},
			{"Last status", tk.LastStatus},
			{"Last duration", duration},
			{"Next run", nextRun},
			{"Total size", HumanBytes(uint64(tk.TotalSize))},
			{"Used size", HumanBytes(uint64(tk.UsedSize))},
			{"Versions", fmt.Sprintf("%d", tk.Versions)},
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
		chip("encrypted", tk.Encrypted.Bool()),
	}))
	parts = append(parts, noteCard(t, width, "  esc to go back · Hyper Backup actions aren't wired up yet"))
	return strings.Join(parts, "\n")
}
