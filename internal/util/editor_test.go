package util_test

import (
	"os"
	"testing"

	"github.com/fimmtiu/tickets/internal/util"
)

func TestEditText_NoEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	_, err := util.EditText("some content")
	if err == nil {
		t.Fatal("expected error when $EDITOR is unset, got nil")
	}
}

func TestEditText_HappyPath_CatPreservesContent(t *testing.T) {
	// Use "cat" as a no-op editor: it reads stdin but we're not piping stdin,
	// so it just exits 0 leaving the temp file unchanged. However on some
	// systems cat with a filename echoes it to stdout and doesn't modify the
	// file. We really want a command that leaves the file alone.
	// Use "true" which ignores arguments and exits 0 — file is left unchanged.
	t.Setenv("EDITOR", "true")
	content := "hello, world!"
	result, err := util.EditText(content)
	if err != nil {
		t.Fatalf("EditText: %v", err)
	}
	if result != content {
		t.Errorf("got %q, want %q", result, content)
	}
}

func TestEditText_WithEditorArguments(t *testing.T) {
	// Use "sh -c true" to test that $EDITOR with arguments is parsed correctly.
	t.Setenv("EDITOR", "sh -c true")
	_, err := util.EditText("test content")
	if err != nil {
		t.Fatalf("EditText with args: %v", err)
	}
}

func TestEditText_TempFileDeletedAfterSuccess(t *testing.T) {
	t.Setenv("EDITOR", "true")

	// Track temp files before and after; the file should not persist.
	tmpDir := os.TempDir()
	beforeEntries, _ := os.ReadDir(tmpDir)

	_, err := util.EditText("some content")
	if err != nil {
		t.Fatalf("EditText: %v", err)
	}

	afterEntries, _ := os.ReadDir(tmpDir)
	// Verify no extra code-factory-edit-*.txt files remain.
	before := countEditFiles(beforeEntries)
	after := countEditFiles(afterEntries)
	if after > before {
		t.Errorf("temp file was not deleted: before=%d after=%d edit files", before, after)
	}
}

func countEditFiles(entries []os.DirEntry) int {
	count := 0
	for _, e := range entries {
		if len(e.Name()) > 18 && e.Name()[:18] == "code-factory-edit-" {
			count++
		}
	}
	return count
}
