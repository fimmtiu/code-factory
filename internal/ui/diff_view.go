package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/storage"
)

// maxCommits is the maximum number of commits shown in the selector.
const maxCommits = 100

// uncommittedHash is the sentinel hash for the "uncommitted changes" pseudo-commit.
const uncommittedHash = "????"

// statusBarHeight is the number of lines consumed by the status bar and its
// horizontal separator.
const statusBarHeight = 1

// separatorLineHeight is the line separating the status bar from the panes.
const separatorLineHeight = 1

// ── Data types ───────────────────────────────────────────────────────────────

// commitEntry represents one commit in the list.
type commitEntry struct {
	Hash    string
	Message string
}

// commitRow represents one row in the commit list. If separator is true, the
// row is a non-selectable divider above the fork-point commit (same pattern
// as the CommandView separator between status groups).
type commitRow struct {
	commit    commitEntry
	separator bool
}

// ── Messages ─────────────────────────────────────────────────────────────────

// diffCommitListMsg carries the result of the async commit-list fetch.
type diffCommitListMsg struct {
	commits      []commitEntry
	forkPointIdx int  // index into commits of the fork-point commit, or -1
	hasUncommit  bool // true if there are uncommitted changes
}

// diffShowStatMsg carries the git show --stat output for a commit.
type diffShowStatMsg struct {
	hash   string
	output string
}

// switchToDiffViewerMsg is sent when the user presses Tab/Enter to view the diff.
type switchToDiffViewerMsg struct {
	startCommit commitEntry
	endCommit   commitEntry
}

// setDiffTicketMsg is sent to configure the DiffView with a ticket's context.
type setDiffTicketMsg struct {
	identifier string
	phase      string
}

// ── DiffView ─────────────────────────────────────────────────────────────────

// DiffView implements the commit selector screen of the Diffs view (F5).
// It shows a two-pane layout: commit list on the left, git show --stat on the right.
type DiffView struct {
	width  int
	height int

	// Ticket context
	identifier string
	phase      string

	// Commit data
	commits      []commitEntry
	rows         []commitRow
	forkPointIdx int

	// Selection state: startCommit is the older (higher index) end of the range,
	// endCommit is the newer (lower index) end. For single selection, they are equal.
	startCommit int
	endCommit   int
	offset      int // first visible row in the commit list

	// Right pane: cached git show --stat output
	statOutput string
	statHash   string // hash for which statOutput was fetched
}

// NewDiffView creates an empty DiffView.
func NewDiffView() DiffView {
	return DiffView{
		forkPointIdx: -1,
	}
}

// Init returns nil; the view loads data when a ticket is set.
func (v DiffView) Init() tea.Cmd {
	return nil
}

// ── Row building ─────────────────────────────────────────────────────────────

// buildCommitRows converts a commit list into rows, inserting a separator
// above the fork-point commit and prepending an uncommitted-changes pseudo-commit.
func buildCommitRows(commits []commitEntry, forkPointIdx int, hasUncommit bool) []commitRow {
	var rows []commitRow

	if hasUncommit {
		rows = append(rows, commitRow{
			commit: commitEntry{Hash: uncommittedHash, Message: "Uncommitted changes"},
		})
	}

	for i, c := range commits {
		if i == forkPointIdx {
			rows = append(rows, commitRow{separator: true})
		}
		rows = append(rows, commitRow{commit: c})
	}

	return rows
}

// ── Label rendering ──────────────────────────────────────────────────────────

// renderCommitLabel returns "<4-char hash> <message>" for a commit.
func renderCommitLabel(c commitEntry) string {
	h := c.Hash
	if h != uncommittedHash {
		if len(h) > 4 {
			h = h[:4]
		}
	}
	return h + " " + c.Message
}

// ── Parse helpers ────────────────────────────────────────────────────────────

// parseGitLog parses "git log --format='%H %s'" output into commitEntry slices.
// It limits the result to maxCommits entries.
func parseGitLog(output string) []commitEntry {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var commits []commitEntry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		hash := parts[0]
		msg := ""
		if len(parts) > 1 {
			msg = parts[1]
		}
		commits = append(commits, commitEntry{Hash: hash, Message: msg})
		if len(commits) >= maxCommits {
			break
		}
	}
	return commits
}

// ── Dimension helpers ────────────────────────────────────────────────────────

// leftPaneWidth returns the width of the commit list pane (~1/3 of terminal).
func (v DiffView) leftPaneWidth() int {
	w := v.width / 3
	if w < 10 {
		w = 10
	}
	return w
}

