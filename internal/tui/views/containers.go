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

// Containers is the read-only Container Manager overview. It folds
// containers, locally stored images, and configured networks into a
// single multi-section list — the same pattern AdminPage uses for
// Users / Interfaces / Logs.
//
// Refresh cadence is 30s: container telemetry (cpu/mem) is live but
// dashboards already cover the high-frequency case; we want this page
// to stay quiet between glances.

type containersMsg struct {
	C   []dsm.Container
	Err error
}
type dockerImagesMsg struct {
	I   []dsm.Image
	Err error
}
type dockerNetworksMsg struct {
	N   []dsm.DockerNetwork
	Err error
}

type ContainersView struct {
	ctx Ctx

	containers []dsm.Container
	images     []dsm.Image
	networks   []dsm.DockerNetwork

	cErr, iErr, nErr error

	cursor int
	filter Filter

	detailContainer *dsm.Container
	detailImage     *dsm.Image
	detailNetwork   *dsm.DockerNetwork
}

func NewContainers(c Ctx) tui.View { return &ContainersView{ctx: c} }

func (v *ContainersView) Name() string                   { return "containers" }
func (v *ContainersView) Title() string                  { return "Containers" }
func (v *ContainersView) Icon() string                   { return "▦" }
func (v *ContainersView) RefreshInterval() time.Duration { return 30 * time.Second }
func (v *ContainersView) Bindings() []key.Binding        { return BaseBindings() }

func (v *ContainersView) Init() tea.Cmd {
	return tea.Batch(v.fetchContainers(), v.fetchImages(), v.fetchNetworks())
}

func (v *ContainersView) fetchContainers() tea.Cmd {
	c := v.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) ([]dsm.Container, error) { return c.Containers(ctx) },
		func(x []dsm.Container, err error) tea.Msg { return containersMsg{C: x, Err: err} },
	)
}

func (v *ContainersView) fetchImages() tea.Cmd {
	c := v.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) ([]dsm.Image, error) { return c.DockerImages(ctx) },
		func(x []dsm.Image, err error) tea.Msg { return dockerImagesMsg{I: x, Err: err} },
	)
}

func (v *ContainersView) fetchNetworks() tea.Cmd {
	c := v.ctx.Client
	if c == nil {
		return nil
	}
	return tui.Fetch(8*time.Second,
		func(ctx context.Context) ([]dsm.DockerNetwork, error) { return c.DockerNetworks(ctx) },
		func(x []dsm.DockerNetwork, err error) tea.Msg { return dockerNetworksMsg{N: x, Err: err} },
	)
}

type containerRowKind int

const (
	contRowContainer containerRowKind = iota
	contRowImage
	contRowNetwork
)

type containerRow struct {
	kind  containerRowKind
	index int
}

func (v *ContainersView) filterContainers() []dsm.Container {
	if v.filter.Value() == "" {
		return v.containers
	}
	out := make([]dsm.Container, 0, len(v.containers))
	for _, c := range v.containers {
		if MatchesAll(v.filter.Value(), c.ID, c.Name, c.Image, c.Status, c.State) {
			out = append(out, c)
		}
	}
	return out
}

func (v *ContainersView) filterImages() []dsm.Image {
	if v.filter.Value() == "" {
		return v.images
	}
	out := make([]dsm.Image, 0, len(v.images))
	for _, i := range v.images {
		if MatchesAll(v.filter.Value(), i.ID, i.Repository, i.Tag, i.RepoTag, i.Description) {
			out = append(out, i)
		}
	}
	return out
}

func (v *ContainersView) filterNetworks() []dsm.DockerNetwork {
	if v.filter.Value() == "" {
		return v.networks
	}
	out := make([]dsm.DockerNetwork, 0, len(v.networks))
	for _, n := range v.networks {
		if MatchesAll(v.filter.Value(), n.ID, n.Name, n.Driver, n.Subnet, n.Gateway) {
			out = append(out, n)
		}
	}
	return out
}

