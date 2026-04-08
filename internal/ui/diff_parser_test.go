package ui

import (
	"testing"
)

func TestParseDiff_NormalWithMultipleHunks(t *testing.T) {
	raw := `diff --git a/main.go b/main.go
index abc1234..def5678 100644
--- a/main.go
+++ b/main.go
@@ -10,6 +10,7 @@ func main() {
 	fmt.Println("hello")
 	fmt.Println("world")
+	fmt.Println("new line")
 	fmt.Println("end")
@@ -30,5 +31,7 @@ func helper() {
 	x := 1
-	y := 2
+	y := 3
+	z := 4
 	return x
`

	files := parseDiff(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f := files[0]
	if f.Name != "main.go" {
		t.Errorf("expected name %q, got %q", "main.go", f.Name)
	}
	if f.Type != DiffNormal {
		t.Errorf("expected type DiffNormal, got %v", f.Type)
	}
	if len(f.Hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(f.Hunks))
	}

	// First hunk
	h0 := f.Hunks[0]
	if h0.OldStart != 10 || h0.OldCount != 6 {
		t.Errorf("hunk 0: expected old 10,6 got %d,%d", h0.OldStart, h0.OldCount)
	}
	if h0.NewStart != 10 || h0.NewCount != 7 {
		t.Errorf("hunk 0: expected new 10,7 got %d,%d", h0.NewStart, h0.NewCount)
	}
	if h0.Context != "func main() {" {
		t.Errorf("hunk 0: expected context %q, got %q", "func main() {", h0.Context)
	}
	if len(h0.Lines) != 4 {
		t.Fatalf("hunk 0: expected 4 lines, got %d", len(h0.Lines))
	}
	if h0.Lines[0].Type != DiffLineContext || h0.Lines[0].Content != "\tfmt.Println(\"hello\")" {
		t.Errorf("hunk 0 line 0: got type=%v content=%q", h0.Lines[0].Type, h0.Lines[0].Content)
	}
	if h0.Lines[2].Type != DiffLineAdded || h0.Lines[2].Content != "\tfmt.Println(\"new line\")" {
		t.Errorf("hunk 0 line 2: got type=%v content=%q", h0.Lines[2].Type, h0.Lines[2].Content)
	}

	// Second hunk
	h1 := f.Hunks[1]
	if h1.OldStart != 30 || h1.OldCount != 5 {
		t.Errorf("hunk 1: expected old 30,5 got %d,%d", h1.OldStart, h1.OldCount)
	}
	if h1.NewStart != 31 || h1.NewCount != 7 {
		t.Errorf("hunk 1: expected new 31,7 got %d,%d", h1.NewStart, h1.NewCount)
	}
	if h1.Context != "func helper() {" {
		t.Errorf("hunk 1: expected context %q, got %q", "func helper() {", h1.Context)
	}
	if len(h1.Lines) != 5 {
		t.Fatalf("hunk 1: expected 5 lines, got %d", len(h1.Lines))
	}
	if h1.Lines[0].Type != DiffLineContext || h1.Lines[0].Content != "\tx := 1" {
		t.Errorf("hunk 1 line 0: got type=%v content=%q", h1.Lines[0].Type, h1.Lines[0].Content)
	}
	if h1.Lines[1].Type != DiffLineRemoved || h1.Lines[1].Content != "\ty := 2" {
		t.Errorf("hunk 1 line 1: got type=%v content=%q", h1.Lines[1].Type, h1.Lines[1].Content)
	}
	if h1.Lines[2].Type != DiffLineAdded || h1.Lines[2].Content != "\ty := 3" {
		t.Errorf("hunk 1 line 2: got type=%v content=%q", h1.Lines[2].Type, h1.Lines[2].Content)
	}
	if h1.Lines[3].Type != DiffLineAdded || h1.Lines[3].Content != "\tz := 4" {
		t.Errorf("hunk 1 line 3: got type=%v content=%q", h1.Lines[3].Type, h1.Lines[3].Content)
	}
	if h1.Lines[4].Type != DiffLineContext || h1.Lines[4].Content != "\treturn x" {
		t.Errorf("hunk 1 line 4: got type=%v content=%q", h1.Lines[4].Type, h1.Lines[4].Content)
	}
}

