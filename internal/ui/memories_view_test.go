package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/code-factory/internal/db"
)

// seedMemoriesView builds a MemoriesView populated from the database and sized,
// returning the view ready for interaction.
func seedMemoriesView(t *testing.T, d *db.DB) MemoriesView {
	t.Helper()
	v := NewMemoriesView(d)
	updated, _ := v.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	v = updated.(MemoriesView)
	msgs := runMsgChain(v.Init())
	for _, msg := range msgs {
		updated, _ = v.Update(msg)
		v = updated.(MemoriesView)
	}
	return v
}

func TestMemoriesView_LoadsNewestFirstAndSelectsTop(t *testing.T) {
	d := openUITestDB(t)
	if _, err := d.AddMemory("", "lesson", "first memory", "T-1"); err != nil {
		t.Fatalf("AddMemory: %v", err)
	}
	if _, err := d.AddMemory("proj", "gotcha", "second memory", "T-2"); err != nil {
		t.Fatalf("AddMemory: %v", err)
	}

	v := seedMemoriesView(t, d)

	if len(v.memories) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(v.memories))
	}
	// ListMemories returns newest-first, so the second insert is on top.
	if v.memories[0].Text != "second memory" {
		t.Errorf("top memory = %q, want %q", v.memories[0].Text, "second memory")
	}
	if v.selected != 0 {
		t.Errorf("selected = %d, want 0 (top item)", v.selected)
	}
}

func TestMemoriesView_EmptyStateInDetailPane(t *testing.T) {
	d := openUITestDB(t)
	v := seedMemoriesView(t, d)

	out := v.View()
	if !strings.Contains(out, "No memories") {
		t.Errorf("expected empty-state %q in view, got:\n%s", "No memories", out)
	}
}

func TestMemoriesView_DetailShowsHeaderAndText(t *testing.T) {
	d := openUITestDB(t)
	if _, err := d.AddMemory("proj-x", "pattern", "the body text", "T-9"); err != nil {
		t.Fatalf("AddMemory: %v", err)
	}
	v := seedMemoriesView(t, d)

	lines := v.detailLines(v.detailInnerWidth())
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"pattern", "proj-x", "T-9", "the body text"} {
		if !strings.Contains(joined, want) {
			t.Errorf("detail lines missing %q; got:\n%s", want, joined)
		}
	}
	// A blank separator line precedes the body text.
	if len(lines) < 5 || lines[3] != "" {
		t.Errorf("expected blank separator at line index 3, got lines: %#v", lines)
	}
}

func TestMemoriesView_LeftRightSwitchesFocus(t *testing.T) {
	d := openUITestDB(t)
	if _, err := d.AddMemory("", "note", "x", "T-1"); err != nil {
		t.Fatalf("AddMemory: %v", err)
	}
	v := seedMemoriesView(t, d)

	if v.focus != memFocusList {
		t.Fatalf("initial focus = %v, want list", v.focus)
	}
	updated, _ := v.Update(tea.KeyMsg{Type: tea.KeyRight})
	v = updated.(MemoriesView)
	if v.focus != memFocusDetail {
		t.Errorf("after right, focus = %v, want detail", v.focus)
	}
	updated, _ = v.Update(tea.KeyMsg{Type: tea.KeyLeft})
	v = updated.(MemoriesView)
	if v.focus != memFocusList {
		t.Errorf("after left, focus = %v, want list", v.focus)
	}
}

func TestMemoriesView_UpDownMovesSelectionInListFocus(t *testing.T) {
	d := openUITestDB(t)
	for _, txt := range []string{"a", "b", "c"} {
		if _, err := d.AddMemory("", "note", txt, "T"); err != nil {
			t.Fatalf("AddMemory: %v", err)
		}
	}
	v := seedMemoriesView(t, d)

	updated, _ := v.Update(tea.KeyMsg{Type: tea.KeyDown})
	v = updated.(MemoriesView)
	if v.selected != 1 {
		t.Errorf("after down, selected = %d, want 1", v.selected)
	}
	updated, _ = v.Update(tea.KeyMsg{Type: tea.KeyUp})
	v = updated.(MemoriesView)
	if v.selected != 0 {
		t.Errorf("after up, selected = %d, want 0", v.selected)
	}
}

func TestMemoriesView_AddKeyOpensDialogEvenWhenEmpty(t *testing.T) {
	d := openUITestDB(t)
	v := seedMemoriesView(t, d)
	if len(v.memories) != 0 {
		t.Fatalf("expected an empty view, got %d memories", len(v.memories))
	}

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")})
	var open *openMemoryDialogMsg
	for _, m := range runMsgChain(cmd) {
		if om, ok := m.(openMemoryDialogMsg); ok {
			open = &om
		}
	}
	if open == nil {
		t.Fatalf("A did not emit openMemoryDialogMsg")
	}
	if open.existing != nil {
		t.Errorf("add dialog should carry no existing memory, got %+v", open.existing)
	}
}

func TestMemoriesView_EditKeyOpensDialogForSelected(t *testing.T) {
	d := openUITestDB(t)
	if _, err := d.AddMemory("", "lesson", "editable", "T-1"); err != nil {
		t.Fatalf("AddMemory: %v", err)
	}
	v := seedMemoriesView(t, d)

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
	var open *openMemoryDialogMsg
	for _, m := range runMsgChain(cmd) {
		if om, ok := m.(openMemoryDialogMsg); ok {
			open = &om
		}
	}
	if open == nil {
		t.Fatalf("E did not emit openMemoryDialogMsg")
	}
	if open.existing == nil || open.existing.Text != "editable" {
		t.Errorf("edit dialog should carry the selected memory, got %+v", open.existing)
	}
}

func TestMemoriesView_DeleteKeyOpensDialogThenDeletes(t *testing.T) {
	d := openUITestDB(t)
	id, err := d.AddMemory("", "lesson", "doomed", "T-1")
	if err != nil {
		t.Fatalf("AddMemory: %v", err)
	}
	v := seedMemoriesView(t, d)

	// Pressing X emits a request to open the confirmation dialog.
	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	msgs := runMsgChain(cmd)
	var open *openDeleteMemoryDialogMsg
	for _, m := range msgs {
		if dm, ok := m.(openDeleteMemoryDialogMsg); ok {
			open = &dm
		}
	}
	if open == nil {
		t.Fatalf("X did not emit openDeleteMemoryDialogMsg; got %#v", msgs)
	}
	if open.id != id {
		t.Errorf("dialog id = %d, want %d", open.id, id)
	}

	// Confirming in the dialog deletes the memory.
	dlg := NewDeleteMemoryDialog(d, open.id, open.label)
	updatedDlg, _ := dlg.Update(tea.KeyMsg{Type: tea.KeyRight}) // focus Delete
	dlg = updatedDlg.(DeleteMemoryDialog)
	_, dcmd := dlg.Update(tea.KeyMsg{Type: tea.KeyEnter})
	for _, m := range runMsgChain(dcmd) {
		if dm, ok := m.(memoryDeletedMsg); ok && dm.err != nil {
			t.Fatalf("delete failed: %v", dm.err)
		}
	}

	remaining, err := d.ListMemories()
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 memories after delete, got %d", len(remaining))
	}
}
