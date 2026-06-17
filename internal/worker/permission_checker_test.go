package worker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	acp "github.com/coder/acp-go-sdk"
)

// writeFakeChecker installs an executable named like the real permissions
// checker into a fresh dir and prepends that dir to PATH for the test, so
// checkPermission's exec.LookPath finds it. The script body decides the
// output, letting each test drive a specific checker verdict.
func writeFakeChecker(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake checker shell script is unix-only")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, permissionCheckerBinary)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCheckPermissionAllow(t *testing.T) {
	writeFakeChecker(t, `echo '{"result":"allow"}'`)
	approved, reason := checkPermission(context.Background(), t.TempDir(), "sess", "`git status`")
	if !approved || reason != "" {
		t.Fatalf("got approved=%v reason=%q, want true and empty", approved, reason)
	}
}

func TestCheckPermissionDeny(t *testing.T) {
	writeFakeChecker(t, `echo '{"result":"deny","command":"rm","reason":"nope"}'`)
	approved, reason := checkPermission(context.Background(), t.TempDir(), "sess", "`rm -rf /`")
	if approved {
		t.Fatal("expected the request to be denied")
	}
	if reason != "nope" {
		t.Fatalf("reason = %q, want %q", reason, "nope")
	}
}

// A denial must surface as "not approved" so the caller falls back to the
// interactive prompt rather than auto-rejecting the request.
func TestCheckPermissionDenyIsNotApproved(t *testing.T) {
	writeFakeChecker(t, `echo '{"result":"deny"}'`)
	if approved, _ := checkPermission(context.Background(), t.TempDir(), "s", "x"); approved {
		t.Fatal("deny must not be reported as approved")
	}
}

// The checker must receive the exact ACP fields: the title as toolCallId and
// the worker identifier as sessionId.
func TestCheckPermissionSendsRequest(t *testing.T) {
	capture := filepath.Join(t.TempDir(), "input.json")
	t.Setenv("CHECKER_CAPTURE", capture)
	writeFakeChecker(t, `cat > "$CHECKER_CAPTURE"; echo '{"result":"allow"}'`)

	if approved, _ := checkPermission(context.Background(), t.TempDir(), "proj/ticket", "Read /tmp/x"); !approved {
		t.Fatal("expected approval")
	}

	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatalf("read captured input: %v", err)
	}
	var got checkerRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal captured input %q: %v", data, err)
	}
	if got.SessionID != "proj/ticket" {
		t.Errorf("sessionId = %q, want %q", got.SessionID, "proj/ticket")
	}
	if got.ToolCall.ToolCallID != "Read /tmp/x" {
		t.Errorf("toolCallId = %q, want %q", got.ToolCall.ToolCallID, "Read /tmp/x")
	}
}

// The checker runs in the worktree so it resolves relative paths and the
// project-level config against the correct repository.
func TestCheckPermissionRunsInWorkdir(t *testing.T) {
	capture := filepath.Join(t.TempDir(), "cwd.txt")
	t.Setenv("CWD_CAPTURE", capture)
	writeFakeChecker(t, `pwd -P > "$CWD_CAPTURE"; echo '{"result":"allow"}'`)

	workdir := t.TempDir()
	if approved, _ := checkPermission(context.Background(), workdir, "s", "x"); !approved {
		t.Fatal("expected approval")
	}
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatalf("read captured cwd: %v", err)
	}
	// macOS temp dirs are under /private symlinks; compare resolved paths.
	wantResolved, _ := filepath.EvalSymlinks(workdir)
	if got := filepath.Clean(string(data[:len(data)-1])); got != wantResolved {
		t.Errorf("checker cwd = %q, want %q", got, wantResolved)
	}
}

// A missing binary is not an error: the checker is optional and the caller
// falls back to prompting.
func TestCheckPermissionMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty dir: nothing to find
	approved, reason := checkPermission(context.Background(), t.TempDir(), "s", "x")
	if approved || reason != "" {
		t.Fatalf("missing binary should yield false and empty reason, got %v/%q", approved, reason)
	}
}

// A non-zero exit (bad config, parse error, etc.) must not approve.
func TestCheckPermissionErrorExit(t *testing.T) {
	writeFakeChecker(t, `echo boom >&2; exit 1`)
	if approved, _ := checkPermission(context.Background(), t.TempDir(), "s", "x"); approved {
		t.Fatal("non-zero exit must not approve")
	}
}

// Garbage on stdout must not approve.
func TestCheckPermissionMalformedOutput(t *testing.T) {
	writeFakeChecker(t, `echo 'not json'`)
	if approved, _ := checkPermission(context.Background(), t.TempDir(), "s", "x"); approved {
		t.Fatal("malformed output must not approve")
	}
}

func TestSelectAllowAlways(t *testing.T) {
	opt := func(k acp.PermissionOptionKind) acp.PermissionOption {
		return acp.PermissionOption{OptionId: acp.PermissionOptionId(string(k)), Kind: k}
	}

	tests := []struct {
		name    string
		options []acp.PermissionOption
		wantID  string
		wantOK  bool
	}{
		{
			name: "prefers allow_always",
			options: []acp.PermissionOption{
				opt(acp.PermissionOptionKindAllowOnce),
				opt(acp.PermissionOptionKindAllowAlways),
				opt(acp.PermissionOptionKindRejectOnce),
			},
			wantID: "allow_always",
			wantOK: true,
		},
		{
			name: "falls back to allow_once",
			options: []acp.PermissionOption{
				opt(acp.PermissionOptionKindRejectOnce),
				opt(acp.PermissionOptionKindAllowOnce),
			},
			wantID: "allow_once",
			wantOK: true,
		},
		{
			name: "falls back to first option",
			options: []acp.PermissionOption{
				opt(acp.PermissionOptionKindRejectOnce),
				opt(acp.PermissionOptionKindRejectAlways),
			},
			wantID: "reject_once",
			wantOK: true,
		},
		{
			name:    "no options",
			options: nil,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, ok := selectAllowAlways(tt.options)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if resp.Outcome.Selected == nil {
				t.Fatal("expected a selected outcome")
			}
			if got := string(resp.Outcome.Selected.OptionId); got != tt.wantID {
				t.Errorf("selected optionId = %q, want %q", got, tt.wantID)
			}
		})
	}
}
