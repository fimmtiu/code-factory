package util

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fimmtiu/code-factory/internal/config"
)

// dirPlaceholder marks the spot in a profile's OpenArgs that should be
// replaced with the absolute path of the directory being opened. The
// "open -a" terminals take the directory from the child process's working
// directory instead, so only profiles driven by a CLI need it.
const dirPlaceholder = "{{dir}}"

// TerminalProfile holds the attributes for a named terminal emulator.
type TerminalProfile struct {
	BundleID string // macOS bundle identifier, e.g. "com.googlecode.iterm2"

	// OpenArgs is the command and arguments that open a window on a
	// directory, e.g. {"open", "-a", "iTerm", "."}. It is passed to exec
	// directly rather than through a shell, so paths containing spaces
	// survive. Any occurrence of dirPlaceholder is expanded first.
	OpenArgs []string
}

// TerminalProfiles maps the supported terminal names to their profiles.
var TerminalProfiles = map[string]TerminalProfile{
	"iterm2":   {BundleID: "com.googlecode.iterm2", OpenArgs: []string{"open", "-a", "iTerm", "."}},
	"terminal": {BundleID: "com.apple.Terminal", OpenArgs: []string{"open", "-a", "Terminal", "."}},
	"cmux":     {BundleID: "com.cmuxterm.app", OpenArgs: []string{"open", "-a", "cmux", "."}},

	// Orca registers no handler for folders, so "open -a Orca ." would
	// merely focus the app and ignore the directory. Its CLI is the only way
	// in, and it resolves a path only to a worktree that Orca already
	// manages: opening a worktree code-factory created itself fails with
	// "selector_not_found" until it is imported into Orca.
	"orca": {
		BundleID: "com.stablyai.orca",
		OpenArgs: []string{"orca", "terminal", "create", "--worktree", "path:" + dirPlaceholder, "--focus"},
	},
}

// SupportedTerminals returns the supported terminal names in alphabetical
// order, for error messages and documentation.
func SupportedTerminals() []string {
	names := make([]string, 0, len(TerminalProfiles))
	for k := range TerminalProfiles {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// ValidateTerminal returns an error if name is not a supported terminal.
func ValidateTerminal(name string) error {
	if _, ok := TerminalProfiles[name]; !ok {
		return fmt.Errorf("unknown terminal %q in settings.json; supported values: %s",
			name, strings.Join(SupportedTerminals(), ", "))
	}
	return nil
}

// TerminalBundleID returns the macOS bundle identifier for the currently
// configured terminal, or empty string if unknown.
func TerminalBundleID() string {
	if p, ok := TerminalProfiles[config.Current.Terminal]; ok {
		return p.BundleID
	}
	return ""
}

// resolveOpenArgs returns the profile's command with dirPlaceholder expanded
// to dir.
func (p TerminalProfile) resolveOpenArgs(dir string) []string {
	args := make([]string, len(p.OpenArgs))
	for i, a := range p.OpenArgs {
		args[i] = strings.ReplaceAll(a, dirPlaceholder, dir)
	}
	return args
}

// OpenTerminal opens a terminal window in dir using the command from the
// currently configured terminal profile. It waits for the launcher to exit so
// that a refusal (an uninstalled app, or a directory Orca does not manage) is
// reported rather than silently dropped; the launchers all return as soon as
// the window is requested. Callers on the UI goroutine should run this inside
// a tea.Cmd.
func OpenTerminal(dir string) error {
	p, ok := TerminalProfiles[config.Current.Terminal]
	if !ok {
		return fmt.Errorf("OpenTerminal: unknown terminal %q", config.Current.Terminal)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("OpenTerminal: %w", err)
	}

	args := p.resolveOpenArgs(abs)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = abs
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if reason := lastNonEmptyLine(stderr.String()); reason != "" {
			return fmt.Errorf("OpenTerminal: %s: %w", reason, err)
		}
		return fmt.Errorf("OpenTerminal: %w", err)
	}
	return nil
}

// lastNonEmptyLine returns the final non-blank line of s, which for a failed
// launcher is the actual complaint. Taking the last line skips the startup
// chatter that Electron-based CLIs print ahead of it.
func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}

// CopyToClipboard copies text to the system clipboard via pbcopy. It waits
// for pbcopy to finish before returning.
func CopyToClipboard(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = bytes.NewBufferString(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("CopyToClipboard: %w", err)
	}
	return nil
}
