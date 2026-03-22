package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fimmtiu/tickets/internal/models"
)

func TestNavigatorSetUnits(t *testing.T) {
	np := NavigatorPane{}
	units := sampleUnits()
	np.SetUnits(units)

	if len(np.tree) == 0 {
		t.Error("expected tree to be populated after SetUnits")
	}
}

func TestNavigatorBuildTreeRoots(t *testing.T) {
	np := NavigatorPane{}
	units := []*models.WorkUnit{
		{Identifier: "proj-a", IsProject: true, Status: models.ProjectOpen},
		{Identifier: "proj-b", IsProject: true, Status: models.ProjectOpen},
	}
	np.SetUnits(units)

	if len(np.tree) != 2 {
		t.Errorf("expected 2 root nodes, got %d", len(np.tree))
	}
}

func TestNavigatorBuildTreeChildren(t *testing.T) {
	np := NavigatorPane{}
	units := []*models.WorkUnit{
		{Identifier: "proj-a", IsProject: true, Status: models.ProjectOpen},
		{Identifier: "proj-a/ticket-1", IsProject: false, Status: models.StatusOpen, Parent: "proj-a"},
		{Identifier: "proj-a/ticket-2", IsProject: false, Status: models.StatusOpen, Parent: "proj-a"},
	}
	np.SetUnits(units)

	if len(np.tree) != 1 {
		t.Errorf("expected 1 root node, got %d", len(np.tree))
	}
	if len(np.tree[0].children) != 2 {
		t.Errorf("expected 2 children under proj-a, got %d", len(np.tree[0].children))
	}
}

func TestNavigatorMoveDown(t *testing.T) {
	np := NavigatorPane{}
	units := sampleUnits()
	np.SetUnits(units)

	initialCursor := np.cursor
	np.MoveDown()

	if np.cursor != initialCursor+1 {
		t.Errorf("expected cursor %d after MoveDown, got %d", initialCursor+1, np.cursor)
	}
}

func TestNavigatorMoveDownAtEnd(t *testing.T) {
	np := NavigatorPane{}
	units := []*models.WorkUnit{
		{Identifier: "proj-a", IsProject: true, Status: models.ProjectOpen},
	}
	np.SetUnits(units)

	// At the end, MoveDown should not go past the last item
	np.MoveDown()
	if np.cursor != 0 {
		t.Errorf("expected cursor to stay at 0 (only one item), got %d", np.cursor)
	}
}

func TestNavigatorMoveUp(t *testing.T) {
	np := NavigatorPane{}
	units := sampleUnits()
	np.SetUnits(units)

	np.cursor = 1
	np.MoveUp()

	if np.cursor != 0 {
		t.Errorf("expected cursor 0 after MoveUp, got %d", np.cursor)
	}
}

func TestNavigatorMoveUpAtTop(t *testing.T) {
	np := NavigatorPane{}
	units := sampleUnits()
	np.SetUnits(units)

	np.cursor = 0
	np.MoveUp()

	if np.cursor != 0 {
		t.Errorf("expected cursor to stay at 0 when already at top, got %d", np.cursor)
	}
}

func TestNavigatorSelected(t *testing.T) {
	np := NavigatorPane{}
	units := sampleUnits()
	np.SetUnits(units)

	sel := np.Selected()
	if sel == nil {
		t.Error("expected Selected() to return a work unit")
	}
}

func TestNavigatorToggleExpand(t *testing.T) {
	np := NavigatorPane{}
	units := []*models.WorkUnit{
		{Identifier: "proj-a", IsProject: true, Status: models.ProjectOpen},
		{Identifier: "proj-a/ticket-1", IsProject: false, Status: models.StatusOpen, Parent: "proj-a"},
	}
	np.SetUnits(units)

	// Initially projects should be expanded (or not), toggle should change state
	initialExpanded := np.nodes[0].expanded
	np.ToggleExpand()
	if np.nodes[0].expanded == initialExpanded {
		t.Error("expected ToggleExpand to change expanded state")
	}
}

