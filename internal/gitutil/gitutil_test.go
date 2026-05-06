package gitutil_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fimmtiu/code-factory/internal/gitutil"
)

// initTestRepo creates an isolated git repo in a temp directory with an initial commit.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err != nil {
			t.Fatalf("setup command %v failed: %v\n%s", args, err, out)
		}
	}

	// Create a file and make an initial commit so HEAD exists.
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# test\n"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}

	addOut, err := exec.Command("git", "-C", dir, "add", "README.md").CombinedOutput()
	if err != nil {
		t.Fatalf("git add failed: %v\n%s", err, addOut)
	}

	commitOut, err := exec.Command("git", "-C", dir, "commit", "-m", "initial commit").CombinedOutput()
	if err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, commitOut)
	}

	return dir
}

// currentBranch returns the current branch name in the given repo.
func currentBranch(t *testing.T, repoRoot string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// branchExists checks whether a named branch exists in the repo.
func branchExists(t *testing.T, repoRoot, branch string) bool {
	t.Helper()
	err := exec.Command("git", "-C", repoRoot, "rev-parse", "--verify", branch).Run()
	return err == nil
}

func TestCreateWorktree(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	branchName := "my-feature"
	worktreePath := filepath.Join(dir, ".code-factory", "my-feature", "worktree")
	err := client.CreateWorktree(dir, worktreePath, branchName)
	if err != nil {
		t.Fatalf("CreateWorktree returned error: %v", err)
	}

	// The worktree directory should exist.
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Fatalf("worktree directory %q was not created", worktreePath)
	}

	// The branch should exist.
	if !branchExists(t, dir, branchName) {
		t.Fatalf("branch %q was not created", branchName)
	}
}

func TestCreateWorktreeCopiesClaudeSettings(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	// Create .claude/settings.json in the repo root.
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create .claude dir: %v", err)
	}
	settings := []byte(`{"permissions":{"allow":["Bash(go *)"]}}`)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), settings, 0644); err != nil {
		t.Fatalf("failed to write settings.json: %v", err)
	}

	worktreePath := filepath.Join(dir, ".code-factory", "test-copy", "worktree")
	if err := client.CreateWorktree(dir, worktreePath, "test-copy"); err != nil {
		t.Fatalf("CreateWorktree returned error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(worktreePath, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("settings.json not copied to worktree: %v", err)
	}
	if string(got) != string(settings) {
		t.Errorf("copied settings mismatch: got %q, want %q", got, settings)
	}

	// settings.local.json was not created, so it should not appear.
	if _, err := os.Stat(filepath.Join(worktreePath, ".claude", "settings.local.json")); !os.IsNotExist(err) {
		t.Fatal("settings.local.json should not exist when absent from repo root")
	}
}

func TestCreateWorktreeCopiesLocalSettings(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create .claude dir: %v", err)
	}
	settings := []byte(`{"permissions":{"allow":["Bash(go *)"]}}`)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), settings, 0644); err != nil {
		t.Fatalf("failed to write settings.json: %v", err)
	}
	localSettings := []byte(`{"permissions":{"allow":["Bash(make *)"]}}`)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), localSettings, 0644); err != nil {
		t.Fatalf("failed to write settings.local.json: %v", err)
	}

	worktreePath := filepath.Join(dir, ".code-factory", "test-local", "worktree")
	if err := client.CreateWorktree(dir, worktreePath, "test-local"); err != nil {
		t.Fatalf("CreateWorktree returned error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(worktreePath, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("settings.local.json not copied to worktree: %v", err)
	}
	if string(got) != string(localSettings) {
		t.Errorf("copied local settings mismatch: got %q, want %q", got, localSettings)
	}
}

