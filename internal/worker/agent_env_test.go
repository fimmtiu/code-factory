package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSettings creates dir/.claude/<name> holding body.
func writeSettings(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude", name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeTokenHelper creates an executable script that prints token on stdout.
func writeTokenHelper(t *testing.T, token string) string {
	t.Helper()
	return writeScript(t, "printf '%s' '"+token+"'\n")
}

// writeFailingHelper creates an executable script that reports msg on stderr
// and exits non-zero.
func writeFailingHelper(t *testing.T, msg string) string {
	t.Helper()
	return writeScript(t, "echo '"+msg+"' >&2\nexit 1\n")
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "helper.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// envValue finds name in a NAME=VALUE slice, reporting whether it is present.
func envValue(pairs []string, name string) (string, bool) {
	for _, pair := range pairs {
		if key, value, ok := strings.Cut(pair, "="); ok && key == name {
			return value, true
		}
	}
	return "", false
}

// isolateHome points $HOME at an empty directory so the developer's real
// ~/.claude/settings.json can't leak into these assertions.
func isolateHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func TestBuildChildEnvRunsAPIKeyHelper(t *testing.T) {
	isolateHome(t)
	// Clear the credentials the test runner itself may have inherited, so the
	// settings file is the only source in play.
	t.Setenv(authTokenVar, "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_BASE_URL", "")
	worktree := t.TempDir()
	helper := writeTokenHelper(t, "gateway-token")
	writeSettings(t, worktree, "settings.json", `{
	  "apiKeyHelper": "`+helper+`",
	  "env": {"ANTHROPIC_BASE_URL": "https://gateway.example"}
	}`)

	env, warnings := buildChildEnv(context.Background(), worktree)

	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if got, ok := envValue(env, authTokenVar); !ok || got != "gateway-token" {
		t.Errorf("%s = %q (present %v), want %q", authTokenVar, got, ok, "gateway-token")
	}
	if got, _ := envValue(env, "ANTHROPIC_BASE_URL"); got != "https://gateway.example" {
		t.Errorf("ANTHROPIC_BASE_URL = %q, want %q", got, "https://gateway.example")
	}
}

func TestBuildChildEnvParentEnvBeatsSettings(t *testing.T) {
	isolateHome(t)
	t.Setenv("ANTHROPIC_BASE_URL", "https://exported.example")
	worktree := t.TempDir()
	writeSettings(t, worktree, "settings.json",
		`{"env": {"ANTHROPIC_BASE_URL": "https://settings.example"}}`)

	env, _ := buildChildEnv(context.Background(), worktree)

	if got, _ := envValue(env, "ANTHROPIC_BASE_URL"); got != "https://exported.example" {
		t.Errorf("ANTHROPIC_BASE_URL = %q, want the exported value to win", got)
	}
}

func TestBuildChildEnvKeepsExistingToken(t *testing.T) {
	isolateHome(t)
	t.Setenv(authTokenVar, "already-set")
	worktree := t.TempDir()
	// A helper that would fail if run, proving it is skipped.
	helper := writeFailingHelper(t, "should not run")
	writeSettings(t, worktree, "settings.json", `{"apiKeyHelper": "`+helper+`"}`)

	env, warnings := buildChildEnv(context.Background(), worktree)

	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if got, _ := envValue(env, authTokenVar); got != "already-set" {
		t.Errorf("%s = %q, want %q", authTokenVar, got, "already-set")
	}
}

func TestBuildChildEnvWarnsWhenHelperFails(t *testing.T) {
	isolateHome(t)
	t.Setenv(authTokenVar, "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	worktree := t.TempDir()
	helper := writeFailingHelper(t, "run dev login")
	writeSettings(t, worktree, "settings.json", `{"apiKeyHelper": "`+helper+`"}`)

	env, warnings := buildChildEnv(context.Background(), worktree)

	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warnings)
	}
	if !strings.Contains(warnings[0], "run dev login") {
		t.Errorf("warning %q should quote the helper's stderr", warnings[0])
	}
	if got, ok := envValue(env, authTokenVar); ok && got != "" {
		t.Errorf("%s = %q, want it left unset", authTokenVar, got)
	}
}

func TestLoadClaudeSettingsLocalFileWins(t *testing.T) {
	isolateHome(t)
	worktree := t.TempDir()
	writeSettings(t, worktree, "settings.json",
		`{"apiKeyHelper": "committed", "env": {"A": "1", "B": "2"}}`)
	writeSettings(t, worktree, "settings.local.json",
		`{"apiKeyHelper": "local", "env": {"B": "override"}}`)

	settings := loadClaudeSettings(worktree)

	if settings.APIKeyHelper != "local" {
		t.Errorf("APIKeyHelper = %q, want %q", settings.APIKeyHelper, "local")
	}
	if settings.Env["A"] != "1" {
		t.Errorf("Env[A] = %q, want %q", settings.Env["A"], "1")
	}
	if settings.Env["B"] != "override" {
		t.Errorf("Env[B] = %q, want %q", settings.Env["B"], "override")
	}
}

func TestLoadClaudeSettingsIgnoresMalformedFile(t *testing.T) {
	isolateHome(t)
	worktree := t.TempDir()
	writeSettings(t, worktree, "settings.json", `{not json`)
	writeSettings(t, worktree, "settings.local.json", `{"env": {"A": "1"}}`)

	settings := loadClaudeSettings(worktree)

	if settings.Env["A"] != "1" {
		t.Errorf("Env[A] = %q, want the readable file to still load", settings.Env["A"])
	}
}
