package worker

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

// sessionUpdateLine builds one newline-delimited JSON-RPC session/update
// notification carrying an agent message chunk of text.
func sessionUpdateLine(text string) string {
	params := map[string]any{
		"sessionId": "sess-1",
		"update": map[string]any{
			"sessionUpdate": "agent_message_chunk",
			"content":       map[string]any{"type": "text", "text": text},
		},
	}
	b, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "session/update",
		"params":  params,
	})
	if err != nil {
		panic(err)
	}
	return string(b) + "\n"
}

// TestSessionUpdates_ArriveInOrder is a regression test for garbled worker
// output. The ACP SDK used to dispatch every inbound notification in its own
// goroutine, so streaming text chunks reached SessionUpdate in whatever order
// the scheduler picked and words from different lines spliced together on the
// Workers pane. The SDK now serializes notifications through an ordered
// queue; this test fails if that guarantee regresses.
func TestSessionUpdates_ArriveInOrder(t *testing.T) {
	const chunkCount = 300

	// A long single line of numbered chunks. Any reordering shows up as a
	// mismatch against the same sequence rebuilt in order.
	var sent strings.Builder
	var wire strings.Builder
	for i := 0; i < chunkCount; i++ {
		chunk := fmt.Sprintf("<%d>", i)
		sent.WriteString(chunk)
		wire.WriteString(sessionUpdateLine(chunk))
	}
	wire.WriteString(sessionUpdateLine("\n"))

	logPath := filepath.Join(t.TempDir(), "agent.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("creating logfile: %v", err)
	}
	defer func() { _ = logFile.Close() }()

	w := NewWorker(1)
	client := &acpWorkerClient{w: w, logFile: logFile}

	// Drive the client with a canned notification stream. The connection
	// spawns its own reader, so wait for the output to settle.
	conn := acp.NewClientSideConnection(client, io.Discard, strings.NewReader(wire.String()))
	defer func() { _ = conn }()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if len(w.GetLastOutput()) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no output arrived within 5s")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for the full line to be committed.
	want := sent.String()
	for time.Now().Before(deadline) {
		if strings.Contains(strings.Join(w.GetLastOutput(), ""), fmt.Sprintf("<%d>", chunkCount-1)) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := logFile.Sync(); err != nil {
		t.Fatalf("syncing logfile: %v", err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading logfile: %v", err)
	}

	if gotStr := strings.TrimSpace(string(got)); gotStr != want {
		gotExcerpt, wantExcerpt := firstDifference(gotStr, want)
		t.Errorf("chunks were reordered.\n got: %s\nwant: %s", gotExcerpt, wantExcerpt)
	}
}

// firstDifference returns trimmed excerpts of got and want around the first
// index where they diverge, so a failure message stays readable.
func firstDifference(got, want string) (string, string) {
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	start := i - 40
	if start < 0 {
		start = 0
	}
	end := i + 40
	gotEnd, wantEnd := end, end
	if gotEnd > len(got) {
		gotEnd = len(got)
	}
	if wantEnd > len(want) {
		wantEnd = len(want)
	}
	return "…" + got[start:gotEnd] + "…", "…" + want[start:wantEnd] + "…"
}

// TestSetLastOutput_CopiesSlice is a regression test for the display buffer
// aliasing the streaming buffer. appendOutput hands its committedLines slice
// straight to SetLastOutput, so without a copy the UI goroutine would read a
// backing array that later appends keep mutating.
func TestSetLastOutput_CopiesSlice(t *testing.T) {
	w := NewWorker(1)

	lines := make([]string, 2, 8) // spare capacity: appends mutate in place
	lines[0] = "first"
	lines[1] = "second"
	w.SetLastOutput(lines)

	lines[0] = "clobbered"
	lines = append(lines, "third")
	_ = lines

	got := w.GetLastOutput()
	if len(got) != 2 {
		t.Fatalf("GetLastOutput() = %v, want 2 lines", got)
	}
	if got[0] != "first" || got[1] != "second" {
		t.Errorf("GetLastOutput() = %v, want [first second]; the stored slice aliased the caller's buffer", got)
	}
}

// TestAppendOutput_CommittedLinesSurviveLaterAppends checks the same aliasing
// hazard through the real streaming path: output read from the worker must not
// change as more chunks arrive.
func TestAppendOutput_CommittedLinesSurviveLaterAppends(t *testing.T) {
	w := NewWorker(1)
	c := &acpWorkerClient{w: w}

	c.appendOutput("alpha\nbravo\n")
	snapshot := w.GetLastOutput()
	before := strings.Join(snapshot, "|")

	for i := 0; i < OutputLines*3; i++ {
		c.appendOutput(fmt.Sprintf("filler-%d\n", i))
	}

	if after := strings.Join(snapshot, "|"); after != before {
		t.Errorf("a previously returned output slice changed under the caller: %q became %q", before, after)
	}
}
