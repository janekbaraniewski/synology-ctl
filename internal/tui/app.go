package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	"github.com/janbaraniewski/synology-ctl/internal/dsm"
)

// App is the root bubbletea Model. It owns the view registry, top/bottom
// chrome, command palette and help overlay.
type App struct {
	client *dsm.Client
	theme  Theme
	keys   KeyMap
	logger *log.Logger

	views   []View
	byName  map[string]View
	active  int
	history []int

	width, height int

	// Command palette state.
	paletteOpen bool
	palette     textinput.Model

	// Help overlay state.
	helpOpen bool

	// Last system snapshot, for the top bar.
	sysInfo *dsm.SystemInfo

	// Last error to surface in the status bar (transient).
	lastErr   error
	errExpire time.Time
}

// NewApp constructs the root model with the provided views (order is preserved
// for the tab bar). The first view becomes active.
func NewApp(client *dsm.Client, theme Theme, logger *log.Logger, views ...View) *App {
	pal := textinput.New()
	pal.Prompt = ""
	pal.CharLimit = 64
	pal.Placeholder = "type a view name and press enter…"

	a := &App{
		client: client,
		theme:  theme,
		keys:   DefaultKeys(),
		logger: logger,
		views:  views,
		byName: make(map[string]View, len(views)),
		palette: pal,
	}
	for _, v := range views {
		a.byName[v.Name()] = v
	}
	return a
}

// Init kicks off the active view and a system-info fetch for the top bar.
func (a *App) Init() tea.Cmd {
	if len(a.views) == 0 {
		return nil
	}
	cmds := []tea.Cmd{
		a.views[a.active].Init(),
		scheduleTick(a.views[a.active].Name(), a.views[a.active].RefreshInterval()),
		a.fetchSysInfo(),
		tea.Tick(30*time.Second, func(t time.Time) tea.Msg { return sysInfoTick{} }),
	}
	return tea.Batch(cmds...)
}

type sysInfoTick struct{}
type sysInfoMsg struct {
	Info *dsm.SystemInfo
	Err  error
}

func (a *App) fetchSysInfo() tea.Cmd {
	if a.client == nil {
		return nil
	}
	c := a.client
	return Fetch(10*time.Second,
		func(ctx context.Context) (*dsm.SystemInfo, error) { return c.SystemInfo(ctx) },
		func(info *dsm.SystemInfo, err error) tea.Msg { return sysInfoMsg{Info: info, Err: err} },
	)
}

// Update is the main event router. It implements modal handling for the
// command palette and help overlay; otherwise it forwards messages to the
// active view (subject to global key bindings).
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		return a, nil

	case sysInfoTick:
		return a, tea.Batch(a.fetchSysInfo(), tea.Tick(30*time.Second, func(t time.Time) tea.Msg { return sysInfoTick{} }))

	case sysInfoMsg:
		if m.Err == nil {
			a.sysInfo = m.Info
		} else {
			a.flashErr(m.Err)
		}
		return a, nil

	case TickMsg:
		// Forward to the named view (which is usually the active one).
		v, ok := a.byName[m.View]
		if !ok {
			return a, nil
		}
		nv, cmd := v.Update(m)
		a.replaceView(nv)
		// Re-schedule if this is the active view.
		var resched tea.Cmd
		if a.views[a.active].Name() == m.View {
			resched = scheduleTick(m.View, nv.RefreshInterval())
		}
		return a, tea.Batch(cmd, resched)

	case tea.KeyMsg:
		// Modal modes consume keys first.
		if a.paletteOpen {
			return a.updatePalette(m)
		}
		if a.helpOpen {
			if key.Matches(m, a.keys.Help, a.keys.Back, a.keys.Quit) {
				a.helpOpen = false
				return a, nil
			}
			return a, nil
		}

		// Global key bindings.
		switch {
		case key.Matches(m, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(m, a.keys.Help):
			a.helpOpen = true
			return a, nil
		case key.Matches(m, a.keys.Palette):
			a.paletteOpen = true
			a.palette.SetValue("")
			a.palette.Focus()
			return a, textinput.Blink
		case key.Matches(m, a.keys.TabNext):
			a.cycle(1)
			return a, a.activate()
		case key.Matches(m, a.keys.TabPrev):
			a.cycle(-1)
			return a, a.activate()
		}
	}

	// Forward everything else to the active view.
	nv, cmd := a.views[a.active].Update(msg)
	a.replaceView(nv)
	return a, cmd
}