func TestParseDiff_BinaryFile(t *testing.T) {
	raw := `diff --git a/image.png b/image.png
index abc1234..def5678 100644
Binary files a/image.png and b/image.png differ
`

	files := parseDiff(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Name != "image.png" {
		t.Errorf("expected name %q, got %q", "image.png", files[0].Name)
	}
	if files[0].Type != DiffBinary {
		t.Errorf("expected DiffBinary, got %v", files[0].Type)
	}
	if len(files[0].Hunks) != 0 {
		t.Errorf("expected no hunks for binary file, got %d", len(files[0].Hunks))
	}
}

func TestParseDiff_DeletedFile(t *testing.T) {
	raw := `diff --git a/old.go b/old.go
deleted file mode 100644
index abc1234..0000000
--- a/old.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package main
-
-func old() {}
`

	files := parseDiff(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Name != "old.go" {
		t.Errorf("expected name %q, got %q", "old.go", files[0].Name)
	}
	if files[0].Type != DiffDelete {
		t.Errorf("expected DiffDelete, got %v", files[0].Type)
	}
	if len(files[0].Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(files[0].Hunks))
	}
	if files[0].Hunks[0].Lines[0].Type != DiffLineRemoved {
		t.Errorf("expected removed line, got %v", files[0].Hunks[0].Lines[0].Type)
	}
}

func TestParseDiff_RenamedFile(t *testing.T) {
	raw := `diff --git a/old_name.go b/new_name.go
similarity index 95%
rename from old_name.go
rename to new_name.go
index abc1234..def5678 100644
--- a/old_name.go
+++ b/new_name.go
@@ -1,3 +1,3 @@ package main
 func example() {
-	return 1
+	return 2
 }
`

	files := parseDiff(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Name != "old_name.go" {
		t.Errorf("expected name %q, got %q", "old_name.go", files[0].Name)
	}
	if files[0].Type != DiffRename {
		t.Errorf("expected DiffRename, got %v", files[0].Type)
	}
	if files[0].RenameTo != "new_name.go" {
		t.Errorf("expected rename to %q, got %q", "new_name.go", files[0].RenameTo)
	}
	if len(files[0].Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(files[0].Hunks))
	}
}

func TestParseDiff_RenamedFileFromDiffHeader(t *testing.T) {
	// Rename detected via similarity index + different a/b paths, no explicit rename lines
	raw := `diff --git a/old_name.go b/new_name.go
similarity index 100%
`

	files := parseDiff(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Type != DiffRename {
		t.Errorf("expected DiffRename, got %v", files[0].Type)
	}
	if files[0].RenameTo != "new_name.go" {
		t.Errorf("expected rename to %q, got %q", "new_name.go", files[0].RenameTo)
	}
}

func TestParseDiff_NewFile(t *testing.T) {
	raw := `diff --git a/brand_new.go b/brand_new.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/brand_new.go
@@ -0,0 +1,5 @@
+package main
+
+func brandNew() {
+	return
+}
`

	files := parseDiff(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Name != "brand_new.go" {
		t.Errorf("expected name %q, got %q", "brand_new.go", files[0].Name)
	}
	if files[0].Type != DiffNew {
		t.Errorf("expected DiffNew, got %v", files[0].Type)
	}
	if len(files[0].Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(files[0].Hunks))
	}
	if len(files[0].Hunks[0].Lines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(files[0].Hunks[0].Lines))
	}
	for i, line := range files[0].Hunks[0].Lines {
		if line.Type != DiffLineAdded {
			t.Errorf("line %d: expected added, got %v", i, line.Type)
		}
	}
}

func TestParseDiff_NoContextInHunkHeader(t *testing.T) {
	raw := `diff --git a/data.txt b/data.txt
index abc1234..def5678 100644
--- a/data.txt
+++ b/data.txt
@@ -1,3 +1,4 @@
 line one
 line two
+line three
 line four
`

	files := parseDiff(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if len(files[0].Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(files[0].Hunks))
	}
	if files[0].Hunks[0].Context != "" {
		t.Errorf("expected empty context, got %q", files[0].Hunks[0].Context)
	}
}

func TestParseDiff_MultipleFiles(t *testing.T) {
	raw := `diff --git a/file1.go b/file1.go
index abc1234..def5678 100644
--- a/file1.go
+++ b/file1.go
@@ -1,3 +1,3 @@ func one() {
 	a := 1
-	b := 2
+	b := 3
diff --git a/file2.go b/file2.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/file2.go
@@ -0,0 +1,2 @@
+package main
+func two() {}
`

	files := parseDiff(raw)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Name != "file1.go" {
		t.Errorf("file 0: expected %q, got %q", "file1.go", files[0].Name)
	}
	if files[0].Type != DiffNormal {
		t.Errorf("file 0: expected DiffNormal, got %v", files[0].Type)
	}
	if files[1].Name != "file2.go" {
		t.Errorf("file 1: expected %q, got %q", "file2.go", files[1].Name)
	}
	if files[1].Type != DiffNew {
		t.Errorf("file 1: expected DiffNew, got %v", files[1].Type)
	}
}

func TestParseDiff_EmptyInput(t *testing.T) {
	files := parseDiff("")
	if len(files) != 0 {
		t.Errorf("expected 0 files for empty input, got %d", len(files))
	}
}

func TestParseDiff_FilenameWithSpaces(t *testing.T) {
	raw := `diff --git a/path with spaces/file.go b/path with spaces/file.go
index abc1234..def5678 100644
--- a/path with spaces/file.go
+++ b/path with spaces/file.go
@@ -1,3 +1,3 @@ func spaced() {
 	a := 1
-	b := 2
+	b := 3
`

	files := parseDiff(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Name != "path with spaces/file.go" {
		t.Errorf("expected %q, got %q", "path with spaces/file.go", files[0].Name)
	}
}
