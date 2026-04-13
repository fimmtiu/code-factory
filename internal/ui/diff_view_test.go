package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/git"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// makeCommit creates a test git.CommitEntry with the given hash and message.
func makeCommit(hash, message string) git.CommitEntry {
	return git.CommitEntry{Hash: hash, Message: message}
}

// TestBuildCommitRows_Basic verifies that commits are converted to rows in order.
func TestBuildCommitRows_Basic(t *testing.T) {
	commits := []git.CommitEntry{
		makeCommit("aaaa1111", "newest commit"),
		makeCommit("bbbb2222", "older commit"),
		makeCommit("cccc3333", "oldest commit"),
	}
	rows := buildCommitRows(commits, -1, false)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	for i, row := range rows {
		if row.separator {
			t.Errorf("row %d: unexpected separator", i)
		}
		if row.commit.Hash != commits[i].Hash {
			t.Errorf("row %d: got hash %q, want %q", i, row.commit.Hash, commits[i].Hash)
		}
	}
}

// TestBuildCommitRows_WithForkPoint verifies that a separator is inserted
// above the fork-point commit.
func TestBuildCommitRows_WithForkPoint(t *testing.T) {
	commits := []git.CommitEntry{
		makeCommit("aaaa1111", "on branch"),
		makeCommit("bbbb2222", "also on branch"),
		makeCommit("cccc3333", "fork-point commit"),
		makeCommit("dddd4444", "below fork-point"),
	}
	// Fork point at index 2 means the separator goes above index 2.
	rows := buildCommitRows(commits, 2, false)
	if len(rows) != 5 { // 4 commits + 1 separator
		t.Fatalf("expected 5 rows, got %d", len(rows))
	}
	// Row 0: aaaa, Row 1: bbbb, Row 2: separator, Row 3: cccc, Row 4: dddd
	if !rows[2].separator {
		t.Errorf("expected row 2 to be separator, got commit %q", rows[2].commit.Hash)
	}
	if rows[3].commit.Hash != "cccc3333" {
		t.Errorf("expected row 3 to be fork-point commit, got %q", rows[3].commit.Hash)
	}
}

// TestBuildCommitRows_WithUncommittedChanges verifies that uncommitted changes
// appear at the top as a pseudo-commit.
func TestBuildCommitRows_WithUncommittedChanges(t *testing.T) {
	commits := []git.CommitEntry{
		makeCommit("aaaa1111", "latest commit"),
	}
	rows := buildCommitRows(commits, -1, true)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].commit.Hash != git.UncommittedHash {
		t.Errorf("expected first row to be uncommitted, got hash %q", rows[0].commit.Hash)
	}
	if rows[0].commit.Message != "Uncommitted changes" {
		t.Errorf("expected uncommitted message, got %q", rows[0].commit.Message)
	}
	if rows[1].commit.Hash != "aaaa1111" {
		t.Errorf("expected second row to be aaaa1111, got %q", rows[1].commit.Hash)
	}
}

// TestBuildCommitRows_ForkPointAtZero verifies that the separator appears
// before the very first commit when fork point is 0.
func TestBuildCommitRows_ForkPointAtZero(t *testing.T) {
	commits := []git.CommitEntry{
		makeCommit("aaaa1111", "fork-point commit"),
		makeCommit("bbbb2222", "older"),
	}
	rows := buildCommitRows(commits, 0, false)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if !rows[0].separator {
		t.Error("expected row 0 to be separator")
	}
	if rows[1].commit.Hash != "aaaa1111" {
		t.Errorf("expected row 1 to be aaaa1111, got %q", rows[1].commit.Hash)
	}
}

// TestBuildCommitRows_UncommittedAndForkPoint verifies both uncommitted
// changes and fork-point separator coexist correctly.
func TestBuildCommitRows_UncommittedAndForkPoint(t *testing.T) {
	commits := []git.CommitEntry{
		makeCommit("aaaa1111", "on branch"),
		makeCommit("bbbb2222", "fork-point"),
	}
	rows := buildCommitRows(commits, 1, true)
	// uncommitted, aaaa, separator, bbbb
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}
	if rows[0].commit.Hash != git.UncommittedHash {
		t.Errorf("expected row 0 to be uncommitted, got %q", rows[0].commit.Hash)
	}
	if rows[1].commit.Hash != "aaaa1111" {
		t.Errorf("expected row 1 to be aaaa1111, got %q", rows[1].commit.Hash)
	}
	if !rows[2].separator {
		t.Error("expected row 2 to be separator")
	}
	if rows[3].commit.Hash != "bbbb2222" {
		t.Errorf("expected row 3 to be bbbb2222, got %q", rows[3].commit.Hash)
	}
}

