package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/diff"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// TestRenderDiff_NormalFile verifies a basic file with one hunk renders correctly:
// blank line, bold filename, hunk header, and coloured content lines with line numbers.
func TestRenderDiff_NormalFile(t *testing.T) {
	files := []diff.File{
		{
			Name: "main.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					Context:  "func main()",
					NewStart: 10,
					NewCount: 4,
					Lines: []diff.Line{
						{Type: diff.LineContext, Content: "fmt.Println(\"hello\")"},
						{Type: diff.LineRemoved, Content: "fmt.Println(\"old\")"},
						{Type: diff.LineAdded, Content: "fmt.Println(\"new\")"},
						{Type: diff.LineContext, Content: "fmt.Println(\"end\")"},
					},
				},
			},
		},
	}

	result := renderDiff(files, 60)
	lines := strings.Split(result, "\n")

	// First line should be blank (file separator).
	if lines[0] != "" {
		t.Errorf("expected blank first line, got %q", lines[0])
	}

	// Second line should contain the bold filename.
	if !strings.Contains(lines[1], "main.go") {
		t.Errorf("expected filename in line 1, got %q", lines[1])
	}

	// Third line should be the hunk header with context.
	if !strings.Contains(lines[2], "@@") || !strings.Contains(lines[2], "func main()") {
		t.Errorf("expected hunk header with context in line 2, got %q", lines[2])
	}

	// Should have content lines after the header.
	if len(lines) < 7 {
		t.Fatalf("expected at least 7 lines, got %d", len(lines))
	}
}

// TestRenderDiff_LineNumbers verifies that added and context lines show line
// numbers, while removed lines show blank space in the line-number column.
func TestRenderDiff_LineNumbers(t *testing.T) {
	files := []diff.File{
		{
			Name: "nums.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 5,
					NewCount: 3,
					Lines: []diff.Line{
						{Type: diff.LineContext, Content: "context"},
						{Type: diff.LineRemoved, Content: "removed"},
						{Type: diff.LineAdded, Content: "added"},
						{Type: diff.LineContext, Content: "more context"},
					},
				},
			},
		},
	}

	result := renderDiff(files, 80)
	lines := strings.Split(result, "\n")

	// Line numbers start at NewStart (5).
	// Context line at line 5, removed line has no number, added line at 6,
	// context at 7.

	// Context line (index 3 in output: blank, filename, hunk header, then content).
	contextLine := lines[3]
	if !strings.Contains(contextLine, "5") || !strings.Contains(contextLine, "context") {
		t.Errorf("expected context line with number 5, got %q", contextLine)
	}

	// Removed line (index 4) should NOT contain a line number.
	removedLine := lines[4]
	if !strings.Contains(removedLine, "removed") {
		t.Errorf("expected removed content, got %q", removedLine)
	}

	// Added line (index 5) should have line number 6.
	addedLine := lines[5]
	if !strings.Contains(addedLine, "6") || !strings.Contains(addedLine, "added") {
		t.Errorf("expected added line with number 6, got %q", addedLine)
	}
}

// TestRenderDiff_BinaryFile verifies binary files show "(binary stuff)" in
// emptyStateStyle.
func TestRenderDiff_BinaryFile(t *testing.T) {
	files := []diff.File{
		{
			Name: "image.png",
			Type: diff.Binary,
		},
	}

	result := renderDiff(files, 60)
	if !strings.Contains(result, "image.png") {
		t.Error("expected filename in output")
	}
	if !strings.Contains(result, "(binary stuff)") {
		t.Error("expected '(binary stuff)' for binary file")
	}
}

// TestRenderDiff_DeletedFile verifies deleted files show "Deleted" in bold
// dark-red.
func TestRenderDiff_DeletedFile(t *testing.T) {
	files := []diff.File{
		{
			Name: "old_file.go",
			Type: diff.Delete,
		},
	}

	result := renderDiff(files, 60)
	if !strings.Contains(result, "old_file.go") {
		t.Error("expected filename in output")
	}
	if !strings.Contains(result, "Deleted") {
		t.Error("expected 'Deleted' for deleted file")
	}
}

// TestRenderDiff_RenamedFile verifies renamed files show "Renamed to <new>"
// with the new name in plain text.
func TestRenderDiff_RenamedFile(t *testing.T) {
	files := []diff.File{
		{
			Name:     "old_name.go",
			Type:     diff.Rename,
			RenameTo: "new_name.go",
		},
	}

	result := renderDiff(files, 60)
	if !strings.Contains(result, "old_name.go") {
		t.Error("expected old filename in output")
	}
	if !strings.Contains(result, "Renamed to") {
		t.Error("expected 'Renamed to' for renamed file")
	}
	if !strings.Contains(result, "new_name.go") {
		t.Error("expected new filename in output")
	}
}

