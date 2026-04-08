package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// makeCommit creates a test commitEntry with the given hash and message.
func makeCommit(hash, message string) commitEntry {
	return commitEntry{Hash: hash, Message: message}
}

// TestBuildCommitRows_Basic verifies that commits are converted to rows in order.
func TestBuildCommitRows_Basic(t *testing.T) {
	commits := []commitEntry{
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
	commits := []commitEntry{
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
	commits := []commitEntry{
		makeCommit("aaaa1111", "latest commit"),
	}
	rows := buildCommitRows(commits, -1, true)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].commit.Hash != uncommittedHash {
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
	commits := []commitEntry{
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
	commits := []commitEntry{
		makeCommit("aaaa1111", "on branch"),
		makeCommit("bbbb2222", "fork-point"),
	}
	rows := buildCommitRows(commits, 1, true)
	// uncommitted, aaaa, separator, bbbb
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}
	if rows[0].commit.Hash != uncommittedHash {
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
	commits := []commitEntry{
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
	if rows[0].commit.Hash != uncommittedHash {
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
	rows := buildCommitRows([]commitEntry{
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
	rows := buildCommitRows([]commitEntry{
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
	rows := buildCommitRows([]commitEntry{
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
	rows := buildCommitRows([]commitEntry{
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
	rows := buildCommitRows([]commitEntry{
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
	rows := buildCommitRows([]commitEntry{
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
	rows := buildCommitRows([]commitEntry{
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

// TestExtendRangeUp verifies shift+up extends the range upward.
func TestExtendRangeUp(t *testing.T) {
	rows := buildCommitRows([]commitEntry{
		makeCommit("aaaa", "newest"),
		makeCommit("bbbb", "middle"),
		makeCommit("cccc", "oldest"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)
	// Start at row 2
	v.cursor = 2
	v.anchor = 2

	v.extendRangeUp(1) // move end up to 1
	if v.cursor != 2 || v.anchor != 1 {
		t.Errorf("after extendRangeUp(1): start=%d end=%d, want 2,1", v.cursor, v.anchor)
	}

	v.extendRangeUp(1) // move end up to 0
	if v.cursor != 2 || v.anchor != 0 {
		t.Errorf("after extendRangeUp(2): start=%d end=%d, want 2,0", v.cursor, v.anchor)
	}
}

// TestExtendRangeDown_SkipsSeparator verifies range extension skips separators.
func TestExtendRangeDown_SkipsSeparator(t *testing.T) {
	rows := buildCommitRows([]commitEntry{
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
	rows := buildCommitRows([]commitEntry{
		makeCommit("aaaa", "on branch"),
		makeCommit("bbbb", "fork-point"),
		makeCommit("cccc", "below fork"),
	}, 1, false)
	// rows: aaaa(0), separator(1), bbbb(2), cccc(3)
	v := makeDiffViewWithRows(rows)
	v.cursor = 3
	v.anchor = 3

	v.extendRangeUp(1) // bbbb (row 2)
	if v.cursor != 3 || v.anchor != 2 {
		t.Errorf("step 1: start=%d end=%d, want 3,2", v.cursor, v.anchor)
	}

	v.extendRangeUp(1) // should skip separator, land on aaaa (row 0)
	if v.cursor != 3 || v.anchor != 0 {
		t.Errorf("step 2: start=%d end=%d, want 3,0", v.cursor, v.anchor)
	}
}

// TestExtendRangeDown_ClampsAtEnd verifies that extending past the end clamps.
func TestExtendRangeDown_ClampsAtEnd(t *testing.T) {
	rows := buildCommitRows([]commitEntry{
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
	rows := buildCommitRows([]commitEntry{
		makeCommit("aaaa", "first"),
		makeCommit("bbbb", "second"),
	}, -1, false)
	v := makeDiffViewWithRows(rows)
	v.cursor = 1
	v.anchor = 1

	v.extendRangeUp(10)
	if v.cursor != 1 || v.anchor != 0 {
		t.Errorf("after extendRangeUp clamp: start=%d end=%d, want 1,0", v.cursor, v.anchor)
	}
}

// ── Stat refresh tests ───────────────────────────────────────────────────────

// TestHandleKey_ShiftUpTriggersStatRefresh verifies that shift+up triggers a
// stat refresh even though it only moves the anchor, not the cursor.
func TestHandleKey_ShiftUpTriggersStatRefresh(t *testing.T) {
	rows := buildCommitRows([]commitEntry{
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
	// anchor should have moved, cursor should not.
	if dv.cursor != 2 {
		t.Errorf("shift+up moved cursor: got %d, want 2", dv.cursor)
	}
	if dv.anchor != 1 {
		t.Errorf("shift+up did not move anchor: got %d, want 1", dv.anchor)
	}
	// A stat refresh should have been triggered.
	if cmd == nil {
		t.Error("shift+up should trigger stat refresh, got nil cmd")
	}
}

// TestHandleKey_ShiftDownTriggersStatRefresh verifies shift+down also triggers
// a stat refresh (cursor moves).
func TestHandleKey_ShiftDownTriggersStatRefresh(t *testing.T) {
	rows := buildCommitRows([]commitEntry{
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
	rows := buildCommitRows([]commitEntry{
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
	rows := buildCommitRows([]commitEntry{
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
	c := commitEntry{Hash: "abcdef1234567890", Message: "fix the bug"}
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
	c := commitEntry{Hash: uncommittedHash, Message: "Uncommitted changes"}
	got := renderCommitLabel(c)
	if !strings.HasPrefix(got, "???? ") {
		t.Errorf("expected uncommitted label to start with '???? ', got %q", got)
	}
}

// TestRenderCommitLabel_ShortHash handles a hash shorter than 4 chars.
func TestRenderCommitLabel_ShortHash(t *testing.T) {
	c := commitEntry{Hash: "ab", Message: "short"}
	got := renderCommitLabel(c)
	if !strings.HasPrefix(got, "ab") {
		t.Errorf("expected label to start with short hash, got %q", got)
	}
}

// ── Parse tests ──────────────────────────────────────────────────────────────

// TestParseGitLog_Normal verifies parsing of standard git log output.
func TestParseGitLog_Normal(t *testing.T) {
	log := "abc123 Fix the thing\ndef456 Add feature\n"
	commits := parseGitLog(log)
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	if commits[0].Hash != "abc123" || commits[0].Message != "Fix the thing" {
		t.Errorf("commit 0: got %+v", commits[0])
	}
	if commits[1].Hash != "def456" || commits[1].Message != "Add feature" {
		t.Errorf("commit 1: got %+v", commits[1])
	}
}

// TestParseGitLog_Empty returns nil for empty input.
func TestParseGitLog_Empty(t *testing.T) {
	commits := parseGitLog("")
	if len(commits) != 0 {
		t.Errorf("expected 0 commits, got %d", len(commits))
	}
}

// TestParseGitLog_Whitespace handles trailing whitespace and blank lines.
func TestParseGitLog_Whitespace(t *testing.T) {
	log := "abc123 Fix\n\n  \n"
	commits := parseGitLog(log)
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
}

// TestParseGitLog_LimitsTo100 verifies that at most 100 commits are returned.
func TestParseGitLog_LimitsTo100(t *testing.T) {
	var lines []string
	for i := 0; i < 150; i++ {
		lines = append(lines, "abcdef1 commit message")
	}
	log := strings.Join(lines, "\n")
	commits := parseGitLog(log)
	if len(commits) != maxCommits {
		t.Errorf("expected %d commits, got %d", maxCommits, len(commits))
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
		rows: buildCommitRows([]commitEntry{
			makeCommit("aaaa", "one"),
		}, -1, false),
	}
	bar := v.renderStatusBar()
	if !strings.Contains(bar, "project/ticket-1") {
		t.Errorf("status bar should contain identifier, got %q", bar)
	}
	if !strings.Contains(bar, "implement") {
		t.Errorf("status bar should contain phase, got %q", bar)
	}
	if !strings.Contains(bar, "1 commit(s) selected") {
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
		rows: buildCommitRows([]commitEntry{
			makeCommit("aaaa", "one"),
			makeCommit("bbbb", "two"),
			makeCommit("cccc", "three"),
		}, -1, false),
	}
	bar := v.renderStatusBar()
	if !strings.Contains(bar, "3 commit(s) selected") {
		t.Errorf("status bar should show 3 selected, got %q", bar)
	}
}

// ── Commit list height test ──────────────────────────────────────────────────

// TestCommitListHeight verifies the height calculation for the commit list.
func TestCommitListHeight(t *testing.T) {
	v := DiffView{width: 80, height: 24}
	h := v.commitListHeight()
	// height - chromeHeight - statusBarHeight(1) - separator(1) - viewBorderOverhead
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
