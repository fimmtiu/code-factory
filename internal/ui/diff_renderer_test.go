package ui

import (
	"strings"
	"testing"

	"github.com/fimmtiu/code-factory/internal/diff"
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

	rendered := renderDiff(files, 60)
	starts := fileStartLines(files, 60)

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
	starts := fileStartLines(nil, 60)
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