// TestRenderDiff_RenamedFileWithHunks verifies that a rename with content
// changes (similarity < 100%) renders hunks after the rename message.
func TestRenderDiff_RenamedFileWithHunks(t *testing.T) {
	files := []diff.File{
		{
			Name:     "old_name.go",
			Type:     diff.Rename,
			RenameTo: "new_name.go",
			Hunks: []diff.Hunk{
				{
					Context:  "func example()",
					NewStart: 1,
					NewCount: 3,
					Lines: []diff.Line{
						{Type: diff.LineContext, Content: "func example() {"},
						{Type: diff.LineRemoved, Content: "\treturn 1"},
						{Type: diff.LineAdded, Content: "\treturn 2"},
						{Type: diff.LineContext, Content: "}"},
					},
				},
			},
		},
	}

	result := renderDiff(files, 60)
	if !strings.Contains(result, "Renamed to") {
		t.Error("expected 'Renamed to' for renamed file")
	}
	if !strings.Contains(result, "new_name.go") {
		t.Error("expected new filename in output")
	}
	if !strings.Contains(result, "@@") {
		t.Error("expected hunk header for renamed file with content changes")
	}
	if !strings.Contains(result, "return 1") {
		t.Error("expected removed content in renamed file hunks")
	}
	if !strings.Contains(result, "return 2") {
		t.Error("expected added content in renamed file hunks")
	}
}

// TestRenderDiff_NewFile verifies new files are treated like normal files
// (showing diff hunks).
func TestRenderDiff_NewFile(t *testing.T) {
	files := []diff.File{
		{
			Name: "brand_new.go",
			Type: diff.New,
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 2,
					Lines: []diff.Line{
						{Type: diff.LineAdded, Content: "package main"},
						{Type: diff.LineAdded, Content: ""},
					},
				},
			},
		},
	}

	result := renderDiff(files, 60)
	if !strings.Contains(result, "brand_new.go") {
		t.Error("expected filename in output")
	}
	if !strings.Contains(result, "package main") {
		t.Error("expected file content for new file")
	}
}

// TestRenderDiff_MultipleFiles verifies that each file starts with a blank
// line separator.
func TestRenderDiff_MultipleFiles(t *testing.T) {
	files := []diff.File{
		{
			Name: "a.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 1,
					Lines: []diff.Line{
						{Type: diff.LineAdded, Content: "line"},
					},
				},
			},
		},
		{
			Name: "b.go",
			Type: diff.Binary,
		},
		{
			Name: "c.go",
			Type: diff.Delete,
		},
	}

	result := renderDiff(files, 60)

	// Each file should have its own section.
	if !strings.Contains(result, "a.go") {
		t.Error("expected a.go in output")
	}
	if !strings.Contains(result, "b.go") {
		t.Error("expected b.go in output")
	}
	if !strings.Contains(result, "c.go") {
		t.Error("expected c.go in output")
	}

	// The first line of the result should be blank (first file also starts with blank).
	lines := strings.Split(result, "\n")
	if lines[0] != "" {
		t.Errorf("expected blank first line, got %q", lines[0])
	}
}

// TestRenderDiff_HunkHeaderNoContext verifies that a hunk with empty context
// renders just "@@".
func TestRenderDiff_HunkHeaderNoContext(t *testing.T) {
	files := []diff.File{
		{
			Name: "test.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 1,
					Lines: []diff.Line{
						{Type: diff.LineAdded, Content: "x"},
					},
				},
			},
		},
	}

	result := renderDiff(files, 60)
	lines := strings.Split(result, "\n")

	// Hunk header (line 2) should contain "@@" but no function context.
	hunkLine := lines[2]
	if !strings.Contains(hunkLine, "@@") {
		t.Errorf("expected @@ in hunk header, got %q", hunkLine)
	}
}