func TestCreateWorktreeNoSettingsNoCopy(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	worktreePath := filepath.Join(dir, ".code-factory", "no-settings", "worktree")
	if err := client.CreateWorktree(dir, worktreePath, "no-settings"); err != nil {
		t.Fatalf("CreateWorktree returned error: %v", err)
	}

	// No .claude/settings.json in repo root, so none should appear in worktree.
	if _, err := os.Stat(filepath.Join(worktreePath, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("settings.json should not exist in worktree when absent from repo root")
	}
}

func TestCreateWorktreeWithSlashIdentifier(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	// CreateWorktree sanitises "/" to "_" in branch names so git accepts them.
	branchName := "project/fix-bug"
	safeBranch := "project_fix-bug"
	worktreePath := filepath.Join(dir, ".code-factory", "project", "fix-bug", "worktree")
	err := client.CreateWorktree(dir, worktreePath, branchName)
	if err != nil {
		t.Fatalf("CreateWorktree returned error for slash identifier: %v", err)
	}

	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Fatalf("worktree directory %q was not created", worktreePath)
	}

	if !branchExists(t, dir, safeBranch) {
		t.Fatalf("branch %q was not created", safeBranch)
	}
}

func TestMergeBranch(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	// Determine the default branch name (could be "main" or "master").
	baseBranch := currentBranch(t, dir)

	// Create a feature branch from base.
	featureBranch := "feature-branch"
	out, err := exec.Command("git", "-C", dir, "checkout", "-b", featureBranch).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to create feature branch: %v\n%s", err, out)
	}

	// Add a commit on the feature branch.
	filePath := filepath.Join(dir, "feature.txt")
	if err := os.WriteFile(filePath, []byte("feature work\n"), 0644); err != nil {
		t.Fatalf("failed to write feature file: %v", err)
	}
	out, err = exec.Command("git", "-C", dir, "add", "feature.txt").CombinedOutput()
	if err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}
	out, err = exec.Command("git", "-C", dir, "commit", "-m", "feature commit").CombinedOutput()
	if err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	// Record the feature branch tip before fast-forwarding.
	featureTip := revParse(t, dir, "HEAD")

	// Go back to base branch before merging.
	out, err = exec.Command("git", "-C", dir, "checkout", baseBranch).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to checkout base branch: %v\n%s", err, out)
	}

	// Fast-forward baseBranch to featureBranch.
	err = client.MergeBranch(dir, featureBranch)
	if err != nil {
		t.Fatalf("MergeBranch returned error: %v", err)
	}

	// The merged file should now be present on baseBranch.
	mergedFile := filepath.Join(dir, "feature.txt")
	if _, err := os.Stat(mergedFile); os.IsNotExist(err) {
		t.Fatal("feature.txt not present after merge")
	}

	// We should still be on baseBranch.
	branch := currentBranch(t, dir)
	if branch != baseBranch {
		t.Fatalf("expected to be on %q after merge, got %q", baseBranch, branch)
	}

	// Fast-forward means base HEAD == feature HEAD (no merge commit).
	baseTip := revParse(t, dir, "HEAD")
	if baseTip != featureTip {
		t.Fatalf("expected fast-forward (base HEAD %q == feature HEAD %q) but got a merge commit", baseTip, featureTip)
	}
}

