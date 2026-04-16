package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// TestWrapLineExpandsTabs verifies that wrapLine expands tabs to spaces
// before measuring width, preventing lipgloss re-wrap overflow.
func TestWrapLineExpandsTabs(t *testing.T) {
	contentW := 50

	// 4 tabs = 4 runes raw but 16 chars after expansion.
	line := "\t\t\t\tresult = append(result, processItem(items[i]))"

	wrapped := wrapLine(line, contentW)

	// After tab expansion, the line is 62 visual chars, so it must wrap.
	if len(wrapped) < 2 {
		t.Errorf("wrapLine should split tab-containing line that exceeds width after expansion; got %d line(s)", len(wrapped))
	}

	// Verify no wrapped line exceeds contentW after tab expansion.
	for i, w := range wrapped {
		if lipgloss.Width(w) > contentW {
			t.Errorf("wrapped line %d exceeds contentW (%d): visual width %d: %q",
				i, contentW, lipgloss.Width(w), w)
		}
	}
}

// TestPaneHeightStableWithTabs verifies that tab-containing code context
// does not cause the content pane to overflow its expected height.
func TestPaneHeightStableWithTabs(t *testing.T) {
	saveTheme(t)

	contentW := 60
	contentH := 15

	// Simulate code context with tab-indented source lines.
	codeLines := []string{
		"  45 | func ParseRequest(toolCallID string) (RequestType, RequestParams, error) {",
		"  46 | \tif toolCallID == \"\" {",
		"  47 | \t\treturn \"\", RequestParams{}, fmt.Errorf(\"empty tool call ID\")",
		"  48 | \t}",
		"> 49 | \tif strings.HasPrefix(toolCallID, \"Write \") {",
		"  50 | \t\tfilename := toolCallID[len(\"Write \"):]",
		"  51 | \t\treturn RequestTypeWrite, RequestParams{Filename: filename}, nil",
	}

	raw := strings.Join(append(
		[]string{
			theme.Current().DetailLabelStyle.Render("File:") + " internal/request/parser.go",
			theme.Current().DetailLabelStyle.Render("Line:") + " 49",
			theme.Current().DetailLabelStyle.Render("Status:") + " open",
			"",
			theme.Current().DetailLabelStyle.Render("Code:"),
		},
		append(codeLines,
			"",
			theme.Current().DetailLabelStyle.Render("Description:"),
			"",
			"Short description.",
		)...,
	), "\n")

	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		lines = append(lines, wrapLine(line, contentW)...)
	}

	end := contentH
	if end > len(lines) {
		end = len(lines)
	}
	var sb strings.Builder
	for i := 0; i < end; i++ {
		sb.WriteString(lines[i])
		if i < end-1 {
			sb.WriteString("\n")
		}
	}

	style := theme.Current().UnfocusedBorderStyle
	rendered := style.Width(contentW).Height(contentH).Render(sb.String())
	renderedH := strings.Count(rendered, "\n") + 1
	expectedH := contentH + 2 // content + 2 border rows

	if renderedH != expectedH {
		t.Errorf("pane height overflow: got %d, want %d", renderedH, expectedH)
		for i := 0; i < end; i++ {
			vw := lipgloss.Width(lines[i])
			if vw > contentW {
				t.Logf("  overflow line %d: visual=%d, over by %d: %q", i, vw, vw-contentW, lines[i])
			}
		}
	}
}

// TestDialogWidthFits verifies the combined pane width fits within
// the dialog's content area.
func TestDialogWidthFits(t *testing.T) {
	saveTheme(t)

	for _, w := range []int{80, 100, 120, 132, 160} {
		t.Run(fmt.Sprintf("width=%d", w), func(t *testing.T) {
			d := &TicketDialog{
				wu:    &models.WorkUnit{Identifier: "test/t"},
				width: w, height: 40,
			}
			d.computeDimensions()

			paneWidth := d.listW + 2 + d.contentW + 2
			dialogContentW := d.width - 6
			if paneWidth > dialogContentW {
				t.Errorf("panes (%d) wider than dialog (%d)", paneWidth, dialogContentW)
			}
		})
	}
}