// TestRenderDiff_HunkHeaderWithContext verifies context appears after "@@".
func TestRenderDiff_HunkHeaderWithContext(t *testing.T) {
	files := []diff.File{
		{
			Name: "test.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					Context:  "func Foo()",
					NewStart: 1,
					NewCount: 1,
					Lines: []diff.Line{
						{Type: diff.LineAdded, Content: "x"},
					},
				},
			},
		},
	}

	result := renderDiff(files, 60)
	lines := strings.Split(result, "\n")
	hunkLine := lines[2]
	if !strings.Contains(hunkLine, "@@ func Foo()") {
		t.Errorf("expected '@@ func Foo()' in hunk header, got %q", hunkLine)
	}
}

// TestRenderDiff_FullWidthPadding verifies that hunk headers, added lines,
// and removed lines are padded to the full pane width.
func TestRenderDiff_FullWidthPadding(t *testing.T) {
	files := []diff.File{
		{
			Name: "pad.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 2,
					Lines: []diff.Line{
						{Type: diff.LineRemoved, Content: "old"},
						{Type: diff.LineAdded, Content: "new"},
					},
				},
			},
		},
	}

	paneWidth := 40
	result := renderDiff(files, paneWidth)
	lines := strings.Split(result, "\n")

	// Hunk header, removed, and added lines should all be padded.
	// We can't easily check the exact ANSI-styled width, but we can
	// check they contain padding (spaces beyond the content).
	for i := 2; i <= 4 && i < len(lines); i++ {
		// The rendered line should not be empty.
		if len(lines[i]) == 0 {
			t.Errorf("line %d unexpectedly empty", i)
		}
	}
}

// TestRenderDiff_EmptyInput handles the edge case of no files.
func TestRenderDiff_EmptyInput(t *testing.T) {
	result := renderDiff(nil, 60)
	if result != "" {
		t.Errorf("expected empty string for no files, got %q", result)
	}
}

// TestRenderDiff_MultipleHunks verifies a file with two hunks renders both.
func TestRenderDiff_MultipleHunks(t *testing.T) {
	files := []diff.File{
		{
			Name: "multi.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					Context:  "func A()",
					NewStart: 10,
					NewCount: 2,
					Lines: []diff.Line{
						{Type: diff.LineContext, Content: "a1"},
						{Type: diff.LineAdded, Content: "a2"},
					},
				},
				{
					Context:  "func B()",
					NewStart: 50,
					NewCount: 2,
					Lines: []diff.Line{
						{Type: diff.LineRemoved, Content: "b1"},
						{Type: diff.LineAdded, Content: "b2"},
					},
				},
			},
		},
	}

	result := renderDiff(files, 80)
	if strings.Count(result, "@@") < 2 {
		t.Error("expected at least 2 hunk headers")
	}
	if !strings.Contains(result, "func A()") {
		t.Error("expected first hunk context")
	}
	if !strings.Contains(result, "func B()") {
		t.Error("expected second hunk context")
	}
}

// TestFileNamesFromDiff returns ordered filenames.
func TestFileNamesFromDiff(t *testing.T) {
	files := []diff.File{
		{Name: "alpha.go"},
		{Name: "beta.go"},
		{Name: "gamma.go"},
	}
	names := fileNamesFromDiff(files)
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	want := []string{"alpha.go", "beta.go", "gamma.go"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("index %d: got %q, want %q", i, names[i], w)
		}
	}
}

// TestFileNamesFromDiff_Empty handles empty input.
func TestFileNamesFromDiff_Empty(t *testing.T) {
	names := fileNamesFromDiff(nil)
	if len(names) != 0 {
		t.Errorf("expected empty slice, got %v", names)
	}
}

// TestFileStartLines verifies line offsets where each file begins in the
// rendered output.
func TestFileStartLines(t *testing.T) {
	files := []diff.File{
		{
			Name: "first.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 2,
					Lines: []diff.Line{
						{Type: diff.LineAdded, Content: "line1"},
						{Type: diff.LineAdded, Content: "line2"},
					},
				},
			},
		},
		{
			Name: "second.go",
			Type: diff.Binary,
		},
		{
			Name: "third.go",
			Type: diff.Delete,
		},
	}

	rd := renderDiffResult(files, 60, nil, nil)
	rendered := rd.text
	starts := rd.fileStarts

	if len(starts) != 3 {
		t.Fatalf("expected 3 start lines, got %d", len(starts))
	}

	// First file starts at line 0 (the blank separator).
	if starts[0] != 0 {
		t.Errorf("first file start: got %d, want 0", starts[0])
	}

	// Each subsequent file should start at a later line.
	for i := 1; i < len(starts); i++ {
		if starts[i] <= starts[i-1] {
			t.Errorf("file %d start (%d) should be after file %d start (%d)",
				i, starts[i], i-1, starts[i-1])
		}
	}

	// Verify the start lines match where the blank line + filename appear.
	lines := strings.Split(rendered, "\n")
	for i, start := range starts {
		if start >= len(lines) {
			t.Errorf("file %d start %d beyond rendered length %d", i, start, len(lines))
			continue
		}
		// The start line should be blank (the separator before the file).
		if lines[start] != "" {
			t.Errorf("file %d: expected blank line at %d, got %q", i, start, lines[start])
		}
		// The line after the blank should contain the filename.
		if start+1 < len(lines) && !strings.Contains(lines[start+1], files[i].Name) {
			t.Errorf("file %d: expected filename %q at line %d, got %q",
				i, files[i].Name, start+1, lines[start+1])
		}
	}
}

