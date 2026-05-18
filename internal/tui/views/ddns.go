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

// DDNSView lists configured Dynamic DNS hostnames and their last-known
// external IPs. The providers list is fetched alongside so the detail
// overlay can show a human-readable provider name even if the record
// only stores the provider's machine key.

type ddnsRecordsMsg struct {
	R   []dsm.DDNSRecord
	Err error
}
type ddnsProvidersMsg struct {
	P   []dsm.DDNSProvider
	Err error
}

type DDNSView struct {
	ctx Ctx

	records   []dsm.DDNSRecord
	providers []dsm.DDNSProvider

	recordsErr, providersErr error

	cursor int
	filter Filter
	loaded bool

	detail *dsm.DDNSRecord
}

func NewDDNS(c Ctx) tui.View { return &DDNSView{ctx: c} }

func (v *DDNSView) Name() string                   { return "ddns" }
func (v *DDNSView) Title() string                  { return "DDNS" }
func (v *DDNSView) Icon() string                   { return "⌬" }
func (v *DDNSView) RefreshInterval() time.Duration { return 60 * time.Second }
func (v *DDNSView) Bindings() []key.Binding        { return BaseBindings() }

func (v *DDNSView) Init() tea.Cmd { return tea.Batch(v.fetchRecords(), v.fetchProviders()) }

func (v *DDNSView) fetchRecords() tea.Cmd {
	c := v.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(10*time.Second,
		func(ctx context.Context) ([]dsm.DDNSRecord, error) { return c.DDNSRecords(ctx) },
		func(r []dsm.DDNSRecord, err error) tea.Msg { return ddnsRecordsMsg{R: r, Err: err} },
	)
}

func (v *DDNSView) fetchProviders() tea.Cmd {
	c := v.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(10*time.Second,
		func(ctx context.Context) ([]dsm.DDNSProvider, error) { return c.DDNSProviders(ctx) },
		func(p []dsm.DDNSProvider, err error) tea.Msg { return ddnsProvidersMsg{P: p, Err: err} },
	)
}

func (v *DDNSView) filtered() []dsm.DDNSRecord {
	if v.filter.Value() == "" {
		return v.records
	}
	out := make([]dsm.DDNSRecord, 0, len(v.records))
	for _, r := range v.records {
		if MatchesAll(v.filter.Value(), r.Hostname, r.Provider, r.Username, r.Status, r.ExternalIPv4, r.ExternalIPv6) {
			out = append(out, r)
		}
	}
	return out
}

func (v *DDNSView) providerDisplay(name string) string {
	for _, p := range v.providers {
		if p.Name == name {
			if p.DisplayName != "" {
				return p.DisplayName
			}
			return p.Name
		}
	}
	return name
}

func (v *DDNSView) Update(msg tea.Msg) (tui.View, tea.Cmd) {
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
		return v, tea.Batch(v.fetchRecords(), v.fetchProviders())
	case ddnsRecordsMsg:
		v.records, v.recordsErr = m.R, m.Err
		v.loaded = true
		v.clampCursor()
	case ddnsProvidersMsg:
		v.providers, v.providersErr = m.P, m.Err
		v.loaded = true
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
			return v, tea.Batch(v.fetchRecords(), v.fetchProviders())
		case "enter":
			rows := v.filtered()
			if v.cursor >= 0 && v.cursor < len(rows) {
				r := rows[v.cursor]
				v.detail = &r
			}
		}
	}
	return v, nil
}

func (v *DDNSView) clampCursor() {
	n := len(v.filtered())
	if v.cursor >= n {
		v.cursor = n - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}

func (v *DDNSView) Render(width, height int) string {
	t := v.ctx.Theme
	if v.detail != nil {
		return renderDDNSDetail(t, width, *v.detail, v.providerDisplay(v.detail.Provider))
	}

	if v.loaded && len(v.records) == 0 && v.recordsErr == nil {
		return fitOrScroll(emptyStateCard(t, width,
			"⌬  Dynamic DNS",
			"No Dynamic DNS hostnames configured.",
			"Open Control Panel → External Access → DDNS to add a hostname."), height)
	}

	records := v.filtered()
	var parts []string
	parts = append(parts, sectionHeader(t, width, "DDNS records", len(records), v.recordsErr))
	if !v.loaded {
		parts = append(parts, "  "+muted(t, "loading…"))
	} else if len(records) == 0 {
		parts = append(parts, "  "+muted(t, "(none matching)"))
	}
	for i, r := range records {
		parts = append(parts, v.renderRow(r, i == v.cursor))
	}
	parts = append(parts, "")
	parts = append(parts, lipgloss.NewStyle().Foreground(t.Muted).Render(
		"  ↑/↓ move · ⏎ details · / filter · esc clear · r refresh"))
	if fr := v.filter.Render(t); fr != "" {
		parts = append(parts, fr)
	}
	return fitOrScroll(strings.Join(parts, "\n"), height)
}

func (v *DDNSView) renderRow(r dsm.DDNSRecord, highlight bool) string {
	t := v.ctx.Theme
	mu := lipgloss.NewStyle().Foreground(t.Muted)
	text := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	ip := r.ExternalIPv4
	if ip == "" {
		ip = r.ExternalIPv6
	}
	if ip == "" {
		ip = "—"
	}
	status := r.Status
	if status == "" {
		if r.Enable.Bool() {
			status = "enabled"
		} else {
			status = "disabled"
		}
	}
	updated := "—"
	if r.LastUpdated > 0 {
		updated = time.Unix(r.LastUpdated, 0).Format("2006-01-02 15:04")
	}
	return lipgloss.JoinHorizontal(lipgloss.Center,
		caretGlyph(t, highlight), " ",
		padRight(text.Render(clipTo(r.Hostname, 30)), 30), " ",
		padRight(mu.Render(v.providerDisplay(r.Provider)), 16), " ",
		padRight(mu.Render(ip), 18), " ",
		padRight(mu.Render(updated), 18), " ",
		t.HealthStyle(status).Render(status),
	)
}

func renderDDNSDetail(t tui.Theme, width int, r dsm.DDNSRecord, providerDisplay string) string {
	if width < 60 {
		width = 60
	}
	status := r.Status
	if status == "" {
		if r.Enable.Bool() {
			status = "enabled"
		} else {
			status = "disabled"
		}
	}
	updated := "—"
	if r.LastUpdated > 0 {
		updated = time.Unix(r.LastUpdated, 0).Format("2006-01-02 15:04")
	}
	parts := []string{
		hero(t, width, "⌬", r.Hostname, status, providerDisplay),
		propsCard(t, width, " Properties ", [][2]string{
			{"ID", fmt.Sprintf("%d", r.ID)},
			{"Hostname", r.Hostname},
			{"Provider", providerDisplay},
			{"Provider key", r.Provider},
			{"Username", r.Username},
			{"External IPv4", r.ExternalIPv4},
			{"External IPv6", r.ExternalIPv6},
			{"Last updated", updated},
			{"Status", r.Status},
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
		chip("enabled", r.Enable.Bool()),
		chip("heartbeat", r.HeartbeatEnable.Bool()),
	}))
	parts = append(parts, noteCard(t, width, "  esc to go back · DDNS write actions aren't wired up yet"))
	return strings.Join(parts, "\n")
}
