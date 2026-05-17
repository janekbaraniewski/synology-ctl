package views

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"

	"github.com/janbaraniewski/synology-ctl/internal/dsm"
	"github.com/janbaraniewski/synology-ctl/internal/tui"
)

// Files is a File Station browser — the user's escape hatch into the
// data living on the NAS. Enter on a folder navigates in, Backspace or
// Esc steps up, `/` filters the current directory.
//
// The view tracks one path at a time and re-fetches on navigation. It
// auto-loads SYNO.FileStation.List.list_share when started at the root.
type Files struct {
	listBase
	ctx Ctx

	// currentPath is empty when we're showing the roots (shares).
	currentPath string
	stack       []string // navigation history (for back)

	shares  []dsm.FileShare
	entries []dsm.FSEntry
	total   int
	err     error
}

type filesSharesMsg struct {
	S   []dsm.FileShare
	Err error
}
type filesListMsg struct {
	E     []dsm.FSEntry
	Total int
	Err   error
}

func NewFiles(c Ctx) tui.View {
	f := &Files{ctx: c}
	f.initBase(c)
	return f
}

func (f *Files) Name() string                   { return "files" }
func (f *Files) Title() string                  { return "Files" }
func (f *Files) Icon() string                   { return "🗁" }
func (f *Files) RefreshInterval() time.Duration { return 0 }
func (f *Files) Bindings() []key.Binding {
	return append(BaseBindings(),
		key.NewBinding(key.WithKeys("backspace"), key.WithHelp("⌫", "up")),
		key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "up a directory")),
	)
}

func (f *Files) Init() tea.Cmd { return f.fetchRoots() }

func (f *Files) fetchRoots() tea.Cmd {
	c := f.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) ([]dsm.FileShare, error) { return c.FileShares(ctx) },
		func(s []dsm.FileShare, err error) tea.Msg { return filesSharesMsg{S: s, Err: err} },
	)
}

func (f *Files) fetchDir(p string) tea.Cmd {
	c := f.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(15*time.Second,
		func(ctx context.Context) ([]dsm.FSEntry, error) {
			items, total, err := c.ListFiles(ctx, p, 0, 1000)
			f.total = total
			return items, err
		},
		func(e []dsm.FSEntry, err error) tea.Msg { return filesListMsg{E: e, Total: f.total, Err: err} },
	)
}

// NavigateTo replaces the current path and fetches its contents.
func (f *Files) NavigateTo(p string) tea.Cmd {
	if p != "" {
		f.stack = append(f.stack, f.currentPath)
	}
	f.currentPath = p
	f.ResetCursor()
	if p == "" {
		return f.fetchRoots()
	}
	return f.fetchDir(p)
}

func (f *Files) up() tea.Cmd {
	if f.currentPath == "" {
		return nil
	}
	if len(f.stack) > 0 {
		prev := f.stack[len(f.stack)-1]
		f.stack = f.stack[:len(f.stack)-1]
		f.currentPath = prev
	} else {
		parent := path.Dir(f.currentPath)
		if parent == "." || parent == "/" {
			f.currentPath = ""
		} else {
			f.currentPath = parent
		}
	}
	f.ResetCursor()
	if f.currentPath == "" {
		return f.fetchRoots()
	}
	return f.fetchDir(f.currentPath)
}

func (f *Files) visibleShares() []dsm.FileShare {
	if f.FilterValue() == "" {
		return f.shares
	}
	out := make([]dsm.FileShare, 0)
	for _, s := range f.shares {
		if f.FilterMatch(s.Name, s.Path) {
			out = append(out, s)
		}
	}
	return out
}

func (f *Files) visibleEntries() []dsm.FSEntry {
	if f.FilterValue() == "" {
		return f.entries
	}
	out := make([]dsm.FSEntry, 0)
	for _, e := range f.entries {
		if f.FilterMatch(e.Name, e.Path, e.Type) {
			out = append(out, e)
		}
	}
	return out
}

func (f *Files) rowCount() int {
	if f.currentPath == "" {
		return len(f.visibleShares())
	}
	return len(f.visibleEntries())
}

func (f *Files) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	if cmd, handled := f.HandleKey(msg, f.rowCount()); handled {
		return f, cmd
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "backspace", "h":
			return f, f.up()
		case "esc":
			if f.currentPath != "" {
				return f, f.up()
			}
		case "r":
			if f.currentPath == "" {
				return f, f.fetchRoots()
			}
			return f, f.fetchDir(f.currentPath)
		}
	}
	if f.IsEnter(msg) {
		if f.currentPath == "" {
			shares := f.visibleShares()
			if f.Cursor() < len(shares) {
				return f, f.NavigateTo(shares[f.Cursor()].Path)
			}
		} else {
			entries := f.visibleEntries()
			if f.Cursor() < len(entries) {
				e := entries[f.Cursor()]
				if e.IsDir {
					return f, f.NavigateTo(e.Path)
				}
			}
		}
	}
	switch m := msg.(type) {
	case filesSharesMsg:
		f.shares, f.err = m.S, m.Err
		f.ClampCursor(f.rowCount())
	case filesListMsg:
		f.entries, f.err = m.E, m.Err
		f.total = m.Total
		f.ClampCursor(f.rowCount())
	}
	return f, nil
}

