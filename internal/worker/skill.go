package worker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadSkillBody returns the named skill's SKILL.md body with the YAML
// frontmatter stripped. It looks under `$HOME/.claude/skills/<name>/SKILL.md`,
// matching where Claude Code installs user-global skills.
//
// We inline the body into worker prompts because the ACP wrapper that drives
// these sessions (claude-code-acp) does not expand leading slash commands the
// way the interactive Claude Code CLI does. A bare "/cf-review …" prompt
// reaches the model as plain text, so the skill's structured protocol never
// runs — review agents file no change requests, refactor agents improvise,
// and so on. Inlining sidesteps the wrapper entirely.
func LoadSkillBody(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("LoadSkillBody %q: home dir: %w", name, err)
	}
	path := filepath.Join(home, ".claude", "skills", name, "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("LoadSkillBody %q: skill not installed at %s", name, path)
		}
		return "", fmt.Errorf("LoadSkillBody %q: read %s: %w", name, path, err)
	}
	return stripFrontmatter(string(data)), nil
}

// stripFrontmatter removes a leading "--- … ---" YAML frontmatter block. If
// the input has no frontmatter, it is returned unchanged.
func stripFrontmatter(s string) string {
	trimmed := strings.TrimLeft(s, "\r\n")
	if !strings.HasPrefix(trimmed, "---") {
		return s
	}
	rest := strings.TrimPrefix(trimmed, "---")
	rest = strings.TrimPrefix(rest, "\r")
	if !strings.HasPrefix(rest, "\n") {
		return s
	}
	rest = rest[1:]
	_, body, ok := strings.Cut(rest, "\n---")
	if !ok {
		return s
	}
	body = strings.TrimPrefix(body, "\r")
	body = strings.TrimPrefix(body, "\n")
	return body
}