func (v *ContainersView) flatten() []containerRow {
	var out []containerRow
	for i := range v.filterContainers() {
		out = append(out, containerRow{contRowContainer, i})
	}
	for i := range v.filterImages() {
		out = append(out, containerRow{contRowImage, i})
	}
	for i := range v.filterNetworks() {
		out = append(out, containerRow{contRowNetwork, i})
	}
	return out
}

func (v *ContainersView) current() (containerRow, bool) {
	rows := v.flatten()
	if v.cursor < 0 || v.cursor >= len(rows) {
		return containerRow{}, false
	}
	return rows[v.cursor], true
}

func (v *ContainersView) Update(msg tea.Msg) (tui.View, tea.Cmd) {
	if v.detailContainer != nil || v.detailImage != nil || v.detailNetwork != nil {
		if km, ok := msg.(tea.KeyMsg); ok && (km.String() == "esc" || km.String() == "q") {
			v.detailContainer, v.detailImage, v.detailNetwork = nil, nil, nil
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
		return v, tea.Batch(v.fetchContainers(), v.fetchImages(), v.fetchNetworks())
	case containersMsg:
		v.containers, v.cErr = m.C, m.Err
		v.clampCursor()
	case dockerImagesMsg:
		v.images, v.iErr = m.I, m.Err
		v.clampCursor()
	case dockerNetworksMsg:
		v.networks, v.nErr = m.N, m.Err
		v.clampCursor()
	case tea.KeyMsg:
		switch m.String() {
		case "j", "down":
			rows := v.flatten()
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
			v.cursor = max(len(v.flatten())-1, 0)
		case "/":
			v.filter.Open()
		case "esc":
			if v.filter.Value() != "" {
				v.filter.Clear()
				v.cursor = 0
			}
		case "r":
			return v, tea.Batch(v.fetchContainers(), v.fetchImages(), v.fetchNetworks())
		case "enter":
			if r, ok := v.current(); ok {
				switch r.kind {
				case contRowContainer:
					c := v.filterContainers()[r.index]
					v.detailContainer = &c
				case contRowImage:
					i := v.filterImages()[r.index]
					v.detailImage = &i
				case contRowNetwork:
					n := v.filterNetworks()[r.index]
					v.detailNetwork = &n
				}
			}
		}
	}
	return v, nil
}

func (v *ContainersView) clampCursor() {
	n := len(v.flatten())
	if v.cursor >= n {
		v.cursor = n - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}

func (v *ContainersView) Render(width, height int) string {
	t := v.ctx.Theme
	if v.detailContainer != nil {
		return renderContainerDetail(t, width, *v.detailContainer)
	}
	if v.detailImage != nil {
		return renderDockerImageDetail(t, width, *v.detailImage)
	}
	if v.detailNetwork != nil {
		return renderDockerNetworkDetail(t, width, *v.detailNetwork)
	}

	// Empty-state when Container Manager isn't installed at all.
	if v.containers != nil && v.images != nil && v.networks != nil &&
		len(v.containers) == 0 && len(v.images) == 0 && len(v.networks) == 0 &&
		v.cErr == nil && v.iErr == nil && v.nErr == nil {
		return fitOrScroll(emptyStateCard(t, width,
			"▦  Container Manager",
			"Container Manager isn't installed, or no containers / images / networks exist yet.",
			"Install Container Manager from Package Center to see your containers here."), height)
	}

	cs := v.filterContainers()
	is := v.filterImages()
	ns := v.filterNetworks()
	cursor := v.cursor
	idx := 0

	var parts []string
	parts = append(parts, sectionHeader(t, width, "Containers", len(cs), v.cErr))
	if v.containers == nil {
		parts = append(parts, "  "+muted(t, "loading…"))
	} else if len(cs) == 0 {
		parts = append(parts, "  "+muted(t, "(none)"))
	}
	for _, c := range cs {
		parts = append(parts, v.renderContainerRow(c, cursor == idx))
		idx++
	}

	parts = append(parts, "", sectionHeader(t, width, "Images", len(is), v.iErr))
	if v.images == nil {
		parts = append(parts, "  "+muted(t, "loading…"))
	} else if len(is) == 0 {
		parts = append(parts, "  "+muted(t, "(none)"))
	}
	for _, i := range is {
		parts = append(parts, v.renderImageRow(i, cursor == idx))
		idx++
	}

	parts = append(parts, "", sectionHeader(t, width, "Networks", len(ns), v.nErr))
	if v.networks == nil {
		parts = append(parts, "  "+muted(t, "loading…"))
	} else if len(ns) == 0 {
		parts = append(parts, "  "+muted(t, "(none)"))
	}
	for _, n := range ns {
		parts = append(parts, v.renderNetworkRow(n, cursor == idx))
		idx++
	}

	parts = append(parts, "")
	parts = append(parts, lipgloss.NewStyle().Foreground(t.Muted).Render(
		"  ↑/↓ move · ⏎ details · / filter · esc clear · r refresh"))
	if fr := v.filter.Render(t); fr != "" {
		parts = append(parts, fr)
	}
	return fitOrScroll(strings.Join(parts, "\n"), height)
}

func (v *ContainersView) renderContainerRow(c dsm.Container, highlight bool) string {
	t := v.ctx.Theme
	mu := lipgloss.NewStyle().Foreground(t.Muted)
	text := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	status := c.Status
	if status == "" {
		status = c.State
	}
	cpu := fmt.Sprintf("%4.1f%%", c.CPU)
	mem := HumanBytes(uint64(c.Memory))
	if c.Memory == 0 {
		mem = "—"
	}
	return lipgloss.JoinHorizontal(lipgloss.Center,
		caretGlyph(t, highlight), " ",
		padRight(text.Render(c.Name), 22), " ",
		padRight(mu.Render(clipTo(c.Image, 30)), 30), " ",
		padLeft(mu.Render(cpu), 7), " ",
		padLeft(mu.Render(mem), 10), " ",
		t.HealthStyle(status).Render(status),
	)
}

func (v *ContainersView) renderImageRow(i dsm.Image, highlight bool) string {
	t := v.ctx.Theme
	mu := lipgloss.NewStyle().Foreground(t.Muted)
	text := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	repo := i.Repository
	if repo == "" {
		repo = i.RepoTag
	}
	if i.Tag != "" && !strings.Contains(repo, ":") {
		repo += ":" + i.Tag
	}
	used := "unused"
	if i.InUse.Bool() || i.Containers > 0 {
		used = "in use"
	}
	return lipgloss.JoinHorizontal(lipgloss.Center,
		caretGlyph(t, highlight), " ",
		padRight(text.Render(clipTo(repo, 38)), 38), " ",
		padRight(mu.Render(clipTo(i.ID, 14)), 14), " ",
		padLeft(mu.Render(HumanBytes(uint64(i.Size))), 10), " ",
		t.HealthStyle(used).Render(used),
	)
}

func (v *ContainersView) renderNetworkRow(n dsm.DockerNetwork, highlight bool) string {
	t := v.ctx.Theme
	mu := lipgloss.NewStyle().Foreground(t.Muted)
	text := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	scope := n.Scope
	if scope == "" {
		scope = "local"
	}
	return lipgloss.JoinHorizontal(lipgloss.Center,
		caretGlyph(t, highlight), " ",
		padRight(text.Render(n.Name), 22), " ",
		padRight(mu.Render(n.Driver), 10), " ",
		padRight(mu.Render(n.Subnet), 22), " ",
		padRight(mu.Render(n.Gateway), 18), " ",
		padLeft(mu.Render(fmt.Sprintf("%d ctrs", n.Containers)), 10),
	)
}

// Inspect implements tui.Inspector: render the cursored container/image
// /network in the right pane so the operator can read full ids, image
// digests, subnets without leaving the list.
func (v *ContainersView) Inspect(width, height int) string {
	_ = height
	t := v.ctx.Theme
	r, ok := v.current()
	if !ok {
		return muted(t, "  (no selection)")
	}
	switch r.kind {
	case contRowContainer:
		return renderContainerInspect(t, width, v.filterContainers()[r.index])
	case contRowImage:
		return renderDockerImageInspect(t, width, v.filterImages()[r.index])
	case contRowNetwork:
		return renderDockerNetworkInspect(t, width, v.filterNetworks()[r.index])
	}
	return ""
}

func renderContainerDetail(t tui.Theme, width int, c dsm.Container) string {
	if width < 60 {
		width = 60
	}
	status := c.Status
	if status == "" {
		status = c.State
	}
	parts := []string{
		hero(t, width, "▦", c.Name, status, c.Image),
		propsCard(t, width, " Properties ", [][2]string{
			{"ID", c.ID},
			{"Name", c.Name},
			{"Image", c.Image},
			{"Status", status},
			{"Command", c.Command},
			{"Created", c.CreatedAt},
			{"Started", c.StartedAt},
			{"Finished", c.FinishedAt},
			{"CPU", fmt.Sprintf("%.2f%%", c.CPU)},
			{"Memory", HumanBytes(uint64(c.Memory))},
			{"Mem %", fmt.Sprintf("%.2f%%", c.MemoryPct)},
			{"Net up", HumanBytes(uint64(c.NetworkUp))},
			{"Net down", HumanBytes(uint64(c.NetworkDown))},
			{"Block in", HumanBytes(uint64(c.BlockIn))},
			{"Block out", HumanBytes(uint64(c.BlockOut))},
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
		chip("enabled", c.Enabled.Bool()),
		chip("package container", c.IsPackage.Bool()),
	}))
	parts = append(parts, noteCard(t, width, "  esc to go back · read-only — synoctl doesn't run container actions yet"))
	return strings.Join(parts, "\n")
}

func renderDockerImageDetail(t tui.Theme, width int, i dsm.Image) string {
	if width < 60 {
		width = 60
	}
	repo := i.Repository
	if repo == "" {
		repo = i.RepoTag
	}
	used := "unused"
	if i.InUse.Bool() || i.Containers > 0 {
		used = "in use"
	}
	created := ""
	if i.Created > 0 {
		created = time.Unix(i.Created, 0).Format("2006-01-02 15:04")
	}
	parts := []string{
		hero(t, width, "⬚", repo, used, i.Tag),
		propsCard(t, width, " Properties ", [][2]string{
			{"ID", i.ID},
			{"Repository", i.Repository},
			{"Tag", i.Tag},
			{"RepoTag", i.RepoTag},
			{"Size", HumanBytes(uint64(i.Size))},
			{"Virtual size", HumanBytes(uint64(i.VirtualSize))},
			{"Created", created},
			{"Containers", fmt.Sprintf("%d", i.Containers)},
		}),
	}
	if i.Description != "" {
		parts = append(parts, t.Card(false).Width(width-2).Render(
			t.Title().Render(" Description ")+"\n"+
				wrap(lipgloss.NewStyle().Foreground(t.Text), i.Description, width-6)))
	}
	parts = append(parts, noteCard(t, width, "  esc to go back"))
	return strings.Join(parts, "\n")
}

func renderDockerNetworkDetail(t tui.Theme, width int, n dsm.DockerNetwork) string {
	if width < 60 {
		width = 60
	}
	scope := n.Scope
	if scope == "" {
		scope = "local"
	}
	parts := []string{
		hero(t, width, "≋", n.Name, scope, n.Driver),
		propsCard(t, width, " Properties ", [][2]string{
			{"ID", n.ID},
			{"Name", n.Name},
			{"Driver", n.Driver},
			{"Scope", n.Scope},
			{"Subnet", n.Subnet},
			{"Gateway", n.Gateway},
			{"IP range", n.IPRange},
			{"Containers", fmt.Sprintf("%d", n.Containers)},
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
		chip("IPv6", n.EnableIPv6.Bool()),
		chip("internal", n.Internal.Bool()),
	}))
	parts = append(parts, noteCard(t, width, "  esc to go back"))
	return strings.Join(parts, "\n")
}

func renderContainerInspect(t tui.Theme, width int, c dsm.Container) string {
	status := c.Status
	if status == "" {
		status = c.State
	}
	parts := []string{
		t.Title().Render(" Container ") + "\n" +
			lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render("  "+c.Name) + "\n" +
			lipgloss.NewStyle().Foreground(t.Muted).Render("  "+clipTo(c.Image, width-4)) + "\n" +
			"  " + t.HealthStyle(status).Render(status),
	}
	parts = append(parts, "",
		muted(t, "  ID")+"\n  "+clipTo(c.ID, width-4),
		muted(t, "  Command")+"\n  "+clipTo(c.Command, width-4),
		"",
		muted(t, "  CPU       ")+fmt.Sprintf("%5.1f%%", c.CPU),
		muted(t, "  Memory    ")+HumanBytes(uint64(c.Memory)),
		muted(t, "  Mem %     ")+fmt.Sprintf("%5.1f%%", c.MemoryPct),
		muted(t, "  Net up    ")+HumanBytes(uint64(c.NetworkUp)),
		muted(t, "  Net down  ")+HumanBytes(uint64(c.NetworkDown)),
	)
	return strings.Join(parts, "\n")
}

func renderDockerImageInspect(t tui.Theme, width int, i dsm.Image) string {
	repo := i.Repository
	if repo == "" {
		repo = i.RepoTag
	}
	tag := i.Tag
	if tag == "" {
		tag = "—"
	}
	created := "—"
	if i.Created > 0 {
		created = time.Unix(i.Created, 0).Format("2006-01-02 15:04")
	}
	return strings.Join([]string{
		t.Title().Render(" Image "),
		lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render("  " + clipTo(repo, width-4)),
		lipgloss.NewStyle().Foreground(t.Muted).Render("  " + tag),
		"",
		muted(t, "  ID") + "\n  " + clipTo(i.ID, width-4),
		"",
		muted(t, "  Size       ") + HumanBytes(uint64(i.Size)),
		muted(t, "  Virtual    ") + HumanBytes(uint64(i.VirtualSize)),
		muted(t, "  Containers ") + fmt.Sprintf("%d", i.Containers),
		muted(t, "  Created    ") + created,
	}, "\n")
}

func renderDockerNetworkInspect(t tui.Theme, width int, n dsm.DockerNetwork) string {
	_ = width
	scope := n.Scope
	if scope == "" {
		scope = "local"
	}
	return strings.Join([]string{
		t.Title().Render(" Network "),
		lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render("  " + n.Name),
		lipgloss.NewStyle().Foreground(t.Muted).Render("  " + n.Driver + " · " + scope),
		"",
		muted(t, "  Subnet   ") + n.Subnet,
		muted(t, "  Gateway  ") + n.Gateway,
		muted(t, "  IP range ") + n.IPRange,
		muted(t, "  Used by  ") + fmt.Sprintf("%d containers", n.Containers),
	}, "\n")
}

// emptyStateCard is the standard "not installed / nothing here yet"
// placeholder. We render it via the same card primitive as the rest of
// the views so it sits well next to populated rows on partial-empty
// pages too.
func emptyStateCard(t tui.Theme, width int, title, headline, hint string) string {
	body := lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render("  "+title) + "\n\n" +
		lipgloss.NewStyle().Foreground(t.Text).Render("  "+headline) + "\n" +
		lipgloss.NewStyle().Foreground(t.Faint).Render("  "+hint)
	return t.Card(false).Width(width - 2).Render(body)
}
