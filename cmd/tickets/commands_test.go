package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fimmtiu/tickets/internal/protocol"
)

// tempSocketPath returns a short temporary Unix socket path that fits within
// macOS's 104-byte limit.
func tempSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "tkt")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "d.sock")
}

// startMockServer starts a Unix socket server that responds to a single command
// using the provided handler. Returns the socket path and a done channel.
func startMockServer(t *testing.T, handler func(cmd protocol.Command) protocol.Response) (socketPath string, done chan struct{}) {
	t.Helper()
	socketPath = tempSocketPath(t)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to listen on unix socket: %v", err)
	}

	done = make(chan struct{})
	go func() {
		defer close(done)
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		cmd, err := protocol.ReadCommand(conn)
		if err != nil {
			return
		}
		resp := handler(cmd)
		if err := protocol.WriteResponse(conn, resp); err != nil {
			panic(err)
		}
	}()

	return socketPath, done
}

// startMultiMockServer handles up to n connections.
func startMultiMockServer(t *testing.T, n int, handler func(cmd protocol.Command) protocol.Response) (socketPath string, done chan struct{}) {
	t.Helper()
	socketPath = tempSocketPath(t)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to listen on unix socket: %v", err)
	}

	done = make(chan struct{})
	go func() {
		defer close(done)
		defer ln.Close()
		for i := 0; i < n; i++ {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				cmd, err := protocol.ReadCommand(conn)
				if err != nil {
					return
				}
				resp := handler(cmd)
				if err := protocol.WriteResponse(conn, resp); err != nil {
					panic(err)
				}
			}()
		}
	}()

	return socketPath, done
}

// captureOutput captures os.Stdout during fn() and returns what was printed.
func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// ---- Tests for subtask 12-1: init, running, exit ----

func TestRunInit_CreatesTicketsDir(t *testing.T) {
	tmp := t.TempDir()
	// Create a .git dir so FindRepoRoot works
	if err := os.Mkdir(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	// Run from tmp
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	out := captureOutput(func() {
		if err := runInit(); err != nil {
			t.Fatalf("runInit returned error: %v", err)
		}
	})

	ticketsDir := filepath.Join(tmp, ".tickets")
	if _, err := os.Stat(ticketsDir); err != nil {
		t.Errorf("expected .tickets/ to exist: %v", err)
	}
	settingsFile := filepath.Join(ticketsDir, ".settings.json")
	if _, err := os.Stat(settingsFile); err != nil {
		t.Errorf("expected .settings.json to exist: %v", err)
	}
	if !strings.Contains(out, "Initialized .tickets/") {
		t.Errorf("expected output to contain 'Initialized .tickets/', got: %q", out)
	}
	if !strings.Contains(out, tmp) {
		t.Errorf("expected output to contain repo root %q, got: %q", tmp, out)
	}
}

func TestRunInit_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// First call
	if err := runInit(); err != nil {
		t.Fatalf("first runInit error: %v", err)
	}
	// Second call should not fail
	if err := runInit(); err != nil {
		t.Fatalf("second runInit error: %v", err)
	}
}

func TestRunInit_NoGitRepo(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	if err := runInit(); err == nil {
		t.Error("expected error when no .git directory, got nil")
	}
}

func TestRunRunning_DaemonRunning(t *testing.T) {
	pidData, _ := json.Marshal(map[string]int{"pid": 42})
	socketPath, done := startMockServer(t, func(cmd protocol.Command) protocol.Response {
		if cmd.Name != "ping" {
			return protocol.Response{Success: false, Error: "unexpected command"}
		}
		return protocol.Response{Success: true, Data: json.RawMessage(pidData)}
	})

	out := captureOutput(func() {
		if err := runRunning(socketPath); err != nil {
			t.Fatalf("runRunning returned error: %v", err)
		}
	})

	<-done
	if !strings.Contains(out, "42") {
		t.Errorf("expected output to contain pid 42, got: %q", out)
	}
	if !strings.Contains(out, "running") {
		t.Errorf("expected output to contain 'running', got: %q", out)
	}
}

func TestRunRunning_NoDaemon(t *testing.T) {
	socketPath := filepath.Join("/tmp", "tkt_no_daemon_running.sock")

	out := captureOutput(func() {
		if err := runRunning(socketPath); err != nil {
			t.Fatalf("runRunning returned error: %v", err)
		}
	})

	if !strings.Contains(out, "No daemon running") {
		t.Errorf("expected 'No daemon running', got: %q", out)
	}
}