func (a *App) replaceView(v View) {
	a.views[a.active] = v
	a.byName[v.Name()] = v
}

func (a *App) cycle(delta int) {
	n := len(a.views)
	a.active = (a.active + delta + n) % n
}

func (a *App) activate() tea.Cmd {
	v := a.views[a.active]
	return tea.Batch(v.Init(), scheduleTick(v.Name(), v.RefreshInterval()))
}

func (a *App) updatePalette(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.Type {
	case tea.KeyEsc:
		a.paletteOpen = false
		a.palette.Blur()
		return a, nil
	case tea.KeyEnter:
		val := strings.TrimSpace(a.palette.Value())
		a.paletteOpen = false
		a.palette.Blur()
		if val == "" {
			return a, nil
		}
		// Match by name, alias-by-prefix, or fuzzy contains.
		if target := a.resolveView(val); target >= 0 {
			a.active = target
			return a, a.activate()
		}
		a.flashErr(fmt.Errorf("no view matches %q", val))
		return a, nil
	}
	var cmd tea.Cmd
	a.palette, cmd = a.palette.Update(m)
	return a, cmd
}

func (a *App) resolveView(q string) int {
	q = strings.ToLower(strings.TrimSpace(q))
	// Exact name
	for i, v := range a.views {
		if v.Name() == q {
			return i
		}
	}
	// Prefix
	for i, v := range a.views {
		if strings.HasPrefix(v.Name(), q) {
			return i
		}
	}
	// Substring
	for i, v := range a.views {
		if strings.Contains(v.Name(), q) {
			return i
		}
	}
	return -1
}

func (a *App) flashErr(err error) {
	a.lastErr = err
	a.errExpire = time.Now().Add(5 * time.Second)
}

// View renders the entire screen.
func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return ""
	}
	top := a.renderTopBar()
	tabs := a.renderTabs()
	hint := a.renderHintBar()
	pal := ""
	if a.paletteOpen {
		pal = a.renderPalette()
	}

	chromeLines := 3 // top + tabs + hint
	if a.paletteOpen {
		chromeLines++
	}
	bodyHeight := a.height - chromeLines
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	body := a.views[a.active].Render(a.width, bodyHeight)

	if a.helpOpen {
		return a.renderHelpOverlay()
	}

	parts := []string{top, tabs, body}
	if pal != "" {
		parts = append(parts, pal)
	}
	parts = append(parts, hint)
	return strings.Join(parts, "\n")
}

func (a *App) renderTopBar() string {
	t := a.theme
	host := "(no connection)"
	dsmVer := ""
	uptime := ""
	if a.client != nil {
		host = a.client.Host()
	}
	if a.sysInfo != nil {
		if a.sysInfo.DSMVersion != "" {
			dsmVer = "DSM " + a.sysInfo.DSMVersion
		} else if a.sysInfo.Version != "" {
			dsmVer = "DSM " + a.sysInfo.Version
		}
		if a.sysInfo.UptimeSeconds != "" {
			uptime = "up " + humanizeUptime(a.sysInfo.UptimeSeconds)
		}
	}

	brand := lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(" synoctl ")
	hostS := lipgloss.NewStyle().Foreground(t.Text).Render(host)
	dot := lipgloss.NewStyle().Foreground(t.Success).Render("●")
	if a.client == nil || !a.client.Authenticated() {
		dot = lipgloss.NewStyle().Foreground(t.Error).Render("●")
	}

	left := lipgloss.JoinHorizontal(lipgloss.Center,
		brand,
		"  ", dot, " ", hostS,
	)
	mid := lipgloss.NewStyle().Foreground(t.Muted).Render(dsmVer)
	clock := time.Now().Format("15:04")
	right := lipgloss.NewStyle().Foreground(t.Muted).Render(uptime + "  " + clock)

	gap := a.width - lipgloss.Width(left) - lipgloss.Width(mid) - lipgloss.Width(right)
	if gap < 2 {
		gap = 2
	}
	pad := strings.Repeat(" ", gap/2)
	tail := strings.Repeat(" ", gap-gap/2)

	bar := lipgloss.NewStyle().
		Background(t.BgAlt).
		Width(a.width).
		Render(left + pad + mid + tail + right)
	return bar
}

