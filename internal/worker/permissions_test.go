package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	acp "github.com/coder/acp-go-sdk"
)

func TestAllowListMatches(t *testing.T) {
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
		name  string
		raw   map[string]any
		want  bool
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
	dir := t.TempDir()
	al := loadAllowList(dir)
	if got := al.matches(acp.RequestPermissionToolCall{RawInput: map[string]any{"command": "ls"}}); got {
		t.Errorf("empty allowList should not match anything; got true")
	}
}
