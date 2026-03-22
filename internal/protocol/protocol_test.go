package protocol_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fimmtiu/tickets/internal/protocol"
)

// --- Command round-trip tests ---

func TestWriteReadCommand_Basic(t *testing.T) {
	cmd := protocol.Command{Name: "ping"}
	var buf bytes.Buffer
	if err := protocol.WriteCommand(&buf, cmd); err != nil {
		t.Fatalf("WriteCommand failed: %v", err)
	}

	got, err := protocol.ReadCommand(&buf)
	if err != nil {
		t.Fatalf("ReadCommand failed: %v", err)
	}
	if got.Name != "ping" {
		t.Errorf("expected Name=ping, got %q", got.Name)
	}
	if len(got.Params) != 0 {
		t.Errorf("expected empty Params, got %v", got.Params)
	}
}

func TestWriteReadCommand_WithParams(t *testing.T) {
	cmd := protocol.Command{
		Name: "create-ticket",
		Params: map[string]string{
			"identifier":  "fix-bug",
			"description": "Fix the nasty bug",
		},
	}
	var buf bytes.Buffer
	if err := protocol.WriteCommand(&buf, cmd); err != nil {
		t.Fatalf("WriteCommand failed: %v", err)
	}

	got, err := protocol.ReadCommand(&buf)
	if err != nil {
		t.Fatalf("ReadCommand failed: %v", err)
	}
	if got.Name != "create-ticket" {
		t.Errorf("expected Name=create-ticket, got %q", got.Name)
	}
	if got.Params["identifier"] != "fix-bug" {
		t.Errorf("expected identifier=fix-bug, got %q", got.Params["identifier"])
	}
	if got.Params["description"] != "Fix the nasty bug" {
		t.Errorf("expected description='Fix the nasty bug', got %q", got.Params["description"])
	}
}

func TestWriteReadCommand_DoneWithIdentifier(t *testing.T) {
	cmd := protocol.Command{
		Name:   "done",
		Params: map[string]string{"identifier": "fix-bug"},
	}
	var buf bytes.Buffer
	if err := protocol.WriteCommand(&buf, cmd); err != nil {
		t.Fatalf("WriteCommand failed: %v", err)
	}

	got, err := protocol.ReadCommand(&buf)
	if err != nil {
		t.Fatalf("ReadCommand failed: %v", err)
	}
	if got.Name != "done" {
		t.Errorf("expected Name=done, got %q", got.Name)
	}
	if got.Params["identifier"] != "fix-bug" {
		t.Errorf("expected identifier=fix-bug, got %q", got.Params["identifier"])
	}
}

func TestCommandDelimitedByNewline(t *testing.T) {
	cmd := protocol.Command{Name: "ping"}
	var buf bytes.Buffer
	if err := protocol.WriteCommand(&buf, cmd); err != nil {
		t.Fatalf("WriteCommand failed: %v", err)
	}
	raw := buf.String()
	if !strings.HasSuffix(raw, "\n") {
		t.Errorf("expected written command to end with newline, got %q", raw)
	}
}

func TestCommandJSONEncoding(t *testing.T) {
	cmd := protocol.Command{Name: "ping"}
	var buf bytes.Buffer
	if err := protocol.WriteCommand(&buf, cmd); err != nil {
		t.Fatalf("WriteCommand failed: %v", err)
	}
	line := strings.TrimSuffix(buf.String(), "\n")
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if raw["command"] != "ping" {
		t.Errorf("expected JSON key 'command'='ping', got %v", raw["command"])
	}
}

func TestCommandOmitsParamsWhenEmpty(t *testing.T) {
	cmd := protocol.Command{Name: "ping"}
	var buf bytes.Buffer
	if err := protocol.WriteCommand(&buf, cmd); err != nil {
		t.Fatalf("WriteCommand failed: %v", err)
	}
	line := strings.TrimSuffix(buf.String(), "\n")
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, ok := raw["params"]; ok {
		t.Error("expected 'params' to be omitted when empty")
	}
}

// --- Response round-trip tests ---

