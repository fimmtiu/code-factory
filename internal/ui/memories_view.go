package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// ── Focus ─────────────────────────────────────────────────────────────────────

type memoryFocus int

const (
	memFocusList memoryFocus = iota
	memFocusDetail
)

// ── Messages ─────────────────────────────────────────────────────────────────

// memoriesRefreshMsg carries the result of a ListMemories fetch.
type memoriesRefreshMsg struct {
	memories []db.Memory
}

// openDeleteMemoryDialogMsg asks the root model to open the delete-confirmation
// dialog for the given memory.
type openDeleteMemoryDialogMsg struct {
	id    int64
	label string
}

// memoryDeletedMsg reports the outcome of a DeleteMemory attempt.
type memoryDeletedMsg struct {
	id  int64
	err error
}

// ── MemoriesView ───────────────────────────────────────────────────────────────

// MemoriesView is the two-pane memory browser. The left pane lists memories
// (id and kind) newest-first; the right pane shows the selected memory's full
// contents. Left/right switches focus between the panes and up/down scrolls the
// focused pane.
type MemoriesView struct {
	database *db.DB

	width  int
	height int

	memories []db.Memory

	selected     int // index into memories of the highlighted row
	listOffset   int // first visible row in the list pane
	detailOffset int // first visible line in the detail pane

	focus memoryFocus
}

// NewMemoriesView creates a new MemoriesView.
func NewMemoriesView(database *db.DB) MemoriesView {
	return MemoriesView{database: database}
}

// Init loads the memories from the database.
func (v MemoriesView) Init() tea.Cmd {
	return v.fetchCmd()
}

// fetchCmd loads all memories and returns them as a memoriesRefreshMsg.
func (v MemoriesView) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		mems, err := v.database.ListMemories()
		if err != nil {
			mems = nil
		}
		return memoriesRefreshMsg{memories: mems}
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func (v MemoriesView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.clampScroll()
		return v, nil

	case memoriesRefreshMsg:
		v.memories = msg.memories
		if v.selected >= len(v.memories) {
			v.selected = max(0, len(v.memories)-1)
		}
		v.detailOffset = 0
		v.clampScroll()
		return v, nil

	case memoryDeletedMsg:
		// Reload so the deleted row disappears and the selection re-clamps.
		return v, v.fetchCmd()

	case tea.KeyMsg:
		if len(v.memories) == 0 {
			return v, nil
		}
		switch msg.String() {
		case "left":
			v.focus = memFocusList
		case "right":
			v.focus = memFocusDetail
		case "x", "X":
			return v.openDeleteDialog()
		case "up":
			if v.focus == memFocusList {
				v.selectUp(1)
			} else {
				v.scrollDetailUp(1)
			}
		case "down":
			if v.focus == memFocusList {
				v.selectDown(1)
			} else {
				v.scrollDetailDown(1)
			}
		case "pgup":
			if v.focus == memFocusList {
				v.selectUp(v.paneHeight())
			} else {
				v.scrollDetailUp(v.paneHeight())
			}
		case "pgdown":
			if v.focus == memFocusList {
				v.selectDown(v.paneHeight())
			} else {
				v.scrollDetailDown(v.paneHeight())
			}
		}
		return v, nil
	}

	return v, nil
}

// openDeleteDialog asks the root model to confirm deletion of the selected memory.
func (v MemoriesView) openDeleteDialog() (tea.Model, tea.Cmd) {
	if v.selected >= len(v.memories) {
		return v, nil
	}
	m := v.memories[v.selected]
	label := fmt.Sprintf("#%d (%s)", m.ID, m.Kind)
	id := m.ID
	return v, func() tea.Msg {
		return openDeleteMemoryDialogMsg{id: id, label: label}
	}
}

// ── Navigation ──────────────────────────────────────────────────────────────

func (v *MemoriesView) selectUp(n int) {
	v.selected -= n
	if v.selected < 0 {
		v.selected = 0
	}
	v.detailOffset = 0
	v.clampScroll()
}

func (v *MemoriesView) selectDown(n int) {
	v.selected += n
	if v.selected >= len(v.memories) {
		v.selected = max(0, len(v.memories)-1)
	}
	v.detailOffset = 0
	v.clampScroll()
}

func (v *MemoriesView) scrollDetailUp(n int) {
	v.detailOffset -= n
	if v.detailOffset < 0 {
		v.detailOffset = 0
	}
}

func (v *MemoriesView) scrollDetailDown(n int) {
	v.detailOffset += n
	v.clampDetailScroll()
}

// clampScroll keeps the selected row visible within the list pane.
func (v *MemoriesView) clampScroll() {
	h := v.paneHeight()
	if h <= 0 {
		return
	}
	if v.selected < v.listOffset {
		v.listOffset = v.selected
	}
	if v.selected >= v.listOffset+h {
		v.listOffset = v.selected - h + 1
	}
	maxOffset := len(v.memories) - h
	if maxOffset < 0 {
		maxOffset = 0
	}
	if v.listOffset > maxOffset {
		v.listOffset = maxOffset
	}
	if v.listOffset < 0 {
		v.listOffset = 0
	}
}

// clampDetailScroll keeps the detail offset within the content bounds.
func (v *MemoriesView) clampDetailScroll() {
	lines := v.detailLines(v.detailInnerWidth())
	h := v.paneHeight()
	maxOffset := len(lines) - h
	if maxOffset < 0 {
		maxOffset = 0
	}
	if v.detailOffset > maxOffset {
		v.detailOffset = maxOffset
	}
	if v.detailOffset < 0 {
		v.detailOffset = 0
	}
}

// ── Dimensions ────────────────────────────────────────────────────────────────

