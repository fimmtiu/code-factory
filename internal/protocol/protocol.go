// Package protocol defines the wire types and I/O helpers for the Unix socket
// communication between the tickets daemon and its clients.
//
// Each connection carries exactly one Command and one Response, framed as
// newline-delimited JSON (one JSON object per line).
package protocol

import (
	"bufio"
	"encoding/json"
	"io"
)

// Command is sent from a client to the daemon.
type Command struct {
	// Name is the command identifier, e.g. "ping", "create-ticket", "done".
	Name string `json:"command"`

	// Params holds optional key/value parameters. The field is omitted from
	// JSON output when nil or empty.
	Params map[string]string `json:"params,omitempty"`
}

// Response is sent from the daemon back to a client.
type Response struct {
	// Success indicates whether the command succeeded.
	Success bool `json:"success"`

	// Data holds an arbitrary JSON payload on success. Omitted when nil.
	Data json.RawMessage `json:"data,omitempty"`

	// Error contains a human-readable error message on failure. Omitted when empty.
	Error string `json:"error,omitempty"`
}

// writeJSON encodes v as JSON and writes it followed by a newline to w.
func writeJSON(w io.Writer, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// readLine reads one newline-delimited line from r, wrapping in a bufio.Reader
// if necessary.
func readLine(r io.Reader) ([]byte, error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	return br.ReadBytes('\n')
}

// WriteCommand JSON-encodes cmd and writes it followed by a newline to w.
func WriteCommand(w io.Writer, cmd Command) error {
	return writeJSON(w, cmd)
}

// ReadCommand reads one newline-delimited JSON line from r and decodes it into
// a Command. Use bufio.NewReader to wrap an underlying reader when calling this
// repeatedly over the same connection.
func ReadCommand(r io.Reader) (Command, error) {
	line, err := readLine(r)
	if err != nil {
		return Command{}, err
	}
	var cmd Command
	if err := json.Unmarshal(line, &cmd); err != nil {
		return Command{}, err
	}
	return cmd, nil
}

// WriteResponse JSON-encodes resp and writes it followed by a newline to w.
func WriteResponse(w io.Writer, resp Response) error {
	return writeJSON(w, resp)
}

// ReadResponse reads one newline-delimited JSON line from r and decodes it into
// a Response.
func ReadResponse(r io.Reader) (Response, error) {
	line, err := readLine(r)
	if err != nil {
		return Response{}, err
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return Response{}, err
	}
	return resp, nil
}
