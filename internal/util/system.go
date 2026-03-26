package util

import (
	"bytes"
	"fmt"
	"os/exec"
)

// OpenTerminal opens a new iTerm window in the given directory. It fires and
// forgets — it does not wait for the process to finish.
func OpenTerminal(dir string) error {
	cmd := exec.Command("open", "-a", "iTerm", dir)
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
