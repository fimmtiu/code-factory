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
		// Override any global merge.conflictStyle so tests are hermetic.
		// diff3 style includes commit-specific hashes in conflict markers
		// which breaks rerere fingerprint matching across branches.
		{"git", "-C", dir, "config", "merge.conflictStyle", "merge"},
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

func TestEnableRerere(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	// EnableRerere should set rerere.enabled=true and rerere.autoUpdate=true.
	if err := client.EnableRerere(dir); err != nil {
		t.Fatalf("EnableRerere: %v", err)
	}

	out, err := exec.Command("git", "-C", dir, "config", "rerere.enabled").Output()
	if err != nil {
		t.Fatalf("git config rerere.enabled: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "true" {
		t.Errorf("expected rerere.enabled=true, got %q", got)
	}

	out, err = exec.Command("git", "-C", dir, "config", "rerere.autoUpdate").Output()
	if err != nil {
		t.Fatalf("git config rerere.autoUpdate: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "true" {
		t.Errorf("expected rerere.autoUpdate=true, got %q", got)
	}
}

func TestEnableRerere_Idempotent(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	// Calling EnableRerere twice should not error.
	if err := client.EnableRerere(dir); err != nil {
		t.Fatalf("first EnableRerere: %v", err)
	}
	if err := client.EnableRerere(dir); err != nil {
		t.Fatalf("second EnableRerere: %v", err)
	}
}

func TestEnableRerere_WorktreeSharesCache(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	// Create a worktree and enable rerere from it.
	worktreePath := filepath.Join(dir, ".code-factory", "rerere-test", "worktree")
	if err := client.CreateWorktree(dir, worktreePath, "rerere-test"); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	if err := client.EnableRerere(worktreePath); err != nil {
		t.Fatalf("EnableRerere in worktree: %v", err)
	}

	// The config should be readable from the worktree.
	out, err := exec.Command("git", "-C", worktreePath, "config", "rerere.enabled").Output()
	if err != nil {
		t.Fatalf("git config rerere.enabled from worktree: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "true" {
		t.Errorf("expected rerere.enabled=true from worktree, got %q", got)
	}
}

func TestRerereAutoResolvesRepeatedConflict(t *testing.T) {
	// Integration test: git rerere records a resolution during one rebase
	// and auto-applies it when an identical conflict occurs in a second
	// rebase (simulating the same conflict resurfacing one cascade level up).
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	baseBranch := currentBranch(t, dir)

	// Enable rerere before any conflicts arise.
	if err := client.EnableRerere(dir); err != nil {
		t.Fatalf("EnableRerere: %v", err)
	}

	// Create a shared file on the base branch.
	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatalf("write shared.txt: %v", err)
	}
	gitRun(t, dir, "add", "shared.txt")
	gitRun(t, dir, "commit", "-m", "add shared.txt")

	// Record the pre-fork commit so we can branch from it later.
	preFork := revParse(t, dir, "HEAD")

	// Branch A modifies shared.txt.
	gitRun(t, dir, "checkout", "-b", "branch-a")
	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("line1\nmodified-by-a\nline3\n"), 0644); err != nil {
		t.Fatalf("write shared.txt on branch-a: %v", err)
	}
	gitRun(t, dir, "add", "shared.txt")
	gitRun(t, dir, "commit", "-m", "branch-a modifies shared.txt")

	// Branch B modifies the same line differently (branches from preFork).
	gitRun(t, dir, "checkout", preFork)
	gitRun(t, dir, "checkout", "-b", "branch-b")
	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("line1\nmodified-by-b\nline3\n"), 0644); err != nil {
		t.Fatalf("write shared.txt on branch-b: %v", err)
	}
	gitRun(t, dir, "add", "shared.txt")
	gitRun(t, dir, "commit", "-m", "branch-b modifies shared.txt")

	// Branch C has the same change as B (branches from preFork).
	gitRun(t, dir, "checkout", preFork)
	gitRun(t, dir, "checkout", "-b", "branch-c")
	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("line1\nmodified-by-b\nline3\n"), 0644); err != nil {
		t.Fatalf("write shared.txt on branch-c: %v", err)
	}
	gitRun(t, dir, "add", "shared.txt")
	gitRun(t, dir, "commit", "-m", "branch-c modifies shared.txt (same as b)")

	// Merge branch-a into base first (fast-forward).
	gitRun(t, dir, "checkout", baseBranch)
	gitRun(t, dir, "merge", "--ff-only", "branch-a")

	// Rebase branch-b onto base — this will conflict.
	gitRun(t, dir, "checkout", "branch-b")
	out, err := exec.Command("git", "-C", dir, "rebase", baseBranch).CombinedOutput()
	if err == nil {
		t.Fatal("expected rebase to conflict, but it succeeded")
	}
	_ = out

	// Resolve the conflict manually: accept branch-a's version.
	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("line1\nmodified-by-a\nline3\n"), 0644); err != nil {
		t.Fatalf("resolve conflict: %v", err)
	}
	gitRun(t, dir, "add", "shared.txt")
	// Explicitly tell rerere to record the postimage (resolution) so it
	// can be replayed later. Without this, rebase --continue may finalize
	// the commit before rerere saves the mapping.
	gitRun(t, dir, "rerere")
	gitRun(t, dir, "rebase", "--continue")

	// Rebase branch-c onto base — rerere should auto-resolve the same
	// conflict pattern using the resolution recorded above.
	gitRun(t, dir, "checkout", "branch-c")
	out, err = exec.Command("git", "-C", dir, "rebase", baseBranch).CombinedOutput()
	if err != nil {
		// Even with rerere.autoUpdate, git rebase may still pause to let
		// the user verify. If rerere resolved the file (no markers), just
		// stage and continue.
		content, readErr := os.ReadFile(filepath.Join(dir, "shared.txt"))
		if readErr != nil {
			t.Fatalf("rebase failed and could not read shared.txt: rebase: %v\n%s", err, out)
		}
		if strings.Contains(string(content), "<<<<<<<") {
			t.Fatalf("rerere did not auto-resolve the conflict; shared.txt still has markers:\n%s", content)
		}
		// Rerere resolved it; stage and continue.
		gitRun(t, dir, "add", "shared.txt")
		gitRun(t, dir, "rebase", "--continue")
	}

	// Verify the resolution was applied correctly.
	content, err := os.ReadFile(filepath.Join(dir, "shared.txt"))
	if err != nil {
		t.Fatalf("read shared.txt: %v", err)
	}
	if string(content) != "line1\nmodified-by-a\nline3\n" {
		t.Errorf("expected rerere to apply same resolution, got:\n%s", content)
	}
}