// bodyHeight returns the height available for the panes (excluding app chrome).
func (v MemoriesView) bodyHeight() int {
	h := v.height - chromeHeight
	if h < 3 {
		h = 3
	}
	return h
}

// paneHeight returns the number of visible content rows inside a pane.
func (v MemoriesView) paneHeight() int {
	h := v.bodyHeight() - viewBorderOverhead
	if h < 1 {
		h = 1
	}
	return h
}

// listPaneWidth returns the outer width of the list pane (~1/3 of the terminal).
func (v MemoriesView) listPaneWidth() int {
	w := v.width / 3
	if w < 16 {
		w = 16
	}
	return w
}

// detailPaneWidth returns the outer width of the detail pane (the remainder).
func (v MemoriesView) detailPaneWidth() int {
	w := v.width - v.listPaneWidth()
	if w < 10 {
		w = 10
	}
	return w
}

func (v MemoriesView) listInnerWidth() int {
	w := v.listPaneWidth() - viewBorderOverhead
	if w < 1 {
		w = 1
	}
	return w
}

func (v MemoriesView) detailInnerWidth() int {
	w := v.detailPaneWidth() - viewBorderOverhead
	if w < 1 {
		w = 1
	}
	return w
}

// ── View ──────────────────────────────────────────────────────────────────────

func (v MemoriesView) View() string {
	innerH := v.paneHeight()

	listBorder := theme.Current().UnfocusedBorderStyle
	listRightChar := "│"
	if v.focus == memFocusList {
		listBorder = theme.Current().FocusedBorderStyle
		listRightChar = "║"
	}
	detailBorder := theme.Current().UnfocusedBorderStyle
	detailRightChar := "│"
	if v.focus == memFocusDetail {
		detailBorder = theme.Current().FocusedBorderStyle
		detailRightChar = "║"
	}

	listInnerW := v.listInnerWidth()
	detailInnerW := v.detailInnerWidth()

	leftPane := listBorder.
		Width(listInnerW).
		Height(innerH).
		Render(v.renderListContent(listInnerW, innerH))
	leftPane = injectScrollbar(leftPane, listRightChar, "█", v.listOffset, len(v.memories), innerH)

	detailLines := v.detailLines(detailInnerW)
	rightPane := detailBorder.
		Width(detailInnerW).
		Height(innerH).
		Render(v.renderDetailContent(detailLines, detailInnerW, innerH))
	rightPane = injectScrollbar(rightPane, detailRightChar, "█", v.detailOffset, len(detailLines), innerH)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

// renderListContent renders the visible slice of the memory list. When there
// are no memories the pane is left blank.
func (v MemoriesView) renderListContent(w, h int) string {
	if len(v.memories) == 0 {
		return ""
	}

	var sb strings.Builder
	end := v.listOffset + h
	if end > len(v.memories) {
		end = len(v.memories)
	}
	for i := v.listOffset; i < end; i++ {
		m := v.memories[i]
		label := truncateLine(fmt.Sprintf("#%d  %s", m.ID, m.Kind), w)
		if i == v.selected {
			pad := w - lipgloss.Width(label)
			if pad < 0 {
				pad = 0
			}
			sb.WriteString(theme.Current().TreeSelectedStyle.Render(label + strings.Repeat(" ", pad)))
		} else {
			sb.WriteString(label)
		}
		if i < end-1 {
			sb.WriteString("\n")
		}
	}
	return clipLines(sb.String(), h)
}

// renderDetailContent renders the visible slice of the detail pane. When there
// are no memories it shows "No memories" centred in the empty-state style.
func (v MemoriesView) renderDetailContent(lines []string, w, h int) string {
	if len(v.memories) == 0 {
		empty := theme.Current().EmptyStateStyle.Render("No memories")
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, empty)
	}

	start := v.detailOffset
	if start >= len(lines) {
		start = max(0, len(lines)-1)
	}
	end := start + h
	if end > len(lines) {
		end = len(lines)
	}
	return clipLines(strings.Join(lines[start:end], "\n"), h)
}

// detailLines builds the full detail content for the selected memory: a header
// (kind, scope, source ticket), a blank separator line, then the wrapped text.
func (v MemoriesView) detailLines(w int) []string {
	if len(v.memories) == 0 || v.selected >= len(v.memories) {
		return nil
	}
	m := v.memories[v.selected]
	label := theme.Current().DetailLabelStyle

	var lines []string
	lines = append(lines, label.Render("Kind:")+"   "+m.Kind)
	lines = append(lines, label.Render("Scope:")+"  "+memoryScopeDisplay(m.Scope))
	lines = append(lines, label.Render("Source:")+" "+memorySourceDisplay(m.SourceTicket))
	lines = append(lines, "")
	lines = append(lines, wordWrap(m.Text, w)...)
	return lines
}

// memoryScopeDisplay renders a memory scope for display, labelling the empty
// (repository-global) scope explicitly.
func memoryScopeDisplay(scope string) string {
	if scope == "" {
		return "(repository-global)"
	}
	return scope
}

// memorySourceDisplay renders a memory's source ticket, showing a placeholder
// when none was recorded.
func memorySourceDisplay(source string) string {
	if source == "" {
		return "(none)"
	}
	return source
}

// ── KeyBindings ───────────────────────────────────────────────────────────────

func (v MemoriesView) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "↑/↓", Description: "Navigate list / scroll detail"},
		{Key: "←/→", Description: "Switch focus between list and detail pane"},
		{Key: "PgUp/PgDn", Description: "Page navigate / scroll"},
		{Key: "X", Description: "Delete selected memory"},
	}
}

func (v MemoriesView) Label() string { return "F6:Memories" }
