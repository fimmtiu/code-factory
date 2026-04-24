package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TextArea is a reusable multi-line text input widget with word wrapping,
// cursor navigation, and standard editing operations.
type TextArea struct {
	lines   [][]rune // logical lines (Enter creates new lines)
	row     int      // cursor logical line index
	col     int      // cursor column within logical line
	width   int      // character width for wrapping/display
	height  int      // visible height in visual lines
	offset  int      // scroll offset in visual lines
	focused bool     // when false, View() omits the cursor block
}

// textSegment represents one visual line within a word-wrapped logical line.
type textSegment struct {
	startCol int // inclusive start index into the logical line's runes
	endCol   int // exclusive end index
}

// wrappedLine is a visual line with its logical line origin.
type wrappedLine struct {
	logRow   int
	startCol int
	endCol   int
}

// NewTextArea creates a new empty TextArea with the given dimensions.
func NewTextArea(width, height int) TextArea {
	return TextArea{
		lines:  [][]rune{{}},
		width:  width,
		height: height,
	}
}

// Value returns the text content as a string.
func (t TextArea) Value() string {
	var sb strings.Builder
	for i, line := range t.lines {
		if i > 0 {
			sb.WriteRune('\n')
		}
		sb.WriteString(string(line))
	}
	return sb.String()
}

// SetFocused controls whether View() renders the cursor block. Callers should
// mirror their own focus state here so a blurred TextArea shows no cursor.
func (t *TextArea) SetFocused(f bool) { t.focused = f }

// SetValue replaces the text area content with the given string.
// The cursor is placed at the end of the content.
func (t *TextArea) SetValue(s string) {
	if s == "" {
		t.lines = [][]rune{{}}
		t.row = 0
		t.col = 0
		t.offset = 0
		return
	}
	parts := strings.Split(s, "\n")
	t.lines = make([][]rune, len(parts))
	for i, p := range parts {
		t.lines[i] = []rune(p)
	}
	t.row = len(t.lines) - 1
	t.col = len(t.lines[t.row])
	t.scrollToCursor()
}

// SetSize updates the dimensions of the text area.
func (t *TextArea) SetSize(width, height int) {
	t.width = width
	t.height = height
	t.scrollToCursor()
}

// Update handles a key message and updates the text area state.
func (t *TextArea) Update(msg tea.KeyMsg) {
	switch msg.String() {
	case "left":
		t.cursorLeft()
	case "right":
		t.cursorRight()
	case "up":
		t.cursorUp()
	case "down":
		t.cursorDown()
	case "home", "ctrl+a":
		t.col = 0
	case "end", "ctrl+e":
		t.col = len(t.lines[t.row])
	case "enter":
		t.insertNewline()
	case "backspace":
		t.backspace()
	case "delete":
		t.deleteChar()
	default:
		if len(msg.Runes) > 0 {
			t.insertRunes(msg.Runes)
		}
	}
	t.scrollToCursor()
}

// cursorStyle renders the character under the cursor with reverse video.
var cursorStyle = lipgloss.NewStyle().Reverse(true)

// View renders the text area content with a reverse-video cursor on the
// character at the cursor position (or a trailing space when at end-of-line).
// Each visual line is padded to t.width so that the caller can render the
// result without applying its own Width(), avoiding double-wrapping.
func (t TextArea) View() string {
	wrapped := t.allWrappedLines()
	cursorVisRow, cursorVisCol := t.cursorVisualPos(wrapped)

	end := t.offset + t.height
	if end > len(wrapped) {
		end = len(wrapped)
	}

	blank := strings.Repeat(" ", t.width)

	var sb strings.Builder
	for i := t.offset; i < t.offset+t.height; i++ {
		if i > t.offset {
			sb.WriteRune('\n')
		}
		if i >= end {
			sb.WriteString(blank)
			continue
		}
		wl := wrapped[i]
		text := string(t.lines[wl.logRow][wl.startCol:wl.endCol])
		visLen := wl.endCol - wl.startCol

		if t.focused && i == cursorVisRow {
			runes := []rune(text)
			if cursorVisCol < len(runes) {
				sb.WriteString(string(runes[:cursorVisCol]))
				sb.WriteString(cursorStyle.Render(string(runes[cursorVisCol])))
				sb.WriteString(string(runes[cursorVisCol+1:]))
			} else {
				sb.WriteString(text)
				sb.WriteString(cursorStyle.Render(" "))
				visLen++
			}
		} else {
			sb.WriteString(text)
		}

		if pad := t.width - visLen; pad > 0 {
			sb.WriteString(strings.Repeat(" ", pad))
		}
	}

	return sb.String()
}

// ── Word wrapping ───────────────────────────────────────────────────────────

// wrapLogicalLine splits a logical line into visual-line segments at word
// boundaries. If a word exceeds the width, it is force-broken.
func wrapLogicalLine(line []rune, width int) []textSegment {
	if width <= 0 {
		width = 1
	}
	if len(line) == 0 {
		return []textSegment{{startCol: 0, endCol: 0}}
	}

	var segs []textSegment
	pos := 0
	for pos < len(line) {
		if pos+width >= len(line) {
			segs = append(segs, textSegment{startCol: pos, endCol: len(line)})
			break
		}

		// Find the last space within [pos, pos+width) to break at.
		breakAt := -1
		for i := pos + width - 1; i > pos; i-- {
			if line[i] == ' ' {
				breakAt = i + 1 // include the space in this segment
				break
			}
		}
		if breakAt <= pos {
			breakAt = pos + width // forced break mid-word
		}

		segs = append(segs, textSegment{startCol: pos, endCol: breakAt})
		pos = breakAt
	}

	if len(segs) == 0 {
		segs = []textSegment{{startCol: 0, endCol: 0}}
	}
	return segs
}

