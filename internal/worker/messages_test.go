package worker

import "testing"

func TestMainToWorkerMessageConstruction(t *testing.T) {
	msg := MainToWorkerMessage{Kind: MsgPause, Payload: ""}
	if msg.Kind != MsgPause {
		t.Errorf("Kind = %q, want %q", msg.Kind, MsgPause)
	}

	msg2 := MainToWorkerMessage{Kind: MsgResponse, Payload: "yes"}
	if msg2.Kind != MsgResponse {
		t.Errorf("Kind = %q, want %q", msg2.Kind, MsgResponse)
	}
	if msg2.Payload != "yes" {
		t.Errorf("Payload = %q, want %q", msg2.Payload, "yes")
	}

	msg4 := MainToWorkerMessage{Kind: MsgUnpause}
	if msg4.Kind != MsgUnpause {
		t.Errorf("Kind = %q, want %q", msg4.Kind, MsgUnpause)
	}
}

func TestMainToWorkerKindConstants(t *testing.T) {
	if MsgPause != "pause" {
		t.Errorf("MsgPause = %q, want %q", MsgPause, "pause")
	}
	if MsgUnpause != "unpause" {
		t.Errorf("MsgUnpause = %q, want %q", MsgUnpause, "unpause")
	}
	if MsgResponse != "response" {
		t.Errorf("MsgResponse = %q, want %q", MsgResponse, "response")
	}
}