// rightPaneWidth returns the width of the stat preview pane (~2/3 of terminal).
func (v DiffView) rightPaneWidth() int {
	w := v.width - v.leftPaneWidth()
	if w < 10 {
		w = 10
	}
	return w
}

// commitListHeight returns the number of visible rows in the commit list body.
func (v DiffView) commitListHeight() int {
	h := v.height - chromeHeight - statusBarHeight - separatorLineHeight - viewBorderOverhead
	if h < 1 {
		h = 1
	}
	return h
}

// ── Selection helpers ────────────────────────────────────────────────────────

// selectedCount returns the number of non-separator commits in the selection range.
func (v DiffView) selectedCount() int {
	lo, hi := v.selectionRange()
	count := 0
	for i := lo; i <= hi; i++ {
		if i < len(v.rows) && !v.rows[i].separator {
			count++
		}
	}
	return count
}

// selectionRange returns (lo, hi) row indices for the current selection,
// with lo <= hi regardless of which direction the range was extended.
func (v DiffView) selectionRange() (int, int) {
	lo, hi := v.endCommit, v.startCommit
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo, hi
}

// currentCommit returns the commit entry at the startCommit position (the
// "cursor" for single-selection and the anchor for range selection).
func (v DiffView) currentCommit() *commitEntry {
	// The cursor position is always startCommit for single selection,
	// but for display/stat purposes we show whichever end moved most recently.
	// Use startCommit as the "current" commit for stat display.
	idx := v.startCommit
	if idx < 0 || idx >= len(v.rows) || v.rows[idx].separator {
		return nil
	}
	return &v.rows[idx].commit
}

// ── Navigation ───────────────────────────────────────────────────────────────

// moveDown moves the single selection down by n steps, skipping separators.
// Both startCommit and endCommit move together.
func (v *DiffView) moveDown(n int) {
	last := len(v.rows) - 1
	if last < 0 {
		return
	}
	for i := 0; i < n; i++ {
		next := v.startCommit + 1
		// Skip separators.
		for next <= last && v.rows[next].separator {
			next++
		}
		if next > last {
			break
		}
		v.startCommit = next
	}
	v.endCommit = v.startCommit
	v.clampScroll()
}

// moveUp moves the single selection up by n steps, skipping separators.
// Both startCommit and endCommit move together.
func (v *DiffView) moveUp(n int) {
	if len(v.rows) == 0 {
		return
	}
	for i := 0; i < n; i++ {
		next := v.startCommit - 1
		// Skip separators.
		for next >= 0 && v.rows[next].separator {
			next--
		}
		if next < 0 {
			break
		}
		v.startCommit = next
	}
	v.endCommit = v.startCommit
	v.clampScroll()
}

// extendRangeDown moves startCommit downward (older) while leaving endCommit fixed.
func (v *DiffView) extendRangeDown(n int) {
	last := len(v.rows) - 1
	if last < 0 {
		return
	}
	for i := 0; i < n; i++ {
		next := v.startCommit + 1
		for next <= last && v.rows[next].separator {
			next++
		}
		if next > last {
			break
		}
		v.startCommit = next
	}
	v.clampScroll()
}

// extendRangeUp moves endCommit upward (newer) while leaving startCommit fixed.
func (v *DiffView) extendRangeUp(n int) {
	if len(v.rows) == 0 {
		return
	}
	for i := 0; i < n; i++ {
		next := v.endCommit - 1
		for next >= 0 && v.rows[next].separator {
			next--
		}
		if next < 0 {
			break
		}
		v.endCommit = next
	}
	v.clampScroll()
}

// clampScroll ensures the cursor (startCommit) is visible in the list.
func (v *DiffView) clampScroll() {
	h := v.commitListHeight()
	if h <= 0 || len(v.rows) == 0 {
		v.offset = 0
		return
	}
	// The visible cursor is startCommit.
	if v.startCommit < v.offset {
		v.offset = v.startCommit
	}
	if v.startCommit >= v.offset+h {
		v.offset = v.startCommit - h + 1
	}
	maxOffset := len(v.rows) - h
	if maxOffset < 0 {
		maxOffset = 0
	}
	if v.offset > maxOffset {
		v.offset = maxOffset
	}
	if v.offset < 0 {
		v.offset = 0
	}
}