// allWrappedLines computes all visual lines across all logical lines.
func (t TextArea) allWrappedLines() []wrappedLine {
	var result []wrappedLine
	for i, line := range t.lines {
		segs := wrapLogicalLine(line, t.width)
		for _, seg := range segs {
			result = append(result, wrappedLine{
				logRow:   i,
				startCol: seg.startCol,
				endCol:   seg.endCol,
			})
		}
	}
	if len(result) == 0 {
		result = []wrappedLine{{logRow: 0, startCol: 0, endCol: 0}}
	}
	return result
}

// cursorVisualPos returns the visual row and column of the cursor.
func (t TextArea) cursorVisualPos(wrapped []wrappedLine) (int, int) {
	for i, wl := range wrapped {
		if wl.logRow != t.row {
			continue
		}
		isLastSeg := i+1 >= len(wrapped) || wrapped[i+1].logRow != t.row
		if t.col >= wl.startCol && (t.col < wl.endCol || (t.col == wl.endCol && isLastSeg)) {
			return i, t.col - wl.startCol
		}
	}
	return 0, 0
}

// setCursorFromVisual positions the cursor from a visual row and column.
func (t *TextArea) setCursorFromVisual(wrapped []wrappedLine, visRow, visCol int) {
	if visRow < 0 {
		visRow = 0
	}
	if visRow >= len(wrapped) {
		visRow = len(wrapped) - 1
	}
	wl := wrapped[visRow]
	t.row = wl.logRow
	t.col = wl.startCol + visCol
	segLen := wl.endCol - wl.startCol
	if visCol > segLen {
		t.col = wl.endCol
	}
	if t.col > len(t.lines[t.row]) {
		t.col = len(t.lines[t.row])
	}
}

// ── Cursor movement ─────────────────────────────────────────────────────────

func (t *TextArea) cursorLeft() {
	if t.col > 0 {
		t.col--
	} else if t.row > 0 {
		t.row--
		t.col = len(t.lines[t.row])
	}
}

func (t *TextArea) cursorRight() {
	if t.col < len(t.lines[t.row]) {
		t.col++
	} else if t.row < len(t.lines)-1 {
		t.row++
		t.col = 0
	}
}

func (t *TextArea) cursorUp() {
	wrapped := t.allWrappedLines()
	visRow, visCol := t.cursorVisualPos(wrapped)
	if visRow > 0 {
		t.setCursorFromVisual(wrapped, visRow-1, visCol)
	}
}

func (t *TextArea) cursorDown() {
	wrapped := t.allWrappedLines()
	visRow, visCol := t.cursorVisualPos(wrapped)
	if visRow < len(wrapped)-1 {
		t.setCursorFromVisual(wrapped, visRow+1, visCol)
	}
}

// ── Editing ─────────────────────────────────────────────────────────────────

func (t *TextArea) insertNewline() {
	line := t.lines[t.row]
	before := make([]rune, t.col)
	copy(before, line[:t.col])
	after := make([]rune, len(line)-t.col)
	copy(after, line[t.col:])

	newLines := make([][]rune, len(t.lines)+1)
	copy(newLines, t.lines[:t.row+1])
	newLines[t.row] = before
	newLines[t.row+1] = after
	copy(newLines[t.row+2:], t.lines[t.row+1:])
	t.lines = newLines

	t.row++
	t.col = 0
}

func (t *TextArea) backspace() {
	if t.col > 0 {
		line := t.lines[t.row]
		t.lines[t.row] = append(line[:t.col-1], line[t.col:]...)
		t.col--
	} else if t.row > 0 {
		prevLen := len(t.lines[t.row-1])
		t.lines[t.row-1] = append(t.lines[t.row-1], t.lines[t.row]...)
		t.lines = append(t.lines[:t.row], t.lines[t.row+1:]...)
		t.row--
		t.col = prevLen
	}
}

func (t *TextArea) deleteChar() {
	line := t.lines[t.row]
	if t.col < len(line) {
		t.lines[t.row] = append(line[:t.col], line[t.col+1:]...)
	} else if t.row < len(t.lines)-1 {
		t.lines[t.row] = append(t.lines[t.row], t.lines[t.row+1]...)
		t.lines = append(t.lines[:t.row+1], t.lines[t.row+2:]...)
	}
}

func (t *TextArea) insertRunes(runes []rune) {
	line := t.lines[t.row]
	newLine := make([]rune, len(line)+len(runes))
	copy(newLine, line[:t.col])
	copy(newLine[t.col:], runes)
	copy(newLine[t.col+len(runes):], line[t.col:])
	t.lines[t.row] = newLine
	t.col += len(runes)
}

// ── Scrolling ───────────────────────────────────────────────────────────────

func (t *TextArea) scrollToCursor() {
	wrapped := t.allWrappedLines()
	visRow, _ := t.cursorVisualPos(wrapped)
	if visRow < t.offset {
		t.offset = visRow
	}
	if visRow >= t.offset+t.height {
		t.offset = visRow - t.height + 1
	}
	if t.offset < 0 {
		t.offset = 0
	}
}
