package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// makeTree is a helper that builds a ProjectView with a hand-crafted treeNode
// slice, ready for filteredTreeNodes tests.
func makeTree(nodes []treeNode) ProjectView {
	return ProjectView{treeNodes: nodes}
}

// wu creates a minimal WorkUnit with the given identifier and description.
func wu(identifier, description string) *models.WorkUnit {
	return &models.WorkUnit{Identifier: identifier, Description: description}
}

// identifiers extracts the identifier strings from a treeNode slice.
func identifiers(nodes []treeNode) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.wu.Identifier
	}
	return ids
}

// TestFilteredTreeNodes_NoFilter confirms that an empty filter returns all nodes.
func TestFilteredTreeNodes_NoFilter(t *testing.T) {
	v := makeTree([]treeNode{
		{wu: wu("project-a", ""), depth: 0},
		{wu: wu("project-a/ticket-1", ""), depth: 1},
	})
	got := v.filteredTreeNodes()
	if len(got) != 2 {
		t.Errorf("expected 2 nodes with no filter, got %d", len(got))
	}
}

// TestFilteredTreeNodes_MatchAndAncestors verifies that a matched node and its
// direct ancestors are included, but siblings and cousins are not.
func TestFilteredTreeNodes_MatchAndAncestors(t *testing.T) {
	//   project-a          depth 0
	//     sub-x            depth 1  ← sibling of sub-y; must NOT appear
	//     sub-y            depth 1  ← ancestor of match; must appear
	//       ticket-match   depth 2  ← matches query
	//       ticket-other   depth 2  ← child sibling; must NOT appear
	v := makeTree([]treeNode{
		{wu: wu("project-a", ""), depth: 0},
		{wu: wu("project-a/sub-x", ""), depth: 1},
		{wu: wu("project-a/sub-y", ""), depth: 1},
		{wu: wu("project-a/sub-y/ticket-match", "contains needle"), depth: 2},
		{wu: wu("project-a/sub-y/ticket-other", ""), depth: 2},
	})
	v.filterText = "needle"

	got := identifiers(v.filteredTreeNodes())
	want := []string{
		"project-a",
		"project-a/sub-y",
		"project-a/sub-y/ticket-match",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestFilteredTreeNodes_MultipleMatches verifies that two matches in different
// subtrees each pull in their own ancestors without cross-contamination.
func TestFilteredTreeNodes_MultipleMatches(t *testing.T) {
	//   project-a          depth 0
	//     sub-x            depth 1  ← ancestor of first match
	//       ticket-m1      depth 2  ← matches
	//       ticket-sibling depth 2  ← sibling; must NOT appear
	//     sub-y            depth 1  ← ancestor of second match
	//       ticket-m2      depth 2  ← matches
	v := makeTree([]treeNode{
		{wu: wu("project-a", ""), depth: 0},
		{wu: wu("project-a/sub-x", ""), depth: 1},
		{wu: wu("project-a/sub-x/ticket-m1", "needle"), depth: 2},
		{wu: wu("project-a/sub-x/ticket-sibling", ""), depth: 2},
		{wu: wu("project-a/sub-y", ""), depth: 1},
		{wu: wu("project-a/sub-y/ticket-m2", "needle"), depth: 2},
	})
	v.filterText = "needle"

	got := identifiers(v.filteredTreeNodes())
	want := []string{
		"project-a",
		"project-a/sub-x",
		"project-a/sub-x/ticket-m1",
		"project-a/sub-y",
		"project-a/sub-y/ticket-m2",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestFilteredTreeNodes_NoMatch returns empty slice when nothing matches.
func TestFilteredTreeNodes_NoMatch(t *testing.T) {
	v := makeTree([]treeNode{
		{wu: wu("project-a", ""), depth: 0},
		{wu: wu("project-a/ticket-1", ""), depth: 1},
	})
	v.filterText = "zzznomatch"
	got := v.filteredTreeNodes()
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", identifiers(got))
	}
}

// TestFilteredTreeNodes_ProjectMatchDoesNotPullChildren verifies that when a
// project itself matches, its children are NOT automatically included.
func TestFilteredTreeNodes_ProjectMatchDoesNotPullChildren(t *testing.T) {
	v := makeTree([]treeNode{
		{wu: wu("matching-project", "needle"), depth: 0},
		{wu: wu("matching-project/child-1", ""), depth: 1},
		{wu: wu("matching-project/child-2", ""), depth: 1},
	})
	v.filterText = "needle"

	got := identifiers(v.filteredTreeNodes())
	want := []string{"matching-project"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got[0] != want[0] {
		t.Errorf("got %q, want %q", got[0], want[0])
	}
}

// ── Theme migration tests ────────────────────────────────────────────────────

// TestRenderProgressBar_UsesThemeStyles verifies that renderProgressBar produces
// output using theme.Current() and doesn't panic when the theme is swapped.
func TestRenderProgressBar_UsesThemeStyles(t *testing.T) {
	saved := theme.Current()
	t.Cleanup(func() { theme.SetCurrent(saved) })

	theme.SetCurrent(theme.Tan())
	result := renderProgressBar(50)
	if result == "" {
		t.Error("renderProgressBar returned empty string")
	}
	if !strings.Contains(result, "50%") {
		t.Errorf("renderProgressBar(50) should contain '50%%', got %q", result)
	}

	// Verify a different theme also produces valid output without panic.
	alt := theme.Tan()
	alt.ProgressFilledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	theme.SetCurrent(alt)
	result2 := renderProgressBar(75)
	if !strings.Contains(result2, "75%") {
		t.Errorf("renderProgressBar(75) with alt theme should contain '75%%', got %q", result2)
	}
}

// TestTreeNodeStyle_UsesThemeStyles verifies treeNodeStyle returns styles
// from the current theme.
func TestTreeNodeStyle_UsesThemeStyles(t *testing.T) {
	saved := theme.Current()
	t.Cleanup(func() { theme.SetCurrent(saved) })
	theme.SetCurrent(theme.Tan())

	v := ProjectView{treeSelected: 0}

	selectedNode := treeNode{wu: &models.WorkUnit{Identifier: "t1"}, depth: 1}
	style := v.treeNodeStyle(selectedNode, 0)
	rendered := style.Render("test")
	if rendered == "" {
		t.Error("treeNodeStyle for selected node returned empty render")
	}

	blockedNode := treeNode{
		wu:    &models.WorkUnit{Identifier: "t2", Phase: models.PhaseBlocked},
		depth: 1,
	}
	style = v.treeNodeStyle(blockedNode, 1)
	rendered = style.Render("test")
	if rendered == "" {
		t.Error("treeNodeStyle for blocked node returned empty render")
	}

	doneNode := treeNode{
		wu:    &models.WorkUnit{Identifier: "t3", Phase: models.PhaseDone},
		depth: 1,
	}
	style = v.treeNodeStyle(doneNode, 1)
	rendered = style.Render("test")
	if rendered == "" {
		t.Error("treeNodeStyle for done node returned empty render")
	}
}

// TestRenderTreeRow_UsesPhaseBadgeStyles verifies that phase badge styles
// from the theme are used when rendering tree rows.
func TestRenderTreeRow_UsesPhaseBadgeStyles(t *testing.T) {
	saved := theme.Current()
	t.Cleanup(func() { theme.SetCurrent(saved) })
	theme.SetCurrent(theme.Tan())

	v := ProjectView{treeSelected: 1} // not selected
	node := treeNode{
		wu: &models.WorkUnit{
			Identifier: "proj/ticket",
			Phase:      models.PhaseImplement,
			IsProject:  false,
		},
		depth: 1,
	}

	row := v.renderTreeRow(node, 0, false, 60)
	if row == "" {
		t.Error("renderTreeRow returned empty string")
	}
	if !strings.Contains(row, "[implement]") {
		t.Errorf("renderTreeRow should contain phase badge, got %q", row)
	}
}

// TestProjectView_View_UsesThemeBorderStyles verifies the View method produces
// output using theme border styles.
func TestProjectView_View_UsesThemeBorderStyles(t *testing.T) {
	saved := theme.Current()
	t.Cleanup(func() { theme.SetCurrent(saved) })
	theme.SetCurrent(theme.Tan())

	v := ProjectView{
		width:  80,
		height: 24,
		treeNodes: []treeNode{
			{wu: &models.WorkUnit{Identifier: "proj", IsProject: true}, depth: 0},
			{wu: &models.WorkUnit{Identifier: "proj/t1", Phase: models.PhaseImplement}, depth: 1},
		},
		repoName: "test-repo",
	}

	output := v.View()
	if output == "" {
		t.Error("View() returned empty string")
	}
	if !strings.Contains(output, "test-repo") {
		t.Error("View output should contain repo name styled by theme.Current().RepoNameStyle")
	}
}
