package ui

import (
	"fmt"
	"strings"
)

// DiffFileType classifies the kind of change a file underwent.
type DiffFileType int

const (
	DiffNormal DiffFileType = iota
	DiffBinary
	DiffDelete
	DiffRename
	DiffNew
)

// DiffLineType classifies a single line within a hunk.
type DiffLineType int

const (
	DiffLineContext DiffLineType = iota
	DiffLineAdded
	DiffLineRemoved
)

// DiffFile represents a single file entry in a unified diff.
type DiffFile struct {
	Name     string
	Type     DiffFileType
	RenameTo string
	Hunks    []DiffHunk
}

// DiffHunk represents one @@ section within a file diff.
type DiffHunk struct {
	Context  string
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
}

// DiffLine represents a single content line within a hunk.
type DiffLine struct {
	Type    DiffLineType
	Content string
}

// parseDiff splits raw `git diff` output into structured DiffFile values.
func parseDiff(raw string) []DiffFile {
	sections := splitDiffSections(raw)
	files := make([]DiffFile, 0, len(sections))
	for _, section := range sections {
		files = append(files, parseFileSection(section))
	}
	return files
}

// splitDiffSections splits the raw diff at "diff --git" boundaries, returning
// one string per file. The leading "diff --git …" line is included in each.
func splitDiffSections(raw string) []string {
	const marker = "diff --git "
	var sections []string
	rest := raw
	for {
		idx := strings.Index(rest, marker)
		if idx == -1 {
			break
		}
		rest = rest[idx:]
		next := strings.Index(rest[1:], marker)
		if next == -1 {
			sections = append(sections, rest)
			break
		}
		sections = append(sections, rest[:next+1])
		rest = rest[next+1:]
	}
	return sections
}

// parseFileSection parses a single file's diff section into a DiffFile.
func parseFileSection(section string) DiffFile {
	lines := strings.Split(section, "\n")
	f := DiffFile{Type: DiffNormal}

	aName, bName := parseGitHeader(lines[0])
	f.Name = aName

	if detectFileType(&f, lines[1:], aName, bName) {
		return f // binary file — no hunks to parse
	}

	f.Hunks = parseHunks(lines)
	return f
}

// detectFileType scans the header lines before the first @@ to determine the
// file's change type (binary, delete, new, rename). It returns true when the
// caller should stop processing (binary files have no hunks).
func detectFileType(f *DiffFile, headerLines []string, aName, bName string) bool {
	hasSimilarityIndex := false
	for _, line := range headerLines {
		if strings.HasPrefix(line, "@@") {
			break
		}
		switch {
		case strings.HasPrefix(line, "Binary files"):
			f.Type = DiffBinary
			return true
		case strings.HasPrefix(line, "deleted file mode"):
			f.Type = DiffDelete
		case strings.HasPrefix(line, "new file mode"):
			f.Type = DiffNew
		case strings.HasPrefix(line, "rename from"):
			f.Type = DiffRename
		case strings.HasPrefix(line, "rename to "):
			f.RenameTo = strings.TrimPrefix(line, "rename to ")
		case strings.HasPrefix(line, "similarity index"):
			hasSimilarityIndex = true
		}
	}

	if hasSimilarityIndex && aName != bName {
		f.Type = DiffRename
		if f.RenameTo == "" {
			f.RenameTo = bName
		}
	}
	return false
}

// parseGitHeader extracts the two filenames from a "diff --git a/X b/Y" line.
// It handles filenames that may contain spaces by splitting at " b/" boundaries
// starting from the right side of the line.
func parseGitHeader(header string) (aName, bName string) {
	// Strip the "diff --git " prefix.
	rest := strings.TrimPrefix(header, "diff --git ")

	// The line is "a/<path> b/<path>". We need to find the split point between
	// the two paths. Because paths can contain spaces, we look for " b/" as
	// the separator, scanning from the middle outward. The a/ path starts at
	// index 2 (after "a/"), and " b/" appears somewhere after.
	sep := " b/"
	// Start searching after "a/" (at least position 2).
	idx := strings.Index(rest[2:], sep)
	if idx == -1 {
		// Fallback: treat the whole thing as the name (shouldn't happen with valid diff).
		name := strings.TrimPrefix(rest, "a/")
		return name, name
	}
	aName = rest[2 : idx+2] // skip leading "a/"
	bName = rest[idx+2+len(sep):]
	return aName, bName
}

// parseHunks extracts all DiffHunk values from the lines of a file section.
func parseHunks(lines []string) []DiffHunk {
	var hunks []DiffHunk
	var current *DiffHunk

	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			h := parseHunkHeader(line)
			hunks = append(hunks, h)
			current = &hunks[len(hunks)-1]
			continue
		}
		if current == nil {
			continue
		}
		dl, ok := classifyLine(line)
		if !ok {
			continue
		}
		current.Lines = append(current.Lines, dl)
	}
	return hunks
}

// parseHunkHeader parses an "@@ -old,count +new,count @@ context" line.
func parseHunkHeader(line string) DiffHunk {
	var h DiffHunk
	// Find the closing "@@" after the opening one.
	rest := line[2:] // skip leading "@@"
	end := strings.Index(rest, "@@")
	if end == -1 {
		return h
	}
	rangePart := strings.TrimSpace(rest[:end])
	h.Context = strings.TrimSpace(rest[end+2:])

	// rangePart looks like "-10,6 +10,7"
	parts := strings.Fields(rangePart)
	if len(parts) >= 1 {
		h.OldStart, h.OldCount = parseRange(parts[0])
	}
	if len(parts) >= 2 {
		h.NewStart, h.NewCount = parseRange(parts[1])
	}
	return h
}

// parseRange parses a range like "-10,6" or "+10,7" into start and count.
func parseRange(s string) (int, int) {
	// Strip the leading - or +.
	s = strings.TrimLeft(s, "-+")
	var start, count int
	if strings.Contains(s, ",") {
		fmt.Sscanf(s, "%d,%d", &start, &count)
	} else {
		fmt.Sscanf(s, "%d", &start)
		count = 1
	}
	return start, count
}

// classifyLine determines the type of a diff content line. It returns false
// for lines that are not part of the diff content (e.g. "\ No newline at end
// of file").
func classifyLine(line string) (DiffLine, bool) {
	if len(line) == 0 {
		return DiffLine{}, false
	}
	switch line[0] {
	case '+':
		return DiffLine{Type: DiffLineAdded, Content: line[1:]}, true
	case '-':
		return DiffLine{Type: DiffLineRemoved, Content: line[1:]}, true
	case ' ':
		return DiffLine{Type: DiffLineContext, Content: line[1:]}, true
	default:
		return DiffLine{}, false
	}
}
