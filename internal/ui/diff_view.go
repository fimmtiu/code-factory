package ui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/git"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/storage"
	"github.com/fimmtiu/code-factory/internal/util"
)

// maxCommits is the maximum number of commits shown in the selector.
const maxCommits = 100

// statusBarHeight is the number of content lines in the selector status bar.
const statusBarHeight = 1

// statusBarBorderHeight is the top border line of the status bar, which
// replaces the old horizontal separator between the status bar and panes.
const statusBarBorderHeight = 1

// ── Data types ───────────────────────────────────────────────────────────────

// commitRow represents one row in the commit list. If separator is true, the
// row is a non-selectable divider above the fork-point commit (same pattern
// as the CommandView separator between status groups).
type commitRow struct {
	commit    git.CommitEntry
	separator bool
}

// ── Messages ─────────────────────────────────────────────────────────────────

// diffCommitListMsg carries the result of the async commit-list fetch.
type diffCommitListMsg struct {
	commits      []git.CommitEntry
	forkPointIdx int    // index into commits of the fork-point commit, or -1
	hasUncommit  bool   // true if there are uncommitted changes
	errMsg       string // non-empty when the commit list could not be fetched
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
	startCommit git.CommitEntry
	endCommit   git.CommitEntry
}

// ── DiffView ─────────────────────────────────────────────────────────────────

// DiffView implements the Diffs view (F5). It has two sub-screens:
// a commit selector (two-pane layout) and a diff viewer (scrollable diff).
// When viewer is non-nil, the viewer sub-screen is shown; otherwise, the
// commit selector is shown.
type DiffView struct {
	width  int
	height int

	// Ticket context
	identifier   string
	phase        string
	isProject    bool
	worktreePath string // resolved once from identifier; empty until set

	// Commit data
	rows []commitRow

	// Selection state: cursor is the actively-moving end of the selection (tracked
	// by clampScroll and highlighted by renderCommitRow). anchor is the fixed end
	// of a range selection. For single selection, they are equal.
	cursor int
	anchor int
	offset int // first visible row in the commit list

	// Right pane: cached git show --stat output
	statOutput string
	statHash   string // hash for which statOutput was fetched

	// errorMsg holds a brief error message displayed in the view.
	errorMsg string

	// viewer is non-nil when the diff viewer sub-screen is active.
	viewer          *DiffViewerModel
	viewerStartHash string // oldest commit hash in the viewed range
	viewerEndHash   string // newest commit hash in the viewed range
}

// NewDiffView creates an empty DiffView.
func NewDiffView() DiffView {
	return DiffView{}
}

// Init returns nil; the view loads data when a ticket is set.
func (v DiffView) Init() tea.Cmd {
	return nil
}

// resetForTicket prepares the DiffView to display a new ticket's commits.
// It resets selection state and resolves the worktree path. The caller should
// check errorMsg after calling; if empty, worktreePath is valid.
func (v *DiffView) resetForTicket(identifier, phase string, isProject bool, worktreePath string, err error) {
	v.identifier = identifier
	v.phase = phase
	v.isProject = isProject
	v.cursor = 0
	v.anchor = 0
	v.offset = 0
	v.rows = nil
	v.statOutput = ""
	v.statHash = ""
	v.viewer = nil
	if err != nil {
		v.errorMsg = fmt.Sprintf("worktree error: %s", err)
		v.worktreePath = ""
	} else {
		v.errorMsg = ""
		v.worktreePath = worktreePath
	}
}

// ── Row building ─────────────────────────────────────────────────────────────