// clampSelected ensures startCommit and endCommit are valid after a data refresh.
func (v *DiffView) clampSelected() {
	if len(v.rows) == 0 {
		v.startCommit = 0
		v.endCommit = 0
		return
	}
	if v.startCommit >= len(v.rows) {
		v.startCommit = len(v.rows) - 1
	}
	if v.startCommit < 0 {
		v.startCommit = 0
	}
	if v.endCommit >= len(v.rows) {
		v.endCommit = len(v.rows) - 1
	}
	if v.endCommit < 0 {
		v.endCommit = 0
	}
	// Skip separators.
	for v.startCommit < len(v.rows) && v.rows[v.startCommit].separator {
		v.startCommit++
	}
	if v.startCommit >= len(v.rows) {
		v.startCommit = len(v.rows) - 1
		for v.startCommit >= 0 && v.rows[v.startCommit].separator {
			v.startCommit--
		}
	}
	if v.startCommit < 0 {
		v.startCommit = 0
	}
	v.endCommit = v.startCommit
}

// ── Commands ─────────────────────────────────────────────────────────────────

// fetchCommitsCmd fetches the commit list and fork point asynchronously.
func fetchCommitsCmd(identifier string) tea.Cmd {
	return func() tea.Msg {
		worktreePath, err := storage.WorktreePathForIdentifier(identifier)
		if err != nil {
			return diffCommitListMsg{forkPointIdx: -1}
		}

		// Fetch non-merge commits.
		logOutput, err := gitOutput(worktreePath, "log", "--no-merges", "--format=%H %s", fmt.Sprintf("-%d", maxCommits))
		if err != nil {
			return diffCommitListMsg{forkPointIdx: -1}
		}
		commits := parseGitLog(logOutput)

		// Detect fork point.
		defaultBranch := detectDefaultBranch(worktreePath)
		forkHash, err := gitOutput(worktreePath, "merge-base", "--fork-point", defaultBranch)
		forkPointIdx := -1
		if err == nil && forkHash != "" {
			for i, c := range commits {
				if c.Hash == forkHash {
					forkPointIdx = i
					break
				}
			}
		}

		// Check for uncommitted changes.
		statusOut, err := gitOutput(worktreePath, "status", "--porcelain")
		hasUncommit := err == nil && strings.TrimSpace(statusOut) != ""

		return diffCommitListMsg{
			commits:      commits,
			forkPointIdx: forkPointIdx,
			hasUncommit:  hasUncommit,
		}
	}
}

// fetchShowStatCmd fetches `git show --stat` output for a commit.
func fetchShowStatCmd(identifier, hash string) tea.Cmd {
	return func() tea.Msg {
		if hash == uncommittedHash {
			worktreePath, err := storage.WorktreePathForIdentifier(identifier)
			if err != nil {
				return diffShowStatMsg{hash: hash, output: "(error)"}
			}
			out, err := gitOutput(worktreePath, "diff", "--stat")
			if err != nil {
				return diffShowStatMsg{hash: hash, output: "(no changes)"}
			}
			return diffShowStatMsg{hash: hash, output: out}
		}

		worktreePath, err := storage.WorktreePathForIdentifier(identifier)
		if err != nil {
			return diffShowStatMsg{hash: hash, output: "(error)"}
		}
		out, err := gitOutput(worktreePath, "show", "--stat", "--format=", hash)
		if err != nil {
			return diffShowStatMsg{hash: hash, output: "(error)"}
		}
		return diffShowStatMsg{hash: hash, output: out}
	}
}

// ── Update ───────────────────────────────────────────────────────────────────

func (v DiffView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.clampScroll()
		return v, nil

	case setDiffTicketMsg:
		v.identifier = msg.identifier
		v.phase = msg.phase
		return v, fetchCommitsCmd(v.identifier)

	case diffCommitListMsg:
		v.commits = msg.commits
		v.forkPointIdx = msg.forkPointIdx
		v.rows = buildCommitRows(msg.commits, msg.forkPointIdx, msg.hasUncommit)
		v.clampSelected()
		v.clampScroll()
		// Fetch stat for the initial selection.
		return v, v.fetchStatForCurrent()

	case diffShowStatMsg:
		v.statHash = msg.hash
		v.statOutput = msg.output
		return v, nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}

	return v, nil
}

func (v DiffView) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	prevStart := v.startCommit

	switch msg.String() {
	case "up":
		v.moveUp(1)
	case "down":
		v.moveDown(1)
	case "pgup":
		v.moveUp(v.commitListHeight())
	case "pgdown":
		v.moveDown(v.commitListHeight())
	case "shift+down":
		v.extendRangeDown(1)
	case "shift+up":
		v.extendRangeUp(1)
	case "tab", "enter":
		return v.switchToDiffViewer()
	default:
		return v, nil
	}

	// If the cursor moved to a different commit, fetch its stat.
	if v.startCommit != prevStart {
		return v, v.fetchStatForCurrent()
	}
	return v, nil
}

