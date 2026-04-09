package worker

import (
	"fmt"
	"strings"
	"testing"
)

func TestAwaitPromptCompletion_PromptSucceeds(t *testing.T) {
	promptCh := make(chan error, 1)
	waitCh := make(chan error, 1)
	promptCh <- nil

	err, procExited := awaitPromptCompletion(promptCh, waitCh)
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if procExited {
		t.Error("procExited = true, want false")
	}
}

func TestAwaitPromptCompletion_PromptFails(t *testing.T) {
	promptCh := make(chan error, 1)
	waitCh := make(chan error, 1)
	promptCh <- fmt.Errorf("connection reset")

	err, procExited := awaitPromptCompletion(promptCh, waitCh)
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
	promptCh := make(chan error, 1)
	waitCh := make(chan error, 1)
	waitCh <- nil // clean exit, but prompt never completed

	err, procExited := awaitPromptCompletion(promptCh, waitCh)
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
	promptCh := make(chan error, 1)
	waitCh := make(chan error, 1)
	waitCh <- fmt.Errorf("exit status 1")

	err, procExited := awaitPromptCompletion(promptCh, waitCh)
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
	promptCh := make(chan error, 1)
	waitCh := make(chan error, 1)
	promptCh <- nil
	waitCh <- nil

	err, _ := awaitPromptCompletion(promptCh, waitCh)
	if err != nil {
		t.Errorf("err = %v, want nil when prompt succeeded", err)
	}
}

func TestAwaitPromptCompletion_BothReadyPromptFails(t *testing.T) {
	// When both channels are ready and the prompt failed, the prompt
	// error should be returned (not the subprocess-exited message).
	promptCh := make(chan error, 1)
	waitCh := make(chan error, 1)
	promptCh <- fmt.Errorf("timeout")
	waitCh <- nil

	err, _ := awaitPromptCompletion(promptCh, waitCh)
	if err == nil {
		t.Fatal("err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "ACP prompt") {
		t.Errorf("err = %v, want to contain %q", err, "ACP prompt")
	}
}
