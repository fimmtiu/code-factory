package daemon_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fimmtiu/tickets/internal/daemon"
	"github.com/fimmtiu/tickets/internal/protocol"
)

// newTestDaemon creates a Daemon without starting its listener, suitable
// for worker tests that drive the queue directly.
func newTestDaemon(t *testing.T) *daemon.Daemon {
	t.Helper()
	return daemon.NewDaemon(tempSocketPath(t))
}

// TestWorkerPingHandler verifies that the built-in ping handler returns a
// successful response containing the daemon's PID.
func TestWorkerPingHandler(t *testing.T) {
	d := newTestDaemon(t)
	w := daemon.NewWorker(d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	// Push a ping command directly onto the queue.
	respCh := make(chan protocol.Response, 1)
	d.Queue() <- &daemon.QueueItem{
		Cmd:      protocol.Command{Name: "ping"},
		Response: respCh,
	}

	select {
	case resp := <-respCh:
		if !resp.Success {
			t.Errorf("expected Success=true, got false (error: %q)", resp.Error)
		}
		// Data should contain a "pid" field.
		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("failed to unmarshal ping data: %v", err)
		}
		pid, ok := data["pid"]
		if !ok {
			t.Error("expected 'pid' field in ping response data")
		}
		if pid.(float64) <= 0 {
			t.Errorf("expected positive pid, got %v", pid)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ping response")
	}
}

// TestWorkerUnknownCommand verifies that an unregistered command returns a
// failure response.
func TestWorkerUnknownCommand(t *testing.T) {
	d := newTestDaemon(t)
	w := daemon.NewWorker(d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	respCh := make(chan protocol.Response, 1)
	d.Queue() <- &daemon.QueueItem{
		Cmd:      protocol.Command{Name: "no-such-command"},
		Response: respCh,
	}

	select {
	case resp := <-respCh:
		if resp.Success {
			t.Error("expected Success=false for unknown command")
		}
		if resp.Error == "" {
			t.Error("expected non-empty error message for unknown command")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for response")
	}
}

// TestWorkerSequentialExecution verifies that multiple commands are processed
// in order (one at a time).
func TestWorkerSequentialExecution(t *testing.T) {
	d := newTestDaemon(t)
	w := daemon.NewWorker(d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	const n = 5
	channels := make([]chan protocol.Response, n)
	for i := range channels {
		channels[i] = make(chan protocol.Response, 1)
	}

	// Enqueue all commands at once.
	for i := 0; i < n; i++ {
		d.Queue() <- &daemon.QueueItem{
			Cmd:      protocol.Command{Name: "ping"},
			Response: channels[i],
		}
	}

	// All responses should arrive.
	for i := 0; i < n; i++ {
		select {
		case resp := <-channels[i]:
			if !resp.Success {
				t.Errorf("command %d: expected Success=true", i)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for response %d", i)
		}
	}
}

// TestWorkerCustomHandler verifies that RegisterHandler allows adding new
// command handlers at runtime.
func TestWorkerCustomHandler(t *testing.T) {
	d := newTestDaemon(t)
	w := daemon.NewWorker(d)
	w.RegisterHandler("echo", func(cmd protocol.Command) protocol.Response {
		msg := cmd.Params["message"]
		data, _ := json.Marshal(map[string]string{"echo": msg})
		return protocol.Response{Success: true, Data: json.RawMessage(data)}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	respCh := make(chan protocol.Response, 1)
	d.Queue() <- &daemon.QueueItem{
		Cmd:      protocol.Command{Name: "echo", Params: map[string]string{"message": "hello"}},
		Response: respCh,
	}

	select {
	case resp := <-respCh:
		if !resp.Success {
			t.Errorf("expected Success=true, got: %q", resp.Error)
		}
		var data map[string]string
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal echo data: %v", err)
		}
		if data["echo"] != "hello" {
			t.Errorf("expected echo='hello', got %q", data["echo"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for echo response")
	}
}

// TestWorkerContextCancellation verifies that Run exits promptly when the
// context is cancelled.
func TestWorkerContextCancellation(t *testing.T) {
	d := newTestDaemon(t)
	w := daemon.NewWorker(d)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	cancel()

	// Close the queue so Run doesn't block on a receive after cancel.
	close(d.Queue())

	select {
	case <-done:
		// Run exited as expected.
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for worker to stop after context cancellation")
	}
}

// TestWorkerLastNonHousekeepingCmd verifies that LastNonHousekeepingCmd is
// updated for non-ping commands but not for ping.
func TestWorkerLastNonHousekeepingCmd(t *testing.T) {
	d := newTestDaemon(t)
	w := daemon.NewWorker(d)

	// Register a non-housekeeping command.
	w.RegisterHandler("work", func(cmd protocol.Command) protocol.Response {
		return protocol.Response{Success: true}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	before := w.LastNonHousekeepingCmd()

	// Send a ping — should NOT update the timestamp.
	respCh := make(chan protocol.Response, 1)
	d.Queue() <- &daemon.QueueItem{Cmd: protocol.Command{Name: "ping"}, Response: respCh}
	<-respCh

	afterPing := w.LastNonHousekeepingCmd()
	if afterPing != before {
		t.Error("ping should not update LastNonHousekeepingCmd")
	}

	// Send a "work" command — should update the timestamp.
	respCh2 := make(chan protocol.Response, 1)
	d.Queue() <- &daemon.QueueItem{Cmd: protocol.Command{Name: "work"}, Response: respCh2}
	<-respCh2

	afterWork := w.LastNonHousekeepingCmd()
	if !afterWork.After(before) {
		t.Error("expected LastNonHousekeepingCmd to be updated after 'work' command")
	}
}