func TestRunExit_DaemonRunning(t *testing.T) {
	socketPath, done := startMockServer(t, func(cmd protocol.Command) protocol.Response {
		if cmd.Name != "exit" {
			return protocol.Response{Success: false, Error: fmt.Sprintf("unexpected command: %s", cmd.Name)}
		}
		return protocol.Response{Success: true}
	})

	out := captureOutput(func() {
		if err := runExit(socketPath); err != nil {
			t.Fatalf("runExit returned error: %v", err)
		}
	})

	<-done
	_ = out // output may vary
}

func TestRunExit_NoDaemon(t *testing.T) {
	socketPath := filepath.Join("/tmp", "tkt_no_daemon_exit.sock")

	out := captureOutput(func() {
		if err := runExit(socketPath); err != nil {
			t.Fatalf("runExit returned error: %v", err)
		}
	})

	if !strings.Contains(out, "No daemon running") {
		t.Errorf("expected 'No daemon running', got: %q", out)
	}
}

// ---- Tests for subtask 12-2: daemon-requiring commands ----

func TestRunStatus(t *testing.T) {
	statusData := json.RawMessage(`{"project_count":2,"ticket_count":5}`)
	socketPath, done := startMockServer(t, func(cmd protocol.Command) protocol.Response {
		if cmd.Name != "status" {
			return protocol.Response{Success: false, Error: "unexpected command"}
		}
		return protocol.Response{Success: true, Data: statusData}
	})

	out := captureOutput(func() {
		if err := runStatus(socketPath); err != nil {
			t.Fatalf("runStatus returned error: %v", err)
		}
	})

	<-done
	if !strings.Contains(out, "project_count") {
		t.Errorf("expected output to contain 'project_count', got: %q", out)
	}
}

func TestRunCreateProject(t *testing.T) {
	socketPath, done := startMockServer(t, func(cmd protocol.Command) protocol.Response {
		if cmd.Name != "create-project" {
			return protocol.Response{Success: false, Error: "unexpected command"}
		}
		if cmd.Params["identifier"] != "my-proj" {
			return protocol.Response{Success: false, Error: "wrong identifier"}
		}
		if cmd.Params["description"] != "A test project" {
			return protocol.Response{Success: false, Error: "wrong description"}
		}
		return protocol.Response{Success: true, Data: json.RawMessage(`{"status":"created"}`)}
	})

	stdin := strings.NewReader(`{"description":"A test project"}`)
	out := captureOutput(func() {
		if err := runCreateProject(socketPath, []string{"my-proj"}, stdin); err != nil {
			t.Fatalf("runCreateProject returned error: %v", err)
		}
	})

	<-done
	if !strings.Contains(out, "created") {
		t.Errorf("expected output to contain 'created', got: %q", out)
	}
}

func TestRunCreateProject_MissingIdentifier(t *testing.T) {
	socketPath := "/tmp/tkt_unused.sock"
	stdin := strings.NewReader(`{"description":"test"}`)
	err := runCreateProject(socketPath, []string{}, stdin)
	if err == nil {
		t.Error("expected error when identifier is missing, got nil")
	}
}

func TestRunCreateTicket(t *testing.T) {
	socketPath, done := startMockServer(t, func(cmd protocol.Command) protocol.Response {
		if cmd.Name != "create-ticket" {
			return protocol.Response{Success: false, Error: "unexpected command"}
		}
		if cmd.Params["identifier"] != "my-ticket" {
			return protocol.Response{Success: false, Error: "wrong identifier"}
		}
		if cmd.Params["description"] != "A test ticket" {
			return protocol.Response{Success: false, Error: "wrong description"}
		}
		if cmd.Params["dependencies"] != "dep1,dep2" {
			return protocol.Response{Success: false, Error: "wrong dependencies"}
		}
		return protocol.Response{Success: true, Data: json.RawMessage(`{"status":"created"}`)}
	})

	stdin := strings.NewReader(`{"description":"A test ticket","dependencies":["dep1","dep2"]}`)
	out := captureOutput(func() {
		if err := runCreateTicket(socketPath, []string{"my-ticket"}, stdin); err != nil {
			t.Fatalf("runCreateTicket returned error: %v", err)
		}
	})

	<-done
	if !strings.Contains(out, "created") {
		t.Errorf("expected output to contain 'created', got: %q", out)
	}
}