func (f *Files) Render(width, height int) string {
	t := f.ctx.Theme
	title := f.titleString()
	if f.err != nil {
		return Card(t, width, title, "\n"+errLine(t, f.err)+"\n", true)
	}
	if f.currentPath == "" {
		return f.renderShares(width, height, title)
	}
	return f.renderEntries(width, height, title)
}

func (f *Files) titleString() string {
	if f.currentPath == "" {
		return " 🗁  Files — pick a shared folder · ⏎ open · / filter "
	}
	return " 🗁  " + f.currentPath + " — ⏎ open · ⌫ up · / filter "
}

func (f *Files) renderShares(width, height int, title string) string {
	t := f.ctx.Theme
	if f.shares == nil {
		return Card(t, width, title, "\n  Loading…\n", true)
	}
	cols := []Column{
		{Header: "NAME", Width: 24},
		{Header: "OWNER", Width: 14},
		{Header: "FREE", Width: 12, Align: lipgloss.Right},
		{Header: "TOTAL", Width: 12, Align: lipgloss.Right},
		{Header: "PATH", Width: 0},
		{Header: "ACCESS", Width: 10, Align: lipgloss.Right},
	}
	rows := make([][]Cell, 0)
	for _, s := range f.visibleShares() {
		owner := s.Add.Owner.User
		if owner == "" && s.Add.Owner.Group != "" {
			owner = "(group) " + s.Add.Owner.Group
		}
		access := "rw"
		if s.Add.VolStatus.ReadOnly {
			access = "ro"
		}
		free := "—"
		total := "—"
		if s.Add.VolStatus.TotalSpace > 0 {
			free = humanize.IBytes(uint64(s.Add.VolStatus.FreeSpace))
			total = humanize.IBytes(uint64(s.Add.VolStatus.TotalSpace))
		}
		rows = append(rows, []Cell{
			Plain(s.Name),
			Plain(owner),
			Plain(free),
			Plain(total),
			Plain(s.Path),
			Styled(access, t.HealthStyle("ok")),
		})
	}
	footerH := 1
	if v := f.FilterFooter(t); v != "" {
		footerH = 2
	}
	body := "\n" + Table(t, width-4, height-3-footerH, cols, rows, f.Cursor()) + "\n"
	if v := f.FilterFooter(t); v != "" {
		body += v + "\n"
	}
	return Card(t, width, title, body, true)
}

func (f *Files) renderEntries(width, height int, title string) string {
	t := f.ctx.Theme
	cols := []Column{
		{Header: "NAME", Width: 0},
		{Header: "SIZE", Width: 12, Align: lipgloss.Right},
		{Header: "MODIFIED", Width: 20},
		{Header: "OWNER", Width: 14},
	}
	rows := make([][]Cell, 0)
	for _, e := range f.visibleEntries() {
		size := "—"
		nameDecor := e.Name
		if e.IsDir {
			nameDecor = lipgloss.NewStyle().Foreground(t.Accent2).Bold(true).Render("📁 " + e.Name)
		} else {
			size = humanize.IBytes(uint64(e.Add.Size))
		}
		mod := "—"
		if e.Add.Time.Mtime > 0 {
			mod = time.Unix(e.Add.Time.Mtime, 0).Format("2006-01-02 15:04")
		}
		owner := e.Add.Owner.User
		rows = append(rows, []Cell{
			Plain(nameDecor),
			Plain(size),
			Plain(mod),
			Plain(owner),
		})
	}
	footerH := 2
	if v := f.FilterFooter(t); v != "" {
		footerH = 3
	}
	body := "\n" + Table(t, width-4, height-3-footerH, cols, rows, f.Cursor()) + "\n"
	muted := lipgloss.NewStyle().Foreground(t.Muted)
	dirs, files := countKinds(f.entries)
	body += muted.Render(fmt.Sprintf("  %d folders · %d files · %s total visible",
		dirs, files, humanize.IBytes(uint64(totalSize(f.entries))))) + "\n"
	if v := f.FilterFooter(t); v != "" {
		body += v + "\n"
	}
	return Card(t, width, title, body, true)
}

func countKinds(es []dsm.FSEntry) (dirs, files int) {
	for _, e := range es {
		if e.IsDir {
			dirs++
		} else {
			files++
		}
	}
	return
}

func totalSize(es []dsm.FSEntry) int64 {
	var s int64
	for _, e := range es {
		if !e.IsDir {
			s += e.Add.Size
		}
	}
	return s
}

// trim trailing slash if any (for prettier breadcrumb display).
var _ = strings.TrimRight
