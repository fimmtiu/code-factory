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

func TestWorkerToMainMessageConstruction(t *testing.T) {
	msg := WorkerToMainMessage{WorkerNumber: 3, Kind: MsgQuestion, Payload: "what now?"}
	if msg.WorkerNumber != 3 {
		t.Errorf("WorkerNumber = %d, want 3", msg.WorkerNumber)
	}
	if msg.Kind != MsgQuestion {
		t.Errorf("Kind = %q, want %q", msg.Kind, MsgQuestion)
	}
	if msg.Payload != "what now?" {
		t.Errorf("Payload = %q, want %q", msg.Payload, "what now?")
	}

	msg2 := WorkerToMainMessage{WorkerNumber: 1, Kind: MsgPermissionRequest, Payload: "delete file"}
	if msg2.WorkerNumber != 1 {
		t.Errorf("WorkerNumber = %d, want 1", msg2.WorkerNumber)
	}
	if msg2.Kind != MsgPermissionRequest {
		t.Errorf("Kind = %q, want %q", msg2.Kind, MsgPermissionRequest)
	}
	if msg2.Payload != "delete file" {
		t.Errorf("Payload = %q, want %q", msg2.Payload, "delete file")
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

func TestWorkerToMainKindConstants(t *testing.T) {
	if MsgQuestion != "question" {
		t.Errorf("MsgQuestion = %q, want %q", MsgQuestion, "question")
	}
	if MsgPermissionRequest != "permission-request" {
		t.Errorf("MsgPermissionRequest = %q, want %q", MsgPermissionRequest, "permission-request")
	}
}