func TestWriteReadResponse_Success(t *testing.T) {
	data, _ := json.Marshal(map[string]int{"pid": 1234})
	resp := protocol.Response{
		Success: true,
		Data:    json.RawMessage(data),
	}
	var buf bytes.Buffer
	if err := protocol.WriteResponse(&buf, resp); err != nil {
		t.Fatalf("WriteResponse failed: %v", err)
	}

	got, err := protocol.ReadResponse(&buf)
	if err != nil {
		t.Fatalf("ReadResponse failed: %v", err)
	}
	if !got.Success {
		t.Error("expected Success=true")
	}
	var inner map[string]int
	if err := json.Unmarshal(got.Data, &inner); err != nil {
		t.Fatalf("unmarshal Data failed: %v", err)
	}
	if inner["pid"] != 1234 {
		t.Errorf("expected pid=1234, got %d", inner["pid"])
	}
}

func TestWriteReadResponse_Error(t *testing.T) {
	resp := protocol.Response{
		Success: false,
		Error:   "parent project does not exist",
	}
	var buf bytes.Buffer
	if err := protocol.WriteResponse(&buf, resp); err != nil {
		t.Fatalf("WriteResponse failed: %v", err)
	}

	got, err := protocol.ReadResponse(&buf)
	if err != nil {
		t.Fatalf("ReadResponse failed: %v", err)
	}
	if got.Success {
		t.Error("expected Success=false")
	}
	if got.Error != "parent project does not exist" {
		t.Errorf("expected error='parent project does not exist', got %q", got.Error)
	}
}

func TestResponseDelimitedByNewline(t *testing.T) {
	resp := protocol.Response{Success: true}
	var buf bytes.Buffer
	if err := protocol.WriteResponse(&buf, resp); err != nil {
		t.Fatalf("WriteResponse failed: %v", err)
	}
	raw := buf.String()
	if !strings.HasSuffix(raw, "\n") {
		t.Errorf("expected written response to end with newline, got %q", raw)
	}
}

func TestResponseJSONEncoding(t *testing.T) {
	resp := protocol.Response{Success: true}
	var buf bytes.Buffer
	if err := protocol.WriteResponse(&buf, resp); err != nil {
		t.Fatalf("WriteResponse failed: %v", err)
	}
	line := strings.TrimSuffix(buf.String(), "\n")
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, ok := raw["success"]; !ok {
		t.Error("expected JSON key 'success' in response")
	}
}

func TestResponseOmitsDataAndErrorWhenAbsent(t *testing.T) {
	resp := protocol.Response{Success: true}
	var buf bytes.Buffer
	if err := protocol.WriteResponse(&buf, resp); err != nil {
		t.Fatalf("WriteResponse failed: %v", err)
	}
	line := strings.TrimSuffix(buf.String(), "\n")
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, ok := raw["error"]; ok {
		t.Error("expected 'error' to be omitted when empty")
	}
}

// --- Multiple messages on same reader ---

// TestMultipleCommandsSequentially verifies that multiple commands written to the
// same buffer can be read back in order when the caller retains a single
// bufio.Reader across calls — the expected usage pattern for persistent connections.
func TestMultipleCommandsSequentially(t *testing.T) {
	commands := []protocol.Command{
		{Name: "ping"},
		{Name: "done", Params: map[string]string{"identifier": "task-1"}},
		{Name: "status"},
	}

	var buf bytes.Buffer
	for _, cmd := range commands {
		if err := protocol.WriteCommand(&buf, cmd); err != nil {
			t.Fatalf("WriteCommand failed: %v", err)
		}
	}

	// Use a single bufio.Reader for all reads — callers must retain this.
	br := bufio.NewReader(&buf)
	for i, expected := range commands {
		got, err := protocol.ReadCommand(br)
		if err != nil {
			t.Fatalf("ReadCommand[%d] failed: %v", i, err)
		}
		if got.Name != expected.Name {
			t.Errorf("cmd[%d]: expected Name=%q, got %q", i, expected.Name, got.Name)
		}
	}
}
