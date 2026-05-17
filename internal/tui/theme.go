// Package tui hosts the bubbletea application — root model, theme, keymap,
// and the registry of views. The theme is Catppuccin-derived (Mocha for
// dark terminals, Latte for light) and exposed as a single Theme value so
// views never reach into raw colours.
package tui

import "github.com/charmbracelet/lipgloss"

// Theme is the full visual palette used by every view. We expose semantic
// slots (Accent, Success, Warn …) rather than raw hex so views don't get
// to make their own colour choices.
type Theme struct {
	// Surfaces
	Bg       lipgloss.AdaptiveColor // page background
	BgAlt    lipgloss.AdaptiveColor // alternate row / sub-card
	Surface  lipgloss.AdaptiveColor // card body
	Border   lipgloss.AdaptiveColor // soft divider
	BorderHi lipgloss.AdaptiveColor // strong divider / focused border

	// Text
	Text  lipgloss.AdaptiveColor // primary text
	Muted lipgloss.AdaptiveColor // secondary text
	Faint lipgloss.AdaptiveColor // tertiary / hint text

	// Semantic accents
	Accent  lipgloss.AdaptiveColor // brand / primary
	Accent2 lipgloss.AdaptiveColor // secondary accent (for variety)
	Success lipgloss.AdaptiveColor
	Warn    lipgloss.AdaptiveColor
	Error   lipgloss.AdaptiveColor
	Info    lipgloss.AdaptiveColor

	// Charts — gradient stops, low → high
	GradLo  lipgloss.AdaptiveColor
	GradMid lipgloss.AdaptiveColor
	GradHi  lipgloss.AdaptiveColor
}

// DefaultTheme returns the Catppuccin-derived palette with light/dark variants.
// We hand-pick from Mocha (dark) and Latte (light) so the result is readable
// on both terminals without burning a runtime detector everywhere.
func DefaultTheme() Theme {
	return Theme{
		Bg:       lipgloss.AdaptiveColor{Light: "#eff1f5", Dark: "#1e1e2e"},
		BgAlt:    lipgloss.AdaptiveColor{Light: "#e6e9ef", Dark: "#181825"},
		Surface:  lipgloss.AdaptiveColor{Light: "#dce0e8", Dark: "#313244"},
		Border:   lipgloss.AdaptiveColor{Light: "#bcc0cc", Dark: "#45475a"},
		BorderHi: lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#cba6f7"},

		Text:  lipgloss.AdaptiveColor{Light: "#4c4f69", Dark: "#cdd6f4"},
		Muted: lipgloss.AdaptiveColor{Light: "#6c6f85", Dark: "#a6adc8"},
		Faint: lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#7f849c"},

		Accent:  lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#cba6f7"}, // mauve
		Accent2: lipgloss.AdaptiveColor{Light: "#1e66f5", Dark: "#89b4fa"}, // blue
		Success: lipgloss.AdaptiveColor{Light: "#40a02b", Dark: "#a6e3a1"},
		Warn:    lipgloss.AdaptiveColor{Light: "#df8e1d", Dark: "#f9e2af"},
		Error:   lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"},
		Info:    lipgloss.AdaptiveColor{Light: "#04a5e5", Dark: "#89dceb"},

		GradLo:  lipgloss.AdaptiveColor{Light: "#40a02b", Dark: "#a6e3a1"}, // green
		GradMid: lipgloss.AdaptiveColor{Light: "#df8e1d", Dark: "#f9e2af"}, // yellow
		GradHi:  lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"}, // red
	}
}

// Card returns a standard rounded-border card style. Pass a focused flag
// to highlight the border when the view owns input focus.
func (t Theme) Card(focused bool) lipgloss.Style {
	border := t.Border
	if focused {
		border = t.BorderHi
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1)
}

// Title returns the style for a card title row (small caps feel via padding).
func (t Theme) Title() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true).
		Padding(0, 1)
}

// Chip is a compact rounded label used for status pills and keybinding hints.
func (t Theme) Chip(c lipgloss.AdaptiveColor) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Bg).
		Background(c).
		Padding(0, 1).
		Bold(true)
}

// SubtleChip is a low-contrast version of Chip for hint bars.
func (t Theme) SubtleChip() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Muted).
		Background(t.BgAlt).
		Padding(0, 1)
}

// HeaderRow is the highlighted row used at the top of tables.
func (t Theme) HeaderRow() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true)
}

// Row is the default table row style.
func (t Theme) Row() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Text)
}

// RowAlt is the alternate (zebra) row style.
func (t Theme) RowAlt() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Text).Background(t.BgAlt)
}

// Selected highlights the active row.
func (t Theme) Selected() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Bg).
		Background(t.Accent).
		Bold(true)
}

// HealthStyle returns a chip style for a normalized health string.
// Inputs: "normal", "ok", "running" → success; "warn", "degrade" → warn;
// "crashed", "error", "stop" → error; anything else → muted.
func (t Theme) HealthStyle(s string) lipgloss.Style {
	switch s {
	case "normal", "ok", "running", "connected", "healthy":
		return t.Chip(t.Success)
	case "warn", "warning", "degrade", "rebuilding":
		return t.Chip(t.Warn)
	case "crashed", "error", "stop", "stopped", "disconnected", "broken":
		return t.Chip(t.Error)
	default:
		return t.SubtleChip()
	}
}