// TestBuildCommitRows_NoForkPoint verifies no separator when forkPointIdx is -1.
func TestBuildCommitRows_NoForkPoint(t *testing.T) {
	commits := []git.CommitEntry{
		makeCommit("aaaa1111", "one"),
		makeCommit("bbbb2222", "two"),
	}
	rows := buildCommitRows(commits, -1, false)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	for _, row := range rows {
		if row.separator {
			t.Error("unexpected separator with forkPointIdx -1")
		}
	}
}

// TestBuildCommitRows_EmptyCommits returns no rows for empty input.
func TestBuildCommitRows_EmptyCommits(t *testing.T) {
	rows := buildCommitRows(nil, -1, false)
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

// TestBuildCommitRows_OnlyUncommitted returns single uncommitted row.
func TestBuildCommitRows_OnlyUncommitted(t *testing.T) {
	rows := buildCommitRows(nil, -1, true)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].commit.Hash != git.UncommittedHash {
		t.Errorf("expected uncommitted hash, got %q", rows[0].commit.Hash)
	}
}

// ── Navigation tests ─────────────────────────────────────────────────────────

func makeDiffViewWithRows(rows []commitRow) DiffView {
	return DiffView{
		rows:   rows,
		width:  80,
		height: 24,
		cursor: 0,
		anchor: 0,
	}
}

// TestMoveDown_Basic verifies simple downward navigation.
func TestMoveDown_Basic(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "first"),
		makeCommit("bbbb", "second"),
		makeCommit("cccc", "third"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)

	v.moveDown(1)
	if v.cursor != 1 || v.anchor != 1 {
		t.Errorf("after moveDown(1): start=%d end=%d, want 1,1", v.cursor, v.anchor)
	}
}

// TestMoveUp_Basic verifies simple upward navigation.
func TestMoveUp_Basic(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "first"),
		makeCommit("bbbb", "second"),
		makeCommit("cccc", "third"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)
	v.cursor = 2
	v.anchor = 2

	v.moveUp(1)
	if v.cursor != 1 || v.anchor != 1 {
		t.Errorf("after moveUp(1): start=%d end=%d, want 1,1", v.cursor, v.anchor)
	}
}

// TestMoveDown_SkipsSeparator verifies that downward navigation skips separators.
func TestMoveDown_SkipsSeparator(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "on branch"),
		makeCommit("bbbb", "fork-point"),
		makeCommit("cccc", "below fork"),
	}, 1, false)
	// rows: aaaa, separator, bbbb, cccc
	v := makeDiffViewWithRows(rows)

	v.moveDown(1) // aaaa -> should skip separator -> bbbb (row index 2)
	if v.cursor != 2 || v.anchor != 2 {
		t.Errorf("after moveDown(1) over separator: start=%d end=%d, want 2,2", v.cursor, v.anchor)
	}
}

// TestMoveUp_SkipsSeparator verifies that upward navigation skips separators.
func TestMoveUp_SkipsSeparator(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "on branch"),
		makeCommit("bbbb", "fork-point"),
		makeCommit("cccc", "below fork"),
	}, 1, false)
	// rows: aaaa, separator, bbbb, cccc
	v := makeDiffViewWithRows(rows)
	v.cursor = 2
	v.anchor = 2

	v.moveUp(1) // bbbb -> should skip separator -> aaaa (row index 0)
	if v.cursor != 0 || v.anchor != 0 {
		t.Errorf("after moveUp(1) over separator: start=%d end=%d, want 0,0", v.cursor, v.anchor)
	}
}

// TestMoveDown_ClampsAtEnd verifies that moving down past the last row clamps.
func TestMoveDown_ClampsAtEnd(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "only"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)

	v.moveDown(5)
	if v.cursor != 0 || v.anchor != 0 {
		t.Errorf("after moveDown past end: start=%d end=%d, want 0,0", v.cursor, v.anchor)
	}
}

// TestMoveUp_ClampsAtStart verifies that moving up past the first row clamps.
func TestMoveUp_ClampsAtStart(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "first"),
		makeCommit("bbbb", "second"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)

	v.moveUp(5)
	if v.cursor != 0 || v.anchor != 0 {
		t.Errorf("after moveUp past start: start=%d end=%d, want 0,0", v.cursor, v.anchor)
	}
}

