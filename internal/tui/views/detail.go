package views

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/janbaraniewski/synology-ctl/internal/tui"
)

// Detail is a modal-style overlay that pretty-prints a row's underlying
// JSON. It's how every list view supports "Enter to drill in" without
// each view authoring its own detail page.
//
// State lives inside the parent view; on Enter the view stashes the
// selected object here, on Esc it clears.
type Detail struct {
	theme   tui.Theme
	title   string
	body    string // pretty-printed JSON
	vp      viewport.Model
	visible bool
}

// NewDetail constructs an empty detail overlay.
func NewDetail(theme tui.Theme) *Detail {
	vp := viewport.New(0, 0)
	return &Detail{theme: theme, vp: vp}
}

// Show populates the overlay and makes it visible.
func (d *Detail) Show(title string, payload any) {
	d.title = title
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		d.body = err.Error()
	} else {
		d.body = colourizeJSON(string(b), d.theme)
	}
	d.vp.SetContent(d.body)
	d.visible = true
}

// Hide closes the overlay.
func (d *Detail) Hide() { d.visible = false }

// Visible reports whether the overlay is open.
func (d *Detail) Visible() bool { return d.visible }

// Update routes key events while the overlay owns input focus. Returns
// true if the message was consumed by the overlay (so the caller can
// short-circuit its own handling).
func (d *Detail) Update(msg tea.Msg) (consumed bool, cmd tea.Cmd) {
	if !d.visible {
		return false, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc", "q":
			d.visible = false
			return true, nil
		}
	}
	var c tea.Cmd
	d.vp, c = d.vp.Update(msg)
	return true, c
}

// Render draws the overlay centered on the given canvas. Returns "" when
// hidden so callers can skip it cheaply.
func (d *Detail) Render(width, height int) string {
	if !d.visible {
		return ""
	}
	w := width - 8
	if w < 40 {
		w = width - 2
	}
	h := height - 6
	if h < 8 {
		h = height - 2
	}
	d.vp.Width = w - 4
	d.vp.Height = h - 4

	title := d.theme.Title().Render(" " + d.title + " — esc to close ")
	card := d.theme.Card(true).Width(w).Height(h).Render(title + "\n\n" + d.vp.View())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card,
		lipgloss.WithWhitespaceForeground(d.theme.Faint),
	)
}

// DetailKeys is the bindings detail-aware views advertise in `?` help.
var DetailKeys = []key.Binding{
	key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "details")),
	key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close details")),
}

// colourizeJSON applies a very light syntax tint to a pretty-printed JSON
// blob — strings in the success colour, numbers/booleans/null in info,
// keys in accent. We do this with naive string scanning rather than
// pulling in a full lexer because the payloads are tiny.
func colourizeJSON(s string, theme tui.Theme) string {
	accent := lipgloss.NewStyle().Foreground(theme.Accent).Render
	str := lipgloss.NewStyle().Foreground(theme.Success).Render
	num := lipgloss.NewStyle().Foreground(theme.Info).Render

	var b strings.Builder
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '"':
			end := i + 1
			for end < len(s) && (s[end] != '"' || s[end-1] == '\\') {
				end++
			}
			if end >= len(s) {
				b.WriteString(s[i:])
				return b.String()
			}
			token := s[i : end+1]
			// Key vs value: look ahead for ":".
			after := end + 1
			for after < len(s) && (s[after] == ' ' || s[after] == '\t') {
				after++
			}
			if after < len(s) && s[after] == ':' {
				b.WriteString(accent(token))
			} else {
				b.WriteString(str(token))
			}
			i = end + 1
		case (c >= '0' && c <= '9') || (c == '-' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9'):
			end := i + 1
			for end < len(s) && (s[end] >= '0' && s[end] <= '9' || s[end] == '.' || s[end] == 'e' || s[end] == 'E' || s[end] == '+' || s[end] == '-') {
				end++
			}
			b.WriteString(num(s[i:end]))
			i = end
		case c == 't' && strings.HasPrefix(s[i:], "true"):
			b.WriteString(num("true"))
			i += 4
		case c == 'f' && strings.HasPrefix(s[i:], "false"):
			b.WriteString(num("false"))
			i += 5
		case c == 'n' && strings.HasPrefix(s[i:], "null"):
			b.WriteString(num("null"))
			i += 4
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// Filter is a tiny on-line text input used by list views for `/`.
type Filter struct {
	active bool
	value  string
}

// IsActive reports whether the user is currently typing in the filter.
func (f *Filter) IsActive() bool { return f.active }

// Value returns the current filter text.
func (f *Filter) Value() string { return f.value }

// Open starts an editing session.
func (f *Filter) Open() { f.active = true }

// Close commits the filter (keeps the value but stops typing).
func (f *Filter) Close() { f.active = false }

// Clear empties the filter.
func (f *Filter) Clear() { f.active = false; f.value = "" }

// Update applies a key event to the filter editor; returns true if the
// event was consumed.
func (f *Filter) Update(msg tea.Msg) bool {
	if !f.active {
		return false
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return false
	}
	switch km.Type {
	case tea.KeyEnter:
		f.active = false
		return true
	case tea.KeyEsc:
		f.active = false
		f.value = ""
		return true
	case tea.KeyBackspace:
		if len(f.value) > 0 {
			f.value = f.value[:len(f.value)-1]
		}
		return true
	case tea.KeyRunes, tea.KeySpace:
		f.value += string(km.Runes)
		return true
	}
	return false
}

// Render returns the inline prompt to draw in the card footer; empty
// when no filter is in play.
func (f Filter) Render(theme tui.Theme) string {
	if !f.active && f.value == "" {
		return ""
	}
	prompt := lipgloss.NewStyle().Foreground(theme.Accent).Bold(true).Render(" / ")
	val := f.value
	if f.active {
		val += "▎"
	}
	return prompt + lipgloss.NewStyle().Foreground(theme.Text).Render(val)
}

// MatchesAll returns true when every haystack cell collectively contains
// the needle (case-insensitive substring). When needle is empty it
// always returns true.
func MatchesAll(needle string, cells ...string) bool {
	if needle == "" {
		return true
	}
	n := strings.ToLower(needle)
	for _, c := range cells {
		if strings.Contains(strings.ToLower(c), n) {
			return true
		}
	}
	return false
}
