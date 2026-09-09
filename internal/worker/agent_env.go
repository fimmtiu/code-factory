package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

// authRequiredCode is the JSON-RPC code claude-code-acp returns whenever the
// underlying Claude Code process reports "Please run /login".
const authRequiredCode = -32000

// authTokenVar is the variable the Claude Agent SDK reads for a bearer token.
// apiKeyHelper output goes here rather than into ANTHROPIC_API_KEY because a
// gateway issues bearer tokens, while ANTHROPIC_API_KEY travels as an x-api-key
// header that a gateway rejects.
const authTokenVar = "ANTHROPIC_AUTH_TOKEN"

// apiKeyHelperTimeout bounds the helper script. Helpers usually shell out to a
// corporate CLI that can hang on a network call, and a hung helper would
// otherwise stall the whole worker before the subprocess even starts.
const apiKeyHelperTimeout = 15 * time.Second

// claudeSettings is the subset of a .claude/settings.json file that affects the
// subprocess environment.
type claudeSettings struct {
	Env          map[string]string `json:"env"`
	APIKeyHelper string            `json:"apiKeyHelper"`
}

// claudeSettingsPaths returns the settings files that apply to a worktree, in
// increasing order of precedence: the user-global file first, then the
// worktree's own committed and local files.
func claudeSettingsPaths(worktree string) []string {
	var paths []string
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, ".claude", "settings.json"))
	}
	for _, name := range []string{"settings.json", "settings.local.json"} {
		paths = append(paths, filepath.Join(worktree, ".claude", name))
	}
	return paths
}

// loadClaudeSettings merges the environment-related settings from every file
// that applies to the worktree. Missing or malformed files are skipped: a
// partial result still beats refusing to start the agent.
func loadClaudeSettings(worktree string) claudeSettings {
	merged := claudeSettings{Env: map[string]string{}}
	for _, path := range claudeSettingsPaths(worktree) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var parsed claudeSettings
		if err := json.Unmarshal(data, &parsed); err != nil {
			continue
		}
		for name, value := range parsed.Env {
			merged.Env[name] = value
		}
		if parsed.APIKeyHelper != "" {
			merged.APIKeyHelper = parsed.APIKeyHelper
		}
	}
	return merged
}

// buildChildEnv assembles the environment for the claude-code-acp subprocess
// and returns any non-fatal problems for the caller to log.
//
// The Claude Agent SDK CLI that claude-code-acp starts does not apply `env` or
// `apiKeyHelper` from .claude/settings.json, even though the wrapper asks for
// settingSources ["user", "project", "local"]. A user whose credentials live
// only in those settings — anyone behind an LLM gateway rather than an OAuth
// login — would leave the subprocess with no credentials at all, and the
// wrapper reports that as the opaque ACP error -32000. So we resolve the
// settings ourselves and pass the result down explicitly.
//
// Resolving the token per run, rather than once at startup, also means a
// long-lived code-factory session picks up a refreshed token on every prompt
// instead of failing once the launch-time token expires.
func buildChildEnv(ctx context.Context, worktree string) ([]string, []string) {
	var warnings []string
	env := envMap(os.Environ())
	settings := loadClaudeSettings(worktree)

	// Settings-file values fill gaps only. A variable exported by the shell
	// that launched code-factory is a deliberate override and wins. An empty
	// value counts as a gap, since an exported-but-empty credential is never
	// what the caller meant.
	for name, value := range settings.Env {
		if env[name] == "" {
			env[name] = value
		}
	}

	if settings.APIKeyHelper != "" && env[authTokenVar] == "" && env["ANTHROPIC_API_KEY"] == "" {
		token, err := runAPIKeyHelper(ctx, settings.APIKeyHelper, worktree)
		switch {
		case err != nil:
			warnings = append(warnings, fmt.Sprintf("apiKeyHelper %s failed, subprocess has no credentials: %v",
				settings.APIKeyHelper, err))
		case token == "":
			warnings = append(warnings, fmt.Sprintf("apiKeyHelper %s printed nothing, subprocess has no credentials",
				settings.APIKeyHelper))
		default:
			env[authTokenVar] = token
		}
	}

	// Pin npx to the global node version so the worktree's .node-version
	// (pinned for project tooling) doesn't override it. nodenv global returns
	// the version from ~/.nodenv/version, ignoring any local .node-version file.
	if env["NODENV_VERSION"] == "" {
		if out, err := exec.Command("nodenv", "global").Output(); err == nil {
			if version := strings.TrimSpace(string(out)); version != "" {
				env["NODENV_VERSION"] = version
			}
		}
	}

	return envSlice(env), warnings
}

// annotateAuthError turns the bare ACP "Authentication required" error into a
// message that names the cause. The wrapper sends -32000 whenever the Claude
// Code subprocess says "Please run /login", which in practice means the
// subprocess received no usable credentials rather than that a login expired.
func annotateAuthError(err error) error {
	var reqErr *acp.RequestError
	if !errors.As(err, &reqErr) || reqErr.Code != authRequiredCode {
		return err
	}
	return fmt.Errorf("%w — the Claude Code subprocess has no credentials. Check apiKeyHelper "+
		"and env.ANTHROPIC_BASE_URL in ~/.claude/settings.json, or export %s before starting code-factory",
		err, authTokenVar)
}

// runAPIKeyHelper runs the configured helper and returns the token it prints.
func runAPIKeyHelper(ctx context.Context, helper, worktree string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, apiKeyHelperTimeout)
	defer cancel()

	// Claude Code treats apiKeyHelper as a shell command line rather than a
	// bare executable path, so run it the same way to accept arguments.
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", helper)
	cmd.Dir = worktree
	var stderr strings.Builder
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		// Helpers explain themselves on stderr ("run 'dev login'"), which is
		// the only actionable part of the failure.
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%w: %s", err, msg)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// envMap converts NAME=VALUE pairs into a map. Entries without '=' are dropped,
// matching how exec treats them.
func envMap(pairs []string) map[string]string {
	env := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		if name, value, ok := strings.Cut(pair, "="); ok {
			env[name] = value
		}
	}
	return env
}

// envSlice converts a map back into sorted NAME=VALUE pairs. Sorting keeps the
// result stable so tests can assert on it.
func envSlice(env map[string]string) []string {
	pairs := make([]string, 0, len(env))
	for name, value := range env {
		pairs = append(pairs, name+"="+value)
	}
	sort.Strings(pairs)
	return pairs
}