func TestNavigatorView(t *testing.T) {
	np := NavigatorPane{}
	units := sampleUnits()
	np.SetUnits(units)

	view := np.View(60, 20)
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestNavigatorViewShowsIdentifiers(t *testing.T) {
	np := NavigatorPane{}
	units := []*models.WorkUnit{
		{Identifier: "my-project", IsProject: true, Status: models.ProjectOpen},
	}
	np.SetUnits(units)

	view := np.View(60, 20)
	if !strings.Contains(view, "my-project") {
		t.Errorf("expected view to contain 'my-project', got: %q", view)
	}
}

func TestNavigatorViewCollapsedHidesChildren(t *testing.T) {
	np := NavigatorPane{}
	units := []*models.WorkUnit{
		{Identifier: "proj-a", IsProject: true, Status: models.ProjectOpen},
		{Identifier: "proj-a/ticket-1", IsProject: false, Status: models.StatusOpen, Parent: "proj-a"},
	}
	np.SetUnits(units)

	// Collapse the project
	np.nodes[0].expanded = false
	np.rebuildVisible()

	view := np.View(60, 20)
	if strings.Contains(view, "ticket-1") {
		t.Error("expected collapsed project to hide child ticket-1")
	}
}

// makeTickets builds n flat ticket WorkUnits with identifiers "ticket-0",
// "ticket-1", ..., "ticket-N-1".
func makeTickets(n int) []*models.WorkUnit {
	units := make([]*models.WorkUnit, n)
	for i := range units {
		units[i] = &models.WorkUnit{
			Identifier: fmt.Sprintf("ticket-%d", i),
			IsProject:  false,
			Status:     models.StatusOpen,
		}
	}
	return units
}

func TestNavigatorViewScrollsCursorIntoView(t *testing.T) {
	np := NavigatorPane{}
	np.SetUnits(makeTickets(20))

	// Move cursor past the bottom of a height-5 window.
	np.cursor = 10

	view := np.View(60, 5)

	// The cursor row must be visible.
	if !strings.Contains(view, "ticket-10") {
		t.Error("expected ticket-10 (cursor) to be visible after scrolling")
	}
	// The first node must have scrolled off the top.
	if strings.Contains(view, "ticket-0") {
		t.Error("expected ticket-0 to be scrolled off the top")
	}
}

func TestNavigatorViewTopNodeVisibleAtStart(t *testing.T) {
	np := NavigatorPane{}
	np.SetUnits(makeTickets(20))
	// cursor at 0 — no scrolling should occur

	view := np.View(60, 5)

	if !strings.Contains(view, "ticket-0") {
		t.Error("expected ticket-0 to be visible when cursor is at top")
	}
	if strings.Contains(view, "ticket-5") {
		t.Error("expected ticket-5 to be off-screen when only 5 rows visible")
	}
}

func TestNavigatorViewScrollsBackUpWhenCursorRises(t *testing.T) {
	np := NavigatorPane{}
	np.SetUnits(makeTickets(20))

	// Scroll down so ticket-0 is off-screen, then back up.
	np.cursor = 10
	np.cursor = 0

	view := np.View(60, 5)

	if !strings.Contains(view, "ticket-0") {
		t.Error("expected ticket-0 to be visible again after cursor returned to top")
	}
}

func TestNavigatorDepth(t *testing.T) {
	np := NavigatorPane{}
	units := []*models.WorkUnit{
		{Identifier: "proj-a", IsProject: true, Status: models.ProjectOpen},
		{Identifier: "proj-a/sub", IsProject: true, Status: models.ProjectOpen, Parent: "proj-a"},
		{Identifier: "proj-a/sub/ticket-1", IsProject: false, Status: models.StatusOpen, Parent: "proj-a/sub"},
	}
	np.SetUnits(units)

	// Find the leaf ticket node and check its depth
	var leafNode *TreeNode
	for _, n := range np.nodes {
		if n.unit.Identifier == "proj-a/sub/ticket-1" {
			leafNode = n
			break
		}
	}
	if leafNode == nil {
		t.Fatal("expected to find proj-a/sub/ticket-1 in visible nodes")
	}
	if leafNode.depth != 2 {
		t.Errorf("expected depth 2 for proj-a/sub/ticket-1, got %d", leafNode.depth)
	}
}
