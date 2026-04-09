package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/storage"
	"github.com/fimmtiu/code-factory/internal/util"
)

// ── Focus ─────────────────────────────────────────────────────────────────────

type projectFocus int

const (
	focusTree projectFocus = iota
	focusDetail
)

// ── Styles ───────────────────────────────────────────────────────────────────

// accentBorder is a DoubleBorder variant that replaces the left edge with a
// solid half-block character (▌) to draw a coloured accent bar on the focused pane.
var accentBorder = func() lipgloss.Border {
	b := lipgloss.DoubleBorder()
	b.Left = "▌"
	b.TopLeft = "╭"
	b.BottomLeft = "╰"
	return b
}()

var (
	// Pane borders
	focusedBorderStyle = lipgloss.NewStyle().
				Border(accentBorder).
				BorderForeground(lipgloss.Color("12")) // blue

	unfocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(colourMuted) // grey

	// Status pane always unfocused
	statusPaneStyle = unfocusedBorderStyle

	// Tree item styles
	treeBlockedStyle  = lipgloss.NewStyle().Foreground(colourMuted)
	treeDoneStyle     = lipgloss.NewStyle().Underline(true)
	treeDefaultStyle  = lipgloss.NewStyle()
	treeSelectedStyle = lipgloss.NewStyle().
				Background(colourPrimary).
				Foreground(colourOnPrimary)

	// Phase badge styles (keyed by ticket phase)
	phaseBadgeStyles = map[models.TicketPhase]lipgloss.Style{
		models.PhaseImplement: lipgloss.NewStyle().Foreground(lipgloss.Color("37")),
		models.PhaseRefactor:  lipgloss.NewStyle().Foreground(lipgloss.Color("166")),
		models.PhaseReview:    lipgloss.NewStyle().Foreground(lipgloss.Color("69")),
		models.PhaseRespond:   lipgloss.NewStyle().Foreground(lipgloss.Color("135")),
		models.PhaseBlocked:   lipgloss.NewStyle().Foreground(lipgloss.Color("124")),
		models.PhaseDone:      lipgloss.NewStyle().Foreground(lipgloss.Color("28")),
	}

	// Detail pane label
	detailLabelStyle = lipgloss.NewStyle().Bold(true)

	// Progress bar segment styles
	progressFilledStyle = lipgloss.NewStyle().Foreground(colourSuccess)
	progressEmptyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)

// Fixed width of the status pane (including border)
const statusPaneWidth = 28

// ── Messages ─────────────────────────────────────────────────────────────────

type projectRefreshMsg struct {
	units   []*models.WorkUnit
	stats   db.TicketStats
	fetched bool // true when this is a DB fetch result (even if units is nil/empty)
}

type projectDescriptionSavedMsg struct{}

// ── Tree node ─────────────────────────────────────────────────────────────────

// treeNode is a flattened row in the tree, with its depth and the work unit.
type treeNode struct {
	wu    *models.WorkUnit
	depth int
}

// ── ProjectView ───────────────────────────────────────────────────────────────

// ProjectView is the three-pane project view.
type ProjectView struct {
	database *db.DB
	waitSecs int
	repoName string // basename of the repo root, shown in the status pane

	// Window dimensions (set by WindowSizeMsg broadcast from root)
	width  int
	height int

	// Data
	units []*models.WorkUnit
	stats db.TicketStats

	// Tree state
	treeNodes    []treeNode
	treeSelected int
	treeOffset   int // first visible row index

	// Filter state
	filterText string
	filtering  bool

	// Detail state
	detailOffset int // first visible line index

	// Focus
	focus projectFocus
}

// NewProjectView creates a new ProjectView.
func NewProjectView(database *db.DB, waitSecs int) ProjectView {
	repoName := ""
	if root, err := storage.FindRepoRoot("."); err == nil {
		repoName = filepath.Base(root)
	}
	return ProjectView{
		database: database,
		waitSecs: waitSecs,
		repoName: repoName,
	}
}