// ── Range selection tests ────────────────────────────────────────────────────

// TestExtendRangeDown verifies shift+down extends the range downward.
func TestExtendRangeDown(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "newest"),
		makeCommit("bbbb", "middle"),
		makeCommit("cccc", "oldest"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)
	// Start at row 0
	v.cursor = 0
	v.anchor = 0

	v.extendRangeDown(1) // move start down to 1
	if v.cursor != 1 || v.anchor != 0 {
		t.Errorf("after extendRangeDown(1): start=%d end=%d, want 1,0", v.cursor, v.anchor)
	}

	v.extendRangeDown(1) // move start down to 2
	if v.cursor != 2 || v.anchor != 0 {
		t.Errorf("after extendRangeDown(2): start=%d end=%d, want 2,0", v.cursor, v.anchor)
	}
}

// TestExtendRangeUp verifies shift+up moves the cursor upward while anchor stays fixed.
func TestExtendRangeUp(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "newest"),
		makeCommit("bbbb", "middle"),
		makeCommit("cccc", "oldest"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)
	// Start at row 2
	v.cursor = 2
	v.anchor = 2

	v.extendRangeUp(1) // cursor moves up to 1, anchor stays at 2
	if v.cursor != 1 || v.anchor != 2 {
		t.Errorf("after extendRangeUp(1): cursor=%d anchor=%d, want 1,2", v.cursor, v.anchor)
	}

	v.extendRangeUp(1) // cursor moves up to 0, anchor stays at 2
	if v.cursor != 0 || v.anchor != 2 {
		t.Errorf("after extendRangeUp(2): cursor=%d anchor=%d, want 0,2", v.cursor, v.anchor)
	}
}

// TestExtendRangeDown_SkipsSeparator verifies range extension skips separators.
func TestExtendRangeDown_SkipsSeparator(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "on branch"),
		makeCommit("bbbb", "fork-point"),
		makeCommit("cccc", "below fork"),
	}, 1, false)
	// rows: aaaa(0), separator(1), bbbb(2), cccc(3)
	v := makeDiffViewWithRows(rows)
	v.cursor = 0
	v.anchor = 0

	v.extendRangeDown(1) // should skip separator, land on bbbb (row 2)
	if v.cursor != 2 || v.anchor != 0 {
		t.Errorf("after extendRangeDown over separator: start=%d end=%d, want 2,0", v.cursor, v.anchor)
	}
}

// TestExtendRangeUp_SkipsSeparator verifies range extension skips separators going up.
func TestExtendRangeUp_SkipsSeparator(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "on branch"),
		makeCommit("bbbb", "fork-point"),
		makeCommit("cccc", "below fork"),
	}, 1, false)
	// rows: aaaa(0), separator(1), bbbb(2), cccc(3)
	v := makeDiffViewWithRows(rows)
	v.cursor = 3
	v.anchor = 3

	v.extendRangeUp(1) // cursor moves to bbbb (row 2), anchor stays at 3
	if v.cursor != 2 || v.anchor != 3 {
		t.Errorf("step 1: cursor=%d anchor=%d, want 2,3", v.cursor, v.anchor)
	}

	v.extendRangeUp(1) // cursor skips separator, lands on aaaa (row 0), anchor stays at 3
	if v.cursor != 0 || v.anchor != 3 {
		t.Errorf("step 2: cursor=%d anchor=%d, want 0,3", v.cursor, v.anchor)
	}
}

// TestExtendRangeDown_ClampsAtEnd verifies that extending past the end clamps.
func TestExtendRangeDown_ClampsAtEnd(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "first"),
		makeCommit("bbbb", "second"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)
	v.cursor = 0
	v.anchor = 0

	v.extendRangeDown(10)
	if v.cursor != 1 || v.anchor != 0 {
		t.Errorf("after extendRangeDown clamp: start=%d end=%d, want 1,0", v.cursor, v.anchor)
	}
}

// TestExtendRangeUp_ClampsAtStart verifies that extending past the start clamps.
func TestExtendRangeUp_ClampsAtStart(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "first"),
		makeCommit("bbbb", "second"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)
	v.cursor = 1
	v.anchor = 1

	v.extendRangeUp(10)
	if v.cursor != 0 || v.anchor != 1 {
		t.Errorf("after extendRangeUp clamp: cursor=%d anchor=%d, want 0,1", v.cursor, v.anchor)
	}
}

