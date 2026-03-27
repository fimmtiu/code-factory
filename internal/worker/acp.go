package worker

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	acp "github.com/coder/acp-go-sdk"

	"github.com/fimmtiu/tickets/internal/models"
)

// acpWorkerClient implements acp.Client on behalf of a worker goroutine.
// It streams session updates to the logfile and LastOutput buffer, and
// forwards permission requests to the main goroutine via the worker channels.
type acpWorkerClient struct {
	w       *Worker
	logFile *os.File
	logMu   sync.Mutex // guards logFile and lastOutputBuf writes

	// db and ticket state needed for permission handling.
	database   dbInterface
	identifier string
	phase      string

	logCh chan<- LogMessage
}

// dbInterface is the minimal subset of db.DB used by the ACP client.
// Defined as an interface so tests can substitute a fake.
type dbInterface interface {
	SetStatus(identifier, phase, status string) error
}

// appendOutput writes text to the logfile and updates LastOutput on the worker.
func (c *acpWorkerClient) appendOutput(text string) {
	c.logMu.Lock()
	defer c.logMu.Unlock()

	if c.logFile != nil && text != "" {
		_, _ = fmt.Fprint(c.logFile, text)
	}

	current := c.w.GetLastOutput()
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			continue
		}
		current = append(current, line)
		if len(current) > 3 {
			current = current[len(current)-3:]
		}
	}
	c.w.SetLastOutput(current)
}

// SessionUpdate receives streaming output from the agent and writes it to the
// logfile. Both agent messages and thought chunks are captured.
func (c *acpWorkerClient) SessionUpdate(_ context.Context, params acp.SessionNotification) error {
	u := params.Update
	switch {
	case u.AgentMessageChunk != nil:
		if u.AgentMessageChunk.Content.Text != nil {
			c.appendOutput(u.AgentMessageChunk.Content.Text.Text)
		}
	case u.AgentThoughtChunk != nil:
		if u.AgentThoughtChunk.Content.Text != nil {
			c.appendOutput("[thought] " + u.AgentThoughtChunk.Content.Text.Text)
		}
	case u.ToolCall != nil:
		title := u.ToolCall.Title
		c.appendOutput(fmt.Sprintf("[tool] %s (%s)\n", title, u.ToolCall.Status))
	case u.ToolCallUpdate != nil:
		status := ""
		if u.ToolCallUpdate.Status != nil {
			status = string(*u.ToolCallUpdate.Status)
		}
		c.appendOutput(fmt.Sprintf("[tool-update] %s: %s\n", u.ToolCallUpdate.ToolCallId, status))
	}
	return nil
}

// RequestPermission is called by the agent when it needs the user to approve
// an action. This method:
//  1. Sets the worker status to AwaitingResponse.
//  2. Marks the ticket as needs-attention.
//  3. Sends a MsgPermissionRequest to the main goroutine.
//  4. Blocks until the main goroutine sends back a MsgPermission response.
func (c *acpWorkerClient) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	title := ""
	if params.ToolCall.Title != nil {
		title = *params.ToolCall.Title
	}
	var optionNames []string
	for _, o := range params.Options {
		optionNames = append(optionNames, fmt.Sprintf("%s (%s)", o.Name, o.Kind))
	}
	payload := title
	if len(optionNames) > 0 {
		payload += "\nOptions: " + strings.Join(optionNames, ", ")
	}

	c.w.Status = StatusAwaitingResponse
	_ = c.database.SetStatus(c.identifier, c.phase, models.StatusNeedsAttention)
	c.logCh <- NewLogMessage(c.w.Number, fmt.Sprintf("permission request for %s: %s", c.identifier, title))

	c.w.FromWorker <- WorkerToMainMessage{
		WorkerNumber: c.w.Number,
		Kind:         MsgPermissionRequest,
		Payload:      payload,
	}

	select {
	case <-ctx.Done():
		c.w.Status = StatusBusy
		_ = c.database.SetStatus(c.identifier, c.phase, models.StatusInProgress)
		return acp.RequestPermissionResponse{
			Outcome: acp.RequestPermissionOutcome{
				Cancelled: &acp.RequestPermissionOutcomeCancelled{},
			},
		}, nil

	case msg := <-c.w.ToWorker:
		c.w.Status = StatusBusy
		_ = c.database.SetStatus(c.identifier, c.phase, models.StatusInProgress)
		c.logCh <- NewLogMessage(c.w.Number, fmt.Sprintf("permission response for %s: %s", c.identifier, msg.Payload))

		for _, o := range params.Options {
			if strings.EqualFold(msg.Payload, string(o.Kind)) ||
				strings.EqualFold(msg.Payload, o.Name) {
				return acp.RequestPermissionResponse{
					Outcome: acp.NewRequestPermissionOutcomeSelected(o.OptionId),
				}, nil
			}
		}
		for _, o := range params.Options {
			if o.Kind == acp.PermissionOptionKindAllowOnce {
				return acp.RequestPermissionResponse{
					Outcome: acp.NewRequestPermissionOutcomeSelected(o.OptionId),
				}, nil
			}
		}
		if len(params.Options) > 0 {
			return acp.RequestPermissionResponse{
				Outcome: acp.NewRequestPermissionOutcomeSelected(params.Options[0].OptionId),
			}, nil
		}
		return acp.RequestPermissionResponse{
			Outcome: acp.RequestPermissionOutcome{
				Cancelled: &acp.RequestPermissionOutcomeCancelled{},
			},
		}, nil
	}
}