// Init fetches initial data and schedules the first periodic refresh.
func (v ProjectView) Init() tea.Cmd {
	return tea.Batch(v.fetchCmd(), v.tickCmd())
}

// ── Commands ──────────────────────────────────────────────────────────────────

// fetchCmd loads data from the database and returns it as a projectRefreshMsg.
func (v ProjectView) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		units, err := v.database.Status()
		if err != nil {
			units = nil
		}
		stats, err := v.database.TicketStats()
		if err != nil {
			stats = db.TicketStats{}
		}
		return projectRefreshMsg{units: units, stats: stats, fetched: true}
	}
}

// tickCmd schedules a data refresh after waitSecs seconds.
func (v ProjectView) tickCmd() tea.Cmd {
	d := time.Duration(v.waitSecs) * time.Second
	if d <= 0 {
		d = 5 * time.Second
	}
	return tea.Tick(d, func(time.Time) tea.Msg {
		return projectRefreshMsg{}
	})
}

// ── Tree building ─────────────────────────────────────────────────────────────

// buildTree converts the flat list of work units into a sorted, depth-annotated
// slice of treeNodes.
func buildTree(units []*models.WorkUnit) []treeNode {
	if len(units) == 0 {
		return nil
	}

	byID := make(map[string]*models.WorkUnit, len(units))
	for _, wu := range units {
		byID[wu.Identifier] = wu
	}

	// Roots are work units with no parent present in the set.
	var roots []*models.WorkUnit
	for _, wu := range units {
		if wu.Parent == "" || byID[wu.Parent] == nil {
			roots = append(roots, wu)
		}
	}

	sort.Slice(roots, func(i, j int) bool {
		return roots[i].Identifier < roots[j].Identifier
	})

	var result []treeNode
	var walk func(wu *models.WorkUnit, depth int)
	walk = func(wu *models.WorkUnit, depth int) {
		result = append(result, treeNode{wu: wu, depth: depth})

		var children []*models.WorkUnit
		for _, u := range units {
			if u.Parent == wu.Identifier {
				children = append(children, u)
			}
		}
		sort.Slice(children, func(i, j int) bool {
			return children[i].Identifier < children[j].Identifier
		})
		for _, child := range children {
			walk(child, depth+1)
		}
	}

	for _, root := range roots {
		walk(root, 0)
	}

	return result
}

// treeLabel returns the display label for a tree node row.
// isLast is true when the node is a project or the last child of its parent,
// which causes a └── connector to be used instead of ├──.
func treeLabel(node treeNode, isLast bool) string {
	if node.depth == 0 {
		return node.wu.Identifier
	}
	prefix := strings.Repeat("    ", node.depth-1)
	connector := "├── "
	if isLast {
		connector = "└── "
	}
	return prefix + connector + node.wu.Identifier
}

// ── Filter ────────────────────────────────────────────────────────────────────

// filteredTreeNodes returns the subset of treeNodes matching filterText.
// When filterText is empty, the full slice is returned as-is.
// For non-empty queries, a node matches if its identifier or description
// contains the query (case-insensitive). Parent nodes of any match are also
// included so the tree hierarchy is preserved.
func (v ProjectView) filteredTreeNodes() []treeNode {
	if v.filterText == "" {
		return v.treeNodes
	}
	query := strings.ToLower(v.filterText)
	matched := make([]bool, len(v.treeNodes))

	// Mark directly matching nodes.
	for i, node := range v.treeNodes {
		id := strings.ToLower(node.wu.Identifier)
		desc := strings.ToLower(node.wu.Description)
		if strings.Contains(id, query) || strings.Contains(desc, query) {
			matched[i] = true
		}
	}

	// Build a parent index in O(n). In pre-order layout, the parent of a
	// node at depth d is the most recently seen node at depth d-1.
	parentIdx := make([]int, len(v.treeNodes))
	// Stack tracks the index of the most recent ancestor at each depth.
	stack := make([]int, 0, 8)
	for i, node := range v.treeNodes {
		d := node.depth
		// Trim to current depth; stack[d-1] is the parent.
		stack = append(stack[:d], i)
		if d == 0 {
			parentIdx[i] = -1 // root has no parent
		} else {
			parentIdx[i] = stack[d-1]
		}
	}

	// For each matched node, walk up via parentIdx to mark ancestors.
	for i := range v.treeNodes {
		if !matched[i] {
			continue
		}
		for j := parentIdx[i]; j >= 0 && !matched[j]; j = parentIdx[j] {
			matched[j] = true
		}
	}

	result := make([]treeNode, 0, len(v.treeNodes))
	for i, node := range v.treeNodes {
		if matched[i] {
			result = append(result, node)
		}
	}
	return result
}

