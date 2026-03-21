package client_test

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fimmtiu/tickets/internal/client"
	"github.com/fimmtiu/tickets/internal/protocol"
)

// startPingServer starts a mock daemon that responds to ping commands.
// It runs until the listener is closed.
func startPingServer(t *testing.T, socketPath string) (stop func()) {
	t.Helper()
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("startPingServer: listen: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // listener closed
			}
			go func(c net.Conn) {
				defer c.Close()
				cmd, err := protocol.ReadCommand(c)
				if err != nil {
					return
				}
				if cmd.Name == "ping" {
					data, _ := json.Marshal(map[string]int{"pid": os.Getpid()})
					protocol.WriteResponse(c, protocol.Response{ //nolint:errcheck
						Success: true,
						Data:    json.RawMessage(data),
					})
				}
			}(conn)
		}
	}()

	return func() { ln.Close() }
}

func TestIsRunning_NoServer(t *testing.T) {
	socketPath := filepath.Join("/tmp", "tkt_isrunning_none.sock")
	if client.IsRunning(socketPath) {
		t.Error("IsRunning returned true when no server is present")
	}
}

func TestIsRunning_WithMockServer(t *testing.T) {
	socketPath := tempSocketPath(t)
	stop := startPingServer(t, socketPath)
	defer stop()

	// Give the goroutine a moment to start listening.
	time.Sleep(10 * time.Millisecond)

	if !client.IsRunning(socketPath) {
		t.Error("IsRunning returned false when mock server is running")
	}
}

func TestIsRunning_ServerStopsBeingRunning(t *testing.T) {
	socketPath := tempSocketPath(t)
	stop := startPingServer(t, socketPath)
	time.Sleep(10 * time.Millisecond)

	if !client.IsRunning(socketPath) {
		t.Error("IsRunning returned false before stop")
	}

	stop()
	// Remove socket file so dial fails cleanly.
	os.Remove(socketPath)
	time.Sleep(10 * time.Millisecond)

	if client.IsRunning(socketPath) {
		t.Error("IsRunning returned true after server stopped")
	}
}

func TestEnsureRunning_AlreadyRunning(t *testing.T) {
	socketPath := tempSocketPath(t)
	stop := startPingServer(t, socketPath)
	defer stop()
	time.Sleep(10 * time.Millisecond)

	// Use an empty repoRoot — StartDaemon should never be called since
	// IsRunning returns true.
	err := client.EnsureRunning(socketPath, "/nonexistent/repo")
	if err != nil {
		t.Errorf("EnsureRunning returned error when daemon already running: %v", err)
	}
}

func TestEnsureRunning_TimeoutWhenNoBinary(t *testing.T) {
	// Use a socket path that will never become active (no daemon binary named
	// "ticketsd" at this path, so StartDaemon will fail to exec or the exec'd
	// process won't bind the socket). EnsureRunning should return an error
	// within its timeout.
	socketPath := filepath.Join("/tmp", "tkt_ensure_timeout.sock")
	os.Remove(socketPath)

	// We expect this to fail quickly — StartDaemon will exec "ticketsd" which
	// won't be available or won't bind the socket, so EnsureRunning should
	// time out and return an error.
	start := time.Now()
	err := client.EnsureRunning(socketPath, "/nonexistent/repo")
	elapsed := time.Since(start)

	if err == nil {
		t.Error("EnsureRunning expected to fail when no daemon available, but returned nil")
	}

	// The default timeout is 5 seconds, but we want to confirm it doesn't hang
	// indefinitely. Give it 10 seconds of leeway.
	if elapsed > 10*time.Second {
		t.Errorf("EnsureRunning took too long: %v (expected < 10s)", elapsed)
	}

	os.Remove(socketPath)
}

func TestStartDaemon_MissingBinaryDoesNotBlock(t *testing.T) {
	// StartDaemon uses cmd.Start (fire-and-forget). Even if the binary is not
	// found, Start itself may return an error or the process may fail silently.
	// We just verify the call returns (doesn't block).
	done := make(chan error, 1)
	go func() {
		done <- client.StartDaemon("/nonexistent/repo")
	}()

	select {
	case <-done:
		// OK — either error or nil, we don't care as long as it returned.
	case <-time.After(2 * time.Second):
		t.Error("StartDaemon blocked for more than 2 seconds")
	}
}