// TestExtendRange_DownThenUp_ContractsRange verifies that shift+down then
// shift+up moves the cursor in both cases, contracting the selection. The
// anchor stays fixed at the original position throughout.
func TestExtendRange_DownThenUp_ContractsRange(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "newest"),
		makeCommit("bbbb", "middle"),
		makeCommit("cccc", "oldest"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)
	v.cursor = 0
	v.anchor = 0

	// Shift+Down twice: cursor goes to 2, anchor stays at 0.
	v.extendRangeDown(1)
	v.extendRangeDown(1)
	if v.cursor != 2 || v.anchor != 0 {
		t.Fatalf("after 2x extendRangeDown: cursor=%d anchor=%d, want 2,0", v.cursor, v.anchor)
	}

	// Shift+Up once: cursor goes back to 1, anchor stays at 0.
	v.extendRangeUp(1)
	if v.cursor != 1 || v.anchor != 0 {
		t.Errorf("after extendRangeUp: cursor=%d anchor=%d, want 1,0", v.cursor, v.anchor)
	}

	// Shift+Up again: cursor goes to 0, anchor stays at 0 (collapsed).
	v.extendRangeUp(1)
	if v.cursor != 0 || v.anchor != 0 {
		t.Errorf("after 2nd extendRangeUp: cursor=%d anchor=%d, want 0,0", v.cursor, v.anchor)
	}
}

// TestExtendRange_UpThenDown_ContractsRange verifies the reverse: shift+up
// from the bottom, then shift+down contracts back.
func TestExtendRange_UpThenDown_ContractsRange(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "newest"),
		makeCommit("bbbb", "middle"),
		makeCommit("cccc", "oldest"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)
	v.cursor = 2
	v.anchor = 2

	// Shift+Up twice: cursor goes to 0, anchor stays at 2.
	v.extendRangeUp(1)
	v.extendRangeUp(1)
	if v.cursor != 0 || v.anchor != 2 {
		t.Fatalf("after 2x extendRangeUp: cursor=%d anchor=%d, want 0,2", v.cursor, v.anchor)
	}

	// Shift+Down once: cursor goes to 1, anchor stays at 2.
	v.extendRangeDown(1)
	if v.cursor != 1 || v.anchor != 2 {
		t.Errorf("after extendRangeDown: cursor=%d anchor=%d, want 1,2", v.cursor, v.anchor)
	}
}

// ── Stat refresh tests ───────────────────────────────────────────────────────

// TestHandleKey_ShiftUpTriggersStatRefresh verifies that shift+up moves the
// cursor and triggers a stat refresh.
func TestHandleKey_ShiftUpTriggersStatRefresh(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "newest"),
		makeCommit("bbbb", "middle"),
		makeCommit("cccc", "oldest"),
	}, -1, false)
	v := DiffView{
		rows:         rows,
		width:        80,
		height:       24,
		cursor:       2,
		anchor:       2,
		identifier:   "proj/ticket",
		worktreePath: "/tmp/fake-worktree",
		statHash:     "cccc", // stat is cached for current cursor
	}

	updated, cmd := v.handleKey(fakeKeyMsg("shift+up"))
	dv := updated.(DiffView)
	// cursor should have moved up, anchor should stay fixed.
	if dv.cursor != 1 {
		t.Errorf("shift+up: cursor got %d, want 1", dv.cursor)
	}
	if dv.anchor != 2 {
		t.Errorf("shift+up: anchor got %d, want 2", dv.anchor)
	}
	// A stat refresh should have been triggered.
	if cmd == nil {
		t.Error("shift+up should trigger stat refresh, got nil cmd")
	}
}

// TestHandleKey_ShiftDownTriggersStatRefresh verifies shift+down also triggers
// a stat refresh (cursor moves).
func TestHandleKey_ShiftDownTriggersStatRefresh(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "newest"),
		makeCommit("bbbb", "middle"),
		makeCommit("cccc", "oldest"),
	}, -1, false)
	v := DiffView{
		rows:         rows,
		width:        80,
		height:       24,
		cursor:       0,
		anchor:       0,
		identifier:   "proj/ticket",
		worktreePath: "/tmp/fake-worktree",
		statHash:     "aaaa",
	}

	updated, cmd := v.handleKey(fakeKeyMsg("shift+down"))
	dv := updated.(DiffView)
	if dv.cursor != 1 {
		t.Errorf("shift+down: cursor got %d, want 1", dv.cursor)
	}
	if dv.anchor != 0 {
		t.Errorf("shift+down: anchor got %d, want 0", dv.anchor)
	}
	if cmd == nil {
		t.Error("shift+down should trigger stat refresh, got nil cmd")
	}
}