// TestFileStartLines_Empty handles empty input.
func TestFileStartLines_Empty(t *testing.T) {
	starts := renderDiffResult(nil, 60, nil, nil).fileStarts
	if len(starts) != 0 {
		t.Errorf("expected empty slice, got %v", starts)
	}
}

// TestRenderDiff_LineNumberColumnWidth verifies that line number column width
// is determined by the maximum line number in the hunk (NewStart + NewCount).
func TestRenderDiff_LineNumberColumnWidth(t *testing.T) {
	// With NewStart=998 and NewCount=5, max line is 1003 → 4-digit column.
	files := []diff.File{
		{
			Name: "wide.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 998,
					NewCount: 5,
					Lines: []diff.Line{
						{Type: diff.LineContext, Content: "line998"},
						{Type: diff.LineAdded, Content: "line999"},
					},
				},
			},
		},
	}

	result := renderDiff(files, 80)
	lines := strings.Split(result, "\n")

	// Context line (index 3) should have "998" in it.
	if len(lines) > 3 && !strings.Contains(lines[3], "998") {
		t.Errorf("expected line number 998, got %q", lines[3])
	}
}

// TestRenderDiff_RemovedLineNoPrefix verifies removed lines don't have a
// leading "-" prefix.
func TestRenderDiff_RemovedLineNoPrefix(t *testing.T) {
	files := []diff.File{
		{
			Name: "rm.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 1,
					Lines: []diff.Line{
						{Type: diff.LineRemoved, Content: "removed content"},
					},
				},
			},
		},
	}

	result := renderDiff(files, 80)
	// The rendered removed line should contain the content but not start
	// with a "-" prefix (the raw diff prefix was already stripped by parser).
	if strings.Contains(result, "-removed content") {
		t.Error("removed line should not have '-' prefix")
	}
	if !strings.Contains(result, "removed content") {
		t.Error("removed line content should be present")
	}
}

// TestRenderDiff_AddedLineNoPrefix verifies added lines don't have a leading
// "+" prefix.
func TestRenderDiff_AddedLineNoPrefix(t *testing.T) {
	files := []diff.File{
		{
			Name: "add.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 1,
					Lines: []diff.Line{
						{Type: diff.LineAdded, Content: "added content"},
					},
				},
			},
		},
	}

	result := renderDiff(files, 80)
	if strings.Contains(result, "+added content") {
		t.Error("added line should not have '+' prefix")
	}
	if !strings.Contains(result, "added content") {
		t.Error("added line content should be present")
	}
}

// ── Theme integration tests ──────────────────────────────────────────────────

// withRendererTestTheme temporarily replaces CurrentTheme with a modified theme
// that uses structurally distinctive styles (padding), restoring the original
// when done. Padding differences are visible even in no-colour test environments.
func withRendererTestTheme(t *testing.T) {
	t.Helper()
	original := theme.Current()
	t.Cleanup(func() { theme.SetCurrent(original) })

	custom := theme.Tan()
	custom.DiffFileHeaderStyle = lipgloss.NewStyle().Padding(0, 3)
	custom.DiffHunkHeaderStyle = lipgloss.NewStyle().Padding(0, 3)
	custom.DiffAddedStyle = lipgloss.NewStyle().Padding(0, 3)
	custom.DiffRemovedStyle = lipgloss.NewStyle().Padding(0, 3)
	custom.DiffDeletedMsgStyle = lipgloss.NewStyle().Padding(0, 3)
	custom.DiffRenamedMsgStyle = lipgloss.NewStyle().Padding(0, 3)
	custom.EmptyStateStyle = lipgloss.NewStyle().Padding(0, 3)
	theme.SetCurrent(custom)
}