// handleFilterInput processes a key press while filtering is active.
func (v ProjectView) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	hadFilter := v.filterText != ""
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		v.filtering = false
		v.filterText = ""
		v.treeSelected = 0
		v.treeOffset = 0
		if hadFilter {
			cmd = ShowNotification("Filter cleared")
		}
		return v, cmd
	case "backspace":
		runes := []rune(v.filterText)
		if len(runes) > 0 {
			v.filterText = string(runes[:len(runes)-1])
		}
	default:
		r := []rune(msg.String())
		if len(r) == 1 && r[0] >= 32 {
			v.filterText += string(r)
		}
	}

	v.treeSelected = 0
	v.treeOffset = 0

	if v.filterText != "" {
		cmd = ShowNotification(`Filtering to "` + v.filterText + `"`)
	}
	return v, cmd
}

// ── Update ────────────────────────────────────────────────────────────────────

func (v ProjectView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.clampScroll()
		return v, nil

	case projectRefreshMsg:
		if !msg.fetched {
			return v, v.fetchCmd()
		}
		v.units = msg.units
		v.stats = msg.stats
		v.treeNodes = buildTree(v.units)
		if v.treeSelected >= len(v.treeNodes) {
			v.treeSelected = max(0, len(v.treeNodes)-1)
		}
		v.clampScroll()
		return v, v.tickCmd()

	case projectDescriptionSavedMsg:
		return v, v.fetchCmd()

	case phaseSetMsg:
		return v, v.fetchCmd()

	case tea.KeyMsg:
		switch v.focus {
		case focusTree:
			return v.updateTreeKey(msg)
		case focusDetail:
			return v.updateDetailKey(msg)
		}
	}

	return v, nil
}

func (v ProjectView) updateTreeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if v.filtering {
		return v.handleFilterInput(msg)
	}

	treeH := v.treeHeight()

	switch msg.String() {
	case "up":
		if v.treeSelected > 0 {
			v.treeSelected--
			v.detailOffset = 0
			v.clampScroll()
		}
	case "down":
		if v.treeSelected < len(v.filteredTreeNodes())-1 {
			v.treeSelected++
			v.detailOffset = 0
			v.clampScroll()
		}
	case "pgup":
		v.treeSelected -= treeH
		if v.treeSelected < 0 {
			v.treeSelected = 0
		}
		v.detailOffset = 0
		v.clampScroll()
	case "pgdown":
		v.treeSelected += treeH
		if v.treeSelected >= len(v.filteredTreeNodes()) {
			v.treeSelected = max(0, len(v.filteredTreeNodes())-1)
		}
		v.detailOffset = 0
		v.clampScroll()
	case "tab":
		v.focus = focusDetail
	case "enter":
		return v.openTicketDialog()
	case "t", "T":
		return v.openTerminal()
	case "e", "E":
		return v.openEditor()
	case "g", "G":
		return v.openDiffView()
	case "p", "P":
		return v.openPhasePicker()
	case "/":
		v.filtering = true
		return v, nil
	}
	return v, nil
}

