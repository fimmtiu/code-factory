package worker

import (
	"fmt"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"
)

func TestAwaitPromptCompletion_PromptSucceeds(t *testing.T) {
	promptCh := make(chan promptResult, 1)
	waitCh := make(chan error, 1)
	promptCh <- promptResult{resp: acp.PromptResponse{StopReason: acp.StopReasonEndTurn}}

	stopReason, err, procExited := awaitPromptCompletion(promptCh, waitCh)
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if procExited {
		t.Error("procExited = true, want false")
	}
	if stopReason != acp.StopReasonEndTurn {
		t.Errorf("stopReason = %q, want %q", stopReason, acp.StopReasonEndTurn)
	}
}

func TestAwaitPromptCompletion_PromptFails(t *testing.T) {
	promptCh := make(chan promptResult, 1)
	waitCh := make(chan error, 1)
	promptCh <- promptResult{err: fmt.Errorf("connection reset")}

	_, err, procExited := awaitPromptCompletion(promptCh, waitCh)
	if err == nil {
		t.Fatal("err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "ACP prompt") {
		t.Errorf("err = %v, want to contain %q", err, "ACP prompt")
	}
	if !strings.Contains(err.Error(), "connection reset") {
		t.Errorf("err = %v, want to contain %q", err, "connection reset")
	}
	if procExited {
		t.Error("procExited = true, want false")
	}
}

func TestAwaitPromptCompletion_ProcessExitsCleanly(t *testing.T) {
	promptCh := make(chan promptResult, 1)
	waitCh := make(chan error, 1)
	waitCh <- nil // clean exit, but prompt never completed

	_, err, procExited := awaitPromptCompletion(promptCh, waitCh)
	if err == nil {
		t.Fatal("err = nil, want non-nil for premature exit")
	}
	if !strings.Contains(err.Error(), "exited before prompt completed") {
		t.Errorf("err = %v, want to contain %q", err, "exited before prompt completed")
	}
	if !procExited {
		t.Error("procExited = false, want true")
	}
}

func TestAwaitPromptCompletion_ProcessExitsWithError(t *testing.T) {
	promptCh := make(chan promptResult, 1)
	waitCh := make(chan error, 1)
	waitCh <- fmt.Errorf("exit status 1")

	_, err, procExited := awaitPromptCompletion(promptCh, waitCh)
	if err == nil {
		t.Fatal("err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "exited before prompt completed") {
		t.Errorf("err = %v, want to contain %q", err, "exited before prompt completed")
	}
	if !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("err = %v, want to contain wrapped exit error", err)
	}
	if !procExited {
		t.Error("procExited = false, want true")
	}
}

func TestAwaitPromptCompletion_BothReadyPromptSucceeds(t *testing.T) {
	// When both channels are ready, the prompt result should be preferred
	// regardless of which the select picks first.
	promptCh := make(chan promptResult, 1)
	waitCh := make(chan error, 1)
	promptCh <- promptResult{resp: acp.PromptResponse{StopReason: acp.StopReasonEndTurn}}
	waitCh <- nil

	_, err, _ := awaitPromptCompletion(promptCh, waitCh)
	if err != nil {
		t.Errorf("err = %v, want nil when prompt succeeded", err)
	}
}

func TestAwaitPromptCompletion_BothReadyPromptFails(t *testing.T) {
	// When both channels are ready and the prompt failed, the prompt
	// error should be returned (not the subprocess-exited message).
	promptCh := make(chan promptResult, 1)
	waitCh := make(chan error, 1)
	promptCh <- promptResult{err: fmt.Errorf("timeout")}
	waitCh <- nil

	_, err, _ := awaitPromptCompletion(promptCh, waitCh)
	if err == nil {
		t.Fatal("err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "ACP prompt") {
		t.Errorf("err = %v, want to contain %q", err, "ACP prompt")
	}
}

func TestAwaitPromptCompletion_ReturnsStopReason(t *testing.T) {
	for _, reason := range []acp.StopReason{
		acp.StopReasonEndTurn,
		acp.StopReasonMaxTokens,
		acp.StopReasonMaxTurnRequests,
		acp.StopReasonRefusal,
	} {
		t.Run(string(reason), func(t *testing.T) {
			promptCh := make(chan promptResult, 1)
			waitCh := make(chan error, 1)
			promptCh <- promptResult{resp: acp.PromptResponse{StopReason: reason}}

			got, err, _ := awaitPromptCompletion(promptCh, waitCh)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != reason {
				t.Errorf("stopReason = %q, want %q", got, reason)
			}
		})
	}
}