// fetchStatForCurrent returns a command to fetch the stat for the current commit.
func (v DiffView) fetchStatForCurrent() tea.Cmd {
	c := v.currentCommit()
	if c == nil || v.identifier == "" {
		return nil
	}
	if c.Hash == v.statHash {
		return nil // already cached
	}
	return fetchShowStatCmd(v.identifier, c.Hash)
}

func (v DiffView) switchToDiffViewer() (tea.Model, tea.Cmd) {
	lo, hi := v.selectionRange()
	if lo < 0 || hi >= len(v.rows) {
		return v, nil
	}
	// Find the actual commit entries at the range boundaries.
	var startC, endC commitEntry
	for i := lo; i <= hi; i++ {
		if !v.rows[i].separator {
			endC = v.rows[i].commit
			break
		}
	}
	for i := hi; i >= lo; i-- {
		if !v.rows[i].separator {
			startC = v.rows[i].commit
			break
		}
	}
	return v, func() tea.Msg {
		return switchToDiffViewerMsg{startCommit: startC, endCommit: endC}
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (v DiffView) View() string {
	if v.identifier == "" {
		paneW := v.width - viewBorderOverhead
		h := v.height - chromeHeight - viewBorderOverhead
		if h < 1 {
			h = 1
		}
		return viewPaneStyle.Width(paneW).Height(h).
			Render(lipgloss.Place(paneW, h, lipgloss.Center, lipgloss.Center,
				emptyStateStyle.Render("No ticket selected")))
	}

	statusBar := v.renderStatusBar()
	separator := strings.Repeat("─", v.width)
	leftPane := v.renderLeftPane()
	rightPane := v.renderRightPane()
	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, separator, panes)
}

// renderStatusBar renders the status bar with ticket info and selection count.
func (v DiffView) renderStatusBar() string {
	left := fmt.Sprintf("Ticket: %s (%s)", v.identifier, v.phase)
	right := fmt.Sprintf("%d commit(s) selected", v.selectedCount())

	spacer := v.width - lipgloss.Width(left) - lipgloss.Width(right)
	if spacer < 2 {
		spacer = 2
	}
	return left + strings.Repeat(" ", spacer) + right
}

// renderLeftPane renders the commit list pane.
func (v DiffView) renderLeftPane() string {
	w := v.leftPaneWidth() - viewBorderOverhead
	if w < 1 {
		w = 1
	}
	h := v.commitListHeight()

	if len(v.rows) == 0 {
		return viewPaneStyle.Width(w).Height(h).
			Render(lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center,
				emptyStateStyle.Render("No commits")))
	}

	lo, hi := v.selectionRange()
	end := v.offset + h
	if end > len(v.rows) {
		end = len(v.rows)
	}

	var sb strings.Builder
	for i := v.offset; i < end; i++ {
		row := v.rows[i]
		if row.separator {
			sep := strings.Repeat("─", w)
			sb.WriteString(diffSeparatorStyle.Render(sep))
		} else {
			label := renderCommitLabel(row.commit)
			label = truncateLine(label, w)
			if i == v.startCommit {
				sb.WriteString(diffSelectedStyle.Width(w).Render(label))
			} else if i >= lo && i <= hi {
				sb.WriteString(diffRangeStyle.Width(w).Render(label))
			} else {
				sb.WriteString(label)
			}
		}
		if i < end-1 {
			sb.WriteString("\n")
		}
	}

	return viewPaneStyle.Width(w).Height(h).Render(clipLines(sb.String(), h))
}

// renderRightPane renders the git show --stat preview pane.
func (v DiffView) renderRightPane() string {
	w := v.rightPaneWidth() - viewBorderOverhead
	if w < 1 {
		w = 1
	}
	h := v.commitListHeight()

	content := v.statOutput
	if content == "" {
		content = emptyStateStyle.Render("(no preview)")
	}

	// Truncate lines to fit the pane width.
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = truncateLine(line, w)
	}
	content = strings.Join(lines, "\n")

	return viewPaneStyle.Width(w).Height(h).Render(clipLines(content, h))
}

// ── KeyBindings ──────────────────────────────────────────────────────────────

func (v DiffView) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "↑/↓", Description: "Navigate commits"},
		{Key: "PgUp/PgDn", Description: "Page navigate"},
		{Key: "Shift+↑/↓", Description: "Extend selection range"},
		{Key: "Tab/Enter", Description: "View diff"},
	}
}
