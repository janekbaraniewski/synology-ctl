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

// FirewallView combines the global firewall status card, the list of
// configured profiles, and the ordered rules for the currently-active
// profile. Switching the cursor between profile rows refetches rules
// for that profile so you can compare without leaving the screen.

type firewallStatusMsg struct {
	S   *dsm.FirewallConf
	Err error
}
type firewallProfilesMsg struct {
	P   []dsm.FirewallProfile
	Err error
}
type firewallRulesMsg struct {
	Profile string
	R       []dsm.FirewallRule
	Err     error
}

type FirewallView struct {
	ctx Ctx

	status   *dsm.FirewallConf
	profiles []dsm.FirewallProfile
	// rules are scoped to the profile we last fetched.
	rulesProfile string
	rules        []dsm.FirewallRule

	statusErr, profilesErr, rulesErr error

	cursor int
	filter Filter
	loaded bool

	detailRule    *dsm.FirewallRule
	detailProfile *dsm.FirewallProfile
}

func NewFirewall(c Ctx) tui.View { return &FirewallView{ctx: c} }

func (v *FirewallView) Name() string                   { return "firewall" }
func (v *FirewallView) Title() string                  { return "Firewall" }
func (v *FirewallView) Icon() string                   { return "▤" }
func (v *FirewallView) RefreshInterval() time.Duration { return 60 * time.Second }
func (v *FirewallView) Bindings() []key.Binding        { return BaseBindings() }

func (v *FirewallView) Init() tea.Cmd {
	return tea.Batch(v.fetchStatus(), v.fetchProfiles(), v.fetchRules(""))
}

func (v *FirewallView) fetchStatus() tea.Cmd {
	c := v.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) (*dsm.FirewallConf, error) { return c.FirewallStatus(ctx) },
		func(s *dsm.FirewallConf, err error) tea.Msg { return firewallStatusMsg{S: s, Err: err} },
	)
}

func (v *FirewallView) fetchProfiles() tea.Cmd {
	c := v.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) ([]dsm.FirewallProfile, error) { return c.FirewallProfiles(ctx) },
		func(p []dsm.FirewallProfile, err error) tea.Msg { return firewallProfilesMsg{P: p, Err: err} },
	)
}

func (v *FirewallView) fetchRules(profile string) tea.Cmd {
	c := v.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) ([]dsm.FirewallRule, error) { return c.FirewallRules(ctx, profile) },
		func(r []dsm.FirewallRule, err error) tea.Msg {
			return firewallRulesMsg{Profile: profile, R: r, Err: err}
		},
	)
}

// flat row model: profiles first, then rules for the active profile.
type firewallRowKind int

const (
	fwRowProfile firewallRowKind = iota
	fwRowRule
)

type firewallRow struct {
	kind  firewallRowKind
	index int
}

func (v *FirewallView) filterProfiles() []dsm.FirewallProfile {
	if v.filter.Value() == "" {
		return v.profiles
	}
	out := make([]dsm.FirewallProfile, 0, len(v.profiles))
	for _, p := range v.profiles {
		if MatchesAll(v.filter.Value(), p.Name, p.Description) {
			out = append(out, p)
		}
	}
	return out
}

func (v *FirewallView) filterRules() []dsm.FirewallRule {
	if v.filter.Value() == "" {
		return v.rules
	}
	out := make([]dsm.FirewallRule, 0, len(v.rules))
	for _, r := range v.rules {
		if MatchesAll(v.filter.Value(), r.Policy, r.Protocol, r.PortDst, r.SrcIP, r.SrcSubnet, r.SrcType, r.Adapter, r.Comment) {
			out = append(out, r)
		}
	}
	return out
}

func (v *FirewallView) flatten() []firewallRow {
	var out []firewallRow
	for i := range v.filterProfiles() {
		out = append(out, firewallRow{fwRowProfile, i})
	}
	for i := range v.filterRules() {
		out = append(out, firewallRow{fwRowRule, i})
	}
	return out
}

func (v *FirewallView) current() (firewallRow, bool) {
	rows := v.flatten()
	if v.cursor < 0 || v.cursor >= len(rows) {
		return firewallRow{}, false
	}
	return rows[v.cursor], true
}

