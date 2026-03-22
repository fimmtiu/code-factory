package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/tickets/internal/models"
)

// TreeNode represents a single node in the navigator tree.
type TreeNode struct {
	unit     *models.WorkUnit
	children []*TreeNode
	expanded bool
	depth    int
}

// NavigatorPane renders a tree-formatted view of projects, subprojects, and
// tickets with cursor navigation and collapse/expand support.
type NavigatorPane struct {
	nodes  []*TreeNode // flat list of currently visible nodes
	cursor int
	tree   []*TreeNode // root nodes only
}

// SetUnits rebuilds the tree from the given flat list of work units, using
// each unit's Parent field to establish the hierarchy.
// buildTree constructs a forest of TreeNodes from units, linking children to
// parents and collecting roots. Orphaned nodes (parent not in the list) are
// treated as roots.
func buildTree(units []*models.WorkUnit) []*TreeNode {
	byID := make(map[string]*TreeNode, len(units))
	for _, u := range units {
		byID[u.Identifier] = &TreeNode{unit: u, expanded: true}
	}

	var roots []*TreeNode
	for _, u := range units {
		node := byID[u.Identifier]
		if u.Parent == "" {
			roots = append(roots, node)
		} else if parent, ok := byID[u.Parent]; ok {
			parent.children = append(parent.children, node)
		} else {
			roots = append(roots, node) // orphan: treat as root
		}
	}

	// Correct depths now that the full tree structure is known.
	for _, root := range roots {
		setDepths(root, 0)
	}
	return roots
}

func (np *NavigatorPane) SetUnits(units []*models.WorkUnit) {
	np.tree = buildTree(units)
	np.rebuildVisible()
	if np.cursor >= len(np.nodes) {
		np.cursor = max(0, len(np.nodes)-1)
	}
}

// setDepths recursively sets depth on each node.
func setDepths(node *TreeNode, depth int) {
	node.depth = depth
	for _, child := range node.children {
		setDepths(child, depth+1)
	}
}

// rebuildVisible flattens the tree into the nodes slice, respecting
// expanded/collapsed state.
func (np *NavigatorPane) rebuildVisible() {
	np.nodes = nil
	for _, root := range np.tree {
		collectVisible(&np.nodes, root)
	}
}

// collectVisible appends node and, if expanded, its descendants to out.
func collectVisible(out *[]*TreeNode, node *TreeNode) {
	*out = append(*out, node)
	if node.expanded {
		for _, child := range node.children {
			collectVisible(out, child)
		}
	}
}

// MoveUp moves the cursor one position up, clamped to 0.
func (np *NavigatorPane) MoveUp() {
	if np.cursor > 0 {
		np.cursor--
	}
}

// MoveDown moves the cursor one position down, clamped to len(nodes)-1.
func (np *NavigatorPane) MoveDown() {
	if np.cursor < len(np.nodes)-1 {
		np.cursor++
	}
}

// PageUp moves the cursor up by height rows, clamped to 0.
func (np *NavigatorPane) PageUp(height int) {
	np.cursor -= height
	if np.cursor < 0 {
		np.cursor = 0
	}
}

// PageDown moves the cursor down by height rows, clamped to len(nodes)-1.
func (np *NavigatorPane) PageDown(height int) {
	np.cursor += height
	if np.cursor >= len(np.nodes) {
		np.cursor = len(np.nodes) - 1
	}
	if np.cursor < 0 {
		np.cursor = 0
	}
}

// ToggleExpand collapses or expands the node at the current cursor position.
func (np *NavigatorPane) ToggleExpand() {
	if len(np.nodes) == 0 {
		return
	}
	node := np.nodes[np.cursor]
	if len(node.children) > 0 {
		node.expanded = !node.expanded
		np.rebuildVisible()
		// Clamp cursor after potential removal of rows
		if np.cursor >= len(np.nodes) {
			np.cursor = len(np.nodes) - 1
		}
	}
}

// Selected returns the WorkUnit at the current cursor position, or nil if
// the navigator is empty.
func (np NavigatorPane) Selected() *models.WorkUnit {
	if len(np.nodes) == 0 {
		return nil
	}
	return np.nodes[np.cursor].unit
}

// View renders the navigator pane as a string with the given dimensions.
// nodePrefix returns the tree-decoration prefix for a node (expand/collapse
// arrow for projects, bullet for tickets).
func nodePrefix(node *TreeNode) string {
	if !node.unit.IsProject {
		return "• "
	}
	if len(node.children) == 0 {
		return "  "
	}
	if node.expanded {
		return "▼ "
	}
	return "▶ "
}

// renderNodeLine builds and styles a single tree row. selected determines
// whether the cursor is on this node.
func renderNodeLine(node *TreeNode, selected bool, width int) string {
	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("4")).
		Foreground(lipgloss.Color("15"))
	projectStyle := lipgloss.NewStyle().Bold(true)
	ticketStyle := lipgloss.NewStyle()

	indent := strings.Repeat("  ", node.depth)
	label := indent + nodePrefix(node) + node.unit.Identifier + " [" + node.unit.Status + "]"
	if width > 2 && len(label) > width-2 {
		label = label[:width-2]
	}

	switch {
	case selected:
		return selectedStyle.Render(label)
	case node.unit.IsProject:
		return projectStyle.Render(label)
	default:
		return ticketStyle.Render(label)
	}
}

// View renders the navigator pane with the given outer dimensions.
// focused controls the border highlight colour.
func (np NavigatorPane) View(width, height int, focused bool) string {
	// The border takes 1 row/col on each side; content area is 2 smaller.
	contentHeight := height - 2

	// Scroll the viewport so the cursor is always visible.
	scrollOffset := 0
	if np.cursor >= contentHeight {
		scrollOffset = np.cursor - contentHeight + 1
	}

	var sb strings.Builder
	visibleLines := 0

	for i := scrollOffset; i < len(np.nodes) && visibleLines < contentHeight; i++ {
		if visibleLines > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(renderNodeLine(np.nodes[i], i == np.cursor, width))
		visibleLines++
	}

	borderColor := lipgloss.Color("240")
	if focused {
		borderColor = lipgloss.Color("12")
	}

	paneStyle := lipgloss.NewStyle().
		Width(width - 2).
		Height(contentHeight).
		Border(lipgloss.NormalBorder(), true).
		BorderForeground(borderColor)

	return paneStyle.Render(sb.String())
}

// max returns the larger of a and b.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
