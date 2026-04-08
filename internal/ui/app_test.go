package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel_HasFiveViews(t *testing.T) {
	m := NewModel(nil, nil, 5)
	for i := 0; i < 5; i++ {
		if m.views[i] == nil {
			t.Errorf("views[%d] is nil", i)
		}
	}
}

func TestNewModel_ViewDiffIsDiffView(t *testing.T) {
	m := NewModel(nil, nil, 5)
	if _, ok := m.views[ViewDiff].(DiffView); !ok {
		t.Errorf("views[ViewDiff] is %T, want DiffView", m.views[ViewDiff])
	}
}

func TestF5SwitchesToDiffsView(t *testing.T) {
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40
	// Ensure we start on a different view.
	m.activeView = ViewProject

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF5})
	model := updated.(Model)
	if model.activeView != ViewDiff {
		t.Errorf("after F5, activeView = %d, want %d (ViewDiff)", model.activeView, ViewDiff)
	}
}

func TestRenderHeader_ShowsFiveTabs(t *testing.T) {
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40
	header := m.renderHeader()
	for _, label := range []string{"F1:", "F2:", "F3:", "F4:", "F5:"} {
		if !strings.Contains(header, label) {
			t.Errorf("header missing %q tab label", label)
		}
	}
}

func TestRenderHeader_ShowsDiffsTab(t *testing.T) {
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40
	header := m.renderHeader()
	if !strings.Contains(header, "Diffs") {
		t.Error("header missing 'Diffs' tab name")
	}
}

func TestGlobalKeyBindings_ContainsF5(t *testing.T) {
	found := false
	for _, kb := range globalKeyBindings {
		if kb.Key == "F5" {
			found = true
			break
		}
	}
	if !found {
		t.Error("globalKeyBindings missing F5 entry")
	}
}