func (v *FirewallView) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	if v.detailRule != nil || v.detailProfile != nil {
		if km, ok := msg.(tea.KeyMsg); ok && (km.String() == "esc" || km.String() == "q") {
			v.detailRule, v.detailProfile = nil, nil
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
		return v, tea.Batch(v.fetchStatus(), v.fetchProfiles(), v.fetchRules(v.rulesProfile))
	case firewallStatusMsg:
		v.status, v.statusErr = m.S, m.Err
		v.loaded = true
	case firewallProfilesMsg:
		v.profiles, v.profilesErr = m.P, m.Err
		v.loaded = true
		v.clampCursor()
	case firewallRulesMsg:
		v.rulesProfile, v.rules, v.rulesErr = m.Profile, m.R, m.Err
		v.loaded = true
		v.clampCursor()
	case tea.KeyMsg:
		switch m.String() {
		case "j", "down":
			rows := v.flatten()
			if v.cursor < len(rows)-1 {
				v.cursor++
				// When landing on a profile row, refetch rules for it.
				if r, ok := v.current(); ok && r.kind == fwRowProfile {
					return v, v.fetchRules(v.filterProfiles()[r.index].Name)
				}
			}
		case "k", "up":
			if v.cursor > 0 {
				v.cursor--
				if r, ok := v.current(); ok && r.kind == fwRowProfile {
					return v, v.fetchRules(v.filterProfiles()[r.index].Name)
				}
			}
		case "g":
			v.cursor = 0
		case "G":
			v.cursor = max(len(v.flatten())-1, 0)
		case "/":
			v.filter.Open()
		case "esc":
			if v.filter.Value() != "" {
				v.filter.Clear()
				v.cursor = 0
			}
		case "r":
			return v, tea.Batch(v.fetchStatus(), v.fetchProfiles(), v.fetchRules(v.rulesProfile))
		case "enter":
			if r, ok := v.current(); ok {
				switch r.kind {
				case fwRowProfile:
					p := v.filterProfiles()[r.index]
					v.detailProfile = &p
				case fwRowRule:
					rule := v.filterRules()[r.index]
					v.detailRule = &rule
				}
			}
		}
	}
	return v, nil
}

