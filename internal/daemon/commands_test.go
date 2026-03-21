package daemon_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/fimmtiu/tickets/internal/daemon"
	"github.com/fimmtiu/tickets/internal/models"
	"github.com/fimmtiu/tickets/internal/protocol"
)

// newTestDaemonWithDir creates a Daemon using the given ticketsDir without
// starting its listener. Suitable for command handler tests.
func newTestDaemonWithDir(t *testing.T, ticketsDir string) *daemon.Daemon {
	t.Helper()
	return daemon.NewDaemon(tempSocketPath(t), ticketsDir)
}

// TestCommandPing verifies that the ping command returns a success response
// containing the current process PID.
func TestCommandPing(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)
	d := newTestDaemonWithDir(t, ticketsDir)

	var stopped bool
	stopFn := func() { stopped = true }

	w := daemon.NewWorker(d, stopFn)
	daemon.RegisterCommands(w, d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	respCh := make(chan protocol.Response, 1)
	d.Queue() <- &daemon.QueueItem{
		Cmd:      protocol.Command{Name: "ping"},
		Response: respCh,
	}

	select {
	case resp := <-respCh:
		if !resp.Success {
			t.Fatalf("ping: expected Success=true, got false (%q)", resp.Error)
		}
		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("ping: unmarshal data: %v", err)
		}
		pid, ok := data["pid"]
		if !ok {
			t.Fatal("ping: expected 'pid' in response data")
		}
		if pid.(float64) != float64(os.Getpid()) {
			t.Errorf("ping: expected pid=%d, got %v", os.Getpid(), pid)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ping: timed out waiting for response")
	}

	if stopped {
		t.Error("ping should not call stopFn")
	}
}

// TestCommandExit verifies that the exit command returns success and triggers
// the stop function asynchronously.
func TestCommandExit(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)
	d := newTestDaemonWithDir(t, ticketsDir)

	stopCh := make(chan struct{})
	stopFn := func() { close(stopCh) }

	w := daemon.NewWorker(d, stopFn)
	daemon.RegisterCommands(w, d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	respCh := make(chan protocol.Response, 1)
	d.Queue() <- &daemon.QueueItem{
		Cmd:      protocol.Command{Name: "exit"},
		Response: respCh,
	}

	select {
	case resp := <-respCh:
		if !resp.Success {
			t.Fatalf("exit: expected Success=true, got false (%q)", resp.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("exit: timed out waiting for response")
	}

	// stopFn must be called (asynchronously) after the response is sent.
	select {
	case <-stopCh:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("exit: timed out waiting for stopFn to be called")
	}
}

// TestCommandStatus verifies that the status command returns a JSON array of
// all work units.
func TestCommandStatus(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	wu1 := models.NewTicket("ticket-one", "first ticket")
	wu2 := models.NewTicket("ticket-two", "second ticket")
	writeTicket(t, ticketsDir, wu1)
	writeTicket(t, ticketsDir, wu2)

	d := newTestDaemonWithDir(t, ticketsDir)
	// Load state.
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}

	w := daemon.NewWorker(d, func() {})
	daemon.RegisterCommands(w, d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	respCh := make(chan protocol.Response, 1)
	d.Queue() <- &daemon.QueueItem{
		Cmd:      protocol.Command{Name: "status"},
		Response: respCh,
	}

	select {
	case resp := <-respCh:
		if !resp.Success {
			t.Fatalf("status: expected Success=true, got false (%q)", resp.Error)
		}
		var units []map[string]interface{}
		if err := json.Unmarshal(resp.Data, &units); err != nil {
			t.Fatalf("status: unmarshal data: %v", err)
		}
		if len(units) != 2 {
			t.Errorf("status: expected 2 units, got %d", len(units))
		}
		// Verify each unit has an identifier field.
		for _, u := range units {
			if _, ok := u["identifier"]; !ok {
				t.Error("status: expected 'identifier' field in unit")
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("status: timed out waiting for response")
	}
}

// TestCommandStatusParentField verifies that nested tickets have the parent field set.
func TestCommandStatusParentField(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	// Create a project with one nested ticket.
	projDir := makeTempProjectDir(t, ticketsDir, "my-proj")
	_ = projDir

	d := newTestDaemonWithDir(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}

	w := daemon.NewWorker(d, func() {})
	daemon.RegisterCommands(w, d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	respCh := make(chan protocol.Response, 1)
	d.Queue() <- &daemon.QueueItem{
		Cmd:      protocol.Command{Name: "status"},
		Response: respCh,
	}

	select {
	case resp := <-respCh:
		if !resp.Success {
			t.Fatalf("status: expected Success=true")
		}
		var units []map[string]interface{}
		if err := json.Unmarshal(resp.Data, &units); err != nil {
			t.Fatalf("status: unmarshal: %v", err)
		}
		// Find the nested ticket and verify it has parent set.
		for _, u := range units {
			id, _ := u["identifier"].(string)
			if id == "my-proj/child-ticket" {
				parent, _ := u["parent"].(string)
				if parent != "my-proj" {
					t.Errorf("expected parent='my-proj', got %q", parent)
				}
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("status: timed out")
	}
}

// makeTempProjectDir creates a project directory with one child ticket.
func makeTempProjectDir(t *testing.T, ticketsDir, projectID string) string {
	t.Helper()
	projDir := ticketsDir + "/" + projectID
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	proj := models.NewProject(projectID, "test project")
	from := "github.com/fimmtiu/tickets/internal/storage"
	_ = from // storage imported in state_test.go
	if err := writeProjectFile(t, projDir, proj); err != nil {
		t.Fatalf("writeProjectFile: %v", err)
	}

	child := models.NewTicket(projectID+"/child-ticket", "child")
	writeTicketToDir(t, projDir, "child-ticket", child)

	return projDir
}

func writeProjectFile(t *testing.T, projDir string, wu *models.WorkUnit) error {
	t.Helper()
	path := projDir + "/.project.json"
	return writeWorkUnitToPath(t, path, wu)
}

func writeTicketToDir(t *testing.T, dir, name string, wu *models.WorkUnit) {
	t.Helper()
	path := dir + "/" + name + ".json"
	if err := writeWorkUnitToPath(t, path, wu); err != nil {
		t.Fatalf("writeTicketToDir: %v", err)
	}
}

func writeWorkUnitToPath(t *testing.T, path string, wu *models.WorkUnit) error {
	t.Helper()
	data, err := json.MarshalIndent(wu, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
