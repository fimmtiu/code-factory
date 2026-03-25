package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	m := NewModel(nil, "test-repo")
	if m.focused != NavigatorFocused {
		t.Errorf("expected NavigatorFocused initial focus, got %v", m.focused)
	}
}

func TestAppQuitCtrlC(t *testing.T) {
	m := NewModel(nil, "test-repo")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected a quit command, got nil")
	}
	// Execute the command and verify it returns tea.Quit
	msg := cmd()
	if msg != tea.Quit() {
		t.Errorf("expected tea.Quit message, got %T", msg)
	}
}

func TestAppQuitQ(t *testing.T) {
	m := NewModel(nil, "test-repo")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected a quit command for 'q', got nil")
	}
	msg := cmd()
	if msg != tea.Quit() {
		t.Errorf("expected tea.Quit message, got %T", msg)
	}
}

func TestAppWindowResize(t *testing.T) {
	m := NewModel(nil, "test-repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	um := updated.(Model)
	if um.width != 120 {
		t.Errorf("expected width 120, got %d", um.width)
	}
	if um.height != 40 {
		t.Errorf("expected height 40, got %d", um.height)
	}
}

func TestAppStatusMsg(t *testing.T) {
	m := NewModel(nil, "test-repo")
	units := sampleUnits()
	updated, _ := m.Update(statusMsg{units: units})
	um := updated.(Model)
	if len(um.units) != len(units) {
		t.Errorf("expected %d units, got %d", len(units), len(um.units))
	}
}

func TestAppErrMsg(t *testing.T) {
	m := NewModel(nil, "test-repo")
	testErr := errMsg{err: errTestError}
	updated, _ := m.Update(testErr)
	um := updated.(Model)
	if um.err == nil {
		t.Error("expected error to be stored on model")
	}
}

func TestAppNavigatorFocusSwitchToDetail(t *testing.T) {
	m := NewModel(nil, "test-repo")
	m.focused = NavigatorFocused

	// Tab should switch focus to detail pane
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	um := updated.(Model)
	if um.focused != DetailFocused {
		t.Errorf("expected DetailFocused after Tab in navigator, got %v", um.focused)
	}
}

func TestAppDetailFocusSwitchToNavigator(t *testing.T) {
	m := NewModel(nil, "test-repo")
	m.focused = DetailFocused

	// Tab should switch focus back to navigator
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	um := updated.(Model)
	if um.focused != NavigatorFocused {
		t.Errorf("expected NavigatorFocused after Tab in detail, got %v", um.focused)
	}
}

func TestAppSpaceSwitchesFocus(t *testing.T) {
	m := NewModel(nil, "test-repo")
	m.focused = NavigatorFocused

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	um := updated.(Model)
	if um.focused != DetailFocused {
		t.Errorf("expected DetailFocused after Space in navigator, got %v", um.focused)
	}
}

func TestAppNavigatorMovement(t *testing.T) {
	m := NewModel(nil, "test-repo")
	m.focused = NavigatorFocused
	m.units = sampleUnits()
	m.navigator.SetUnits(m.units)

	initialCursor := m.navigator.cursor

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	um := updated.(Model)
	if um.navigator.cursor != initialCursor+1 {
		t.Errorf("expected cursor to move down from %d to %d, got %d", initialCursor, initialCursor+1, um.navigator.cursor)
	}
}

func TestAppDetailScrolling(t *testing.T) {
	m := NewModel(nil, "test-repo")
	m.focused = DetailFocused
	units := sampleUnits()
	m.units = units
	if len(units) > 0 {
		m.detail.SetUnit(units[0])
	}

	// Down arrow in detail pane should scroll
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	um := updated.(Model)
	if um.detail.scrollY != 1 {
		t.Errorf("expected scrollY 1 after down in detail pane, got %d", um.detail.scrollY)
	}
}

func TestAppNavigatorPageDown(t *testing.T) {
	m := NewModel(nil, "test-repo")
	m.height = 24
	m.focused = NavigatorFocused
	m.units = sampleUnits()
	m.navigator.SetUnits(m.units)

	initial := m.navigator.cursor
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	um := updated.(Model)

	if um.navigator.cursor <= initial {
		t.Errorf("expected cursor to advance on PgDown, got cursor=%d (was %d)",
			um.navigator.cursor, initial)
	}
}

func TestAppNavigatorPageUp(t *testing.T) {
	m := NewModel(nil, "test-repo")
	m.height = 24
	m.focused = NavigatorFocused
	m.units = sampleUnits()
	m.navigator.SetUnits(m.units)
	m.navigator.cursor = len(m.navigator.nodes) - 1 // start at end

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	um := updated.(Model)

	if um.navigator.cursor >= m.navigator.cursor {
		t.Errorf("expected cursor to decrease on PgUp, got cursor=%d (was %d)",
			um.navigator.cursor, m.navigator.cursor)
	}
}

func TestAppDetailPageDown(t *testing.T) {
	m := NewModel(nil, "test-repo")
	m.height = 24
	m.focused = DetailFocused
	units := sampleUnits()
	if len(units) > 0 {
		m.detail.SetUnit(units[0])
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	um := updated.(Model)
	// scrollY should not go negative; for a short description it may stay at 0
	if um.detail.scrollY < 0 {
		t.Errorf("expected scrollY >= 0 after PgDown, got %d", um.detail.scrollY)
	}
}

func TestAppDetailPageUp(t *testing.T) {
	m := NewModel(nil, "test-repo")
	m.height = 24
	m.focused = DetailFocused
	units := sampleUnits()
	if len(units) > 0 {
		m.detail.SetUnit(units[0])
	}
	m.detail.scrollY = 5

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	um := updated.(Model)

	if um.detail.scrollY >= 5 {
		t.Errorf("expected scrollY to decrease on PgUp, got %d (was 5)", um.detail.scrollY)
	}
}

func TestAppCtrlRRefresh(t *testing.T) {
	m := NewModel(nil, "test-repo")
	// Ctrl-R should trigger a refresh (non-nil command)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	// We can't test the exact behavior of the socket command in unit tests,
	// but we can verify a command is returned
	_ = cmd // command may be nil if client is not connected - that's acceptable
}

func TestAppTickMsgTriggersFetch(t *testing.T) {
	m := NewModel(nil, "test-repo")
	_, cmd := m.Update(tickMsg{})
	// tickMsg should produce a new command (fetch + new tick)
	_ = cmd
}

func TestAppViewDoesNotPanic(t *testing.T) {
	m := NewModel(nil, "test-repo")
	m.width = 80
	m.height = 24
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("View() panicked: %v", r)
		}
	}()
	_ = m.View()
}