// fakeKeyMsg creates a tea.KeyMsg for testing.
func fakeKeyMsg(key string) tea.KeyMsg {
	switch key {
	case "shift+up":
		return tea.KeyMsg{Type: tea.KeyShiftUp}
	case "shift+down":
		return tea.KeyMsg{Type: tea.KeyShiftDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

// ── Selection count tests ────────────────────────────────────────────────────

// TestSelectedCount_Single verifies count is 1 for single selection.
func TestSelectedCount_Single(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "first"),
		makeCommit("bbbb", "second"),
		makeCommit("cccc", "third"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)
	v.cursor = 1
	v.anchor = 1

	got := v.selectedCount()
	if got != 1 {
		t.Errorf("selectedCount: got %d, want 1", got)
	}
}

// TestSelectedCount_Range verifies count for a range that spans a separator.
func TestSelectedCount_Range(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "on branch"),
		makeCommit("bbbb", "fork-point"),
		makeCommit("cccc", "below fork"),
	}, 1, false)
	// rows: aaaa(0), separator(1), bbbb(2), cccc(3)
	v := makeDiffViewWithRows(rows)
	v.cursor = 3 // cccc (oldest)
	v.anchor = 0 // aaaa (newest)

	got := v.selectedCount()
	// Should count aaaa, bbbb, cccc = 3 (skip separator)
	if got != 3 {
		t.Errorf("selectedCount with separator: got %d, want 3", got)
	}
}

// ── Render tests ─────────────────────────────────────────────────────────────

// TestRenderCommitLabel verifies the "<4-char hash> <message>" format.
func TestRenderCommitLabel(t *testing.T) {
	c := git.CommitEntry{Hash: "abcdef1234567890", Message: "fix the bug"}
	got := renderCommitLabel(c)
	if !strings.HasPrefix(got, "abcd ") {
		t.Errorf("expected label to start with first 4 chars of hash, got %q", got)
	}
	if !strings.Contains(got, "fix the bug") {
		t.Errorf("expected label to contain message, got %q", got)
	}
}

// TestRenderCommitLabel_Uncommitted verifies the "????" prefix for uncommitted changes.
func TestRenderCommitLabel_Uncommitted(t *testing.T) {
	c := git.CommitEntry{Hash: git.UncommittedHash, Message: "Uncommitted changes"}
	got := renderCommitLabel(c)
	if !strings.HasPrefix(got, "???? ") {
		t.Errorf("expected uncommitted label to start with '???? ', got %q", got)
	}
}

// TestRenderCommitLabel_ShortHash handles a hash shorter than 4 chars.
func TestRenderCommitLabel_ShortHash(t *testing.T) {
	c := git.CommitEntry{Hash: "ab", Message: "short"}
	got := renderCommitLabel(c)
	if !strings.HasPrefix(got, "ab") {
		t.Errorf("expected label to start with short hash, got %q", got)
	}
}

// ── switchToDiffViewer guard tests ────────────────────────────────────────────

// TestSwitchToDiffViewer_AllSeparators returns nil cmd when all rows are separators.
func TestSwitchToDiffViewer_AllSeparators(t *testing.T) {
	v := DiffView{
		rows: []commitRow{
			{separator: true},
			{separator: true},
		},
		width:  80,
		height: 24,
		cursor: 0,
		anchor: 1,
	}
	_, cmd := v.switchToDiffViewer()
	if cmd != nil {
		t.Error("expected nil cmd when all rows in selection are separators")
	}
}

// TestSwitchToDiffViewer_ValidSelection returns a non-nil cmd for valid commits.
func TestSwitchToDiffViewer_ValidSelection(t *testing.T) {
	v := DiffView{
		rows: []commitRow{
			{commit: makeCommit("aaaa", "first")},
			{commit: makeCommit("bbbb", "second")},
		},
		width:  80,
		height: 24,
		cursor: 0,
		anchor: 1,
	}
	_, cmd := v.switchToDiffViewer()
	if cmd == nil {
		t.Error("expected non-nil cmd for valid commit selection")
	}
}

// ── Error display tests ──────────────────────────────────────────────────────

