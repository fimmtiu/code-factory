package db

import (
	"testing"
	"time"
)

// TestPruneMemories_AgeOut lives in-package so it can backdate created_at
// directly; the public API always stamps memories with the current time.
func TestPruneMemories_AgeOut(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir, dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	id, err := d.AddMemory("", "note", "old note", "")
	if err != nil {
		t.Fatalf("AddMemory old: %v", err)
	}
	old := time.Now().Add(-10 * 24 * time.Hour).Unix()
	if _, err := d.db.Exec(`UPDATE memories SET created_at = ? WHERE id = ?`, old, id); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	if _, err := d.AddMemory("", "note", "fresh note", ""); err != nil {
		t.Fatalf("AddMemory fresh: %v", err)
	}

	res, err := d.PruneMemories(0, 7*24*time.Hour, false)
	if err != nil {
		t.Fatalf("PruneMemories: %v", err)
	}
	if res.AgedOut != 1 {
		t.Errorf("expected 1 aged-out, got %d", res.AgedOut)
	}
	remaining, err := d.ListMemories()
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	if len(remaining) != 1 || remaining[0].Text != "fresh note" {
		t.Errorf("expected only the fresh note to remain, got %v", remaining)
	}
}
