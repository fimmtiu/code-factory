package client

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

const (
	ensureRunningTimeout  = 5 * time.Second
	ensureRunningInterval = 100 * time.Millisecond
)

// IsRunning reports whether the tickets daemon is reachable at socketPath by
// attempting a Ping. It returns false on any error.
func IsRunning(socketPath string) bool {
	c := NewClient(socketPath)
	_, err := c.Ping()
	return err == nil
}

// StartDaemon starts the ticketsd binary in the background with repoRoot as
// its argument. The process is started in its own session (Setsid) so it
// outlives the calling process. StartDaemon does not wait for the process to
// become ready; use EnsureRunning for that.
func StartDaemon(repoRoot string) error {
	cmd := exec.Command("ticketsd", repoRoot)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

// EnsureRunning checks whether the daemon is already running at socketPath. If
// it is, it returns nil immediately. Otherwise it calls StartDaemon and polls
// IsRunning every 100 ms for up to 5 seconds. It returns an error if the
// daemon is not reachable after the timeout.
func EnsureRunning(socketPath, repoRoot string) error {
	if IsRunning(socketPath) {
		return nil
	}

	if err := StartDaemon(repoRoot); err != nil {
		return fmt.Errorf("autostart: start daemon: %w", err)
	}

	deadline := time.Now().Add(ensureRunningTimeout)
	for time.Now().Before(deadline) {
		if IsRunning(socketPath) {
			return nil
		}
		time.Sleep(ensureRunningInterval)
	}

	return fmt.Errorf("autostart: daemon did not become ready within %s", ensureRunningTimeout)
}