// TestRenderStatusBar_WithError verifies the error is shown in the status bar.
func TestRenderStatusBar_WithError(t *testing.T) {
	v := DiffView{
		identifier: "proj/ticket",
		phase:      "implement",
		width:      80,
		height:     24,
		errorMsg:   "worktree error: path not found",
	}
	bar := v.renderStatusBar(v.width)
	if !strings.Contains(bar, "worktree error: path not found") {
		t.Errorf("status bar should contain error message, got %q", bar)
	}
	// Should not show commit count when there's an error.
	if strings.Contains(bar, "selected") {
		t.Error("status bar should not show commit count when error is displayed")
	}
}

// TestRenderStatusBar_ErrorClearedOnSuccess verifies the error is cleared when
// a new ticket is set successfully.
func TestRenderStatusBar_ErrorClearedOnSuccess(t *testing.T) {
	v := DiffView{
		identifier: "proj/ticket",
		phase:      "implement",
		width:      80,
		height:     24,
		errorMsg:   "worktree error: path not found",
		rows: buildCommitRows([]git.CommitEntry{
			makeCommit("aaaa", "one"),
		}, -1, false),
	}
	// Verify error is shown initially.
	bar := v.renderStatusBar(v.width)
	if !strings.Contains(bar, "worktree error") {
		t.Error("expected error in status bar initially")
	}

	// Clear the error and verify it's gone.
	v.errorMsg = ""
	bar = v.renderStatusBar(v.width)
	if strings.Contains(bar, "worktree error") {
		t.Error("expected error to be cleared from status bar")
	}
	if !strings.Contains(bar, "selected") {
		t.Error("expected commit count in status bar after error cleared")
	}
}

// ── Status bar test ──────────────────────────────────────────────────────────

// TestRenderStatusBar verifies the status bar content.
func TestRenderStatusBar(t *testing.T) {
	v := DiffView{
		identifier: "project/ticket-1",
		phase:      "implement",
		cursor:     0,
		anchor:     0,
		width:      80,
		height:     24,
		rows: buildCommitRows([]git.CommitEntry{
			makeCommit("aaaa", "one"),
		}, -1, false),
	}
	bar := v.renderStatusBar(v.width)
	if !strings.Contains(bar, "project/ticket-1") {
		t.Errorf("status bar should contain identifier, got %q", bar)
	}
	if !strings.Contains(bar, "implement") {
		t.Errorf("status bar should contain phase, got %q", bar)
	}
	if !strings.Contains(bar, "1 commit selected") {
		t.Errorf("status bar should contain selection count, got %q", bar)
	}
}

// TestRenderStatusBar_Range verifies the status bar with a range selection.
func TestRenderStatusBar_Range(t *testing.T) {
	v := DiffView{
		identifier: "my-proj/my-ticket",
		phase:      "review",
		cursor:     2,
		anchor:     0,
		width:      80,
		height:     24,
		rows: buildCommitRows([]git.CommitEntry{
			makeCommit("aaaa", "one"),
			makeCommit("bbbb", "two"),
			makeCommit("cccc", "three"),
		}, -1, false),
	}
	bar := v.renderStatusBar(v.width)
	if !strings.Contains(bar, "3 commits selected") {
		t.Errorf("status bar should show 3 selected, got %q", bar)
	}
}

// ── Commit list height test ──────────────────────────────────────────────────

// TestCommitListHeight verifies the height calculation for the commit list.
func TestCommitListHeight(t *testing.T) {
	v := DiffView{width: 80, height: 24}
	h := v.commitListHeight()
	// height - chromeHeight - statusBarHeight(1) - statusBarBorderHeight(1) - viewBorderOverhead
	expected := 24 - chromeHeight - 1 - 1 - viewBorderOverhead
	if h != expected {
		t.Errorf("commitListHeight: got %d, want %d", h, expected)
	}
}

// ── Left pane width test ─────────────────────────────────────────────────────

// TestLeftPaneWidth verifies the left pane is approximately 1/3 of terminal width.
func TestLeftPaneWidth(t *testing.T) {
	v := DiffView{width: 90, height: 24}
	w := v.leftPaneWidth()
	if w != 30 {
		t.Errorf("leftPaneWidth for width=90: got %d, want 30", w)
	}
}

// ── Error propagation tests ──────────────────────────────────────────────────

// TestDiffCommitListMsg_WithError verifies that an error in the commit list
// fetch is displayed to the user via errorMsg.
func TestDiffCommitListMsg_WithError(t *testing.T) {
	v := DiffView{
		width:      80,
		height:     24,
		identifier: "proj/ticket",
		phase:      "implement",
	}

	updated, cmd := v.Update(diffCommitListMsg{
		forkPointIdx: -1,
		errMsg:       "exit status 128",
	})
	dv := updated.(DiffView)
	if dv.errorMsg == "" {
		t.Error("expected errorMsg to be set when commit list fetch fails")
	}
	if !strings.Contains(dv.errorMsg, "exit status 128") {
		t.Errorf("errorMsg should contain the original error, got %q", dv.errorMsg)
	}
	if len(dv.rows) != 0 {
		t.Errorf("expected no rows on error, got %d", len(dv.rows))
	}
	if cmd != nil {
		t.Error("expected nil cmd on error")
	}
}

