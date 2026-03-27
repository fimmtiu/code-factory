package util_test

import (
	"os"
	"testing"

	"github.com/fimmtiu/tickets/internal/config"
	"github.com/fimmtiu/tickets/internal/util"
)

func init() {
	// Provide a minimal config.Current so util functions can read it.
	config.Current = config.Default()
}

func TestEditText_NoCommand(t *testing.T) {
	config.Current.BlockingEditorCommand = ""
	defer func() { config.Current.BlockingEditorCommand = "true" }()

	_, err := util.EditText("some content")
	if err == nil {
		t.Fatal("expected error when command is empty, got nil")
	}
}

func TestEditText_HappyPath_CatPreservesContent(t *testing.T) {
	config.Current.BlockingEditorCommand = "true"
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
	config.Current.BlockingEditorCommand = "sh -c true"
	_, err := util.EditText("test content")
	if err != nil {
		t.Fatalf("EditText with args: %v", err)
	}
}

func TestEditText_TempFileDeletedAfterSuccess(t *testing.T) {
	config.Current.BlockingEditorCommand = "true"
	tmpDir := os.TempDir()
	beforeEntries, _ := os.ReadDir(tmpDir)

	_, err := util.EditText("some content")
	if err != nil {
		t.Fatalf("EditText: %v", err)
	}

	afterEntries, _ := os.ReadDir(tmpDir)
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
