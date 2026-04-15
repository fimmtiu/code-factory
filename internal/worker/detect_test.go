package worker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMakefileTargets(t *testing.T) {
	content := `build:
	go build ./...

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f binary

INSTALL_DIR ?= $(HOME)/bin

.PHONY: build test lint clean
`
	targets := parseMakefileTargets(content)
	for _, name := range []string{"build", "test", "lint", "clean"} {
		if !targets[name] {
			t.Errorf("expected target %q to be found", name)
		}
	}
	// Variable assignments and .PHONY should not appear as targets.
	if targets["INSTALL_DIR"] {
		t.Error("variable assignment should not be a target")
	}
}

func TestParseMakefileTargets_Empty(t *testing.T) {
	targets := parseMakefileTargets("")
	if len(targets) != 0 {
		t.Errorf("expected no targets, got %v", targets)
	}
}

func TestDetectTooling_GoProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n"), 0644); err != nil {
		t.Fatal(err)
	}

	build, test, lint := detectTooling(dir)
	if build != "go build ./..." {
		t.Errorf("build = %q, want %q", build, "go build ./...")
	}
	if test != "go test ./..." {
		t.Errorf("test = %q, want %q", test, "go test ./...")
	}
	if lint != "go vet ./..." {
		t.Errorf("lint = %q, want %q", lint, "go vet ./...")
	}
}

func TestDetectTooling_MakefileOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n"), 0644); err != nil {
		t.Fatal(err)
	}
	makefile := "build:\n\tgo build ./cmd/...\ntest:\n\tgo test ./...\nlint:\n\tmake vet\n"
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	build, test, lint := detectTooling(dir)
	if build != "make build" {
		t.Errorf("build = %q, want %q", build, "make build")
	}
	if test != "make test" {
		t.Errorf("test = %q, want %q", test, "make test")
	}
	if lint != "make lint" {
		t.Errorf("lint = %q, want %q", lint, "make lint")
	}
}

func TestDetectTooling_NodeProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	build, test, lint := detectTooling(dir)
	if build != "npm run build" {
		t.Errorf("build = %q, want %q", build, "npm run build")
	}
	if test != "npm test" {
		t.Errorf("test = %q, want %q", test, "npm test")
	}
	if lint != "npm run lint" {
		t.Errorf("lint = %q, want %q", lint, "npm run lint")
	}
}

func TestDetectTooling_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	build, test, lint := detectTooling(dir)
	if build != "" || test != "" || lint != "" {
		t.Errorf("expected empty commands, got build=%q test=%q lint=%q", build, test, lint)
	}
}

func TestFormatEnvBlock_AllFields(t *testing.T) {
	env := WorktreeEnv{
		DefaultBranch: "main",
		Branchpoint:   "abc123",
		BuildCmd:      "make build",
		TestCmd:       "make test",
		LintCmd:       "make lint",
	}
	block := env.FormatEnvBlock()
	if block == "" {
		t.Fatal("expected non-empty block")
	}
	for _, want := range []string{"DEFAULT_BRANCH", "main", "BRANCHPOINT", "abc123", "BUILD_CMD", "TEST_CMD", "LINT_CMD"} {
		if !contains(block, want) {
			t.Errorf("block missing %q", want)
		}
	}
}

func TestFormatEnvBlock_Empty(t *testing.T) {
	env := WorktreeEnv{}
	if env.FormatEnvBlock() != "" {
		t.Error("expected empty block for empty env")
	}
}

func TestFormatEnvBlock_Partial(t *testing.T) {
	env := WorktreeEnv{BuildCmd: "make build"}
	block := env.FormatEnvBlock()
	if block == "" {
		t.Fatal("expected non-empty block")
	}
	if contains(block, "DEFAULT_BRANCH") {
		t.Error("should not contain DEFAULT_BRANCH when not set")
	}
	if !contains(block, "BUILD_CMD") {
		t.Error("should contain BUILD_CMD")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
