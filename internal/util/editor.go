// Package util provides shared utility functions for code-factory.
package util

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fimmtiu/code-factory/internal/config"
)

// EditText writes existingContent to a temporary file, opens the blocking
// editor from config.Current, waits for it to exit, reads back the file
// contents, deletes the temp file, and returns the contents.
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

// OpenFileInEditor opens an existing file directly in the blocking editor from
// config.Current and waits for the editor to exit. The file is not modified
// or deleted by this function.
func OpenFileInEditor(path string) error {
	command := config.Current.BlockingEditorCommand
	if command == "" {
		return fmt.Errorf("OpenFileInEditor: no editor command configured")
	}
	parts := strings.Fields(command)
	args := append(parts[1:], path)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func editTextImpl(existingContent string, deleteAfter bool) (string, string, error) {
	command := config.Current.BlockingEditorCommand
	if command == "" {
		return "", "", fmt.Errorf("EditText: no editor command configured")
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

	// Command may contain arguments (e.g. "cursor --wait"), so split on spaces.
	parts := strings.Fields(command)
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