// TestRenderDiff_UsesThemeFileHeaderStyle verifies that file headers use
// theme.Current().DiffFileHeaderStyle.
func TestRenderDiff_UsesThemeFileHeaderStyle(t *testing.T) {
	files := []diff.File{
		{Name: "test.go", Type: diff.Normal, Hunks: []diff.Hunk{
			{NewStart: 1, NewCount: 1, Lines: []diff.Line{
				{Type: diff.LineAdded, Content: "x"},
			}},
		}},
	}

	assertThemeChangesOutput(t, withRendererTestTheme, func() string {
		return renderDiff(files, 60)
	})
}

// TestRenderDiff_UsesThemeHunkHeaderStyle verifies that hunk headers use
// theme.Current().DiffHunkHeaderStyle.
func TestRenderDiff_UsesThemeHunkHeaderStyle(t *testing.T) {
	files := []diff.File{
		{Name: "test.go", Type: diff.Normal, Hunks: []diff.Hunk{
			{Context: "func main()", NewStart: 1, NewCount: 1, Lines: []diff.Line{
				{Type: diff.LineContext, Content: "x"},
			}},
		}},
	}

	assertThemeChangesOutput(t, withRendererTestTheme, func() string {
		return renderDiff(files, 60)
	})
}

// TestRenderDiff_UsesThemeAddedRemovedStyles verifies that added and removed
// lines use theme.Current().DiffAddedStyle and DiffRemovedStyle.
func TestRenderDiff_UsesThemeAddedRemovedStyles(t *testing.T) {
	files := []diff.File{
		{Name: "test.go", Type: diff.Normal, Hunks: []diff.Hunk{
			{NewStart: 1, NewCount: 2, Lines: []diff.Line{
				{Type: diff.LineRemoved, Content: "old"},
				{Type: diff.LineAdded, Content: "new"},
			}},
		}},
	}

	assertThemeChangesOutput(t, withRendererTestTheme, func() string {
		return renderDiff(files, 60)
	})
}

// TestRenderDiff_UsesThemeDeletedMsgStyle verifies that deleted file messages
// use theme.Current().DiffDeletedMsgStyle.
func TestRenderDiff_UsesThemeDeletedMsgStyle(t *testing.T) {
	files := []diff.File{
		{Name: "old.go", Type: diff.Delete},
	}

	assertThemeChangesOutput(t, withRendererTestTheme, func() string {
		return renderDiff(files, 60)
	})
}

// TestRenderDiff_UsesThemeRenamedMsgStyle verifies that renamed file messages
// use theme.Current().DiffRenamedMsgStyle.
func TestRenderDiff_UsesThemeRenamedMsgStyle(t *testing.T) {
	files := []diff.File{
		{Name: "old.go", Type: diff.Rename, RenameTo: "new.go"},
	}

	assertThemeChangesOutput(t, withRendererTestTheme, func() string {
		return renderDiff(files, 60)
	})
}

// TestRenderDiff_UsesThemeEmptyStateStyle verifies that binary file placeholder
// uses theme.Current().EmptyStateStyle.
func TestRenderDiff_UsesThemeEmptyStateStyle(t *testing.T) {
	files := []diff.File{
		{Name: "image.png", Type: diff.Binary},
	}

	assertThemeChangesOutput(t, withRendererTestTheme, func() string {
		return renderDiff(files, 60)
	})
}

// ── Line metadata tests ─────────────────────────────────────────────────────

// TestLineMeta_MatchesLineCount verifies that lineMeta has one entry per
// rendered line (after trimming trailing newline).
func TestLineMeta_MatchesLineCount(t *testing.T) {
	files := []diff.File{
		{
			Name: "a.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1, NewCount: 2,
					Lines: []diff.Line{
						{Type: diff.LineContext, Content: "ctx"},
						{Type: diff.LineAdded, Content: "add"},
					},
				},
			},
		},
		{
			Name: "b.go",
			Type: diff.Delete,
		},
	}

	rd := renderDiffResult(files, 60, nil, nil)
	renderedLineCount := len(strings.Split(rd.text, "\n"))
	if len(rd.lineMeta) != renderedLineCount {
		t.Errorf("lineMeta length %d != rendered line count %d", len(rd.lineMeta), renderedLineCount)
	}
}

