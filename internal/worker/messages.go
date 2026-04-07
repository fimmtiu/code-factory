package worker

// MainToWorkerKind is the type for message kinds sent from the main goroutine
// to a worker.
type MainToWorkerKind string

const (
	// MsgPause instructs a worker to pause after its current unit of work.
	MsgPause MainToWorkerKind = "pause"

	// MsgUnpause instructs a worker to resume normal operation.
	MsgUnpause MainToWorkerKind = "unpause"

	// MsgResponse delivers an answer to a question or permission request
	// from the agent. The answer text is in Payload.
	MsgResponse MainToWorkerKind = "response"
)

// MainToWorkerMessage is a message sent from the main goroutine to a worker.
type MainToWorkerMessage struct {
	Kind    MainToWorkerKind
	Payload string
}

