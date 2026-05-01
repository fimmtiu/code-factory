package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// buildHint renders alternating key/description pairs with each key bolded.
// Example: buildHint("Q", "quit", "?", "help") → bold("Q")+" quit  "+bold("?")+" help"
func buildHint(pairs ...string) string {
	var sb strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			sb.WriteString(theme.Current().HintDescStyle.Render("  "))
		}
		sb.WriteString(theme.Current().HintKeyStyle.Render(pairs[i]))
		sb.WriteString(theme.Current().HintDescStyle.Render(" " + pairs[i+1]))
	}
	return sb.String()
}

// viewBorderOverhead is the number of rows (and columns) consumed by viewPaneStyle.
const viewBorderOverhead = 2

// ViewID is an enum for the five main views.
type ViewID int

const (
	ViewProject ViewID = iota
	ViewCommand
	ViewWorker
	ViewDiff
	ViewLog
)

// viewCount is the total number of views; used for tab cycling.
const viewCount = ViewLog + 1

// viewModel is the interface that each view sub-model must satisfy.
type viewModel interface {
	tea.Model
	KeyBindings() []KeyBinding
	Label() string // e.g. "F1:Projects"; used by renderHeader
}

// clipLines truncates content to at most maxLines lines, preventing overflow
// when lipgloss line-wrapping produces more lines than the pane expects.
// lipgloss's Height() pads short content but does not clip tall content, so
// without this guard a wrapped line pushes the bottom border off-screen.
func clipLines(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	return strings.Join(lines[:maxLines], "\n")
}

// truncateLine truncates s to at most maxWidth visible runes, appending an
// ellipsis if truncation occurred.
func truncateLine(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if maxWidth > 1 {
		return string(runes[:maxWidth-1]) + "…"
	}
	return string(runes[:maxWidth])
}

// nextView returns the next view in the cycle (project → command → worker → diffs → log → project).
func nextView(current ViewID) ViewID {
	return (current + 1) % viewCount
}

// prevView returns the previous view in the cycle.
func prevView(current ViewID) ViewID {
	return (current + viewCount - 1) % viewCount
}