func TestMergeBranch_NonFastForwardFails(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	baseBranch := currentBranch(t, dir)

	// Create a feature branch and commit on it.
	if out, err := exec.Command("git", "-C", dir, "checkout", "-b", "diverge").CombinedOutput(); err != nil {
		t.Fatalf("checkout diverge failed: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("feature\n"), 0644); err != nil {
		t.Fatalf("write f.txt: %v", err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "f.txt").CombinedOutput(); err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-m", "feature").CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}

	// Advance baseBranch with an unrelated commit so it has diverged from diverge.
	if out, err := exec.Command("git", "-C", dir, "checkout", baseBranch).CombinedOutput(); err != nil {
		t.Fatalf("checkout base: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("base\n"), 0644); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "b.txt").CombinedOutput(); err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-m", "base").CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}

	// diverge is no longer a descendant of baseBranch, so --ff-only must fail.
	if err := client.MergeBranch(dir, "diverge"); err == nil {
		t.Fatal("expected MergeBranch to fail when fast-forward is not possible")
	}
}

func revParse(t *testing.T, repoRoot, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", ref).Output()
	if err != nil {
		t.Fatalf("rev-parse %q: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

func TestSquashSinceMergeBase(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()
	baseBranch := currentBranch(t, dir)

	if out, err := exec.Command("git", "-C", dir, "checkout", "-b", "feature").CombinedOutput(); err != nil {
		t.Fatalf("checkout feature: %v\n%s", err, out)
	}
	for i, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if out, err := exec.Command("git", "-C", dir, "add", name).CombinedOutput(); err != nil {
			t.Fatalf("add %s: %v\n%s", name, err, out)
		}
		msg := []string{"add a", "add b", "add c"}[i]
		if out, err := exec.Command("git", "-C", dir, "commit", "-m", msg).CombinedOutput(); err != nil {
			t.Fatalf("commit %s: %v\n%s", name, err, out)
		}
	}

	if err := client.SquashSinceMergeBase(dir, baseBranch, "feature: summary"); err != nil {
		t.Fatalf("SquashSinceMergeBase: %v", err)
	}

	// Exactly one commit since base.
	out, err := exec.Command("git", "-C", dir, "rev-list", "--count", baseBranch+"..HEAD").Output()
	if err != nil {
		t.Fatalf("rev-list: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "1" {
		t.Fatalf("expected 1 commit since base after squash, got %q", got)
	}

	// All three files present at the squashed tip.
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to remain after squash: %v", name, err)
		}
	}

	msgOut, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%B").Output()
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	msg := string(msgOut)
	if !strings.HasPrefix(msg, "feature: summary") {
		t.Errorf("expected commit message subject %q, got: %s", "feature: summary", msg)
	}
	for _, want := range []string{"* add a", "* add b", "* add c"} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected squashed body to contain %q, got: %s", want, msg)
		}
	}

	// Squashing again must be a no-op (only one commit since base).
	tipBefore := revParse(t, dir, "HEAD")
	if err := client.SquashSinceMergeBase(dir, baseBranch, "feature: summary"); err != nil {
		t.Fatalf("idempotent SquashSinceMergeBase: %v", err)
	}
	if got := revParse(t, dir, "HEAD"); got != tipBefore {
		t.Errorf("expected idempotent squash, but HEAD moved from %s to %s", tipBefore, got)
	}
}

func TestSquashSinceMergeBase_SingleCommitNoOp(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()
	baseBranch := currentBranch(t, dir)

	if out, err := exec.Command("git", "-C", dir, "checkout", "-b", "single").CombinedOutput(); err != nil {
		t.Fatalf("checkout single: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatalf("write x: %v", err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "x.txt").CombinedOutput(); err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-m", "only commit").CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}

	tipBefore := revParse(t, dir, "HEAD")
	if err := client.SquashSinceMergeBase(dir, baseBranch, "single: summary"); err != nil {
		t.Fatalf("SquashSinceMergeBase: %v", err)
	}
	if got := revParse(t, dir, "HEAD"); got != tipBefore {
		t.Errorf("expected single-commit branch to be unchanged, but HEAD moved from %s to %s", tipBefore, got)
	}
}

func TestRemoveWorktree(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	branchName := "remove-me"
	worktreePath := filepath.Join(dir, ".code-factory", "remove-me", "worktree")

	// Create the worktree first.
	if err := client.CreateWorktree(dir, worktreePath, branchName); err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Fatal("worktree was not created before removal test")
	}

	// Now remove it.
	if err := client.RemoveWorktree(dir, worktreePath, branchName); err != nil {
		t.Fatalf("RemoveWorktree returned error: %v", err)
	}

	// The worktree directory should be gone.
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Fatalf("worktree directory %q still exists after removal", worktreePath)
	}
}