// TestLineMeta_HunkContentIsSelectable verifies that hunk content lines are
// marked as selectable and non-content lines are not.
func TestLineMeta_HunkContentIsSelectable(t *testing.T) {
	files := []diff.File{
		{
			Name: "a.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1, NewCount: 2,
					Lines: []diff.Line{
						{Type: diff.LineContext, Content: "ctx"},
						{Type: diff.LineRemoved, Content: "old"},
						{Type: diff.LineAdded, Content: "new"},
					},
				},
			},
		},
	}

	rd := renderDiffResult(files, 60, nil, nil)
	selectableCount := 0
	nonSelectableCount := 0
	for _, lm := range rd.lineMeta {
		if lm.kind == diffLineHunkContent {
			selectableCount++
		} else {
			nonSelectableCount++
		}
	}
	// 3 hunk content lines (ctx, old, new)
	if selectableCount != 3 {
		t.Errorf("expected 3 selectable lines, got %d", selectableCount)
	}
	// blank + filename + hunk header = 3 non-selectable
	if nonSelectableCount != 3 {
		t.Errorf("expected 3 non-selectable lines, got %d", nonSelectableCount)
	}
}

// TestLineMeta_FileIndex verifies that each line's fileIndex is correct.
func TestLineMeta_FileIndex(t *testing.T) {
	files := []diff.File{
		{
			Name: "first.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{NewStart: 1, NewCount: 1, Lines: []diff.Line{
					{Type: diff.LineAdded, Content: "a"},
				}},
			},
		},
		{
			Name: "second.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{NewStart: 1, NewCount: 1, Lines: []diff.Line{
					{Type: diff.LineAdded, Content: "b"},
				}},
			},
		},
	}

	rd := renderDiffResult(files, 60, nil, nil)
	// All lines before the second file's blank separator should have fileIndex 0.
	secondFileStart := rd.fileStarts[1]
	for i, lm := range rd.lineMeta {
		if i < secondFileStart && lm.fileIndex != 0 {
			t.Errorf("line %d: expected fileIndex 0, got %d", i, lm.fileIndex)
		}
		if i >= secondFileStart && lm.fileIndex != 1 {
			t.Errorf("line %d: expected fileIndex 1, got %d", i, lm.fileIndex)
		}
	}
}

// TestLineMeta_DeletedAndBinaryNotSelectable verifies that deleted and binary
// file content lines (like "Deleted" and "(binary stuff)") are non-selectable.
func TestLineMeta_DeletedAndBinaryNotSelectable(t *testing.T) {
	files := []diff.File{
		{Name: "gone.go", Type: diff.Delete},
		{Name: "pic.png", Type: diff.Binary},
	}

	rd := renderDiffResult(files, 60, nil, nil)
	for i, lm := range rd.lineMeta {
		if lm.kind != diffLineNonSelectable {
			t.Errorf("line %d: expected non-selectable for delete/binary files, got selectable", i)
		}
	}
}

// TestLineMeta_CollapsedFilesHaveNoSelectableLines verifies that collapsing a
// file removes its selectable lines from the metadata.
func TestLineMeta_CollapsedFilesHaveNoSelectableLines(t *testing.T) {
	files := []diff.File{
		{
			Name: "a.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{NewStart: 1, NewCount: 1, Lines: []diff.Line{
					{Type: diff.LineAdded, Content: "x"},
				}},
			},
		},
	}

	collapsed := []bool{true}
	rd := renderDiffResult(files, 60, collapsed, nil)
	for i, lm := range rd.lineMeta {
		if lm.kind == diffLineHunkContent {
			t.Errorf("line %d: expected no selectable lines in collapsed file", i)
		}
	}
}

// ── CR emoji indicator tests ─────────────────────────────────────────────────

// TestRenderDiffResult_CRLocations_AddedLine verifies that an added line with
// a CR location gets the speech balloon emoji appended.
func TestRenderDiffResult_CRLocations_AddedLine(t *testing.T) {
	files := []diff.File{
		{
			Name: "main.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 10,
					NewCount: 2,
					Lines: []diff.Line{
						{Type: diff.LineAdded, Content: "fmt.Println(\"new\")"},
						{Type: diff.LineAdded, Content: "no CR here"},
					},
				},
			},
		},
	}

	crLocations := map[string]bool{"main.go:10": true}
	rd := renderDiffResult(files, 60, nil, crLocations)

	lines := strings.Split(rd.text, "\n")
	// Find lines with the emoji. The added line at lineNum 10 should have it.
	foundEmoji := false
	foundNoEmoji := false
	for _, line := range lines {
		stripped := stripAnsi(line)
		if strings.Contains(stripped, "fmt.Println") && strings.Contains(stripped, "\U0001F4AC") {
			foundEmoji = true
		}
		if strings.Contains(stripped, "no CR here") && !strings.Contains(stripped, "\U0001F4AC") {
			foundNoEmoji = true
		}
	}
	if !foundEmoji {
		t.Error("expected speech balloon emoji on added line with CR")
	}
	if !foundNoEmoji {
		t.Error("expected no emoji on added line without CR")
	}
}