// gitRun runs a git command and fails the test on error.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	fullArgs := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", fullArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
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

func TestFindForbiddenMarkers_FindsTODOAndPanic(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()
	baseBranch := currentBranch(t, dir)

	if out, err := exec.Command("git", "-C", dir, "checkout", "-b", "feat").CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v\n%s", err, out)
	}
	contents := "package x\n" +
		"func A() {\n" +
		"\t// TODO: implement A\n" +
		"\tpanic(\"unimplemented\")\n" +
		"}\n"
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(contents), 0644); err != nil {
		t.Fatalf("write x.go: %v", err)
	}
	// Test files should be skipped, even if they contain matching markers.
	testContents := "package x\n// TODO: cover edge cases\n"
	if err := os.WriteFile(filepath.Join(dir, "x_test.go"), []byte(testContents), 0644); err != nil {
		t.Fatalf("write x_test.go: %v", err)
	}
	for _, name := range []string{"x.go", "x_test.go"} {
		if out, err := exec.Command("git", "-C", dir, "add", name).CombinedOutput(); err != nil {
			t.Fatalf("add %s: %v\n%s", name, err, out)
		}
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-m", "stub").CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}

	hits, err := client.FindForbiddenMarkers(dir, baseBranch)
	if err != nil {
		t.Fatalf("FindForbiddenMarkers: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits in x.go (TODO + panic), got %d: %v", len(hits), hits)
	}
	for _, hit := range hits {
		if !strings.HasPrefix(hit, "x.go:") {
			t.Errorf("hit should be in x.go (test file should be skipped): %q", hit)
		}
	}
	// One hit names the TODO line, the other names the panic line.
	var sawTodo, sawPanic bool
	for _, hit := range hits {
		if strings.Contains(hit, "TODO") {
			sawTodo = true
		}
		if strings.Contains(hit, "unimplemented") {
			sawPanic = true
		}
	}
	if !sawTodo || !sawPanic {
		t.Errorf("expected both TODO and unimplemented hits, got: %v", hits)
	}
}

func TestFindForbiddenMarkers_CleanDiffReturnsNoHits(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()
	baseBranch := currentBranch(t, dir)

	if out, err := exec.Command("git", "-C", dir, "checkout", "-b", "clean").CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "y.go"), []byte("package x\nfunc Y() {}\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "y.go").CombinedOutput(); err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-m", "y").CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}

	hits, err := client.FindForbiddenMarkers(dir, baseBranch)
	if err != nil {
		t.Fatalf("FindForbiddenMarkers: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected no hits, got %v", hits)
	}
}

func TestFindForbiddenMarkers_IgnoresPreExistingMarkersOnBase(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	// Put a TODO into the base branch before the feature branch diverges.
	// FindForbiddenMarkers must not flag it because it isn't an *added* line
	// on the feature branch.
	if err := os.WriteFile(filepath.Join(dir, "z.go"), []byte("package x\n// TODO: pre-existing\n"), 0644); err != nil {
		t.Fatalf("write base: %v", err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "z.go").CombinedOutput(); err != nil {
		t.Fatalf("add base: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-m", "base TODO").CombinedOutput(); err != nil {
		t.Fatalf("commit base: %v\n%s", err, out)
	}
	baseBranch := currentBranch(t, dir)

	if out, err := exec.Command("git", "-C", dir, "checkout", "-b", "innocent").CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "innocent.go"), []byte("package x\nfunc I() {}\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "innocent.go").CombinedOutput(); err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-m", "innocent").CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}

	hits, err := client.FindForbiddenMarkers(dir, baseBranch)
	if err != nil {
		t.Fatalf("FindForbiddenMarkers: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected no hits on innocent feature branch, got %v", hits)
	}
}

func TestFindForbiddenMarkers_MultiLanguageStubs(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()
	baseBranch := currentBranch(t, dir)

	if out, err := exec.Command("git", "-C", dir, "checkout", "-b", "polyglot").CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v\n%s", err, out)
	}
	files := map[string]string{
		"a.py": "def a():\n    raise NotImplementedError\n",
		"b.rb": "def b\n  raise NotImplementedError\nend\n",
		"c.rs": "fn c() { unimplemented!() }\n",
		"d.rs": "fn d() { todo!() }\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if out, err := exec.Command("git", "-C", dir, "add", name).CombinedOutput(); err != nil {
			t.Fatalf("add %s: %v\n%s", name, err, out)
		}
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-m", "polyglot stubs").CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}

	hits, err := client.FindForbiddenMarkers(dir, baseBranch)
	if err != nil {
		t.Fatalf("FindForbiddenMarkers: %v", err)
	}
	if len(hits) != 4 {
		t.Fatalf("expected 4 hits (one per language), got %d: %v", len(hits), hits)
	}
}