// TestDiffCommitListMsg_SuccessClearsError verifies that a successful fetch
// does not set errorMsg.
func TestDiffCommitListMsg_SuccessClearsError(t *testing.T) {
	v := DiffView{
		width:      80,
		height:     24,
		identifier: "proj/ticket",
		phase:      "implement",
		errorMsg:   "previous error",
	}

	updated, _ := v.Update(diffCommitListMsg{
		commits:      []git.CommitEntry{makeCommit("aaaa", "one")},
		forkPointIdx: -1,
	})
	dv := updated.(DiffView)
	if dv.errorMsg != "" {
		t.Errorf("errorMsg should be empty on success, got %q", dv.errorMsg)
	}
	if len(dv.rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(dv.rows))
	}
}

// ── resetForTicket tests ─────────────────────────────────────────────────────

// TestResetForTicket_ClearsStaleState verifies that resetForTicket clears
// all fields from the previous ticket, preventing stale-state flash.
func TestResetForTicket_ClearsStaleState(t *testing.T) {
	v := DiffView{
		width:      80,
		height:     24,
		identifier: "old/ticket",
		phase:      "implement",
		rows: buildCommitRows([]git.CommitEntry{
			makeCommit("aaaa", "old commit"),
		}, -1, false),
		cursor:     1,
		anchor:     1,
		offset:     5,
		statOutput: "old stat output",
		statHash:   "aaaa",
		viewer:     &DiffViewerModel{},
	}

	v.resetForTicket("new/ticket", "review", false, "/tmp/worktree", nil)

	if v.identifier != "new/ticket" {
		t.Errorf("identifier = %q, want %q", v.identifier, "new/ticket")
	}
	if v.phase != "review" {
		t.Errorf("phase = %q, want %q", v.phase, "review")
	}
	if len(v.rows) != 0 {
		t.Errorf("rows should be cleared, got %d rows", len(v.rows))
	}
	if v.cursor != 0 {
		t.Errorf("cursor = %d, want 0", v.cursor)
	}
	if v.anchor != 0 {
		t.Errorf("anchor = %d, want 0", v.anchor)
	}
	if v.offset != 0 {
		t.Errorf("offset = %d, want 0", v.offset)
	}
	if v.statOutput != "" {
		t.Errorf("statOutput = %q, want empty", v.statOutput)
	}
	if v.statHash != "" {
		t.Errorf("statHash = %q, want empty", v.statHash)
	}
	if v.viewer != nil {
		t.Error("viewer should be nil")
	}
	if v.errorMsg != "" {
		t.Errorf("errorMsg = %q, want empty", v.errorMsg)
	}
}

// ── HintPairs tests ──────────────────────────────────────────────────────────

// TestHintPairs_CommitSelector verifies hint pairs for the commit selector screen.
func TestHintPairs_CommitSelector(t *testing.T) {
	v := DiffView{width: 80, height: 24}
	pairs := v.HintPairs()
	if len(pairs) == 0 {
		t.Fatal("expected non-empty hint pairs for commit selector")
	}
	// Should contain "navigate" for the commit selector screen.
	found := false
	for _, p := range pairs {
		if p == "navigate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'navigate' in commit selector hint pairs")
	}
}

// TestHintPairs_ViewerMode verifies hint pairs for the viewer screen.
func TestHintPairs_ViewerMode(t *testing.T) {
	v := DiffView{
		width:  80,
		height: 24,
		viewer: &DiffViewerModel{},
	}
	pairs := v.HintPairs()
	if len(pairs) == 0 {
		t.Fatal("expected non-empty hint pairs for viewer")
	}
	// Should contain "scroll" for the viewer screen.
	found := false
	for _, p := range pairs {
		if p == "scroll" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'scroll' in viewer hint pairs")
	}
}

// ── Theme integration tests ──────────────────────────────────────────────────