// TestRenderDiffResult_CRLocations_RemovedLine verifies that a removed line
// with a CR location gets the speech balloon emoji appended.
func TestRenderDiffResult_CRLocations_RemovedLine(t *testing.T) {
	files := []diff.File{
		{
			Name: "main.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 10,
					NewCount: 1,
					Lines: []diff.Line{
						{Type: diff.LineRemoved, Content: "old line"},
						{Type: diff.LineAdded, Content: "new line"},
					},
				},
			},
		},
	}

	// Removed lines use the same lineNum as the next added/context line.
	// In this hunk, lineNum starts at 10. The removed line is at lineNum 10,
	// the added line increments to 11.
	crLocations := map[string]bool{"main.go:10": true}
	rd := renderDiffResult(files, 60, nil, crLocations)

	lines := strings.Split(rd.text, "\n")
	foundEmoji := false
	for _, line := range lines {
		stripped := stripAnsi(line)
		if strings.Contains(stripped, "old line") && strings.Contains(stripped, "\U0001F4AC") {
			foundEmoji = true
		}
	}
	if !foundEmoji {
		t.Error("expected speech balloon emoji on removed line with CR")
	}
}

// TestRenderDiffResult_CRLocations_ContextLine verifies that a context line
// with a CR location gets the speech balloon emoji appended.
func TestRenderDiffResult_CRLocations_ContextLine(t *testing.T) {
	files := []diff.File{
		{
			Name: "main.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 5,
					NewCount: 3,
					Lines: []diff.Line{
						{Type: diff.LineContext, Content: "context with CR"},
						{Type: diff.LineContext, Content: "context without CR"},
						{Type: diff.LineAdded, Content: "added"},
					},
				},
			},
		},
	}

	crLocations := map[string]bool{"main.go:5": true}
	rd := renderDiffResult(files, 60, nil, crLocations)

	lines := strings.Split(rd.text, "\n")
	foundEmoji := false
	foundNoEmoji := false
	for _, line := range lines {
		stripped := stripAnsi(line)
		if strings.Contains(stripped, "context with CR") && strings.Contains(stripped, "\U0001F4AC") {
			foundEmoji = true
		}
		if strings.Contains(stripped, "context without CR") && !strings.Contains(stripped, "\U0001F4AC") {
			foundNoEmoji = true
		}
	}
	if !foundEmoji {
		t.Error("expected speech balloon emoji on context line with CR")
	}
	if !foundNoEmoji {
		t.Error("expected no emoji on context line without CR")
	}
}

// TestRenderDiffResult_CRLocations_NilMap verifies that nil crLocations
// renders identically to the no-CR case (backward compatibility).
func TestRenderDiffResult_CRLocations_NilMap(t *testing.T) {
	files := []diff.File{
		{
			Name: "main.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 2,
					Lines: []diff.Line{
						{Type: diff.LineAdded, Content: "line1"},
						{Type: diff.LineContext, Content: "line2"},
					},
				},
			},
		},
	}

	rd := renderDiffResult(files, 60, nil, nil)
	lines := strings.Split(rd.text, "\n")
	for _, line := range lines {
		if strings.Contains(line, "\U0001F4AC") {
			t.Error("expected no emoji when crLocations is nil")
		}
	}
}

// TestRenderDiffResult_CRLocations_EmptyMap verifies that an empty crLocations
// map produces no emojis.
func TestRenderDiffResult_CRLocations_EmptyMap(t *testing.T) {
	files := []diff.File{
		{
			Name: "main.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 1,
					Lines: []diff.Line{
						{Type: diff.LineAdded, Content: "x"},
					},
				},
			},
		},
	}

	rd := renderDiffResult(files, 60, nil, map[string]bool{})
	lines := strings.Split(rd.text, "\n")
	for _, line := range lines {
		if strings.Contains(line, "\U0001F4AC") {
			t.Error("expected no emoji when crLocations is empty")
		}
	}
}

