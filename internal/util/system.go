package util

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/fimmtiu/code-factory/internal/config"
)

// TerminalProfile holds the attributes for a named terminal emulator.
type TerminalProfile struct {
	BundleID    string // macOS bundle identifier, e.g. "com.googlecode.iterm2"
	OpenCommand string // command to open a window in a directory, e.g. "open -a iTerm ."
}

// TerminalProfiles maps the supported terminal names to their profiles.
var TerminalProfiles = map[string]TerminalProfile{
	"iterm2":   {BundleID: "com.googlecode.iterm2", OpenCommand: "open -a iTerm ."},
	"terminal": {BundleID: "com.apple.Terminal", OpenCommand: "open -a Terminal ."},
}

// ValidateTerminal returns an error if name is not a supported terminal.
func ValidateTerminal(name string) error {
	if _, ok := TerminalProfiles[name]; !ok {
		supported := make([]string, 0, len(TerminalProfiles))
		for k := range TerminalProfiles {
			supported = append(supported, k)
		}
		return fmt.Errorf("unknown terminal %q in settings.json; supported values: %s",
			name, strings.Join(supported, ", "))
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

// OpenTerminal opens a terminal window in dir using the command from the
// currently configured terminal profile. It fires and forgets — it does not
// wait for the process to finish.
func OpenTerminal(dir string) error {
	p, ok := TerminalProfiles[config.Current.Terminal]
	if !ok {
		return fmt.Errorf("OpenTerminal: unknown terminal %q", config.Current.Terminal)
	}
	parts := strings.Fields(p.OpenCommand)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("OpenTerminal: %w", err)
	}
	return nil
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
