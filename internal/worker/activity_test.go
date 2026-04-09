package worker

import (
	"context"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"

	"github.com/fimmtiu/code-factory/internal/models"
)

// stubDB is a minimal dbInterface for activity tests.
type stubDB struct{}

func (stubDB) SetStatus(string, models.TicketPhase, models.TicketStatus) error { return nil }

func newTestClient() (*acpWorkerClient, *Worker) {
	w := NewWorker(1)
	logCh := make(chan LogMessage, 100)
	c := &acpWorkerClient{
		w:          w,
		database:   stubDB{},
		identifier: "proj/t1",
		phase:      models.PhaseImplement,
		logCh:      logCh,
	}
	return c, w
}

func sessionNotification(u acp.SessionUpdate) acp.SessionNotification {
	return acp.SessionNotification{Update: u}
}

// --- Worker field tests ---

func TestWorker_GetSetActivity(t *testing.T) {
	w := NewWorker(1)
	if got := w.GetActivity(); got != "" {
		t.Errorf("new worker activity = %q, want empty", got)
	}
	w.SetActivity("thinking")
	if got := w.GetActivity(); got != "thinking" {
		t.Errorf("activity = %q, want %q", got, "thinking")
	}
	w.SetActivity("")
	if got := w.GetActivity(); got != "" {
		t.Errorf("cleared activity = %q, want empty", got)
	}
}

func TestWorker_GetSetLastActivityAt(t *testing.T) {
	w := NewWorker(1)
	if got := w.GetLastActivityAt(); !got.IsZero() {
		t.Errorf("new worker LastActivityAt = %v, want zero", got)
	}
	now := time.Now()
	w.SetLastActivityAt(now)
	if got := w.GetLastActivityAt(); !got.Equal(now) {
		t.Errorf("LastActivityAt = %v, want %v", got, now)
	}
	w.SetLastActivityAt(time.Time{})
	if got := w.GetLastActivityAt(); !got.IsZero() {
		t.Errorf("cleared LastActivityAt = %v, want zero", got)
	}
}

// --- SessionUpdate activity tracking ---

func TestSessionUpdate_ThoughtSetsThinking(t *testing.T) {
	c, w := newTestClient()
	err := c.SessionUpdate(context.Background(), sessionNotification(
		acp.UpdateAgentThoughtText("considering approaches..."),
	))
	if err != nil {
		t.Fatal(err)
	}
	if got := w.GetActivity(); got != "thinking" {
		t.Errorf("activity = %q, want %q", got, "thinking")
	}
}

func TestSessionUpdate_MessageSetsResponding(t *testing.T) {
	c, w := newTestClient()
	err := c.SessionUpdate(context.Background(), sessionNotification(
		acp.UpdateAgentMessageText("here is my analysis..."),
	))
	if err != nil {
		t.Fatal(err)
	}
	if got := w.GetActivity(); got != "responding" {
		t.Errorf("activity = %q, want %q", got, "responding")
	}
}

func TestSessionUpdate_ToolCallInProgressSetsToolActivity(t *testing.T) {
	c, w := newTestClient()
	err := c.SessionUpdate(context.Background(), sessionNotification(
		acp.StartToolCall("call-1", "Bash", acp.WithStartStatus(acp.ToolCallStatusInProgress)),
	))
	if err != nil {
		t.Fatal(err)
	}
	if got := w.GetActivity(); got != "tool: Bash" {
		t.Errorf("activity = %q, want %q", got, "tool: Bash")
	}
}

func TestSessionUpdate_ToolCallCompletedClearsActivity(t *testing.T) {
	c, w := newTestClient()
	// Set activity via an in-progress tool call.
	_ = c.SessionUpdate(context.Background(), sessionNotification(
		acp.StartToolCall("call-1", "Bash", acp.WithStartStatus(acp.ToolCallStatusInProgress)),
	))
	// Complete it.
	_ = c.SessionUpdate(context.Background(), sessionNotification(
		acp.StartToolCall("call-1", "Bash", acp.WithStartStatus(acp.ToolCallStatusCompleted)),
	))
	if got := w.GetActivity(); got != "" {
		t.Errorf("activity after completed tool call = %q, want empty", got)
	}
}

func TestSessionUpdate_ToolCallFailedClearsActivity(t *testing.T) {
	c, w := newTestClient()
	_ = c.SessionUpdate(context.Background(), sessionNotification(
		acp.StartToolCall("call-1", "Bash", acp.WithStartStatus(acp.ToolCallStatusInProgress)),
	))
	_ = c.SessionUpdate(context.Background(), sessionNotification(
		acp.StartToolCall("call-1", "Bash", acp.WithStartStatus(acp.ToolCallStatusFailed)),
	))
	if got := w.GetActivity(); got != "" {
		t.Errorf("activity after failed tool call = %q, want empty", got)
	}
}

func TestSessionUpdate_ToolCallUpdateCompletedClearsActivity(t *testing.T) {
	c, w := newTestClient()
	// Set activity via a tool call.
	_ = c.SessionUpdate(context.Background(), sessionNotification(
		acp.StartToolCall("call-1", "Read", acp.WithStartStatus(acp.ToolCallStatusInProgress)),
	))
	// Complete via update.
	_ = c.SessionUpdate(context.Background(), sessionNotification(
		acp.UpdateToolCall("call-1", acp.WithUpdateStatus(acp.ToolCallStatusCompleted)),
	))
	if got := w.GetActivity(); got != "" {
		t.Errorf("activity after completed tool update = %q, want empty", got)
	}
}

func TestSessionUpdate_UpdatesLastActivityAt(t *testing.T) {
	c, w := newTestClient()
	before := time.Now()
	_ = c.SessionUpdate(context.Background(), sessionNotification(
		acp.UpdateAgentThoughtText("thinking..."),
	))
	after := time.Now()

	got := w.GetLastActivityAt()
	if got.Before(before) || got.After(after) {
		t.Errorf("LastActivityAt = %v, want between %v and %v", got, before, after)
	}
}

func TestSessionUpdate_EachEventUpdatesLastActivityAt(t *testing.T) {
	c, w := newTestClient()

	// Thought chunk.
	_ = c.SessionUpdate(context.Background(), sessionNotification(
		acp.UpdateAgentThoughtText("thinking..."),
	))
	t1 := w.GetLastActivityAt()
	if t1.IsZero() {
		t.Fatal("LastActivityAt not set after thought chunk")
	}

	// Tool call.
	_ = c.SessionUpdate(context.Background(), sessionNotification(
		acp.StartToolCall("call-1", "Bash", acp.WithStartStatus(acp.ToolCallStatusInProgress)),
	))
	t2 := w.GetLastActivityAt()
	if t2.Before(t1) {
		t.Error("LastActivityAt did not advance after tool call")
	}

	// Message chunk.
	_ = c.SessionUpdate(context.Background(), sessionNotification(
		acp.UpdateAgentMessageText("result"),
	))
	t3 := w.GetLastActivityAt()
	if t3.Before(t2) {
		t.Error("LastActivityAt did not advance after message chunk")
	}
}
