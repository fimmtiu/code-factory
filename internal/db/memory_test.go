package db_test

import (
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