func (v ProjectView) updateDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	detailH := v.detailHeight()

	switch msg.String() {
	case "up":
		if v.detailOffset > 0 {
			v.detailOffset--
		}
	case "down":
		v.detailOffset++
		v.clampDetailScroll()
	case "pgup":
		v.detailOffset -= detailH
		if v.detailOffset < 0 {
			v.detailOffset = 0
		}
	case "pgdown":
		v.detailOffset += detailH
		v.clampDetailScroll()
	case "tab":
		v.focus = focusTree
	case "enter":
		return v.openTicketDialog()
	case "t", "T":
		return v.openTerminal()
	case "e", "E":
		return v.openEditor()
	case "g", "G":
		return v.openDiffView()
	}
	return v, nil
}

func (v ProjectView) openTicketDialog() (tea.Model, tea.Cmd) {
	nodes := v.filteredTreeNodes()
	if len(nodes) == 0 {
		return v, nil
	}
	wu := nodes[v.treeSelected].wu
	if wu.IsProject {
		return v, nil
	}
	return v, func() tea.Msg { return openTicketDialogMsg{wu: wu} }
}

func (v ProjectView) openPhasePicker() (tea.Model, tea.Cmd) {
	nodes := v.filteredTreeNodes()
	if len(nodes) == 0 {
		return v, nil
	}
	wu := nodes[v.treeSelected].wu
	if wu.IsProject || (wu.Status != models.StatusIdle && wu.Status != models.StatusUserReview) {
		return v, nil
	}
	return v, func() tea.Msg { return openPhasePickerMsg{wu: wu} }
}

func (v ProjectView) openTerminal() (tea.Model, tea.Cmd) {
	nodes := v.filteredTreeNodes()
	if len(nodes) == 0 {
		return v, nil
	}
	wu := nodes[v.treeSelected].wu
	dir, err := storage.WorktreePathForIdentifier(wu.Identifier)
	if err != nil {
		return v, nil
	}
	_ = util.OpenTerminal(dir)
	return v, nil
}

func (v ProjectView) openDiffView() (tea.Model, tea.Cmd) {
	nodes := v.filteredTreeNodes()
	if len(nodes) == 0 {
		return v, nil
	}
	wu := nodes[v.treeSelected].wu
	identifier := wu.Identifier
	phase := string(wu.Phase)
	isProject := wu.IsProject
	return v, func() tea.Msg {
		return openDiffViewMsg{identifier: identifier, phase: phase, isProject: isProject}
	}
}

func (v ProjectView) openEditor() (tea.Model, tea.Cmd) {
	nodes := v.filteredTreeNodes()
	if len(nodes) == 0 || v.treeSelected >= len(nodes) {
		return v, nil
	}
	wu := nodes[v.treeSelected].wu
	currentDesc := wu.Description
	identifier := wu.Identifier
	database := v.database
	return v, wrapEditorCmd(func() tea.Msg {
		newDesc, err := util.EditText(currentDesc)
		if err != nil {
			return nil
		}
		if err := database.UpdateDescription(identifier, newDesc); err != nil {
			return nil
		}
		return projectDescriptionSavedMsg{}
	})
}

// ── Scroll helpers ────────────────────────────────────────────────────────────

func (v *ProjectView) clampScroll() {
	h := v.treeHeight()
	if h <= 0 {
		return
	}
	// Ensure selected is visible.
	if v.treeSelected < v.treeOffset {
		v.treeOffset = v.treeSelected
	}
	if v.treeSelected >= v.treeOffset+h {
		v.treeOffset = v.treeSelected - h + 1
	}
	// Clamp offset to valid range.
	maxOffset := len(v.filteredTreeNodes()) - h
	if maxOffset < 0 {
		maxOffset = 0
	}
	if v.treeOffset > maxOffset {
		v.treeOffset = maxOffset
	}
	if v.treeOffset < 0 {
		v.treeOffset = 0
	}
}

