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

var (
	// Pane borders
	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(lipgloss.Color("12")) // blue

	unfocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("240")) // grey

	// Status pane always unfocused
	statusPaneStyle = unfocusedBorderStyle

	// Tree item styles
	treeBlockedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	treeDoneStyle     = lipgloss.NewStyle().Underline(true)
	treeDefaultStyle  = lipgloss.NewStyle()
	treeSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("230"))

	// Detail pane label
	detailLabelStyle = lipgloss.NewStyle().Bold(true)
)

// Fixed width of the status pane (including border)
const statusPaneWidth = 28

// ── Messages ─────────────────────────────────────────────────────────────────

type projectRefreshMsg struct {
	units []*models.WorkUnit
	stats db.TicketStats
}

type projectDescriptionSavedMsg struct{}

// openChangeRequestDialogMsg requests the root model to open the change
// request dialog for the given work unit.
type openChangeRequestDialogMsg struct {
	wu *models.WorkUnit
}

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
		return projectRefreshMsg{units: units, stats: stats}
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

func (v ProjectView) scheduledRefreshCmd() tea.Cmd {
	d := time.Duration(v.waitSecs) * time.Second
	if d <= 0 {
		d = 5 * time.Second
	}
	return tea.Tick(d, func(time.Time) tea.Msg {
		return projectRefreshMsg{} // will trigger another fetch
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

// ── Update ────────────────────────────────────────────────────────────────────

func (v ProjectView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.clampScroll()
		return v, nil

	case projectRefreshMsg:
		// If this is a ticker ping (empty units), fetch real data.
		if msg.units == nil && msg.stats == (db.TicketStats{}) {
			return v, v.fetchCmd()
		}
		v.units = msg.units
		v.stats = msg.stats
		v.treeNodes = buildTree(v.units)
		if v.treeSelected >= len(v.treeNodes) {
			v.treeSelected = max(0, len(v.treeNodes)-1)
		}
		v.clampScroll()
		return v, v.scheduledRefreshCmd()

	case projectDescriptionSavedMsg:
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
	treeH := v.treeHeight()

	switch msg.String() {
	case "up":
		if v.treeSelected > 0 {
			v.treeSelected--
			v.clampScroll()
		}
	case "down":
		if v.treeSelected < len(v.treeNodes)-1 {
			v.treeSelected++
			v.clampScroll()
		}
	case "pgup":
		v.treeSelected -= treeH
		if v.treeSelected < 0 {
			v.treeSelected = 0
		}
		v.clampScroll()
	case "pgdown":
		v.treeSelected += treeH
		if v.treeSelected >= len(v.treeNodes) {
			v.treeSelected = max(0, len(v.treeNodes)-1)
		}
		v.clampScroll()
	case "tab":
		v.focus = focusDetail
	case "enter":
		return v.openChangeRequestDialog()
	case "t", "T":
		return v.openTerminal()
	case "e", "E":
		return v.openEditor()
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
		return v.openChangeRequestDialog()
	case "t", "T":
		return v.openTerminal()
	case "e", "E":
		return v.openEditor()
	}
	return v, nil
}

func (v ProjectView) openChangeRequestDialog() (tea.Model, tea.Cmd) {
	if len(v.treeNodes) == 0 {
		return v, nil
	}
	wu := v.treeNodes[v.treeSelected].wu
	// Projects don't have change requests: no-op
	if wu.IsProject {
		return v, nil
	}
	return v, func() tea.Msg {
		return openChangeRequestDialogMsg{wu: wu}
	}
}

func (v ProjectView) openTerminal() (tea.Model, tea.Cmd) {
	if len(v.treeNodes) == 0 {
		return v, nil
	}
	wu := v.treeNodes[v.treeSelected].wu
	dir, err := storage.WorktreePathForIdentifier(wu.Identifier)
	if err != nil {
		return v, nil
	}
	_ = util.OpenTerminal(dir)
	return v, nil
}

func (v ProjectView) openEditor() (tea.Model, tea.Cmd) {
	if len(v.treeNodes) == 0 {
		return v, nil
	}
	wu := v.treeNodes[v.treeSelected].wu
	newDesc, err := util.EditText(wu.Description)
	if err != nil {
		return v, nil
	}
	identifier := wu.Identifier
	database := v.database
	return v, func() tea.Msg {
		if err := database.UpdateDescription(identifier, newDesc); err != nil {
			return nil
		}
		return projectDescriptionSavedMsg{}
	}
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
	maxOffset := len(v.treeNodes) - h
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

	// ── Tree pane ──
	treeContent := v.renderTreeContent()
	treeInnerW := v.treeWidth()
	treeBorderStyle := unfocusedBorderStyle
	if v.focus == focusTree {
		treeBorderStyle = focusedBorderStyle
	}
	treePane := treeBorderStyle.
		Width(treeInnerW).
		Height(topH - 2).
		Render(treeContent)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, statusPane, treePane)

	// ── Detail pane ──
	detailContent := v.renderDetailContent()
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

// renderStatusContent returns the text content for the status pane.
func (v ProjectView) renderStatusContent() string {
	total := v.stats.Total
	done := v.stats.Done
	open := total - done
	pct := 0
	if total > 0 {
		pct = done * 100 / total
	}
	stats := fmt.Sprintf("Tickets: %d\nOpen:    %d\nDone:    %d%%", total, open, pct)
	return repoNameStyle.Render(v.repoName) + "\n\n" + stats
}

// renderTreeContent returns the text content for the tree pane.
func (v ProjectView) renderTreeContent() string {
	if len(v.treeNodes) == 0 {
		return "(no work units)"
	}

	h := v.treeHeight()
	w := v.treeWidth()
	total := len(v.treeNodes)

	var sb strings.Builder
	end := v.treeOffset + h
	if end > total {
		end = total
	}

	for i := v.treeOffset; i < end; i++ {
		node := v.treeNodes[i]
		isLast := node.wu.IsProject || i+1 >= total || v.treeNodes[i+1].depth < node.depth
		label := treeLabel(node, isLast)

		if lipgloss.Width(label) > w {
			runes := []rune(label)
			if w > 1 {
				label = string(runes[:w-1]) + "…"
			} else {
				label = string(runes[:w])
			}
		}

		var baseStyle lipgloss.Style
		if i == v.treeSelected {
			baseStyle = treeSelectedStyle.Width(w)
		} else if node.wu.Phase == models.PhaseBlocked {
			baseStyle = treeBlockedStyle
		} else if node.wu.Phase == models.PhaseDone || (node.wu.IsProject && node.wu.Phase == models.ProjectPhaseDone) {
			baseStyle = treeDoneStyle
		} else {
			baseStyle = treeDefaultStyle
		}
		if node.wu.IsProject {
			baseStyle = baseStyle.Bold(true)
		}
		styled := baseStyle.Render(label)

		sb.WriteString(styled)
		if i < end-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// detailLines returns the full content of the detail pane as individual lines.
func (v ProjectView) detailLines() []string {
	if len(v.treeNodes) == 0 {
		return []string{"(no selection)"}
	}
	wu := v.treeNodes[v.treeSelected].wu
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
	lines := v.detailLines()
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
		{Key: "Enter", Description: "Open change request dialog (tickets only)"},
		{Key: "T", Description: "Open terminal in work unit worktree"},
		{Key: "E", Description: "Edit work unit description"},
	}
}
