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

// selectionEnd identifies which end of the commit range selection moved most
// recently, so the stat preview can display the relevant commit.
type selectionEnd int

const (
	movedCursor selectionEnd = iota
	movedAnchor
)

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
// DiffView.Update handles this by kicking off an async diff fetch; the result
// arrives as diffContentMsg which activates the viewer screen.
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

// DiffView implements the Diffs view (F5). It has two sub-screens:
// a commit selector (two-pane layout) and a diff viewer (scrollable diff).
// The viewerActive flag controls which sub-screen is shown.
type DiffView struct {
	width  int
	height int

	// Ticket context
	identifier   string
	phase        string
	worktreePath string // resolved once from identifier; empty until set

	// Commit data
	rows []commitRow

	// Selection state: cursor is the actively-moving end of the selection (tracked
	// by clampScroll and highlighted by renderCommitRow). anchor is the fixed end
	// of a range selection. For single selection, they are equal.
	cursor int
	anchor int
	offset int // first visible row in the commit list

	// lastMoved tracks which end of the selection moved most recently.
	// Used by currentCommit() to display the stat for the end the user just moved.
	lastMoved selectionEnd

	// Right pane: cached git show --stat output
	statOutput string
	statHash   string // hash for which statOutput was fetched

	// errorMsg holds a brief error message displayed in the view.
	errorMsg string

	// ── Viewer screen state ──────────────────────────────────────────────
	viewerActive     bool   // true when showing the diff viewer
	viewerText       string // pre-rendered diff content
	viewerFileStarts []int  // line offset where each file begins
	viewerFileNames  []string
	viewerOffset     int // first visible line in the viewer pane
}

// NewDiffView creates an empty DiffView.
func NewDiffView() DiffView {
	return DiffView{}
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
	lo, hi := v.anchor, v.cursor
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo, hi
}

// currentCommit returns the commit at whichever end of the selection moved
// most recently, used for stat display. Falls back to cursor if unset.
func (v DiffView) currentCommit() *commitEntry {
	idx := v.cursor
	if v.lastMoved == movedAnchor {
		idx = v.anchor
	}
	if idx < 0 || idx >= len(v.rows) || v.rows[idx].separator {
		return nil
	}
	return &v.rows[idx].commit
}

// ── Navigation ───────────────────────────────────────────────────────────────

// nextSelectableRow returns the next non-separator row index from `from` in the
// given direction (+1 for down, -1 for up). Returns -1 if no selectable row exists.
func (v *DiffView) nextSelectableRow(from, direction int) int {
	next := from + direction
	for next >= 0 && next < len(v.rows) && v.rows[next].separator {
		next += direction
	}
	if next < 0 || next >= len(v.rows) {
		return -1
	}
	return next
}

// advanceCursor moves a cursor index n steps in the given direction, skipping separators.
func (v *DiffView) advanceCursor(cursor int, n, direction int) int {
	for i := 0; i < n; i++ {
		next := v.nextSelectableRow(cursor, direction)
		if next == -1 {
			break
		}
		cursor = next
	}
	return cursor
}

// moveDown moves the single selection down by n steps, skipping separators.
// Both cursor and anchor move together.
func (v *DiffView) moveDown(n int) {
	if len(v.rows) == 0 {
		return
	}
	v.cursor = v.advanceCursor(v.cursor, n, 1)
	v.anchor = v.cursor
	v.lastMoved = movedCursor
	v.clampScroll()
}

// moveUp moves the single selection up by n steps, skipping separators.
// Both cursor and anchor move together.
func (v *DiffView) moveUp(n int) {
	if len(v.rows) == 0 {
		return
	}
	v.cursor = v.advanceCursor(v.cursor, n, -1)
	v.anchor = v.cursor
	v.lastMoved = movedCursor
	v.clampScroll()
}

// extendRangeDown moves cursor downward (older) while leaving anchor fixed.
func (v *DiffView) extendRangeDown(n int) {
	if len(v.rows) == 0 {
		return
	}
	v.cursor = v.advanceCursor(v.cursor, n, 1)
	v.lastMoved = movedCursor
	v.clampScroll()
}

// extendRangeUp moves anchor upward (newer) while leaving cursor fixed.
func (v *DiffView) extendRangeUp(n int) {
	if len(v.rows) == 0 {
		return
	}
	v.anchor = v.advanceCursor(v.anchor, n, -1)
	v.lastMoved = movedAnchor
	v.clampScroll()
}