// buildCommitRows converts a commit list into rows, inserting a separator
// above the fork-point commit and prepending an uncommitted-changes pseudo-commit.
func buildCommitRows(commits []git.CommitEntry, forkPointIdx int, hasUncommit bool) []commitRow {
	var rows []commitRow

	if hasUncommit {
		rows = append(rows, commitRow{
			commit: git.CommitEntry{Hash: git.UncommittedHash, Message: "Uncommitted changes"},
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

// commitHashStyle renders the short hash prefix in bold medium grey.
var commitHashStyle = lipgloss.NewStyle().Bold(true).Foreground(colourMuted)

// renderCommitLabel returns "<4-char hash> <message>" as plain text.
// Hash styling is applied later by renderCommitRow for unselected rows.
func renderCommitLabel(c git.CommitEntry) string {
	h := c.Hash
	if h != git.UncommittedHash {
		if len(h) > 4 {
			h = h[:4]
		}
	}
	return h + " " + c.Message
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
	h := v.height - chromeHeight - statusBarHeight - statusBarBorderHeight - viewBorderOverhead
	if h < 1 {
		h = 1
	}
	return h
}

// viewerPaneHeight returns the content-pane height available to the viewer,
// after accounting for the app chrome, viewer status bar, and separator.
func (v DiffView) viewerPaneHeight() int {
	h := v.height - chromeHeight - viewerStatusBarHeight - statusBarBorderHeight - viewBorderOverhead
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

// currentCommit returns the commit at the cursor position, used for stat display.
func (v DiffView) currentCommit() *git.CommitEntry {
	if v.cursor < 0 || v.cursor >= len(v.rows) || v.rows[v.cursor].separator {
		return nil
	}
	return &v.rows[v.cursor].commit
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
	v.clampScroll()
}

// extendRangeDown moves cursor downward (older) while leaving anchor fixed.
func (v *DiffView) extendRangeDown(n int) {
	if len(v.rows) == 0 {
		return
	}
	v.cursor = v.advanceCursor(v.cursor, n, 1)
	v.clampScroll()
}

// extendRangeUp moves cursor upward (newer) while leaving anchor fixed.
func (v *DiffView) extendRangeUp(n int) {
	if len(v.rows) == 0 {
		return
	}
	v.cursor = v.advanceCursor(v.cursor, n, -1)
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

// parentBranch returns the branch name to compare against for fork-point
// detection. For nested tickets (e.g. "proj/ticket"), this is the parent
// project's branch (e.g. "proj"). For top-level tickets, it falls back to
// the repository's default branch (main/master).
func parentBranch(identifier, worktreePath string) string {
	if parent, ok := models.ParentIdentifierOf(identifier); ok {
		return strings.ReplaceAll(parent, "/", "_")
	}
	return git.DetectDefaultBranch(worktreePath)
}

// fetchCommitsCmd fetches the commit list and fork point asynchronously.
func fetchCommitsCmd(worktreePath, identifier string) tea.Cmd {
	return func() tea.Msg {
		commits, err := git.FetchCommitList(worktreePath, maxCommits)
		if err != nil {
			return diffCommitListMsg{forkPointIdx: -1, errMsg: err.Error()}
		}

		baseBranch := parentBranch(identifier, worktreePath)
		forkPointIdx := matchForkPoint(commits, worktreePath, baseBranch)

		hasUncommit, _ := git.HasUncommittedChanges(worktreePath)

		return diffCommitListMsg{
			commits:      commits,
			forkPointIdx: forkPointIdx,
			hasUncommit:  hasUncommit,
		}
	}
}

// matchForkPoint finds the index in commits that matches the fork point of
// the given branch, or -1 if not found.
func matchForkPoint(commits []git.CommitEntry, worktreePath, defaultBranch string) int {
	forkHash, err := git.FetchForkPoint(worktreePath, defaultBranch)
	if err != nil || forkHash == "" {
		return -1
	}
	for i, c := range commits {
		if c.Hash == forkHash {
			return i
		}
	}
	return -1
}

// fetchShowStatCmd fetches `git show --stat` output for a commit.
func fetchShowStatCmd(worktreePath, hash string) tea.Cmd {
	return func() tea.Msg {
		out, err := git.FetchShowStat(worktreePath, hash)
		if err != nil {
			if hash == git.UncommittedHash {
				return diffShowStatMsg{hash: hash, output: "(no changes)"}
			}
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
		if v.viewer != nil {
			v.viewer.setSize(v.width, v.viewerPaneHeight())
		}
		return v, nil

	case openDiffViewMsg:
		wp, err := storage.WorktreePathForIdentifier(msg.identifier)
		v.resetForTicket(msg.identifier, msg.phase, msg.isProject, wp, err)
		if err != nil {
			return v, nil
		}
		return v, fetchCommitsCmd(wp, msg.identifier)

	case diffCommitListMsg:
		if msg.errMsg != "" {
			v.errorMsg = fmt.Sprintf("git error: %s", msg.errMsg)
			return v, nil
		}
		v.errorMsg = ""
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
		v.viewerStartHash = msg.startCommit.Hash
		v.viewerEndHash = msg.endCommit.Hash
		return v, fetchDiffCmd(v.worktreePath, msg.startCommit, msg.endCommit)

	case diffContentMsg:
		v.viewer = newDiffViewerModel(msg.files, v.width, v.viewerPaneHeight())
		return v, nil

	case tea.KeyMsg:
		if v.viewer != nil {
			if isViewerExitKey(msg) {
				v.viewer = nil
				return v, nil
			}
			v.viewer.Update(msg)
			return v, nil
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
	case "t", "T":
		return v.openTerminal()
	case "e", "E":
		return v.openEditorNonblocking()
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

func (v DiffView) openTerminal() (tea.Model, tea.Cmd) {
	if v.worktreePath == "" {
		return v, nil
	}
	_ = util.OpenTerminal(v.worktreePath)
	return v, nil
}

func (v DiffView) openEditorNonblocking() (tea.Model, tea.Cmd) {
	if v.worktreePath == "" {
		return v, nil
	}
	_ = exec.Command(util.NonblockingEditorCommand(), v.worktreePath).Start()
	return v, nil
}

func (v DiffView) switchToDiffViewer() (tea.Model, tea.Cmd) {
	lo, hi := v.selectionRange()
	if lo < 0 || hi >= len(v.rows) {
		return v, nil
	}
	// Find the actual commit entries at the range boundaries.
	var startC, endC git.CommitEntry
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
	// Guard: if no non-separator commits were found, do nothing.
	if startC.Hash == "" || endC.Hash == "" {
		return v, nil
	}
	return v, func() tea.Msg {
		return switchToDiffViewerMsg{startCommit: startC, endCommit: endC}
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (v DiffView) View() string {
	if v.viewer != nil {
		innerW := v.width - viewBorderOverhead
		statusBar := renderViewerStatusBar(innerW, v.identifier, v.phase, v.isProject, v.viewerStartHash, v.viewerEndHash, v.viewer)
		styledBar := diffStatusBarStyle.Width(innerW).Render(statusBar)
		pane := connectPaneTop(v.viewer.renderPane(), true, true)
		return lipgloss.JoinVertical(lipgloss.Left, styledBar, pane)
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

	innerW := v.width - viewBorderOverhead
	statusBar := v.renderStatusBar(innerW)
	styledBar := diffStatusBarStyle.Width(innerW).Render(statusBar)
	leftPane := connectPaneTop(v.renderLeftPane(), true, false)
	rightPane := connectPaneTop(v.renderRightPane(), false, true)
	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	return lipgloss.JoinVertical(lipgloss.Left, styledBar, panes)
}

// connectPaneTop replaces the top-left and/or top-right rounded corners of a
// bordered pane with T-junctions so the pane visually connects to the status
// bar border directly above it.
func connectPaneTop(rendered string, left, right bool) string {
	lines := strings.SplitN(rendered, "\n", 2)
	if len(lines) == 0 {
		return rendered
	}
	if left {
		lines[0] = strings.Replace(lines[0], "╭", "├", 1)
	}
	if right {
		if idx := strings.LastIndex(lines[0], "╮"); idx >= 0 {
			lines[0] = lines[0][:idx] + "┤" + lines[0][idx+len("╮"):]
		}
	}
	return strings.Join(lines, "\n")
}

// diffLabelBold is the style for the "Ticket: <id>" / "Project: <id>" label.
var diffLabelBold = lipgloss.NewStyle().Bold(true)

// renderDiffLabel returns the styled left-hand label for Diffs status bars,
// e.g. "**Ticket: proj/ticket** (implement)".
func renderDiffLabel(identifier, phase string, isProject bool) string {
	kind := "Ticket"
	if isProject {
		kind = "Project"
	}
	boldPart := diffLabelBold.Render(fmt.Sprintf("%s: %s", kind, identifier))
	return fmt.Sprintf("%s (%s)", boldPart, phase)
}

// diffErrorStyle renders error messages in the diff view status bar.
var diffErrorStyle = lipgloss.NewStyle().Foreground(colourDanger)

// renderStatusBar renders the status bar with ticket info and selection count.
// width is the inner content width (excluding border overhead).
func (v DiffView) renderStatusBar(width int) string {
	left := renderDiffLabel(v.identifier, v.phase, v.isProject)

	if v.errorMsg != "" {
		errText := diffErrorStyle.Render(v.errorMsg)
		spacer := width - lipgloss.Width(left) - lipgloss.Width(errText)
		if spacer < 2 {
			spacer = 2
		}
		return left + strings.Repeat(" ", spacer) + errText
	}

	n := v.selectedCount()
	noun := "commits"
	if n == 1 {
		noun = "commit"
	}
	right := fmt.Sprintf("%d %s selected", n, noun)

	spacer := width - lipgloss.Width(left) - lipgloss.Width(right)
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
	// Style the hash prefix for unselected rows.
	h := row.commit.Hash
	if h != git.UncommittedHash && len(h) > 4 {
		h = h[:4]
	}
	return commitHashStyle.Render(h) + label[len(h):]
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

	// Strip leading spaces (git --stat indents each line) and truncate
	// to fit the pane width. Trimming avoids a first-line indent mismatch
	// caused by lipgloss stripping leading whitespace from the first line.
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = truncateLine(strings.TrimLeft(line, " "), w)
	}
	content = strings.Join(lines, "\n")

	return viewPaneStyle.Width(w).Height(h).Render(clipLines(content, h))
}

// ── KeyBindings ──────────────────────────────────────────────────────────────

func (v DiffView) KeyBindings() []KeyBinding {
	if v.viewer != nil {
		return v.viewer.KeyBindings()
	}
	return []KeyBinding{
		{Key: "↑/↓", Description: "Navigate commits"},
		{Key: "PgUp/PgDn", Description: "Page navigate"},
		{Key: "Shift+↑/↓", Description: "Extend selection range"},
		{Key: "T", Description: "Open terminal in worktree"},
		{Key: "E", Description: "Open worktree in Cursor"},
		{Key: "Tab/Enter", Description: "View diff"},
	}
}

// HintPairs returns alternating key/description pairs for the footer hint bar.
// The root model calls this instead of inspecting DiffView internals.
func (v DiffView) HintPairs() []string {
	if v.viewer != nil {
		return []string{"↑/↓", "scroll", "PgUp/Dn", "page", "C", "collapse/expand", "Tab/Esc/Enter", "back"}
	}
	return []string{"↑/↓", "navigate", "PgUp/Dn", "page", "Shift+↑/↓", "extend range", "T", "open terminal", "E", "open editor", "Tab", "view diff"}
}

func (v DiffView) Label() string { return "F5:Diffs" }