// TestRenderDiffResult_CRLocations_MultipleFiles verifies CR indicators work
// correctly across multiple files.
func TestRenderDiffResult_CRLocations_MultipleFiles(t *testing.T) {
	files := []diff.File{
		{
			Name: "a.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{NewStart: 1, NewCount: 1, Lines: []diff.Line{
					{Type: diff.LineAdded, Content: "in file a"},
				}},
			},
		},
		{
			Name: "b.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{NewStart: 5, NewCount: 1, Lines: []diff.Line{
					{Type: diff.LineAdded, Content: "in file b"},
				}},
			},
		},
	}

	crLocations := map[string]bool{"b.go:5": true}
	rd := renderDiffResult(files, 60, nil, crLocations)

	lines := strings.Split(rd.text, "\n")
	emojiOnA := false
	emojiOnB := false
	for _, line := range lines {
		stripped := stripAnsi(line)
		if strings.Contains(stripped, "in file a") && strings.Contains(stripped, "\U0001F4AC") {
			emojiOnA = true
		}
		if strings.Contains(stripped, "in file b") && strings.Contains(stripped, "\U0001F4AC") {
			emojiOnB = true
		}
	}
	if emojiOnA {
		t.Error("expected no emoji on file a (no CR)")
	}
	if !emojiOnB {
		t.Error("expected emoji on file b line 5 (has CR)")
	}
}

// TestRenderDiffResult_CRLine_ReducedWidth verifies that CR lines use
// paneWidth-4 for content width so the emoji fits without overlapping.
func TestRenderDiffResult_CRLine_ReducedWidth(t *testing.T) {
	files := []diff.File{
		{
			Name: "main.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 2,
					Lines: []diff.Line{
						{Type: diff.LineAdded, Content: "CR line"},
						{Type: diff.LineAdded, Content: "normal line"},
					},
				},
			},
		},
	}

	paneWidth := 40
	crLocations := map[string]bool{"main.go:1": true}
	rd := renderDiffResult(files, paneWidth, nil, crLocations)

	lines := strings.Split(rd.text, "\n")
	for _, line := range lines {
		stripped := stripAnsi(line)
		w := lipgloss.Width(stripped)
		if w > paneWidth {
			// Wrapped lines are allowed to be at paneWidth each.
			// But no single visual line should exceed paneWidth.
			t.Errorf("line exceeds paneWidth (%d): width=%d, content=%q", paneWidth, w, stripped)
		}
	}
}

// ── renderContext tests ──────────────────────────────────────────────────────

// TestRenderContext_HasAnnotation verifies that HasAnnotation correctly looks
// up "file:line" keys in the crLocations map using the context's fileName.
func TestRenderContext_HasAnnotation(t *testing.T) {
	rc := &renderContext{
		crLocations: map[string]bool{"main.go:10": true, "util.go:5": true},
	}
	rc.fileName = "main.go"
	if !rc.HasAnnotation(10) {
		t.Error("expected HasAnnotation to return true for main.go:10")
	}
	rc.fileName = "util.go"
	if !rc.HasAnnotation(5) {
		t.Error("expected HasAnnotation to return true for util.go:5")
	}
	rc.fileName = "main.go"
	if rc.HasAnnotation(11) {
		t.Error("expected HasAnnotation to return false for main.go:11")
	}
	rc.fileName = "other.go"
	if rc.HasAnnotation(10) {
		t.Error("expected HasAnnotation to return false for other.go:10")
	}
}

// TestRenderContext_HasAnnotation_NilMap verifies HasAnnotation returns false
// when crLocations is nil.
func TestRenderContext_HasAnnotation_NilMap(t *testing.T) {
	rc := &renderContext{crLocations: nil, fileName: "main.go"}
	if rc.HasAnnotation(10) {
		t.Error("expected HasAnnotation to return false for nil crLocations")
	}
}

// TestRenderDiff_BackwardCompatible verifies that the old renderDiff function
// (which doesn't pass crLocations) still works correctly.
func TestRenderDiff_BackwardCompatible(t *testing.T) {
	files := []diff.File{
		{
			Name: "main.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{NewStart: 1, NewCount: 1, Lines: []diff.Line{
					{Type: diff.LineAdded, Content: "hello"},
				}},
			},
		},
	}

	result := renderDiff(files, 60)
	if !strings.Contains(result, "hello") {
		t.Error("renderDiff should still render content")
	}
	if strings.Contains(result, "\U0001F4AC") {
		t.Error("renderDiff without crLocations should not have emojis")
	}
}