// clampScroll ensures the cursor is visible in the list.
func (v *DiffView) clampScroll() {
	h := v.commitListHeight()
	if h <= 0 || len(v.rows) == 0 {
		v.offset = 0
		return
	}
	// The visible cursor position.
	if v.cursor < v.offset {
		v.offset = v.cursor
	}
	if v.cursor >= v.offset+h {
		v.offset = v.cursor - h + 1
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

// clampSelected ensures cursor and anchor are valid after a data refresh.
func (v *DiffView) clampSelected() {
	if len(v.rows) == 0 {
		v.cursor = 0
		v.anchor = 0
		return
	}
	if v.cursor >= len(v.rows) {
		v.cursor = len(v.rows) - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.anchor >= len(v.rows) {
		v.anchor = len(v.rows) - 1
	}
	if v.anchor < 0 {
		v.anchor = 0
	}
	// Skip separators.
	for v.cursor < len(v.rows) && v.rows[v.cursor].separator {
		v.cursor++
	}
	if v.cursor >= len(v.rows) {
		v.cursor = len(v.rows) - 1
		for v.cursor >= 0 && v.rows[v.cursor].separator {
			v.cursor--
		}
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
	v.anchor = v.cursor
}

// ── Commands ─────────────────────────────────────────────────────────────────

// fetchCommitsCmd fetches the commit list and fork point asynchronously.
func fetchCommitsCmd(worktreePath string) tea.Cmd {
	return func() tea.Msg {
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
func fetchShowStatCmd(worktreePath, hash string) tea.Cmd {
	return func() tea.Msg {
		if hash == uncommittedHash {
			out, err := gitOutput(worktreePath, "diff", "--stat")
			if err != nil {
				return diffShowStatMsg{hash: hash, output: "(no changes)"}
			}
			return diffShowStatMsg{hash: hash, output: out}
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
		if v.viewerActive {
			v.viewerClampScroll()
		}
		return v, nil

	case setDiffTicketMsg:
		v.identifier = msg.identifier
		v.phase = msg.phase
		wp, err := storage.WorktreePathForIdentifier(v.identifier)
		if err != nil {
			v.errorMsg = fmt.Sprintf("worktree error: %s", err)
			return v, nil
		}
		v.errorMsg = ""
		v.worktreePath = wp
		return v, fetchCommitsCmd(v.worktreePath)

	case diffCommitListMsg:
		v.rows = buildCommitRows(msg.commits, msg.forkPointIdx, msg.hasUncommit)
		v.clampSelected()
		v.clampScroll()
		// Fetch stat for the initial selection.
		return v, v.fetchStatForCurrent()

	case diffShowStatMsg:
		v.statHash = msg.hash
		v.statOutput = msg.output
		return v, nil

	case switchToDiffViewerMsg:
		// Kick off an async diff fetch for the selected range.
		if v.worktreePath == "" {
			return v, nil
		}
		return v, fetchDiffCmd(v.worktreePath, msg.startCommit, msg.endCommit)

	case diffContentMsg:
		v.enterViewerMode(msg.files, v.width)
		return v, nil

	case tea.KeyMsg:
		if v.viewerActive {
			return v.handleViewerKey(msg)
		}
		return v.handleKey(msg)
	}

	return v, nil
}

func (v DiffView) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	prevCursor := v.cursor
	prevAnchor := v.anchor

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

	// If either end of the selection changed, refresh the stat.
	if v.cursor != prevCursor || v.anchor != prevAnchor {
		return v, v.fetchStatForCurrent()
	}
	return v, nil
}

// fetchStatForCurrent returns a command to fetch the stat for the current commit.
func (v DiffView) fetchStatForCurrent() tea.Cmd {
	c := v.currentCommit()
	if c == nil || v.worktreePath == "" {
		return nil
	}
	if c.Hash == v.statHash {
		return nil // already cached
	}
	return fetchShowStatCmd(v.worktreePath, c.Hash)
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
	if v.viewerActive {
		return v.viewerView()
	}

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

// diffErrorStyle renders error messages in the diff view status bar.
var diffErrorStyle = lipgloss.NewStyle().Foreground(colourDanger)

// renderStatusBar renders the status bar with ticket info and selection count.
func (v DiffView) renderStatusBar() string {
	left := fmt.Sprintf("Ticket: %s (%s)", v.identifier, v.phase)

	if v.errorMsg != "" {
		errText := diffErrorStyle.Render(v.errorMsg)
		spacer := v.width - lipgloss.Width(left) - lipgloss.Width(errText)
		if spacer < 2 {
			spacer = 2
		}
		return left + strings.Repeat(" ", spacer) + errText
	}

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
		sb.WriteString(v.renderCommitRow(i, w, lo, hi))
		if i < end-1 {
			sb.WriteString("\n")
		}
	}

	return viewPaneStyle.Width(w).Height(h).Render(clipLines(sb.String(), h))
}

// renderCommitRow renders a single row in the commit list with appropriate styling.
func (v DiffView) renderCommitRow(i, w, lo, hi int) string {
	row := v.rows[i]
	if row.separator {
		return diffSeparatorStyle.Render(strings.Repeat("─", w))
	}
	label := truncateLine(renderCommitLabel(row.commit), w)
	if i == v.cursor {
		return diffSelectedStyle.Width(w).Render(label)
	}
	if i >= lo && i <= hi {
		return diffRangeStyle.Width(w).Render(label)
	}
	return label
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
	if v.viewerActive {
		return viewerKeyBindings()
	}
	return []KeyBinding{
		{Key: "↑/↓", Description: "Navigate commits"},
		{Key: "PgUp/PgDn", Description: "Page navigate"},
		{Key: "Shift+↑/↓", Description: "Extend selection range"},
		{Key: "Tab/Enter", Description: "View diff"},
	}
}

func (v DiffView) Label() string { return "F5:Diffs" }
