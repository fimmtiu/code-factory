package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fimmtiu/code-factory/internal/config"
	"github.com/fimmtiu/code-factory/internal/git"
)

const (
	worktreeIncludeFile = ".worktreeinclude"
	setupMarkerFile     = "code-factory-setup"
)

// worktreeSetupCommand returns the configured worktree setup command, or an
// empty string if none is configured.
func worktreeSetupCommand() string {
	if config.Current == nil {
		return ""
	}
	return config.Current.WorktreeSetup
}

// prepareWorktree makes a ticket worktree ready for agents. It copies the
// gitignored files that the main checkout's .worktreeinclude names (such as
// .env), then runs setupCmd in the worktree. Output goes to a new file at
// logPath, which is created only if there is work to do.
//
// The work runs one time per worktree and setup command. A marker in the
// worktree's private git directory records the command that succeeded, and
// git deletes that directory when it removes the worktree. If setupCmd fails,
// no marker is written, so the next run tries again.
func prepareWorktree(ctx context.Context, repoRoot, worktreePath, setupCmd, logPath string) error {
	includePath := filepath.Join(repoRoot, worktreeIncludeFile)
	hasInclude, err := pathExists(includePath)
	if err != nil {
		return err
	}
	if setupCmd == "" && !hasInclude {
		return nil
	}

	gitDir, err := git.Output(worktreePath, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return fmt.Errorf("find git directory of %s: %w", worktreePath, err)
	}
	marker := filepath.Join(gitDir, setupMarkerFile)
	if done, err := os.ReadFile(marker); err == nil && string(done) == setupCmd {
		return nil
	}

	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create setup log: %w", err)
	}
	defer func() { _ = logFile.Close() }()

	if hasInclude {
		copied, err := copyWorktreeIncludes(repoRoot, includePath, worktreePath)
		if err != nil {
			_, _ = fmt.Fprintf(logFile, "copy %s files: %v\n", worktreeIncludeFile, err)
			return fmt.Errorf("copy %s files: %w", worktreeIncludeFile, err)
		}
		for _, rel := range copied {
			_, _ = fmt.Fprintf(logFile, "copied %s\n", rel)
		}
	}

	if setupCmd != "" {
		_, _ = fmt.Fprintf(logFile, "=== SETUP ===\n%s\n\n", setupCmd)
		if err := runSetupCommand(ctx, worktreePath, setupCmd, logFile); err != nil {
			_, _ = fmt.Fprintf(logFile, "\n=== SETUP FAILED ===\n%v\n", err)
			return fmt.Errorf("worktree setup command %q: %w", setupCmd, err)
		}
	}

	return os.WriteFile(marker, []byte(setupCmd), 0644)
}

// runSetupCommand runs setupCmd with sh in dir, sending stdout and stderr to
// out. The command runs in its own session for the same reasons as the agent
// process (see runACP): cancelling ctx stops the whole process tree, and the
// command cannot write to the TUI's terminal through /dev/tty.
func runSetupCommand(ctx context.Context, dir, setupCmd string, out io.Writer) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", setupCmd)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = 10 * time.Second
	return cmd.Run()
}

// copyWorktreeIncludes copies into worktreePath each untracked file in
// repoRoot that matches a pattern in includePath and that git also ignores.
// This is the same rule that Claude Code uses for .worktreeinclude. Files
// that already exist in the worktree are kept. It returns the repo-relative
// paths that it copied.
func copyWorktreeIncludes(repoRoot, includePath, worktreePath string) ([]string, error) {
	matched, err := exec.Command("git", "-C", repoRoot, "ls-files", "-z",
		"--others", "--ignored", "--exclude-from="+includePath).Output()
	if err != nil {
		return nil, fmt.Errorf("list files that match %s: %w", includePath, err)
	}
	if len(matched) == 0 {
		return nil, nil
	}

	checkIgnore := exec.Command("git", "-C", repoRoot, "check-ignore", "-z", "--stdin")
	checkIgnore.Stdin = bytes.NewReader(matched)
	ignored, err := checkIgnore.Output()
	// check-ignore exits 1 when no path is ignored.
	var exitErr *exec.ExitError
	if err != nil && !(errors.As(err, &exitErr) && exitErr.ExitCode() == 1) {
		return nil, fmt.Errorf("check which files git ignores: %w", err)
	}

	var copied []string
	for _, rel := range strings.Split(string(ignored), "\x00") {
		if rel == "" {
			continue
		}
		dst := filepath.Join(worktreePath, rel)
		exists, err := pathExists(dst)
		if err != nil {
			return copied, err
		}
		if exists {
			continue
		}
		if err := copyPath(filepath.Join(repoRoot, rel), dst); err != nil {
			return copied, err
		}
		copied = append(copied, rel)
	}
	return copied, nil
}

// copyPath copies a regular file or a symlink from src to dst, and creates
// the parent directories of dst as necessary.
func copyPath(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// pathExists reports whether path exists, without following a symlink.
func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}
