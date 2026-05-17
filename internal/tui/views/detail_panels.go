package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janbaraniewski/synology-ctl/internal/tui"
)

// hero renders the top header row of a detail screen: an icon + title, a
// status chip, and a muted subtitle.
func hero(t tui.Theme, width int, icon, title, status, subtitle string) string {
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(icon+"  "+title),
	)
	if status != "" {
		header = lipgloss.JoinHorizontal(lipgloss.Top, header, "  ", t.HealthStyle(status).Render(" "+status+" "))
	}
	if subtitle != "" {
		header = lipgloss.JoinHorizontal(lipgloss.Top, header, "  ",
			lipgloss.NewStyle().Foreground(t.Muted).Render(subtitle))
	}
	return t.Card(true).Width(width - 2).Render(header)
}

// propsCard renders a "Properties" panel with two-column key/value pairs.
func propsCard(t tui.Theme, width int, title string, kv [][2]string) string {
	inner := width - 6
	if inner < 30 {
		inner = 30
	}
	rows := renderTwoColumnProps(t, inner, kv)
	return t.Card(false).Width(width - 2).Render(t.Title().Render(title) + "\n" + rows)
}

// chipsCard renders a panel that's just a row of styled chips.
func chipsCard(t tui.Theme, width int, title string, chips []string) string {
	body := t.Title().Render(title) + "\n  " + strings.Join(chips, "   ")
	return t.Card(false).Width(width - 2).Render(body)
}

// noteCard is a one-line muted footer for explanatory text.
func noteCard(t tui.Theme, width int, text string) string {
	return t.Card(false).Width(width - 2).Render(
		lipgloss.NewStyle().Foreground(t.Faint).Render(text))
}

// gaugeCard renders a single-line "headline + gauge + sub" card useful
// for capacity / utilization summaries.
func gaugeCard(t tui.Theme, width int, title, big string, ratio float64, sub string) string {
	innerW := width - 6
	barW := innerW - 14
	if barW < 16 {
		barW = innerW
	}
	bar := Gauge(t, barW, ratio)
	bigS := lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(big)
	subS := lipgloss.NewStyle().Foreground(t.Muted).Render(sub)
	body := t.Title().Render(title) + "\n" + bar + "   " + bigS + "\n" + subS
	return t.Card(false).Width(width - 2).Render(body)
}

// yesNo formats a bool as a friendly chip-style string.
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
