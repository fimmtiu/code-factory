package gitutil

// FakeGitClient implements GitClient without invoking real git. It records
// worktree paths passed to CreateWorktree for test assertions.
type FakeGitClient struct {
	WorktreesCreated []string
}

func (f *FakeGitClient) CreateWorktree(_, worktreePath, _ string) error {
	f.WorktreesCreated = append(f.WorktreesCreated, worktreePath)
	return nil
}
func (f *FakeGitClient) MergeBranch(_, _ string) error              { return nil }
func (f *FakeGitClient) RemoveWorktree(_, _, _ string) error        { return nil }
func (f *FakeGitClient) GetHeadCommit(_ string) (string, error)     { return "", nil }
func (f *FakeGitClient) GetCurrentBranch(_ string) (string, error)  { return "main", nil }
func (f *FakeGitClient) RebaseOnto(_, _ string) error               { return nil }