func (v *FirewallView) clampCursor() {
	n := len(v.flatten())
	if v.cursor >= n {
		v.cursor = n - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}

func (v *FirewallView) Render(width, height int) string {
	t := v.ctx.Theme
	if v.detailRule != nil {
		return renderFirewallRuleDetail(t, width, *v.detailRule)
	}
	if v.detailProfile != nil {
		return renderFirewallProfileDetail(t, width, *v.detailProfile)
	}

	if v.loaded && v.status == nil && len(v.profiles) == 0 &&
		v.statusErr == nil && v.profilesErr == nil {
		return fitOrScroll(emptyStateCard(t, width,
			"▤  Firewall",
			"Firewall API isn't advertised by this DSM build.",
			"Open Control Panel → Security → Firewall in DSM to set up profiles."), height)
	}

	profs := v.filterProfiles()
	rules := v.filterRules()
	cursor := v.cursor
	idx := 0

	var parts []string
	parts = append(parts, v.renderStatusCard(width))
	parts = append(parts, "")
	parts = append(parts, sectionHeader(t, width, "Profiles", len(profs), v.profilesErr))
	if v.profiles == nil {
		parts = append(parts, "  "+muted(t, "loading…"))
	} else if len(profs) == 0 {
		parts = append(parts, "  "+muted(t, "(none)"))
	}
	for _, p := range profs {
		parts = append(parts, v.renderProfileRow(p, cursor == idx))
		idx++
	}

	ruleTitle := "Rules"
	if v.rulesProfile != "" {
		ruleTitle = "Rules · " + v.rulesProfile
	}
	parts = append(parts, "", sectionHeader(t, width, ruleTitle, len(rules), v.rulesErr))
	if v.rules == nil {
		parts = append(parts, "  "+muted(t, "loading…"))
	} else if len(rules) == 0 {
		parts = append(parts, "  "+muted(t, "(no rules)"))
	}
	for _, r := range rules {
		parts = append(parts, v.renderRuleRow(r, cursor == idx))
		idx++
	}

	parts = append(parts, "")
	parts = append(parts, lipgloss.NewStyle().Foreground(t.Muted).Render(
		"  ↑/↓ move (cursor on profile refetches its rules) · ⏎ details · / filter · r refresh"))
	if fr := v.filter.Render(t); fr != "" {
		parts = append(parts, fr)
	}
	return fitOrScroll(strings.Join(parts, "\n"), height)
}

func (v *FirewallView) renderStatusCard(width int) string {
	t := v.ctx.Theme
	if v.status == nil {
		return t.Card(false).Width(width - 2).Render(
			t.Title().Render(" Firewall ") + "\n  " + muted(t, "loading…"))
	}
	mu := lipgloss.NewStyle().Foreground(t.Muted)
	text := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	state := "disabled"
	if v.status.Enable.Bool() {
		state = "enabled"
	}
	chip := t.HealthStyle(state).Render(" " + state + " ")
	pair := func(k, val string) string {
		if val == "" {
			val = "—"
		}
		return mu.Render(k+":") + " " + text.Render(val)
	}
	row1 := strings.Join([]string{
		mu.Render("Status:") + " " + chip,
		pair("Active profile", v.status.ProfileName),
		pair("Default policy", v.status.DefaultPolicy),
	}, "   ")
	row2 := strings.Join([]string{
		pair("Geo-DB", v.status.GeoDBVersion),
		mu.Render("Log denied:") + " " + boolChip(t, v.status.LogDeny.Bool()),
		mu.Render("Notify denied:") + " " + boolChip(t, v.status.NotifyDeny.Bool()),
	}, "   ")
	body := t.Title().Render(" Firewall ") + "\n" + row1 + "\n" + row2
	return t.Card(false).Width(width - 2).Render(body)
}

func boolChip(t tui.Theme, on bool) string {
	if on {
		return t.HealthStyle("enabled").Render(" on ")
	}
	return t.HealthStyle("disabled").Render(" off ")
}

func (v *FirewallView) renderProfileRow(p dsm.FirewallProfile, highlight bool) string {
	t := v.ctx.Theme
	mu := lipgloss.NewStyle().Foreground(t.Muted)
	text := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	active := ""
	if v.status != nil && v.status.ProfileName == p.Name {
		active = lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render("● active")
	}
	defaultTag := ""
	if p.IsDefault.Bool() {
		defaultTag = mu.Render("default")
	}
	return lipgloss.JoinHorizontal(lipgloss.Center,
		caretGlyph(t, highlight), " ",
		padRight(text.Render(p.Name), 22), " ",
		padRight(mu.Render(clipTo(p.Description, 30)), 30), " ",
		padLeft(mu.Render(fmt.Sprintf("%d rules", p.RuleCount)), 12), " ",
		padRight(defaultTag, 10), " ",
		active,
	)
}

func (v *FirewallView) renderRuleRow(r dsm.FirewallRule, highlight bool) string {
	t := v.ctx.Theme
	mu := lipgloss.NewStyle().Foreground(t.Muted)
	text := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	policy := r.Policy
	if policy == "" {
		policy = "—"
	}
	src := r.SrcIP
	if src == "" {
		src = r.SrcSubnet
	}
	if src == "" {
		src = r.SrcType
	}
	enabled := "disabled"
	if r.Enable.Bool() {
		enabled = "enabled"
	}
	return lipgloss.JoinHorizontal(lipgloss.Center,
		caretGlyph(t, highlight), " ",
		padLeft(mu.Render(fmt.Sprintf("%d", r.Order)), 4), " ",
		padRight(t.HealthStyle(map[string]string{"accept": "ok", "drop": "stopped"}[strings.ToLower(policy)]).Render(strings.ToUpper(policy)), 10), " ",
		padRight(text.Render(r.Protocol), 8), " ",
		padRight(mu.Render(r.PortDst), 12), " ",
		padRight(mu.Render(clipTo(src, 22)), 22), " ",
		padRight(mu.Render(r.Adapter), 10), " ",
		t.HealthStyle(enabled).Render(enabled),
	)
}

// Inspect implements tui.Inspector — useful here because firewall
// rules carry comments and source-geo lists that won't fit in a row.
func (v *FirewallView) Inspect(width, height int) string {
	_ = height
	t := v.ctx.Theme
	r, ok := v.current()
	if !ok {
		return muted(t, "  (no selection)")
	}
	switch r.kind {
	case fwRowProfile:
		return renderFirewallProfileInspect(t, width, v.filterProfiles()[r.index])
	case fwRowRule:
		return renderFirewallRuleInspect(t, width, v.filterRules()[r.index])
	}
	return ""
}

func renderFirewallProfileDetail(t tui.Theme, width int, p dsm.FirewallProfile) string {
	if width < 60 {
		width = 60
	}
	parts := []string{
		hero(t, width, "▤", p.Name, "", p.Description),
		propsCard(t, width, " Properties ", [][2]string{
			{"ID", fmt.Sprintf("%d", p.ID)},
			{"Name", p.Name},
			{"Description", p.Description},
			{"Rule count", fmt.Sprintf("%d", p.RuleCount)},
			{"In use", yesNo(p.InUse.Bool())},
			{"Default", yesNo(p.IsDefault.Bool())},
		}),
		noteCard(t, width, "  esc to go back · move the cursor onto a profile in the list to load its rules"),
	}
	return strings.Join(parts, "\n")
}

func renderFirewallRuleDetail(t tui.Theme, width int, r dsm.FirewallRule) string {
	if width < 60 {
		width = 60
	}
	enabled := "disabled"
	if r.Enable.Bool() {
		enabled = "enabled"
	}
	parts := []string{
		hero(t, width, "▤", fmt.Sprintf("Rule #%d", r.Order), enabled, r.Policy),
		propsCard(t, width, " Properties ", [][2]string{
			{"Rule ID", fmt.Sprintf("%d", r.RuleID)},
			{"Profile ID", fmt.Sprintf("%d", r.ProfileID)},
			{"Order", fmt.Sprintf("%d", r.Order)},
			{"Policy", r.Policy},
			{"Protocol", r.Protocol},
			{"Dst port", r.PortDst},
			{"Source type", r.SrcType},
			{"Source IP", r.SrcIP},
			{"Source subnet", r.SrcSubnet},
			{"Adapter", r.Adapter},
		}),
	}
	if len(r.SrcGeo) > 0 {
		body := t.Title().Render(" Source geo ") + "\n  " +
			lipgloss.NewStyle().Foreground(t.Text).Render(strings.Join(r.SrcGeo, ", "))
		parts = append(parts, t.Card(false).Width(width-2).Render(body))
	}
	if r.Comment != "" {
		body := t.Title().Render(" Comment ") + "\n" +
			wrap(lipgloss.NewStyle().Foreground(t.Text), r.Comment, width-6)
		parts = append(parts, t.Card(false).Width(width-2).Render(body))
	}
	parts = append(parts, noteCard(t, width, "  esc to go back · rule editing isn't wired up yet"))
	return strings.Join(parts, "\n")
}

func renderFirewallProfileInspect(t tui.Theme, width int, p dsm.FirewallProfile) string {
	_ = width
	return strings.Join([]string{
		t.Title().Render(" Profile "),
		lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render("  " + p.Name),
		lipgloss.NewStyle().Foreground(t.Muted).Render("  " + p.Description),
		"",
		muted(t, "  Rule count ") + fmt.Sprintf("%d", p.RuleCount),
		muted(t, "  In use     ") + yesNo(p.InUse.Bool()),
		muted(t, "  Default    ") + yesNo(p.IsDefault.Bool()),
	}, "\n")
}

func renderFirewallRuleInspect(t tui.Theme, width int, r dsm.FirewallRule) string {
	_ = width
	enabled := "disabled"
	if r.Enable.Bool() {
		enabled = "enabled"
	}
	src := r.SrcIP
	if src == "" {
		src = r.SrcSubnet
	}
	if src == "" {
		src = r.SrcType
	}
	geo := "—"
	if len(r.SrcGeo) > 0 {
		geo = strings.Join(r.SrcGeo, ", ")
	}
	return strings.Join([]string{
		t.Title().Render(" Rule "),
		lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(fmt.Sprintf("  #%d  %s", r.Order, strings.ToUpper(r.Policy))),
		"  " + t.HealthStyle(enabled).Render(enabled),
		"",
		muted(t, "  Protocol  ") + r.Protocol,
		muted(t, "  Dst port  ") + r.PortDst,
		muted(t, "  Source    ") + src,
		muted(t, "  Adapter   ") + r.Adapter,
		muted(t, "  Geo       ") + geo,
		"",
		muted(t, "  Comment") + "\n  " + r.Comment,
	}, "\n")
}
