package db_test

import (
	"fmt"
	"testing"

	"github.com/fimmtiu/code-factory/internal/db"
)

func TestMemory_ScopedRetrieval(t *testing.T) {
	d, _, _ := openTestDB(t)

	// Global, subtree-scoped, and an unrelated-scope memory.
	mustAdd(t, d, "", "gotcha", "global note")
	mustAdd(t, d, "proj/server", "lesson", "server subtree lesson")
	mustAdd(t, d, "proj/server/svc", "pattern", "exact-scope pattern")
	mustAdd(t, d, "proj/client", "lesson", "unrelated subtree lesson")

	got := textsFor(t, d, "proj/server/svc/ticket-1")

	// Should see global + ancestor (proj/server) + nothing from proj/client.
	want := map[string]bool{
		"global note":           true,
		"server subtree lesson": true,
		"exact-scope pattern":   true,
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d memories, got %d: %v", len(want), len(got), got)
	}
	for _, txt := range got {
		if !want[txt] {
			t.Errorf("unexpected memory in scope: %q", txt)
		}
	}
}

func TestMemory_GlobalScopeReachesEveryTicket(t *testing.T) {
	d, _, _ := openTestDB(t)
	mustAdd(t, d, "", "note", "applies everywhere")

	for _, id := range []string{"a/b/c", "x", "totally/unrelated/path"} {
		got := textsFor(t, d, id)
		if len(got) != 1 || got[0] != "applies everywhere" {
			t.Errorf("identifier %q: expected the global memory, got %v", id, got)
		}
	}
}

func TestMemory_LimitCaps(t *testing.T) {
	d, _, _ := openTestDB(t)
	for i := 0; i < 5; i++ {
		mustAdd(t, d, "", "note", "n")
	}
	got, err := d.MemoriesForIdentifier("proj/ticket", 3)
	if err != nil {
		t.Fatalf("MemoriesForIdentifier: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected limit to cap at 3, got %d", len(got))
	}
}

func TestMemory_DeleteUnknownIDErrors(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.DeleteMemory(999); err == nil {
		t.Error("expected error deleting nonexistent memory id")
	}
}

func TestMemory_AddRejectsEmptyText(t *testing.T) {
	d, _, _ := openTestDB(t)
	if _, err := d.AddMemory("", "lesson", "   ", ""); err == nil {
		t.Error("expected error adding memory with empty text")
	}
}

func TestPruneMemories_DedupKeepsNewest(t *testing.T) {
	d, _, _ := openTestDB(t)
	mustAdd(t, d, "proj", "lesson", "Always run make lint")
	mustAdd(t, d, "proj", "lesson", "always   run MAKE lint") // normalizes to the same text
	mustAdd(t, d, "proj", "lesson", "a different lesson")

	res, err := d.PruneMemories(0, 0, false)
	if err != nil {
		t.Fatalf("PruneMemories: %v", err)
	}
	if res.Duplicates != 1 {
		t.Errorf("expected 1 duplicate removed, got %d", res.Duplicates)
	}
	if got := textsFor(t, d, "proj/ticket"); len(got) != 2 {
		t.Fatalf("expected 2 memories after dedup, got %d: %v", len(got), got)
	}
}

func TestPruneMemories_PerScopeCap(t *testing.T) {
	d, _, _ := openTestDB(t)
	for i := 0; i < 4; i++ {
		mustAdd(t, d, "proj", "note", fmt.Sprintf("note %d", i))
	}
	mustAdd(t, d, "other", "note", "keep me")

	res, err := d.PruneMemories(2, 0, false)
	if err != nil {
		t.Fatalf("PruneMemories: %v", err)
	}
	if res.OverCap != 2 {
		t.Errorf("expected 2 over-cap removed, got %d", res.OverCap)
	}
	all, err := d.ListMemories()
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	if len(all) != 3 { // proj keeps newest 2, other keeps its 1
		t.Errorf("expected 3 memories remaining, got %d", len(all))
	}
}

func TestPruneMemories_DryRunDeletesNothing(t *testing.T) {
	d, _, _ := openTestDB(t)
	mustAdd(t, d, "proj", "note", "dup")
	mustAdd(t, d, "proj", "note", "dup")

	res, err := d.PruneMemories(0, 0, true)
	if err != nil {
		t.Fatalf("PruneMemories: %v", err)
	}
	if res.Duplicates != 1 {
		t.Errorf("expected dry run to report 1 duplicate, got %d", res.Duplicates)
	}
	if len(res.Deleted) != 0 {
		t.Errorf("expected dry run to delete nothing, got %v", res.Deleted)
	}
	if all, _ := d.ListMemories(); len(all) != 2 {
		t.Errorf("expected both memories to remain after dry run, got %d", len(all))
	}
}

func mustAdd(t *testing.T, d *db.DB, scope, kind, text string) {
	t.Helper()
	if _, err := d.AddMemory(scope, kind, text, ""); err != nil {
		t.Fatalf("AddMemory(%q, %q): %v", scope, text, err)
	}
}

func textsFor(t *testing.T, d *db.DB, identifier string) []string {
	t.Helper()
	memories, err := d.MemoriesForIdentifier(identifier, 0)
	if err != nil {
		t.Fatalf("MemoriesForIdentifier(%q): %v", identifier, err)
	}
	texts := make([]string, len(memories))
	for i, m := range memories {
		texts[i] = m.Text
	}
	return texts
}
