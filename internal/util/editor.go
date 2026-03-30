// Package util provides shared utility functions for code-factory.
package util

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fimmtiu/code-factory/internal/config"
)

// EditorProfile holds the shell commands for a named editor.
type EditorProfile struct {
	Blocking           string // waits for the editor to close, e.g. "cursor --wait"
	Nonblocking        string // opens in the background, e.g. "cursor"
	BlockingAtLocation string // blocking open at a file:line, e.g. "cursor --wait --goto"
}

// EditorProfiles maps the supported editor names to their shell commands.
var EditorProfiles = map[string]EditorProfile{
	"cursor": {"cursor --wait", "cursor", "cursor --wait --goto"},
	"vscode": {"code --wait", "code", "code --wait --goto"},
}

// ValidateEditor returns an error if name is not a supported editor.
// Call this at startup after config.Init so the program exits cleanly on
// an invalid setting.
func ValidateEditor(name string) error {
	if _, ok := EditorProfiles[name]; !ok {
		supported := make([]string, 0, len(EditorProfiles))
		for k := range EditorProfiles {
			supported = append(supported, k)
		}
		return fmt.Errorf("unknown editor %q in settings.json; supported values: %s",
			name, strings.Join(supported, ", "))
	}
	return nil
}

// NonblockingEditorCommand returns the nonblocking editor command for the
// currently configured editor (e.g. "cursor" or "code").
func NonblockingEditorCommand() string {
	if p, ok := EditorProfiles[config.Current.Editor]; ok {
		return p.Nonblocking
	}
	return ""
}

func blockingEditorCommand() string {
	if p, ok := EditorProfiles[config.Current.Editor]; ok {
		return p.Blocking
	}
	return ""
}

// blockingEditorAtLocationCommand returns the full command string that opens
// the blocking editor at filename:lineNo. For example:
// "cursor --wait --goto internal/db/db.go:42".
func blockingEditorAtLocationCommand(filename string, lineNo int) string {
	if p, ok := EditorProfiles[config.Current.Editor]; ok {
		return fmt.Sprintf("%s %s:%d", p.BlockingAtLocation, filename, lineNo)
	}
	return ""
}

// OpenEditorAtLocation opens filename at lineNo in the blocking editor and
// waits for the editor to close. The file is opened directly — no temp file
// is created.
func OpenEditorAtLocation(filename string, lineNo int) error {
	command := blockingEditorAtLocationCommand(filename, lineNo)
	if command == "" {
		return fmt.Errorf("OpenEditorAtLocation: no editor command configured")
	}
	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// EditText writes existingContent to a temporary file, opens the blocking
// editor from config.Current, waits for it to exit, reads back the file
// contents, deletes the temp file, and returns the contents.
func EditText(existingContent string) (string, error) {
	content, _, err := editTextImpl(existingContent, true)
	return content, err
}

// EditTextAtLine is like EditText but opens the editor positioned at lineNo
// (1-based) using the BlockingAtLocation command, so the cursor lands on the
// correct line rather than at the top of the file.
func EditTextAtLine(existingContent string, lineNo int) (string, error) {
	tmpFile, err := os.CreateTemp("", "code-factory-edit-*.txt")
	if err != nil {
		return "", fmt.Errorf("EditTextAtLine: create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(existingContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("EditTextAtLine: write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("EditTextAtLine: close temp file: %w", err)
	}

	if err := OpenEditorAtLocation(tmpPath, lineNo); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("EditTextAtLine: editor exited with error: %w", err)
	}

	data, err := os.ReadFile(tmpPath)
	os.Remove(tmpPath)
	if err != nil {
		return "", fmt.Errorf("EditTextAtLine: read temp file: %w", err)
	}
	return string(data), nil
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
	command := blockingEditorCommand()
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
	command := blockingEditorCommand()
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