func (a *App) renderTabs() string {
	t := a.theme
	var parts []string
	for i, v := range a.views {
		label := fmt.Sprintf(" %s %s ", v.Icon(), v.Title())
		if i == a.active {
			parts = append(parts, lipgloss.NewStyle().Foreground(t.Bg).Background(t.Accent).Bold(true).Render(label))
		} else {
			parts = append(parts, lipgloss.NewStyle().Foreground(t.Muted).Background(t.BgAlt).Render(label))
		}
	}
	row := strings.Join(parts, "")
	// Right-pad to full width with the alt bg.
	pad := a.width - lipgloss.Width(row)
	if pad > 0 {
		row += lipgloss.NewStyle().Background(t.BgAlt).Render(strings.Repeat(" ", pad))
	}
	return row
}

func (a *App) renderHintBar() string {
	t := a.theme
	if a.lastErr != nil && time.Now().Before(a.errExpire) {
		return lipgloss.NewStyle().
			Background(t.BgAlt).
			Foreground(t.Error).
			Width(a.width).
			Render(" ⚠ " + a.lastErr.Error())
	}

	chip := func(k, label string) string {
		return t.Chip(t.Accent2).Render(k) + lipgloss.NewStyle().Foreground(t.Muted).Render(" "+label+"  ")
	}
	hints := []string{
		chip("⇥", "view"),
		chip(":", "command"),
		chip("/", "filter"),
		chip("r", "refresh"),
		chip("a", "actions"),
		chip("?", "help"),
		chip("q", "quit"),
	}
	row := " " + strings.Join(hints, "")
	pad := a.width - lipgloss.Width(row)
	if pad > 0 {
		row += lipgloss.NewStyle().Background(t.BgAlt).Render(strings.Repeat(" ", pad))
	}
	return lipgloss.NewStyle().Background(t.BgAlt).Width(a.width).Render(row)
}

func (a *App) renderPalette() string {
	t := a.theme
	prompt := lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(" : ")
	a.palette.Width = a.width - 6
	return lipgloss.NewStyle().
		Background(t.BgAlt).
		Width(a.width).
		Render(prompt + a.palette.View())
}

func (a *App) renderHelpOverlay() string {
	t := a.theme
	var lines []string
	add := func(k key.Binding) {
		h := k.Help()
		lines = append(lines,
			t.Chip(t.Accent).Render(" "+h.Key+" ")+"  "+lipgloss.NewStyle().Foreground(t.Text).Render(h.Desc),
		)
	}
	global := []key.Binding{
		a.keys.Help, a.keys.Quit, a.keys.Palette, a.keys.Filter, a.keys.Refresh,
		a.keys.TabNext, a.keys.TabPrev, a.keys.Action, a.keys.YankPath,
		a.keys.Up, a.keys.Down, a.keys.Top, a.keys.Bottom, a.keys.PageUp, a.keys.PageDown,
		a.keys.Enter, a.keys.Back,
	}
	lines = append(lines, t.Title().Render("Global"))
	for _, k := range global {
		add(k)
	}
	local := a.views[a.active].Bindings()
	if len(local) > 0 {
		lines = append(lines, "", t.Title().Render(a.views[a.active].Title()+" — view"))
		sort.Slice(local, func(i, j int) bool { return local[i].Help().Desc < local[j].Help().Desc })
		for _, k := range local {
			add(k)
		}
	}

	content := strings.Join(lines, "\n")
	card := t.Card(true).Render(t.Title().Render(" Help — press ? or esc to close ") + "\n\n" + content)
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, card,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(t.Faint),
	)
}

// humanizeUptime accepts DSM's "%d:%d:%d:%d" format (days:hours:minutes:seconds)
// or a fallback Go duration string and returns a compact form like "47d 3h".
func humanizeUptime(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, ":")
	if len(parts) == 4 {
		return parts[0] + "d " + parts[1] + "h"
	}
	return s
}