// ReadTextFile serves file read requests from the agent by reading from disk.
func (c *acpWorkerClient) ReadTextFile(_ context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	b, err := os.ReadFile(params.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, fmt.Errorf("read %s: %w", params.Path, err)
	}
	return acp.ReadTextFileResponse{Content: string(b)}, nil
}

// WriteTextFile serves file write requests from the agent by writing to disk.
func (c *acpWorkerClient) WriteTextFile(_ context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	if err := os.MkdirAll(strings.TrimSuffix(params.Path, "/"+lastSegment(params.Path)), 0o755); err != nil {
		return acp.WriteTextFileResponse{}, fmt.Errorf("mkdir for %s: %w", params.Path, err)
	}
	if err := os.WriteFile(params.Path, []byte(params.Content), 0o644); err != nil {
		return acp.WriteTextFileResponse{}, fmt.Errorf("write %s: %w", params.Path, err)
	}
	return acp.WriteTextFileResponse{}, nil
}

// Terminal methods are no-ops; we do not support terminal features.
func (c *acpWorkerClient) CreateTerminal(_ context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{TerminalId: "term-unsupported"}, nil
}

func (c *acpWorkerClient) KillTerminalCommand(_ context.Context, _ acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error) {
	return acp.KillTerminalCommandResponse{}, nil
}

func (c *acpWorkerClient) TerminalOutput(_ context.Context, _ acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{Output: "", Truncated: false}, nil
}

func (c *acpWorkerClient) ReleaseTerminal(_ context.Context, _ acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, nil
}

func (c *acpWorkerClient) WaitForTerminalExit(_ context.Context, _ acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, nil
}

// Ensure acpWorkerClient satisfies the acp.Client interface at compile time.
var _ acp.Client = (*acpWorkerClient)(nil)

// runACP launches Claude Code as an ACP subprocess in the ticket's worktree
// directory, sends the prompt, and streams all output to the logfile.
// It returns after the prompt completes or the context is cancelled.
func runACP(
	ctx context.Context,
	w *Worker,
	database dbInterface,
	logCh chan<- LogMessage,
	worktreePath string,
	identifier string,
	phase string,
	prompt string,
	logfilePath string,
) error {
	logFile, err := os.Create(logfilePath)
	if err != nil {
		return fmt.Errorf("create logfile %s: %w", logfilePath, err)
	}
	defer logFile.Close()

	_, _ = fmt.Fprintf(logFile, "=== PROMPT ===\n%s\n=== OUTPUT ===\n", prompt)

	cmd := exec.CommandContext(ctx, "npx", "-y", "@zed-industries/claude-code-acp@latest")
	cmd.Dir = worktreePath
	cmd.Stderr = newPrefixWriter(logFile, "[stderr] ")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Claude Code: %w", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	client := &acpWorkerClient{
		w:          w,
		logFile:    logFile,
		database:   database,
		identifier: identifier,
		phase:      phase,
		logCh:      logCh,
	}

	conn := acp.NewClientSideConnection(client, stdin, stdout)

	_, err = conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapability{ReadTextFile: true, WriteTextFile: true},
		},
	})
	if err != nil {
		return fmt.Errorf("ACP initialize: %w", err)
	}

	sessResp, err := conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        worktreePath,
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		return fmt.Errorf("ACP new session: %w", err)
	}

	_, err = conn.Prompt(ctx, acp.PromptRequest{
		SessionId: sessResp.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock(prompt)},
	})
	if err != nil {
		return fmt.Errorf("ACP prompt: %w", err)
	}

	_ = cmd.Wait()
	return nil
}

// prefixWriter prepends a prefix string to every line written to the
// underlying writer.
type prefixWriter struct {
	w      *os.File
	prefix string
	buf    *bufio.Writer
}

func newPrefixWriter(f *os.File, prefix string) *prefixWriter {
	return &prefixWriter{w: f, prefix: prefix, buf: bufio.NewWriter(f)}
}

func (pw *prefixWriter) Write(p []byte) (int, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(p)))
	for scanner.Scan() {
		_, _ = fmt.Fprintf(pw.buf, "%s%s\n", pw.prefix, scanner.Text())
	}
	_ = pw.buf.Flush()
	return len(p), nil
}

// lastSegment returns the last path segment of path.
func lastSegment(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return path
	}
	return path[idx+1:]
}