func (v *ProjectView) clampDetailScroll() {
	lines := v.detailLines()
	h := v.detailHeight()
	maxOffset := len(lines) - h
	if maxOffset < 0 {
		maxOffset = 0
	}
	if v.detailOffset > maxOffset {
		v.detailOffset = maxOffset
	}
	if v.detailOffset < 0 {
		v.detailOffset = 0
	}
}

// ── Dimension helpers ─────────────────────────────────────────────────────────

// chromeHeight is the number of lines consumed by the app header and footer
// (tab bar + help hint), so the view can compute the available body area.
const chromeHeight = 2

// topHalfHeight returns the height available for the top panes (status + tree).
// It is half of the body area (total height minus chrome).
func (v ProjectView) topHalfHeight() int {
	body := v.height - chromeHeight
	if body < 2 {
		body = 2
	}
	return body / 2
}

// treeHeight returns the number of visible rows in the tree pane (inner content).
func (v ProjectView) treeHeight() int {
	h := v.topHalfHeight() - 2
	if h < 1 {
		h = 1
	}
	return h
}

// detailHeight returns the number of visible lines in the detail pane (inner).
func (v ProjectView) detailHeight() int {
	body := v.height - chromeHeight
	if body < 2 {
		body = 2
	}
	h := body - v.topHalfHeight() - 2
	if h < 1 {
		h = 1
	}
	return h
}

// treeWidth returns the inner content width for the tree pane.
// statusPaneWidth already includes its borders; the -2 accounts for the tree pane's borders.
func (v ProjectView) treeWidth() int {
	w := v.width - statusPaneWidth - 2
	if w < 1 {
		w = 1
	}
	return w
}

// ── View ──────────────────────────────────────────────────────────────────────

func (v ProjectView) View() string {
	topH := v.topHalfHeight()

	// ── Status pane ──
	statusContent := v.renderStatusContent()
	statusInnerW := statusPaneWidth - 2
	if statusInnerW < 1 {
		statusInnerW = 1
	}
	statusPane := statusPaneStyle.
		Width(statusInnerW).
		Height(topH - 2).
		Render(statusContent)

	// ── Pre-compute filtered tree nodes (used by tree, scrollbar, and detail) ──
	nodes := v.filteredTreeNodes()

	// ── Tree pane ──
	treeContent := v.renderTreeContentFor(nodes)
	treeInnerW := v.treeWidth()
	treeBorderStyle := unfocusedBorderStyle
	if v.focus == focusTree {
		treeBorderStyle = focusedBorderStyle
	}
	treePane := treeBorderStyle.
		Width(treeInnerW).
		Height(topH - 2).
		Render(treeContent)
	treeRightChar := "│"
	if v.focus == focusTree {
		treeRightChar = "║"
	}
	treePane = injectScrollbar(treePane, treeRightChar, "█", v.treeOffset, len(nodes), topH-2)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, statusPane, treePane)

	// ── Detail pane ──
	detailContent := v.renderDetailContentFor(nodes)
	detailInnerW := v.width - 2
	if detailInnerW < 1 {
		detailInnerW = 1
	}
	detailBorderStyle := unfocusedBorderStyle
	if v.focus == focusDetail {
		detailBorderStyle = focusedBorderStyle
	}
	body := v.height - chromeHeight
	if body < 4 {
		body = 4
	}
	bottomH := body - topH
	if bottomH < 3 {
		bottomH = 3
	}
	detailPane := detailBorderStyle.
		Width(detailInnerW).
		Height(bottomH - 2).
		Render(detailContent)

	return lipgloss.JoinVertical(lipgloss.Left, topRow, detailPane)
}

// repoNameStyle renders the repository name bold and underlined.
var repoNameStyle = lipgloss.NewStyle().Bold(true).Underline(true)

// renderProgressBar returns a 10-cell Unicode block progress bar with coloured
// segments followed by the percentage value, e.g. "████░░░░░░ 40%".
func renderProgressBar(pct int) string {
	const barWidth = 10
	filled := pct * barWidth / 100
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled
	bar := progressFilledStyle.Render(strings.Repeat("█", filled)) +
		progressEmptyStyle.Render(strings.Repeat("░", empty))
	return fmt.Sprintf("%s %d%%", bar, pct)
}

