package ui

import (
	"testing"

	"github.com/fimmtiu/code-factory/internal/models"
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
