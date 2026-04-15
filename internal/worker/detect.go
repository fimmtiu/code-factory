package worker

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeEnv holds pre-detected git state and tooling commands for a ticket
// worktree. The worker detects these once before invoking a skill, saving each
// skill from having to rediscover the same information.
type WorktreeEnv struct {
	DefaultBranch string
	Branchpoint   string
	BuildCmd      string
	TestCmd       string
	LintCmd       string
}

// DetectWorktreeEnv detects git state and project tooling for the given
// worktree path. Individual fields are left empty if detection fails;
// the caller (and downstream skill) should fall back to manual detection
// for any missing value.
func DetectWorktreeEnv(worktreePath string) WorktreeEnv {
	env := WorktreeEnv{}
	env.DefaultBranch = detectDefaultBranch(worktreePath)
	if env.DefaultBranch != "" {
		env.Branchpoint = detectBranchpoint(worktreePath, env.DefaultBranch)
	}
	env.BuildCmd, env.TestCmd, env.LintCmd = detectTooling(worktreePath)
	return env
}

// FormatEnvBlock returns a prompt-ready Markdown block describing the
// pre-detected environment. Returns empty string if nothing was detected.
func (e WorktreeEnv) FormatEnvBlock() string {
	var lines []string
	if e.DefaultBranch != "" {
		lines = append(lines, "- DEFAULT_BRANCH: `"+e.DefaultBranch+"`")
	}
	if e.Branchpoint != "" {
		lines = append(lines, "- BRANCHPOINT: `"+e.Branchpoint+"`")
	}
	if e.BuildCmd != "" {
		lines = append(lines, "- BUILD_CMD: `"+e.BuildCmd+"`")
	}
	if e.TestCmd != "" {
		lines = append(lines, "- TEST_CMD: `"+e.TestCmd+"`")
	}
	if e.LintCmd != "" {
		lines = append(lines, "- LINT_CMD: `"+e.LintCmd+"`")
	}
	if len(lines) == 0 {
		return ""
	}
	return "## Pre-detected environment\n\n" +
		"The following values have been pre-detected for this worktree. " +
		"Use them directly instead of re-detecting them. " +
		"If a value you need is missing, detect it yourself.\n\n" +
		strings.Join(lines, "\n") + "\n\n"
}

func detectDefaultBranch(worktreePath string) string {
	out, err := gitOutput(worktreePath, "branch", "-l", "main", "master", "--format=%(refname:short)")
	if err != nil || out == "" {
		return ""
	}
	lines := strings.SplitN(out, "\n", 2)
	return strings.TrimSpace(lines[0])
}

func detectBranchpoint(worktreePath, defaultBranch string) string {
	// Try with origin/ prefix first (matches what the skills do).
	out, err := gitOutput(worktreePath, "merge-base", "origin/"+defaultBranch, "HEAD")
	if err == nil && out != "" {
		return strings.TrimSpace(out)
	}
	// Fall back without origin/ prefix.
	out, err = gitOutput(worktreePath, "merge-base", defaultBranch, "HEAD")
	if err == nil {
		return strings.TrimSpace(out)
	}
	return ""
}

func detectTooling(worktreePath string) (buildCmd, testCmd, lintCmd string) {
	hasMakefile := fileExists(filepath.Join(worktreePath, "Makefile"))

	if hasMakefile {
		content, err := os.ReadFile(filepath.Join(worktreePath, "Makefile"))
		if err == nil {
			targets := parseMakefileTargets(string(content))
			if targets["build"] {
				buildCmd = "make build"
			}
			if targets["test"] {
				testCmd = "make test"
			}
			if targets["lint"] {
				lintCmd = "make lint"
			}
		}
	}

	// Fill in gaps with language-specific defaults.
	switch {
	case fileExists(filepath.Join(worktreePath, "go.mod")):
		if buildCmd == "" {
			buildCmd = "go build ./..."
		}
		if testCmd == "" {
			testCmd = "go test ./..."
		}
		if lintCmd == "" {
			lintCmd = "go vet ./..."
		}
	case fileExists(filepath.Join(worktreePath, "package.json")):
		if buildCmd == "" {
			buildCmd = "npm run build"
		}
		if testCmd == "" {
			testCmd = "npm test"
		}
		if lintCmd == "" {
			lintCmd = "npm run lint"
		}
	case fileExists(filepath.Join(worktreePath, "Cargo.toml")):
		if buildCmd == "" {
			buildCmd = "cargo build"
		}
		if testCmd == "" {
			testCmd = "cargo test"
		}
		if lintCmd == "" {
			lintCmd = "cargo clippy"
		}
	case fileExists(filepath.Join(worktreePath, "pyproject.toml")):
		if testCmd == "" {
			testCmd = "pytest"
		}
	}

	return buildCmd, testCmd, lintCmd
}

// parseMakefileTargets extracts top-level target names from Makefile content.
func parseMakefileTargets(content string) map[string]bool {
	targets := make(map[string]bool)
	for _, line := range strings.Split(content, "\n") {
		// Skip indented lines (recipe lines) and variable assignments.
		if len(line) == 0 || line[0] == '\t' || line[0] == ' ' || line[0] == '#' {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		name := strings.TrimSpace(line[:idx])
		if name != "" && !strings.ContainsAny(name, " \t=?") {
			targets[name] = true
		}
	}
	return targets
}

func gitOutput(worktreePath string, args ...string) (string, error) {
	allArgs := append([]string{"-C", worktreePath}, args...)
	cmd := exec.Command("git", allArgs...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
