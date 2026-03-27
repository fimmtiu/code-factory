package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fimmtiu/tickets/internal/models"
)

// mockScript is the sequence of output lines the mock worker emits.
// The first batch is emitted before the needs-attention pause; the second
// batch after the user responds.
var mockScriptBefore = []string{
	"Reading existing specs...",
	"Analysing code structure...",
	"Planning implementation approach...",
	"Writing new test cases...",
	"Running go test ./...",
}

var mockScriptAfter = []string{
	"All tests pass.",
	"Implementing changes...",
	"Running go vet ./...",
	"Committing work...",
	"Done.",
}

// MockWorkFn is a WorkFn that simulates an ACP agent session for UI testing.
// It streams fake output lines, pauses at needs-attention waiting for a real
// user response (just like a real worker would), then writes a timestamped
// file to the worktree and commits it.
// The prompt and logfilePath parameters are accepted to satisfy the WorkFn
// signature; logfilePath is written with the fake output for Log view testing.
func MockWorkFn(ctx context.Context, w *Worker, database dbInterface, logCh chan<- LogMessage, worktreePath, identifier, phase, prompt, logfilePath string) error {
	// Open the logfile so the Log view's E/C keys work on mock entries.
	var logFile *os.File
	if logfilePath != "" {
		if f, err := os.Create(logfilePath); err == nil {
			logFile = f
			defer logFile.Close()
			fmt.Fprintf(logFile, "=== MOCK PROMPT ===\n%s\n=== MOCK OUTPUT ===\n", prompt)
		}
	}

	writeLog := func(line string) {
		if logFile != nil {
			fmt.Fprintln(logFile, line)
		}
	}

	emitLine := func(line string) {
		writeLog(line)
		current := w.GetLastOutput()
		current = append(current, line)
		if len(current) > 3 {
			current = current[len(current)-3:]
		}
		w.SetLastOutput(current)
		logCh <- NewLogMessage(w.Number, fmt.Sprintf("[mock] %s", line))
	}

	emit := func(lines []string) bool {
		for _, line := range lines {
			select {
			case <-ctx.Done():
				return false
			case <-time.After(800 * time.Millisecond):
			}
			emitLine(line)
		}
		return true
	}

	// Phase 1: stream output before the needs-attention pause.
	if !emit(mockScriptBefore) {
		return nil
	}

	// Phase 2: needs-attention — block until the user sends a response.
	w.Status = StatusAwaitingResponse
	if err := database.SetStatus(identifier, phase, models.StatusNeedsAttention); err != nil {
		logCh <- NewLogMessage(w.Number, fmt.Sprintf("[mock] error setting needs-attention: %v", err))
	}
	question := "Mock question: please confirm you want to proceed with these changes."
	writeLog("[mock] asking: " + question)
	logCh <- NewLogMessage(w.Number, fmt.Sprintf("[mock] asking user: %s", question))
	w.FromWorker <- WorkerToMainMessage{
		WorkerNumber: w.Number,
		Kind:         MsgQuestion,
		Payload:      question,
	}

	// Wait for the user's response or context cancellation.
	select {
	case <-ctx.Done():
		return nil
	case msg := <-w.ToWorker:
		w.Status = StatusBusy
		if err := database.SetStatus(identifier, phase, models.StatusInProgress); err != nil {
			logCh <- NewLogMessage(w.Number, fmt.Sprintf("[mock] error restoring in-progress: %v", err))
		}
		writeLog("[mock] response: " + msg.Payload)
		logCh <- NewLogMessage(w.Number, fmt.Sprintf("[mock] received response: %s", msg.Payload))
	}

	// Phase 3: stream output after the response.
	if !emit(mockScriptAfter) {
		return nil
	}

	// Phase 4: write a timestamped file to the worktree and commit it.
	ticketName := filepath.Base(strings.ReplaceAll(identifier, "/", string(filepath.Separator)))
	filename := ticketName + "_mock_work.md"
	filePath := filepath.Join(worktreePath, filename)

	content := fmt.Sprintf("# Mock work for %s\n\nCompleted at: %s\n", identifier, time.Now().Format(time.RFC3339))
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("mock: write %s: %w", filePath, err)
	}

	gitRun := func(args ...string) error {
		cmd := exec.Command("git", append([]string{"-C", worktreePath}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	if err := gitRun("add", filename); err != nil {
		return fmt.Errorf("mock: %w", err)
	}
	commitMsg := fmt.Sprintf("mock work: %s at %s", identifier, time.Now().Format("2006-01-02 15:04:05"))
	if err := gitRun(
		"-c", "user.email=mock@code-factory",
		"-c", "user.name=Code Factory Mock",
		"commit", "-m", commitMsg,
	); err != nil {
		return fmt.Errorf("mock: %w", err)
	}

	w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("[mock] committed %s", filename))
	return nil
}
