package client_test

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/fimmtiu/tickets/internal/client"
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

// startMockServer starts a Unix socket server that responds to commands using
// the provided handler function. It returns the socket path and a channel that
// is closed when the server goroutine exits. The server handles exactly one
// connection.
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

// startMultiMockServer is like startMockServer but handles n connections.
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

func TestSendCommand_Success(t *testing.T) {
	expectedResp := protocol.Response{
		Success: true,
		Data:    json.RawMessage(`{"result":"ok"}`),
	}

	socketPath, done := startMockServer(t, func(cmd protocol.Command) protocol.Response {
		if cmd.Name != "test-cmd" {
			t.Errorf("expected command name %q, got %q", "test-cmd", cmd.Name)
		}
		if cmd.Params["key"] != "value" {
			t.Errorf("expected param key=%q, got %q", "value", cmd.Params["key"])
		}
		return expectedResp
	})

	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{
		Name:   "test-cmd",
		Params: map[string]string{"key": "value"},
	})
	if err != nil {
		t.Fatalf("SendCommand returned unexpected error: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got false")
	}
	if string(resp.Data) != `{"result":"ok"}` {
		t.Errorf("unexpected response data: %s", resp.Data)
	}

	<-done
}

func TestSendCommand_NoServer(t *testing.T) {
	socketPath := filepath.Join("/tmp", "tkt_nonexistent_12345.sock")
	c := client.NewClient(socketPath)
	_, err := c.SendCommand(protocol.Command{Name: "ping"})
	if err == nil {
		t.Fatal("expected error when connecting to nonexistent socket, got nil")
	}
}

func TestSendCommand_ErrorResponse(t *testing.T) {
	socketPath, done := startMockServer(t, func(cmd protocol.Command) protocol.Response {
		return protocol.Response{Success: false, Error: "something went wrong"}
	})

	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{Name: "bad-cmd"})
	if err != nil {
		t.Fatalf("SendCommand returned unexpected error: %v", err)
	}
	if resp.Success {
		t.Errorf("expected success=false, got true")
	}
	if resp.Error != "something went wrong" {
		t.Errorf("expected error %q, got %q", "something went wrong", resp.Error)
	}

	<-done
}

func TestSendCommand_EachConnectionIsIndependent(t *testing.T) {
	// Each call to SendCommand should open a new connection.
	callCount := 0
	socketPath, done := startMultiMockServer(t, 2, func(cmd protocol.Command) protocol.Response {
		callCount++
		return protocol.Response{Success: true}
	})

	c := client.NewClient(socketPath)

	_, err := c.SendCommand(protocol.Command{Name: "cmd1"})
	if err != nil {
		t.Fatalf("first SendCommand error: %v", err)
	}
	_, err = c.SendCommand(protocol.Command{Name: "cmd2"})
	if err != nil {
		t.Fatalf("second SendCommand error: %v", err)
	}

	<-done
	if callCount != 2 {
		t.Errorf("expected 2 handler calls, got %d", callCount)
	}
}

func TestPing_Success(t *testing.T) {
	expectedPID := 12345
	pidData, _ := json.Marshal(map[string]int{"pid": expectedPID})

	socketPath, done := startMockServer(t, func(cmd protocol.Command) protocol.Response {
		if cmd.Name != "ping" {
			t.Errorf("expected command %q, got %q", "ping", cmd.Name)
		}
		return protocol.Response{
			Success: true,
			Data:    json.RawMessage(pidData),
		}
	})

	c := client.NewClient(socketPath)
	pid, err := c.Ping()
	if err != nil {
		t.Fatalf("Ping returned unexpected error: %v", err)
	}
	if pid != expectedPID {
		t.Errorf("expected pid=%d, got %d", expectedPID, pid)
	}

	<-done
}

func TestPing_Failure(t *testing.T) {
	socketPath, done := startMockServer(t, func(cmd protocol.Command) protocol.Response {
		return protocol.Response{Success: false, Error: "daemon error"}
	})

	c := client.NewClient(socketPath)
	_, err := c.Ping()
	if err == nil {
		t.Fatal("expected error from failed ping, got nil")
	}

	<-done
}

func TestPing_NoServer(t *testing.T) {
	socketPath := filepath.Join("/tmp", "tkt_nonexistent_ping.sock")
	c := client.NewClient(socketPath)
	_, err := c.Ping()
	if err == nil {
		t.Fatal("expected error when no server present, got nil")
	}
}

func TestNewClient_SocketPath(t *testing.T) {
	path := "/tmp/test.sock"
	c := client.NewClient(path)
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.SocketPath() != path {
		t.Errorf("expected SocketPath()=%q, got %q", path, c.SocketPath())
	}
}

func TestSendCommand_ClosesConnectionAfterResponse(t *testing.T) {
	// Verify connection is closed: the server should see a read error after writing response.
	sawClose := make(chan bool, 1)

	socketPath := tempSocketPath(t)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			sawClose <- false
			return
		}
		defer conn.Close()

		_, err = protocol.ReadCommand(conn)
		if err != nil {
			sawClose <- false
			return
		}
		if err := protocol.WriteResponse(conn, protocol.Response{Success: true}); err != nil {
		panic(err)
	}

		// Try to read one more byte — should get EOF because client closed.
		buf := make([]byte, 1)
		_, readErr := conn.Read(buf)
		sawClose <- (readErr != nil)
	}()

	c := client.NewClient(socketPath)
	_, err = c.SendCommand(protocol.Command{Name: "test"})
	if err != nil {
		t.Fatalf("SendCommand error: %v", err)
	}

	closed := <-sawClose
	if !closed {
		t.Error("expected connection to be closed after response, but server saw no EOF")
	}
}
