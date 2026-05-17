package views

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janbaraniewski/synology-ctl/internal/tui"
)

// listBase is embedded by every table-driven view. It centralises the
// keyboard handling that's identical everywhere (cursor movement, `/`
// filter, `enter` to open a detail overlay) so each view only writes the
// truly view-specific code: which data it fetches and how rows are
// rendered.
//
// Usage:
//
//	type Foo struct {
//	    listBase
//	    ctx  Ctx
//	    rows []Item
//	}
//
//	func (f *Foo) Update(msg tea.Msg) (tui.View, tea.Cmd) {
//	    if cmd, handled := f.listBase.HandleKey(msg, len(f.visibleRows())); handled {
//	        return f, cmd
//	    }
//	    // … view-specific message handling …
//	}
type listBase struct {
	cursor int
	filter Filter
	detail *Detail
}

// initBase wires the detail overlay against the view context theme.
func (b *listBase) initBase(ctx Ctx) {
	if b.detail == nil {
		b.detail = NewDetail(ctx.Theme)
	}
}

// BaseBindings returns the help-overlay bindings that are always
// available in a list view.
func BaseBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "details")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter / close details")),
	}
}

// HandleKey processes the generic list keys. The caller passes the number
// of rows currently visible (after its own filtering). Returns true when
// the message was consumed so the caller can skip its own handling.
func (b *listBase) HandleKey(msg tea.Msg, rowCount int) (tea.Cmd, bool) {
	// The detail overlay swallows everything when open.
	if b.detail != nil && b.detail.Visible() {
		_, cmd := b.detail.Update(msg)
		return cmd, true
	}
	// Filter editing swallows runes when open.
	if b.filter.IsActive() {
		if b.filter.Update(msg) {
			return nil, true
		}
		return nil, false
	}

	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}
	switch km.String() {
	case "j", "down":
		if b.cursor < rowCount-1 {
			b.cursor++
		}
		return nil, true
	case "k", "up":
		if b.cursor > 0 {
			b.cursor--
		}
		return nil, true
	case "g":
		b.cursor = 0
		return nil, true
	case "G":
		b.cursor = rowCount - 1
		if b.cursor < 0 {
			b.cursor = 0
		}
		return nil, true
	case "/":
		b.filter.Open()
		return nil, true
	case "esc":
		if b.filter.Value() != "" {
			b.filter.Clear()
			b.cursor = 0
			return nil, true
		}
	}
	return nil, false
}

// IsEnter reports whether the message is the Enter key (used by callers
// to open the detail overlay with their own payload).
func (b *listBase) IsEnter(msg tea.Msg) bool {
	if b.detail != nil && b.detail.Visible() {
		return false
	}
	if b.filter.IsActive() {
		return false
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		return km.Type == tea.KeyEnter
	}
	return false
}

// Cursor returns the current cursor index, clamped to [0, max).
func (b *listBase) Cursor() int { return b.cursor }

// ResetCursor jumps back to row 0.
func (b *listBase) ResetCursor() { b.cursor = 0 }

// ClampCursor keeps the cursor in range after the underlying row count
// changes (e.g. after a refresh).
func (b *listBase) ClampCursor(rowCount int) {
	if b.cursor >= rowCount {
		b.cursor = rowCount - 1
	}
	if b.cursor < 0 {
		b.cursor = 0
	}
}

// FilterValue exposes the current filter substring.
func (b *listBase) FilterValue() string { return b.filter.Value() }

// FilterMatch is sugar for MatchesAll(filter.Value(), …).
func (b *listBase) FilterMatch(cells ...string) bool {
	return MatchesAll(b.filter.Value(), cells...)
}

// DetailVisible reports whether the inspector overlay is currently shown.
func (b *listBase) DetailVisible() bool { return b.detail != nil && b.detail.Visible() }

// ShowDetail opens the inspector with the given payload.
func (b *listBase) ShowDetail(title string, payload any) {
	if b.detail == nil {
		return
	}
	b.detail.Show(title, payload)
}

// RenderDetail draws the overlay; returns "" when hidden.
func (b *listBase) RenderDetail(width, height int) string {
	if b.detail == nil {
		return ""
	}
	return b.detail.Render(width, height)
}

// FilterFooter renders the inline `/value` prompt for inclusion in a
// card footer. Returns "" when no filter is in play.
func (b *listBase) FilterFooter(theme tui.Theme) string {
	v := b.filter.Render(theme)
	if v == "" {
		return ""
	}
	hint := " "
	if !b.filter.IsActive() && b.filter.Value() != "" {
		hint = strings.Repeat(" ", 1)
	}
	return hint + v
}
