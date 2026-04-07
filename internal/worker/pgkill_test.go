package worker

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestProcessGroupKill_ChildHoldsPipeOpen reproduces the exact deadlock
// observed in production: a parent process (like npx) spawns a child (like
// node/claude-code). When we Kill() just the parent, the child inherits
// the pipe fds and holds them open, so cmd.Wait() hangs forever.
//
// This test proves that killing the process GROUP (via Setpgid + negative
// PID kill) terminates the child too, allowing cmd.Wait() to return.
func TestProcessGroupKill_ChildHoldsPipeOpen(t *testing.T) {
	// The parent script spawns a long-lived child that inherits stderr,
	// then exits itself. This mirrors npx spawning node.
	cmd := exec.Command("bash", "-c", `
		# Spawn a child that sleeps for 60s, inheriting our stderr fd.
		sleep 60 &
		# Parent exits immediately.
		exit 0
	`)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Set Stderr so cmd.Wait() creates an internal copy goroutine for it.
	// This is the goroutine that hangs when the child holds the pipe open.
	cmd.Stderr = &discardWriter{}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for the parent bash to exit (it exits immediately).
	// But don't call cmd.Wait() yet — that's what hangs.
	time.Sleep(200 * time.Millisecond)

	// Terminate the entire process group.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)

	// cmd.Wait() must now return promptly because the child is also dead
	// and no process holds the stderr pipe open.
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// Success — cmd.Wait() returned.
	case <-time.After(3 * time.Second):
		t.Fatal("cmd.Wait() did not return within 3s after process group kill — child is still holding pipe open")
	}
}

// TestProcessKillOnly_ChildHoldsPipeOpen demonstrates the bug: killing only
// the parent leaves the child alive, holding the pipe, and cmd.Wait() hangs.
func TestProcessKillOnly_ChildHoldsPipeOpen(t *testing.T) {
	cmd := exec.Command("bash", "-c", `
		sleep 60 &
		exit 0
	`)
	// Do NOT set Setpgid — this is the broken case.

	cmd.Stderr = &discardWriter{}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Kill only the parent process (the old behavior).
	_ = cmd.Process.Kill()

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// On some systems this might return if the OS reaps the child.
		// That's fine — the test is primarily to contrast with the group kill test.
	case <-time.After(3 * time.Second):
		// This is the expected outcome: cmd.Wait() hangs because the
		// orphaned child holds the stderr pipe open.
		t.Log("confirmed: cmd.Wait() hangs when only the parent is killed (expected)")
		// Clean up: kill the child via the process group if possible,
		// otherwise just let the test finish.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		// Try to reap
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			// Give up — this is the cleanup path
		}
	}
}

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }
