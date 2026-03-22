package daemon_test

import (
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/fimmtiu/tickets/internal/daemon"
	"github.com/fimmtiu/tickets/internal/protocol"
)

// tempSocketPath returns a socket path that fits within the Unix socket path
// limit (108 bytes on macOS/Linux). t.TempDir() paths can exceed this limit
// when the test name is long, so we use os.MkdirTemp with a short prefix.
func tempSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "dtest")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "d.sock")
}

// TestDaemonStartStop verifies that a daemon can start and stop cleanly,
// and that the socket file is removed after Stop.
func TestDaemonStartStop(t *testing.T) {
	sockPath := tempSocketPath(t)
	d := daemon.NewDaemon(sockPath, "")

	if err := d.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Socket file should exist while running.
	if _, err := net.Dial("unix", sockPath); err != nil {
		t.Fatalf("expected to connect to socket, got: %v", err)
	}

	d.Stop()

	// Socket file should be gone after Stop.
	if _, err := net.Dial("unix", sockPath); err == nil {
		t.Error("expected connection to fail after Stop, but it succeeded")
	}
}

// TestDaemonAcceptsConnection verifies that a client can connect, send a
// command, and receive a response when the queue is consumed externally.
func TestDaemonAcceptsConnection(t *testing.T) {
	sockPath := tempSocketPath(t)
	d := daemon.NewDaemon(sockPath, "")

	if err := d.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer d.Stop()

	// Consume one item from the queue in a goroutine (simulating a worker).
	go func() {
		item, ok := <-d.Queue()
		if !ok {
			return
		}
		item.Response <- protocol.Response{Success: true}
	}()

	// Connect as a client and send a ping command.
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	cmd := protocol.Command{Name: "ping"}
	if err := protocol.WriteCommand(conn, cmd); err != nil {
		t.Fatalf("WriteCommand failed: %v", err)
	}

	resp, err := protocol.ReadResponse(conn)
	if err != nil {
		t.Fatalf("ReadResponse failed: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected Success=true, got false")
	}
}

// TestDaemonQueueReceivesCommand verifies that sending a command over the
// socket results in a QueueItem with the correct command being pushed to
// the daemon's queue.
func TestDaemonQueueReceivesCommand(t *testing.T) {
	sockPath := tempSocketPath(t)
	d := daemon.NewDaemon(sockPath, "")

	if err := d.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer d.Stop()

	// Connect and send a command.
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	cmd := protocol.Command{Name: "ping"}
	if err := protocol.WriteCommand(conn, cmd); err != nil {
		t.Fatalf("WriteCommand failed: %v", err)
	}

	// The queue item should arrive promptly.
	select {
	case item := <-d.Queue():
		if item.Cmd.Name != "ping" {
			t.Errorf("expected command name 'ping', got %q", item.Cmd.Name)
		}
		// Reply so the handler goroutine can finish.
		item.Response <- protocol.Response{Success: true}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for queue item")
	}
}

// TestDaemonStaleSocket verifies that when a stale (unresponsive) socket
// file exists, Start removes it and creates a new listener.
func TestDaemonStaleSocket(t *testing.T) {
	sockPath := tempSocketPath(t)

	// Create a socket file that isn't actually serving anything (stale).
	staleListener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create stale socket: %v", err)
	}
	// Close the listener immediately so it won't respond to pings.
	staleListener.Close()

	d := daemon.NewDaemon(sockPath, "")
	if err := d.Start(); err != nil {
		t.Fatalf("Start failed on stale socket: %v", err)
	}
	defer d.Stop()

	// The new daemon should accept connections.
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("expected to connect to new daemon, got: %v", err)
	}
	conn.Close()
}

// TestDaemonAlreadyRunning verifies that starting a second daemon when one
// is already active returns an error.
func TestDaemonAlreadyRunning(t *testing.T) {
	sockPath := tempSocketPath(t)

	d1 := daemon.NewDaemon(sockPath, "")
	if err := d1.Start(); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}
	defer d1.Stop()

	// The first daemon needs a worker that handles ping so IsRunning can
	// detect it as alive.
	go func() {
		for item := range d1.Queue() {
			item.Response <- protocol.Response{Success: true}
		}
	}()

	d2 := daemon.NewDaemon(sockPath, "")
	err := d2.Start()
	if err == nil {
		d2.Stop()
		t.Fatal("expected error starting second daemon, got nil")
	}
}

// TestDaemonStopsOnSIGHUP verifies that sending SIGHUP to the process causes
// a running daemon to stop. We use SIGHUP rather than SIGINT because SIGINT
// would also trigger the default Go test runner signal handler and abort the
// test binary; SIGHUP is forwarded to the daemon's watchSignals goroutine
// without interfering with the test runner.
func TestDaemonStopsOnSIGHUP(t *testing.T) {
	sockPath := tempSocketPath(t)
	d := daemon.NewDaemon(sockPath, "")

	if err := d.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Send SIGHUP to ourselves — watchSignals will receive it and call Stop.
	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		d.Stop()
		t.Fatalf("Kill(SIGHUP) failed: %v", err)
	}

	// Wait blocks until the daemon has fully stopped (context cancelled and
	// all goroutines exited). Use a timeout via a separate goroutine so the
	// test doesn't hang indefinitely.
	done := make(chan struct{})
	go func() {
		d.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Daemon stopped as expected.
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not stop within 5 seconds after SIGHUP")
	}

	// Socket should have been cleaned up.
	if _, err := net.Dial("unix", sockPath); err == nil {
		t.Error("expected socket to be removed after daemon stopped")
	}
}

// TestIsRunning verifies the standalone IsRunning helper.
func TestIsRunning(t *testing.T) {
	sockPath := tempSocketPath(t)

	// No socket yet → not running.
	running, err := daemon.IsRunning(sockPath)
	if err != nil {
		t.Fatalf("IsRunning returned unexpected error: %v", err)
	}
	if running {
		t.Error("expected IsRunning=false when socket doesn't exist")
	}

	// Start a daemon with a ping responder.
	d := daemon.NewDaemon(sockPath, "")
	if err := d.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer d.Stop()

	go func() {
		for item := range d.Queue() {
			item.Response <- protocol.Response{Success: true}
		}
	}()

	// Give the ping responder a moment to be ready.
	time.Sleep(10 * time.Millisecond)

	running, err = daemon.IsRunning(sockPath)
	if err != nil {
		t.Fatalf("IsRunning returned unexpected error: %v", err)
	}
	if !running {
		t.Error("expected IsRunning=true while daemon is running")
	}
}