// renderStatusContent returns the text content for the status pane.
func (v ProjectView) renderStatusContent() string {
	total := v.stats.Total
	done := v.stats.Done
	open := total - done
	pct := 0
	if total > 0 {
		pct = done * 100 / total
	}
	stats := fmt.Sprintf("Tickets: %d\nOpen:    %d\n", total, open) + renderProgressBar(pct)
	return repoNameStyle.Render(v.repoName) + "\n\n" + stats
}

// renderTreeContent returns the text content for the tree pane.
func (v ProjectView) renderTreeContent() string {
	return v.renderTreeContentFor(v.filteredTreeNodes())
}

// renderTreeContentFor returns the text content for the tree pane using
// pre-computed filtered nodes (avoids redundant filteredTreeNodes calls).
func (v ProjectView) renderTreeContentFor(nodes []treeNode) string {
	if len(nodes) == 0 {
		if v.filterText != "" {
			return "(no matches)"
		}
		return "(no work units)"
	}

	h := v.treeHeight()
	w := v.treeWidth()
	total := len(nodes)

	var sb strings.Builder
	end := v.treeOffset + h
	if end > total {
		end = total
	}

	for i := v.treeOffset; i < end; i++ {
		isLast := nodes[i].wu.IsProject || i+1 >= total || nodes[i+1].depth < nodes[i].depth
		sb.WriteString(v.renderTreeRow(nodes[i], i, isLast, w))
		if i < end-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// renderTreeRow renders a single row in the tree pane, including the
// identifier label, phase badge (for tickets), and styling.
func (v ProjectView) renderTreeRow(node treeNode, idx int, isLast bool, w int) string {
	label := treeLabel(node, isLast)

	// Build phase badge for tickets only.
	badge := ""
	styledBadge := ""
	if !node.wu.IsProject {
		badge = "[" + string(node.wu.Phase) + "]"
		if bs, ok := phaseBadgeStyles[node.wu.Phase]; ok {
			styledBadge = bs.Render(badge)
		} else {
			styledBadge = badge
		}
	}

	// Determine available width for the identifier portion.
	badgeWidth := lipgloss.Width(badge)
	identW := w
	if badge != "" {
		identW = w - badgeWidth - 1
		if identW < 1 {
			identW = 1
		}
	}

	// Truncate label to fit identifier width.
	if lipgloss.Width(label) > identW {
		runes := []rune(label)
		if identW > 1 {
			label = string(runes[:identW-1]) + "…"
		} else {
			label = string(runes[:identW])
		}
	}

	baseStyle := v.treeNodeStyle(node, idx)

	if badge == "" {
		return baseStyle.Width(w).Render(label)
	}

	// Ticket row: render identifier portion, pad to fill, append badge.
	styledLabel := baseStyle.Render(label)
	padLen := w - lipgloss.Width(styledLabel) - badgeWidth
	if padLen < 1 {
		padLen = 1
	}
	pad := strings.Repeat(" ", padLen)
	if idx == v.treeSelected {
		pad = treeSelectedStyle.Render(pad)
	}
	return styledLabel + pad + styledBadge
}

// treeNodeStyle returns the appropriate lipgloss style for a tree node based
// on its phase, project status, and whether it is selected.
func (v ProjectView) treeNodeStyle(node treeNode, idx int) lipgloss.Style {
	var style lipgloss.Style
	switch {
	case idx == v.treeSelected:
		style = treeSelectedStyle
	case node.wu.Phase == models.PhaseBlocked:
		style = treeBlockedStyle
	case node.wu.Phase == models.PhaseDone || (node.wu.IsProject && node.wu.Phase == models.ProjectPhaseDone):
		style = treeDoneStyle
	default:
		style = treeDefaultStyle
	}
	if node.wu.IsProject {
		style = style.Bold(true)
	}
	return style
}

// detailLines returns the full content of the detail pane as individual lines.
func (v ProjectView) detailLines() []string {
	return v.detailLinesFor(v.filteredTreeNodes())
}

// detailLinesFor returns the full content of the detail pane using
// pre-computed filtered nodes (avoids redundant filteredTreeNodes calls).
func (v ProjectView) detailLinesFor(nodes []treeNode) []string {
	if len(nodes) == 0 {
		return []string{"(no selection)"}
	}
	wu := nodes[v.treeSelected].wu
	return buildDetailLines(wu, v.width-2)
}

// buildDetailLines formats work unit details into wrapped lines.
func buildDetailLines(wu *models.WorkUnit, width int) []string {
	var lines []string

	addLabel := func(label, value string) {
		lines = append(lines, detailLabelStyle.Render(label+":")+" "+value)
	}

	if wu.IsProject {
		addLabel("Type", "project")
	} else {
		addLabel("Type", "ticket")
	}
	addLabel("Identifier", wu.Identifier)
	addLabel("Phase", string(wu.Phase))
	if !wu.IsProject {
		addLabel("Status", string(wu.Status))
	}
	if len(wu.Dependencies) > 0 {
		addLabel("Dependencies", strings.Join(wu.Dependencies, ", "))
	}

	lines = append(lines, "")
	lines = append(lines, detailLabelStyle.Render("Description:"))
	descLines := wordWrap(wu.Description, width-2)
	lines = append(lines, descLines...)

	if !wu.IsProject && len(wu.ChangeRequests) > 0 {
		lines = append(lines, "")
		lines = append(lines, detailLabelStyle.Render("Change Requests:"))
		for _, cr := range wu.ChangeRequests {
			lines = append(lines, fmt.Sprintf("  [%s] %s — %s (%s)", cr.Status, cr.CodeLocation, cr.Author, cr.Date.Format("2006-01-02")))
			wrapped := wordWrap("  "+cr.Description, width-4)
			lines = append(lines, wrapped...)
		}
	}

	return lines
}

// renderDetailContent returns the visible slice of detail lines, joined.
func (v ProjectView) renderDetailContent() string {
	return v.renderDetailContentFor(v.filteredTreeNodes())
}

// renderDetailContentFor returns the visible slice of detail lines using
// pre-computed filtered nodes (avoids redundant filteredTreeNodes calls).
func (v ProjectView) renderDetailContentFor(nodes []treeNode) string {
	lines := v.detailLinesFor(nodes)
	h := v.detailHeight()

	start := v.detailOffset
	if start >= len(lines) {
		start = max(0, len(lines)-1)
	}
	end := start + h
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

// ── Word wrap ─────────────────────────────────────────────────────────────────

// wordWrap wraps text to at most width runes per line.
func wordWrap(text string, width int) []string {
	if width <= 0 {
		width = 80
	}
	var result []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			result = append(result, "")
			continue
		}
		line := words[0]
		for _, word := range words[1:] {
			if len(line)+1+len(word) <= width {
				line += " " + word
			} else {
				result = append(result, line)
				line = word
			}
		}
		result = append(result, line)
	}
	return result
}

// ── KeyBindings ───────────────────────────────────────────────────────────────

func (v ProjectView) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "↑/↓", Description: "Navigate / scroll"},
		{Key: "PgUp/PgDn", Description: "Page navigate / scroll"},
		{Key: "Tab", Description: "Switch focus between tree and detail pane"},
		{Key: "Enter", Description: "Open ticket dialog (tickets only)"},
		{Key: "T", Description: "Open terminal in work unit worktree"},
		{Key: "E", Description: "Edit work unit description"},
		{Key: "G", Description: "View diff"},
		{Key: "P", Description: "Set phase (idle tickets only)"},
		{Key: "/", Description: "Filter tree by substring"},
	}
}

func (v ProjectView) Label() string { return "F1:Projects" }
