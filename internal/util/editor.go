// Package util provides shared utility functions for code-factory.
package util

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// EditText writes existingContent to a temporary file, opens $EDITOR on it,
// waits for the editor to exit, reads back the file contents, deletes the
// temp file, and returns the contents.
func EditText(existingContent string) (string, error) {
	content, _, err := editTextImpl(existingContent, true)
	return content, err
}

// EditTextKeepFile is like EditText but does not delete the temporary file.
// It returns both the file contents and the path to the temp file. The caller
// is responsible for the file's lifetime.
func EditTextKeepFile(existingContent string) (content, path string, err error) {
	return editTextImpl(existingContent, false)
}

func editTextImpl(existingContent string, deleteAfter bool) (string, string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return "", "", fmt.Errorf("EditText: $EDITOR is not set")
	}

	tmpFile, err := os.CreateTemp("", "code-factory-edit-*.txt")
	if err != nil {
		return "", "", fmt.Errorf("EditText: create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(existingContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("EditText: write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("EditText: close temp file: %w", err)
	}

	// $EDITOR may contain arguments (e.g. "vim -u NONE"), so split on spaces.
	parts := strings.Fields(editor)
	args := append(parts[1:], tmpPath)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("EditText: editor exited with error: %w", err)
	}

	data, err := os.ReadFile(tmpPath)
	if deleteAfter {
		os.Remove(tmpPath)
	}
	if err != nil {
		return "", "", fmt.Errorf("EditText: read temp file: %w", err)
	}

	return string(data), tmpPath, nil
}