// withTestTheme temporarily replaces CurrentTheme with a modified theme that
// uses structurally distinctive styles (padding, not just colour), restoring
// the original when the test completes. Padding differences are visible even
// in no-colour test environments.
func withTestTheme(t *testing.T) {
	t.Helper()
	original := theme.Current()
	t.Cleanup(func() { theme.SetCurrent(original) })

	custom := theme.Tan()
	custom.DiffSelectedStyle = lipgloss.NewStyle().Padding(0, 3)
	custom.CommitHashStyle = lipgloss.NewStyle().Padding(0, 3)
	custom.DiffSeparatorStyle = lipgloss.NewStyle().Padding(0, 3)
	custom.DiffRangeStyle = lipgloss.NewStyle().Padding(0, 3)
	custom.EmptyStateStyle = lipgloss.NewStyle().Padding(0, 3)
	custom.DiffErrorStyle = lipgloss.NewStyle().Padding(0, 3)
	custom.DiffLabelBold = lipgloss.NewStyle().Padding(0, 3)
	custom.ViewPaneStyle = lipgloss.NewStyle().Padding(1, 1)
	custom.DiffStatusBarStyle = lipgloss.NewStyle().Padding(0, 3)
	theme.SetCurrent(custom)
}

// TestRenderCommitRow_UsesThemeSelectedStyle verifies that the cursor row uses
// theme.Current().DiffSelectedStyle.
func TestRenderCommitRow_UsesThemeSelectedStyle(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa1111", "first commit"),
		makeCommit("bbbb2222", "second commit"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)
	v.cursor = 0

	assertThemeChangesOutput(t, withTestTheme, func() string {
		return v.renderCommitRow(0, 40, 0, 0)
	})
}

// TestRenderCommitRow_UsesThemeRangeStyle verifies that range-selected rows
// use theme.Current().DiffRangeStyle.
func TestRenderCommitRow_UsesThemeRangeStyle(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa1111", "first"),
		makeCommit("bbbb2222", "second"),
		makeCommit("cccc3333", "third"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)
	v.cursor = 2

	assertThemeChangesOutput(t, withTestTheme, func() string {
		return v.renderCommitRow(1, 40, 0, 2)
	})
}

// TestRenderCommitRow_UsesThemeCommitHashStyle verifies that unselected rows
// style the hash prefix using theme.Current().CommitHashStyle.
func TestRenderCommitRow_UsesThemeCommitHashStyle(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa1111", "first"),
		makeCommit("bbbb2222", "second"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)
	v.cursor = 0

	assertThemeChangesOutput(t, withTestTheme, func() string {
		// Row 1 is unselected (cursor is at 0, range is 0..0)
		return v.renderCommitRow(1, 40, 0, 0)
	})
}

// TestRenderCommitRow_UsesThemeSeparatorStyle verifies that separator rows
// use theme.Current().DiffSeparatorStyle.
func TestRenderCommitRow_UsesThemeSeparatorStyle(t *testing.T) {
	rows := buildCommitRows([]git.CommitEntry{
		makeCommit("aaaa", "on branch"),
		makeCommit("bbbb", "fork-point"),
	}, 1, false)
	// rows: aaaa(0), separator(1), bbbb(2)
	v := makeDiffViewWithRows(rows)

	assertThemeChangesOutput(t, withTestTheme, func() string {
		return v.renderCommitRow(1, 40, 0, 0)
	})
}

// TestRenderDiffLabel_UsesThemeLabelBold verifies that renderDiffLabel uses
// theme.Current().DiffLabelBold.
func TestRenderDiffLabel_UsesThemeLabelBold(t *testing.T) {
	assertThemeChangesOutput(t, withTestTheme, func() string {
		return renderDiffLabel("proj/ticket", "implement", false)
	})
}

// TestRenderStatusBar_UsesThemeErrorStyle verifies that the error message in
// the status bar uses theme.Current().DiffErrorStyle.
func TestRenderStatusBar_UsesThemeErrorStyle(t *testing.T) {
	v := DiffView{
		identifier: "proj/ticket",
		phase:      "implement",
		width:      80,
		height:     24,
		errorMsg:   "test error",
	}

	assertThemeChangesOutput(t, withTestTheme, func() string {
		return v.renderStatusBar(80)
	})
}

// TestDiffView_EmptyStateUsesTheme verifies that the "No ticket selected"
// empty state uses theme.Current().EmptyStateStyle and ViewPaneStyle.
func TestDiffView_EmptyStateUsesTheme(t *testing.T) {
	v := DiffView{
		width:  80,
		height: 24,
	}

	assertThemeChangesOutput(t, withTestTheme, func() string {
		return v.View()
	})
}
