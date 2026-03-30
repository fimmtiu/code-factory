package util_test

import (
	"os"
	"testing"

	"github.com/fimmtiu/code-factory/internal/config"
	"github.com/fimmtiu/code-factory/internal/util"
)

func init() {
	config.Current = config.Default()
	// Register lightweight test profiles that don't require real editors.
	util.EditorProfiles["test-noop"] = util.EditorProfile{Blocking: "true", Nonblocking: "true"}
	util.EditorProfiles["test-sh"] = util.EditorProfile{Blocking: "sh -c true", Nonblocking: "sh"}
}

func TestEditText_NoCommand(t *testing.T) {
	// An unknown editor name produces an empty command → error.
	config.Current.Editor = "unknown-editor-that-does-not-exist"
	defer func() { config.Current.Editor = "test-noop" }()

	_, err := util.EditText("some content")
	if err == nil {
		t.Fatal("expected error when editor is unconfigured, got nil")
	}
}

func TestEditText_HappyPath_CatPreservesContent(t *testing.T) {
	config.Current.Editor = "test-noop"
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
	config.Current.Editor = "test-sh"
	_, err := util.EditText("test content")
	if err != nil {
		t.Fatalf("EditText with args: %v", err)
	}
}

func TestEditText_TempFileDeletedAfterSuccess(t *testing.T) {
	config.Current.Editor = "test-noop"
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
