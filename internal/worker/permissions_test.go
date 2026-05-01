package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	acp "github.com/coder/acp-go-sdk"
)

// restoreHome puts $HOME back to prev when the deferring test ends, so a
// per-test override never leaks into a subsequent test.
func restoreHome(prev string) {
	_ = os.Setenv("HOME", prev)
}

func TestAllowListMatches(t *testing.T) {
	// Isolate from the test runner's real home so user-global rules don't
	// bleed into the worktree-only assertions below.
	defer restoreHome(os.Getenv("HOME"))
	os.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := `{
	  "permissions": {
	    "allow": [
	      "Bash(ls *)",
	      "Bash(git:*)",
	      "Bash(echo hello)",
	      "Write(**/*.go)",
	      "Write(**/go.mod)",
	      "Write(*.md)",
	      "Edit(**/*.rb)",
	      "Read(**/*.txt)"
	    ]
	  }
	}`
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	al := loadAllowList(dir)

	cases := []struct {
		name string
		raw  map[string]any
		want bool
	}{
		{"bash wildcard", map[string]any{"command": "ls -la /tmp"}, true},
		{"bash wildcard bare", map[string]any{"command": "ls "}, true},
		{"bash wildcard ls only", map[string]any{"command": "ls"}, false}, // "ls *" requires the trailing space+args
		{"bash legacy prefix exact", map[string]any{"command": "git"}, true},
		{"bash legacy prefix args", map[string]any{"command": "git status"}, true},
		{"bash exact", map[string]any{"command": "echo hello"}, true},
		{"bash exact mismatch", map[string]any{"command": "echo world"}, false},
		{"bash unknown", map[string]any{"command": "rm -rf /"}, false},

		{"write nested go", map[string]any{"file_path": filepath.Join(dir, "foo/bar.go"), "content": "x"}, true},
		{"write top go", map[string]any{"file_path": filepath.Join(dir, "main.go"), "content": "x"}, true},
		{"write go.mod nested", map[string]any{"file_path": filepath.Join(dir, "sub/go.mod"), "content": "x"}, true},
		{"write go.mod top", map[string]any{"file_path": filepath.Join(dir, "go.mod"), "content": "x"}, true},
		{"write md top only", map[string]any{"file_path": filepath.Join(dir, "README.md"), "content": "x"}, true},
		{"write md nested rejected", map[string]any{"file_path": filepath.Join(dir, "docs/intro.md"), "content": "x"}, false},
		{"write unknown ext", map[string]any{"file_path": filepath.Join(dir, "foo.py"), "content": "x"}, false},

		{"edit ruby", map[string]any{"file_path": filepath.Join(dir, "lib/foo.rb"), "old_string": "a", "new_string": "b"}, true},
		{"edit go via write rule rejected", map[string]any{"file_path": filepath.Join(dir, "foo.go"), "old_string": "a", "new_string": "b"}, false},

		{"read txt", map[string]any{"file_path": filepath.Join(dir, "notes.txt")}, true},
		{"read go rejected", map[string]any{"file_path": filepath.Join(dir, "foo.go")}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			rawJSON, _ := json.Marshal(tc.raw)
			var raw any
			_ = json.Unmarshal(rawJSON, &raw)
			got := al.matches(acp.RequestPermissionToolCall{RawInput: raw})
			if got != tc.want {
				t.Errorf("matches(%v) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestAllowListMissingFiles(t *testing.T) {
	// Point HOME at a fresh empty dir so the user-global settings.json
	// doesn't bleed real rules from the test runner's home directory.
	defer restoreHome(os.Getenv("HOME"))
	os.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	al := loadAllowList(dir)
	if got := al.matches(acp.RequestPermissionToolCall{RawInput: map[string]any{"command": "ls"}}); got {
		t.Errorf("empty allowList should not match anything; got true")
	}
}

// TestAllowListMergesUserHomeAndWorktree verifies that rules from both
// $HOME/.claude/settings.json and the worktree's .claude/settings.json are
// loaded. Without this, a user's home-level allowlist was ignored and every
// worktree had to repeat the same rules.
func TestAllowListMergesUserHomeAndWorktree(t *testing.T) {
	defer restoreHome(os.Getenv("HOME"))
	home := t.TempDir()
	os.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	homeSettings := `{
	  "permissions": {
	    "allow": ["Bash(rm -f /tmp/*)"]
	  }
	}`
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(homeSettings), 0o644); err != nil {
		t.Fatal(err)
	}

	worktree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(worktree, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	worktreeSettings := `{
	  "permissions": {
	    "allow": ["Bash(make test)"]
	  }
	}`
	if err := os.WriteFile(filepath.Join(worktree, ".claude", "settings.json"), []byte(worktreeSettings), 0o644); err != nil {
		t.Fatal(err)
	}

	al := loadAllowList(worktree)

	cases := []struct {
		name string
		cmd  string
		want bool
	}{
		{"home rule matches", "rm -f /tmp/cf-review-blah.txt", true},
		{"worktree rule matches", "make test", true},
		{"unknown command rejected", "rm -rf /", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rawJSON, _ := json.Marshal(map[string]any{"command": tc.cmd})
			var raw any
			_ = json.Unmarshal(rawJSON, &raw)
			got := al.matches(acp.RequestPermissionToolCall{RawInput: raw})
			if got != tc.want {
				t.Errorf("matches(%q) = %v, want %v", tc.cmd, got, tc.want)
			}
		})
	}
}
