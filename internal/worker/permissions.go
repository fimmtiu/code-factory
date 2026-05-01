package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	acp "github.com/coder/acp-go-sdk"
)

// allowList holds compiled permission rules from .claude/settings*.json
// for a single worktree. It implements the subset of Claude Code's permission
// rule matching needed to short-circuit ACP permission prompts: Bash, Read,
// Write, Edit, and NotebookEdit. The wrapper at @zed-industries/claude-code-acp
// calls canUseTool for every tool invocation regardless of the loaded
// allowlist, so we re-check rules on the client side and auto-approve
// matching calls before they reach the TUI.
type allowList struct {
	worktree  string
	bashRules []bashRule
	pathRules []pathRule
}

type bashRule struct {
	matcher *regexp.Regexp // non-nil when pattern uses * or :*
	exact   string         // used when matcher is nil
}

type pathRule struct {
	tool    string // "Read", "Write", "Edit", "NotebookEdit"
	matcher *regexp.Regexp
}

// loadAllowList reads Claude Code permission rules from the user's
// $HOME/.claude/settings.json plus the worktree's .claude/settings.json
// and .claude/settings.local.json, in that order, and builds a matcher.
// The user-global file is read first so that worktree-local rules can
// extend (but not override) it; in practice both append to the same
// list since allow rules don't conflict. Missing or malformed files are
// silently ignored: a partially-loaded allowList is still useful, and a
// completely empty one falls through to the normal prompt path.
func loadAllowList(worktree string) *allowList {
	al := &allowList{worktree: worktree}
	var paths []string
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, ".claude", "settings.json"))
	}
	for _, name := range []string{"settings.json", "settings.local.json"} {
		paths = append(paths, filepath.Join(worktree, ".claude", name))
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var parsed struct {
			Permissions struct {
				Allow []string `json:"allow"`
			} `json:"permissions"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			continue
		}
		for _, rule := range parsed.Permissions.Allow {
			al.addRule(rule)
		}
	}
	return al
}

func (al *allowList) addRule(rule string) {
	open := strings.IndexByte(rule, '(')
	if open < 0 || !strings.HasSuffix(rule, ")") {
		return
	}
	tool := rule[:open]
	body := rule[open+1 : len(rule)-1]
	switch tool {
	case "Bash":
		al.bashRules = append(al.bashRules, compileBashRule(body))
	case "Read", "Write", "Edit", "NotebookEdit":
		al.pathRules = append(al.pathRules, pathRule{
			tool:    tool,
			matcher: compilePathPattern(body),
		})
	}
}

// compileBashRule mirrors Claude Code's three Bash rule forms:
//   - "cmd:*"   legacy prefix match: cmd, or cmd followed by whitespace+args
//   - pattern with "*"   wildcard match where * means any characters
//   - anything else   exact string match
func compileBashRule(body string) bashRule {
	if strings.HasSuffix(body, ":*") {
		prefix := body[:len(body)-2]
		return bashRule{matcher: regexp.MustCompile("^" + regexp.QuoteMeta(prefix) + `(\s.*)?$`)}
	}
	if strings.Contains(body, "*") {
		var sb strings.Builder
		sb.WriteString("^")
		for _, ch := range body {
			if ch == '*' {
				sb.WriteString(".*")
			} else {
				sb.WriteString(regexp.QuoteMeta(string(ch)))
			}
		}
		sb.WriteString("$")
		return bashRule{matcher: regexp.MustCompile(sb.String())}
	}
	return bashRule{exact: body}
}

// compilePathPattern builds a regex for a gitignore-like file path pattern:
// "*" matches any characters except "/", "**" matches across "/", "?" matches
// a single non-slash character. Other characters match literally.
func compilePathPattern(body string) *regexp.Regexp {
	var sb strings.Builder
	sb.WriteString("^")
	for i := 0; i < len(body); {
		ch := body[i]
		switch {
		case ch == '*' && i+1 < len(body) && body[i+1] == '*':
			sb.WriteString(".*")
			i += 2
			if i < len(body) && body[i] == '/' {
				// "**/" allows zero or more leading path segments, so the
				// slash itself is optional when the prefix is empty.
				sb.WriteString("/?")
				i++
			}
		case ch == '*':
			sb.WriteString("[^/]*")
			i++
		case ch == '?':
			sb.WriteString("[^/]")
			i++
		default:
			sb.WriteString(regexp.QuoteMeta(string(ch)))
			i++
		}
	}
	sb.WriteString("$")
	return regexp.MustCompile(sb.String())
}

// matches reports whether the tool call should be auto-approved. The wrapper
// gives us a structured ToolCall but not the original tool name, so we infer
// the tool from the shape of RawInput.
func (al *allowList) matches(tc acp.RequestPermissionToolCall) bool {
	if al == nil {
		return false
	}
	raw, ok := tc.RawInput.(map[string]any)
	if !ok {
		return false
	}

	if cmd, ok := raw["command"].(string); ok && cmd != "" {
		for _, r := range al.bashRules {
			if r.matcher != nil {
				if r.matcher.MatchString(cmd) {
					return true
				}
			} else if r.exact == cmd {
				return true
			}
		}
		return false
	}

	path, _ := raw["file_path"].(string)
	if path == "" {
		path, _ = raw["notebook_path"].(string)
	}
	if path == "" {
		return false
	}

	tool := inferPathTool(raw)
	if tool == "" {
		return false
	}

	candidates := []string{path}
	if rel, err := filepath.Rel(al.worktree, path); err == nil && !strings.HasPrefix(rel, "..") {
		candidates = append(candidates, rel)
	}

	for _, r := range al.pathRules {
		if r.tool != tool {
			continue
		}
		for _, c := range candidates {
			if r.matcher.MatchString(c) {
				return true
			}
		}
	}
	return false
}

func inferPathTool(raw map[string]any) string {
	if _, ok := raw["notebook_path"]; ok {
		if _, hasNew := raw["new_source"]; hasNew {
			return "NotebookEdit"
		}
		return "Read"
	}
	if _, ok := raw["old_string"]; ok {
		return "Edit"
	}
	if _, ok := raw["content"]; ok {
		return "Write"
	}
	return "Read"
}