func TestRunCreateTicket_NoDependencies(t *testing.T) {
	socketPath, done := startMockServer(t, func(cmd protocol.Command) protocol.Response {
		if cmd.Name != "create-ticket" {
			return protocol.Response{Success: false, Error: "unexpected command"}
		}
		if cmd.Params["dependencies"] != "" {
			return protocol.Response{Success: false, Error: "expected empty dependencies"}
		}
		return protocol.Response{Success: true, Data: json.RawMessage(`{"status":"created"}`)}
	})

	stdin := strings.NewReader(`{"description":"No deps ticket"}`)
	captureOutput(func() {
		if err := runCreateTicket(socketPath, []string{"t1"}, stdin); err != nil {
			t.Fatalf("runCreateTicket returned error: %v", err)
		}
	})

	<-done
}

func TestRunCreateTicket_MissingIdentifier(t *testing.T) {
	socketPath := "/tmp/tkt_unused2.sock"
	stdin := strings.NewReader(`{"description":"test"}`)
	err := runCreateTicket(socketPath, []string{}, stdin)
	if err == nil {
		t.Error("expected error when identifier is missing, got nil")
	}
}

func TestRunSetStatus(t *testing.T) {
	socketPath, done := startMockServer(t, func(cmd protocol.Command) protocol.Response {
		if cmd.Name != "set-status" {
			return protocol.Response{Success: false, Error: "unexpected command"}
		}
		if cmd.Params["identifier"] != "my-ticket" || cmd.Params["status"] != "review-ready" {
			return protocol.Response{Success: false, Error: "wrong params"}
		}
		return protocol.Response{Success: true}
	})

	captureOutput(func() {
		if err := runSetStatus(socketPath, []string{"my-ticket", "review-ready"}); err != nil {
			t.Fatalf("runSetStatus returned error: %v", err)
		}
	})
	<-done
}

func TestRunSetStatus_MissingArgs(t *testing.T) {
	err := runSetStatus("/tmp/unused.sock", []string{"only-one"})
	if err == nil {
		t.Error("expected error when status is missing, got nil")
	}
}

func TestRunClaim(t *testing.T) {
	claimData := json.RawMessage(`{"identifier":"my-ticket","claimed_by":"1234"}`)
	socketPath, done := startMockServer(t, func(cmd protocol.Command) protocol.Response {
		if cmd.Name != "claim" {
			return protocol.Response{Success: false, Error: "unexpected command"}
		}
		if cmd.Params["pid"] != "1234" {
			return protocol.Response{Success: false, Error: "wrong pid"}
		}
		return protocol.Response{Success: true, Data: claimData}
	})

	out := captureOutput(func() {
		if err := runClaim(socketPath, []string{"1234"}); err != nil {
			t.Fatalf("runClaim returned error: %v", err)
		}
	})
	<-done
	if !strings.Contains(out, "claimed_by") {
		t.Errorf("expected output to contain 'claimed_by', got: %q", out)
	}
}

func TestRunClaim_MissingPID(t *testing.T) {
	err := runClaim("/tmp/unused.sock", []string{})
	if err == nil {
		t.Error("expected error when pid is missing, got nil")
	}
}

func TestRunRelease(t *testing.T) {
	socketPath, done := startMockServer(t, func(cmd protocol.Command) protocol.Response {
		if cmd.Name != "release" {
			return protocol.Response{Success: false, Error: "unexpected command"}
		}
		if cmd.Params["identifier"] != "my-ticket" {
			return protocol.Response{Success: false, Error: "wrong identifier"}
		}
		return protocol.Response{Success: true}
	})

	captureOutput(func() {
		if err := runRelease(socketPath, []string{"my-ticket"}); err != nil {
			t.Fatalf("runRelease returned error: %v", err)
		}
	})
	<-done
}

func TestRunRelease_MissingIdentifier(t *testing.T) {
	err := runRelease("/tmp/unused.sock", []string{})
	if err == nil {
		t.Error("expected error when identifier is missing, got nil")
	}
}

func TestRunCommand_UnknownSubcommand(t *testing.T) {
	err := runCommand("no-such-command", []string{})
	if err == nil {
		t.Error("expected error for unknown subcommand, got nil")
	}
}
