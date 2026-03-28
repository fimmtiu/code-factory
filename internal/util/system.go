package util

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/fimmtiu/code-factory/internal/config"
)

// OpenTerminal opens a terminal window in dir using the command from
// config.Current. It fires and forgets — it does not wait for the process
// to finish.
func OpenTerminal(dir string) error {
	command := config.Current.OpenTerminalCommand
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("OpenTerminal: empty command")
	}
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
