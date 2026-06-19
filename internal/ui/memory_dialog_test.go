package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// sendKey feeds one key string to a MemoryDialog and returns the updated dialog
// plus any command produced.
func sendKey(d MemoryDialog, key string) (MemoryDialog, tea.Cmd) {
	var msg tea.KeyMsg
	switch key {
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, cmd := d.Update(msg)
	return updated.(MemoryDialog), cmd
}

func TestMemoryDialog_AddCreatesRepositoryWideMemory(t *testing.T) {
	d := openUITestDB(t)
	dlg := NewMemoryDialog(d, nil, 80)

	dlg, _ = sendKey(dlg, "down") // kind: lesson -> pattern
	dlg, _ = sendKey(dlg, "tab")  // focus text area
	dlg, _ = sendKey(dlg, "a new lesson body")
	dlg, _ = sendKey(dlg, "tab") // -> cancel
	dlg, _ = sendKey(dlg, "tab") // -> OK
	_, cmd := sendKey(dlg, "enter")

	var saved *memorySavedMsg
	for _, m := range runMsgChain(cmd) {
		if sm, ok := m.(memorySavedMsg); ok {
			saved = &sm
		}
	}
	if saved == nil {
		t.Fatalf("OK did not emit memorySavedMsg")
	}
	if saved.err != nil {
		t.Fatalf("save failed: %v", saved.err)
	}
	if saved.edited {
		t.Errorf("add should report edited=false")
	}

	all, err := d.ListMemories()
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(all))
	}
	if all[0].Kind != "pattern" || all[0].Text != "a new lesson body" {
		t.Errorf("unexpected memory: kind=%q text=%q", all[0].Kind, all[0].Text)
	}
	if all[0].Scope != "" {
		t.Errorf("new memory should be repository-wide, got scope %q", all[0].Scope)
	}
}

func TestMemoryDialog_EditChangesKindKeepingScopeAndText(t *testing.T) {
	d := openUITestDB(t)
	if _, err := d.AddMemory("proj/server", "lesson", "keep this text", "T-1"); err != nil {
		t.Fatalf("AddMemory: %v", err)
	}
	all, _ := d.ListMemories()
	existing := all[0]

	dlg := NewMemoryDialog(d, &existing, 80)
	// Kind should start on the memory's current kind.
	if memoryKinds[dlg.kindIdx] != "lesson" {
		t.Fatalf("edit dialog started on kind %q, want lesson", memoryKinds[dlg.kindIdx])
	}

	dlg, _ = sendKey(dlg, "down") // lesson -> pattern
	dlg, _ = sendKey(dlg, "down") // pattern -> gotcha
	dlg, _ = sendKey(dlg, "tab")  // -> text
	dlg, _ = sendKey(dlg, "tab")  // -> cancel
	dlg, _ = sendKey(dlg, "tab")  // -> OK
	_, cmd := sendKey(dlg, "enter")

	var saved *memorySavedMsg
	for _, m := range runMsgChain(cmd) {
		if sm, ok := m.(memorySavedMsg); ok {
			saved = &sm
		}
	}
	if saved == nil || saved.err != nil || !saved.edited {
		t.Fatalf("expected successful edit, got %+v", saved)
	}

	updated, _ := d.ListMemories()
	m := updated[0]
	if m.Kind != "gotcha" {
		t.Errorf("kind = %q, want gotcha", m.Kind)
	}
	if m.Text != "keep this text" {
		t.Errorf("text changed: %q", m.Text)
	}
	if m.Scope != "proj/server" {
		t.Errorf("scope changed: %q", m.Scope)
	}
}

func TestMemoryDialog_EmptyTextShowsErrorAndDoesNotSave(t *testing.T) {
	d := openUITestDB(t)
	dlg := NewMemoryDialog(d, nil, 80)

	dlg, _ = sendKey(dlg, "tab") // -> text (left empty)
	dlg, _ = sendKey(dlg, "tab") // -> cancel
	dlg, _ = sendKey(dlg, "tab") // -> OK
	dlg, cmd := sendKey(dlg, "enter")

	if dlg.errMsg == "" {
		t.Error("expected an error message for empty text")
	}
	for _, m := range runMsgChain(cmd) {
		if _, ok := m.(memorySavedMsg); ok {
			t.Error("empty submit should not emit memorySavedMsg")
		}
	}
	if all, _ := d.ListMemories(); len(all) != 0 {
		t.Errorf("expected no memory saved, got %d", len(all))
	}
}
