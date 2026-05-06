package gitutil

// FakeGitClient implements GitClient without invoking real git. It records
// worktree paths passed to CreateWorktree, merge targets, and rebase targets
// for test assertions.
type FakeGitClient struct {
	WorktreesCreated []string
	MergeTargets     []string
	RebaseTargets    []string // ontoBranch values passed to RebaseOnto
	RebasesAborted   []string // worktreeDir values passed to AbortRebase
	Squashes         []string // worktreeDir values passed to SquashSinceMergeBase
	RereresEnabled   []string // worktreeDir values passed to EnableRerere

	// RebaseErr, if non-nil, is returned from RebaseOnto instead of success.
	// Useful for exercising the conflict path in callers.
	RebaseErr error

	// RebaseErrByOnto, if non-nil, lets tests target a specific rebase target
	// (the ontoBranch argument). When the key matches, the corresponding error
	// is returned; otherwise RebaseErr (which may itself be nil) applies.
	RebaseErrByOnto map[string]error

	// RebaseErrFunc, if non-nil, is consulted on every RebaseOnto call. A
	// non-nil return value overrides RebaseErrByOnto and RebaseErr. Useful
	// for tests that need to fail a specific call in a sequence (e.g. only
	// the second rebase in a cascading completion).
	RebaseErrFunc func(worktreeDir, ontoBranch string) error
}

func (f *FakeGitClient) CreateWorktree(_, worktreePath, _ string) error {
	f.WorktreesCreated = append(f.WorktreesCreated, worktreePath)
	return nil
}
func (f *FakeGitClient) MergeBranch(worktreeDir, _ string) error {
	f.MergeTargets = append(f.MergeTargets, worktreeDir)
	return nil
}
func (f *FakeGitClient) RemoveWorktree(_, _, _ string) error       { return nil }
func (f *FakeGitClient) GetHeadCommit(_ string) (string, error)    { return "", nil }
func (f *FakeGitClient) GetCurrentBranch(_ string) (string, error) { return "main", nil }
func (f *FakeGitClient) RebaseOnto(worktreeDir, ontoBranch string) error {
	f.RebaseTargets = append(f.RebaseTargets, ontoBranch)
	if f.RebaseErrFunc != nil {
		if err := f.RebaseErrFunc(worktreeDir, ontoBranch); err != nil {
			return err
		}
	}
	if err, ok := f.RebaseErrByOnto[ontoBranch]; ok {
		return err
	}
	return f.RebaseErr
}
func (f *FakeGitClient) AbortRebase(worktreeDir string) error {
	f.RebasesAborted = append(f.RebasesAborted, worktreeDir)
	return nil
}
func (f *FakeGitClient) SquashSinceMergeBase(worktreeDir, _, _ string) error {
	f.Squashes = append(f.Squashes, worktreeDir)
	return nil
}
func (f *FakeGitClient) EnableRerere(worktreeDir string) error {
	f.RereresEnabled = append(f.RereresEnabled, worktreeDir)
	return nil
}
