// Package client provides a client for communicating with the tickets daemon
// over a Unix domain socket. Each call to SendCommand opens a new connection,
// sends one Command, reads one Response, and closes the connection.
package client

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/fimmtiu/tickets/internal/protocol"
)

// Client holds the configuration needed to connect to the tickets daemon.
type Client struct {
	socketPath string
}

// NewClient returns a new Client that will connect to the daemon at socketPath.
func NewClient(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

// SocketPath returns the Unix socket path this client connects to.
func (c *Client) SocketPath() string {
	return c.socketPath
}

// SendCommand opens a new Unix socket connection to the daemon, writes cmd,
// reads the Response, and closes the connection before returning.
func (c *Client) SendCommand(cmd protocol.Command) (protocol.Response, error) {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return protocol.Response{}, fmt.Errorf("client: dial %s: %w", c.socketPath, err)
	}
	defer conn.Close()

	if err := protocol.WriteCommand(conn, cmd); err != nil {
		return protocol.Response{}, fmt.Errorf("client: write command: %w", err)
	}

	resp, err := protocol.ReadResponse(conn)
	if err != nil {
		return protocol.Response{}, fmt.Errorf("client: read response: %w", err)
	}

	return resp, nil
}

// Ping sends a "ping" command to the daemon and returns the daemon's PID on
// success. It returns an error if the connection fails or the daemon reports
// failure.
func (c *Client) Ping() (int, error) {
	resp, err := c.SendCommand(protocol.Command{Name: "ping"})
	if err != nil {
		return 0, err
	}
	if !resp.Success {
		return 0, fmt.Errorf("client: ping failed: %s", resp.Error)
	}

	var payload struct {
		PID int `json:"pid"`
	}
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		return 0, fmt.Errorf("client: parse ping response: %w", err)
	}

	return payload.PID, nil
}
