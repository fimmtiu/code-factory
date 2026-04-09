package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/fimmtiu/code-factory/internal/worker"
)

// --- formatLastActivity tests ---

func TestFormatLastActivity_Seconds(t *testing.T) {
	tests := []struct {
		dur  time.Duration
		want string
	}{
		{0, "0s ago"},
		{30 * time.Second, "30s ago"},
		{59 * time.Second, "59s ago"},
	}
	for _, tt := range tests {
		if got := formatLastActivity(tt.dur); got != tt.want {
			t.Errorf("formatLastActivity(%v) = %q, want %q", tt.dur, got, tt.want)
		}
	}
}

func TestFormatLastActivity_Minutes(t *testing.T) {
	tests := []struct {
		dur  time.Duration
		want string
	}{
		{60 * time.Second, "1m ago"},
		{90 * time.Second, "1m ago"},
		{5 * time.Minute, "5m ago"},
		{59*time.Minute + 59*time.Second, "59m ago"},
	}
	for _, tt := range tests {
		if got := formatLastActivity(tt.dur); got != tt.want {
			t.Errorf("formatLastActivity(%v) = %q, want %q", tt.dur, got, tt.want)
		}
	}
}

func TestFormatLastActivity_Hours(t *testing.T) {
	tests := []struct {
		dur  time.Duration
		want string
	}{
		{time.Hour, "1h ago"},
		{2*time.Hour + 30*time.Minute, "2h ago"},
	}
	for _, tt := range tests {
		if got := formatLastActivity(tt.dur); got != tt.want {
			t.Errorf("formatLastActivity(%v) = %q, want %q", tt.dur, got, tt.want)
		}
	}
}

// --- renderStatusLine tests ---

func TestRenderStatusLine_BusyShowsTicketNotStatus(t *testing.T) {
	w := worker.NewWorker(3)
	w.Status = worker.StatusBusy
	w.SetCurrentTicket("implement diff-viewer/keybinding-integration")

	v := NewWorkerView(nil)
	line := v.renderStatusLine(w)

	if !strings.Contains(line, "Worker 3: implement diff-viewer/keybinding-integration") {
		t.Errorf("busy status line should show phase and ticket:\n%s", line)
	}
	if strings.Contains(line, "busy") {
		t.Errorf("busy status line should not contain the word 'busy':\n%s", line)
	}
}

func TestRenderStatusLine_IdleShowsIdle(t *testing.T) {
	w := worker.NewWorker(2)
	w.Status = worker.StatusIdle

	v := NewWorkerView(nil)
	line := v.renderStatusLine(w)

	if !strings.Contains(line, "Worker 2: idle") {
		t.Errorf("idle status line should say 'idle':\n%s", line)
	}
}

func TestRenderStatusLine_ShowsActivityWhenBusy(t *testing.T) {
	w := worker.NewWorker(1)
	w.Status = worker.StatusBusy
	w.SetActivity("thinking")
	w.SetCurrentTicket("review proj/t1")

	v := NewWorkerView(nil)
	line := v.renderStatusLine(w)

	if !strings.Contains(line, "thinking") {
		t.Errorf("status line does not contain activity %q:\n%s", "thinking", line)
	}
}

func TestRenderStatusLine_ShowsToolActivity(t *testing.T) {
	w := worker.NewWorker(1)
	w.Status = worker.StatusBusy
	w.SetActivity("tool: Bash")
	w.SetCurrentTicket("implement proj/t1")

	v := NewWorkerView(nil)
	line := v.renderStatusLine(w)

	if !strings.Contains(line, "tool: Bash") {
		t.Errorf("status line does not contain activity %q:\n%s", "tool: Bash", line)
	}
}

func TestRenderStatusLine_ShowsLastActivity(t *testing.T) {
	w := worker.NewWorker(1)
	w.Status = worker.StatusBusy
	w.SetLastActivityAt(time.Now().Add(-5 * time.Minute))
	w.SetCurrentTicket("review proj/t1")

	v := NewWorkerView(nil)
	line := v.renderStatusLine(w)

	if !strings.Contains(line, "5m ago") {
		t.Errorf("status line does not contain %q:\n%s", "5m ago", line)
	}
}

func TestRenderStatusLine_HidesRecentLastActivity(t *testing.T) {
	w := worker.NewWorker(1)
	w.Status = worker.StatusBusy
	w.SetLastActivityAt(time.Now().Add(-1 * time.Second))
	w.SetCurrentTicket("review proj/t1")

	v := NewWorkerView(nil)
	line := v.renderStatusLine(w)

	if strings.Contains(line, "ago") {
		t.Errorf("status line should not show very recent last activity:\n%s", line)
	}
}

func TestRenderStatusLine_HidesActivityWhenIdle(t *testing.T) {
	w := worker.NewWorker(1)
	w.Status = worker.StatusIdle
	w.SetActivity("thinking")

	v := NewWorkerView(nil)
	line := v.renderStatusLine(w)

	if strings.Contains(line, "thinking") {
		t.Errorf("idle status line should not contain activity:\n%s", line)
	}
}

func TestRenderStatusLine_HidesActivityWhenPaused(t *testing.T) {
	w := worker.NewWorker(1)
	w.Status = worker.StatusBusy
	w.Paused = true
	w.SetActivity("thinking")

	v := NewWorkerView(nil)
	line := v.renderStatusLine(w)

	if strings.Contains(line, "thinking") {
		t.Errorf("paused status line should not contain activity:\n%s", line)
	}
}

func TestRenderStatusLine_ShowsActivityForAwaitingResponse(t *testing.T) {
	w := worker.NewWorker(1)
	w.Status = worker.StatusAwaitingResponse
	w.SetLastActivityAt(time.Now().Add(-2 * time.Minute))
	w.SetCurrentTicket("review proj/t1")

	v := NewWorkerView(nil)
	line := v.renderStatusLine(w)

	if !strings.Contains(line, "2m ago") {
		t.Errorf("awaiting-response status line should show last activity:\n%s", line)
	}
}
