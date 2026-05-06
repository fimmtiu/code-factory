package worker

import (
	"os/exec"
	"testing"
)

func TestRefactorCommitsAdded(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(cmd.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("commit", "--allow-empty", "-m", "init")
	headOut, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	preHEAD := string(headOut[:len(headOut)-1])

	t.Run("no commits added", func(t *testing.T) {
		got, err := refactorCommitsAdded(dir, preHEAD)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got {
			t.Error("got true, want false (no new commits)")
		}
	})

	t.Run("non-refactor commit", func(t *testing.T) {
		run("commit", "--allow-empty", "-m", "fix: unrelated change")
		got, err := refactorCommitsAdded(dir, preHEAD)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got {
			t.Error("got true, want false (only non-refactor commit added)")
		}
	})

	t.Run("refactor commit present", func(t *testing.T) {
		run("commit", "--allow-empty", "-m", "refactor: extract helper")
		got, err := refactorCommitsAdded(dir, preHEAD)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if !got {
			t.Error("got false, want true (refactor: commit added)")
		}
	})
}

func TestCommitsAddedSince(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(cmd.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("commit", "--allow-empty", "-m", "init")
	headOut, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	preHEAD := string(headOut[:len(headOut)-1])

	t.Run("no commits added", func(t *testing.T) {
		got, err := commitsAddedSince(dir, preHEAD)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got {
			t.Error("got true, want false (HEAD unchanged)")
		}
	})

	t.Run("commit added", func(t *testing.T) {
		run("commit", "--allow-empty", "-m", "any subject")
		got, err := commitsAddedSince(dir, preHEAD)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if !got {
			t.Error("got false, want true (one commit added)")
		}
	})
}
