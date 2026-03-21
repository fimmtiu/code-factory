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
func (np *NavigatorPane) SetUnits(units []*models.WorkUnit) {
	// Build a map for quick lookup
	byID := make(map[string]*TreeNode, len(units))
	for _, u := range units {
		node := &TreeNode{
			unit:     u,
			expanded: true, // start expanded
		}
		byID[u.Identifier] = node
	}

	// Attach children to parents; collect roots
	np.tree = nil
	for _, u := range units {
		node := byID[u.Identifier]
		if u.Parent == "" {
			node.depth = 0
			np.tree = append(np.tree, node)
		} else if parent, ok := byID[u.Parent]; ok {
			node.depth = parent.depth + 1
			parent.children = append(parent.children, node)
		} else {
			// Orphan: parent not in list, treat as root
			node.depth = 0
			np.tree = append(np.tree, node)
		}
	}

	// Recompute depths after building tree (since map iteration order varies)
	for _, root := range np.tree {
		setDepths(root, 0)
	}

	np.rebuildVisible()

	// Clamp cursor
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
func (np NavigatorPane) View(width, height int) string {
	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("4")).
		Foreground(lipgloss.Color("15"))
	projectStyle := lipgloss.NewStyle().Bold(true)
	ticketStyle := lipgloss.NewStyle()

	var sb strings.Builder
	visibleLines := 0

	for i, node := range np.nodes {
		if visibleLines >= height {
			break
		}

		indent := strings.Repeat("  ", node.depth)

		var prefix string
		if node.unit.IsProject {
			if len(node.children) > 0 {
				if node.expanded {
					prefix = "▼ "
				} else {
					prefix = "▶ "
				}
			} else {
				prefix = "  "
			}
		} else {
			prefix = "• "
		}

		label := indent + prefix + node.unit.Identifier + " [" + node.unit.Status + "]"

		// Trim to width
		if width > 2 && len(label) > width-2 {
			label = label[:width-2]
		}

		var line string
		if i == np.cursor {
			line = selectedStyle.Render(label)
		} else if node.unit.IsProject {
			line = projectStyle.Render(label)
		} else {
			line = ticketStyle.Render(label)
		}

		if visibleLines > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(line)
		visibleLines++
	}

	paneStyle := lipgloss.NewStyle().
		Width(width).
		Height(height)

	return paneStyle.Render(sb.String())
}

// max returns the larger of a and b.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
