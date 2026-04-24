package ui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func key(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func TestPicklist_FilteringIsCaseInsensitive(t *testing.T) {
	p := NewPicklist([]PicklistItem{
		{ID: "alpha"}, {ID: "Beta"}, {ID: "gamma"},
	}, 40)
	p.SetFocused(true)
	p.Update(keyRunes("BE"))

	got := p.filtered()
	if len(got) != 1 || got[0].ID != "Beta" {
		t.Fatalf("filtered() = %+v, want single Beta", got)
	}
}

func TestPicklist_EnterAddsHighlightedItemAndClearsQuery(t *testing.T) {
	p := NewPicklist([]PicklistItem{{ID: "alpha"}, {ID: "beta"}}, 40)
	p.SetFocused(true)
	p.Update(keyRunes("be"))
	p.Update(key(tea.KeyEnter))

	if ids := p.PickedIDs(); !reflect.DeepEqual(ids, []string{"beta"}) {
		t.Fatalf("PickedIDs() = %v, want [beta]", ids)
	}
	if string(p.query) != "" {
		t.Fatalf("query = %q, want empty", string(p.query))
	}
	// Adding again should be a no-op.
	p.Update(keyRunes("beta"))
	p.Update(key(tea.KeyEnter))
	if ids := p.PickedIDs(); !reflect.DeepEqual(ids, []string{"beta"}) {
		t.Fatalf("duplicate add: PickedIDs() = %v, want [beta]", ids)
	}
}

func TestPicklist_BackspaceOnEmptyQueryRemovesLastChip(t *testing.T) {
	p := NewPicklist([]PicklistItem{{ID: "alpha"}, {ID: "beta"}}, 40)
	p.SetFocused(true)
	p.AddPicked("alpha")
	p.AddPicked("beta")
	p.Update(key(tea.KeyBackspace))
	if ids := p.PickedIDs(); !reflect.DeepEqual(ids, []string{"alpha"}) {
		t.Fatalf("after backspace: %v, want [alpha]", ids)
	}
}

func TestPicklist_AlreadyPickedItemsAreExcludedFromSuggestions(t *testing.T) {
	p := NewPicklist([]PicklistItem{{ID: "alpha"}, {ID: "beta"}}, 40)
	p.SetFocused(true)
	p.AddPicked("alpha")
	got := p.filtered()
	if len(got) != 1 || got[0].ID != "beta" {
		t.Fatalf("filtered() = %v, want [beta]", got)
	}
}

func TestPicklist_UpDownAdjustsHighlight(t *testing.T) {
	p := NewPicklist([]PicklistItem{{ID: "a"}, {ID: "b"}, {ID: "c"}}, 40)
	p.SetFocused(true)
	p.Update(key(tea.KeyDown))
	p.Update(key(tea.KeyDown))
	if p.highlight != 2 {
		t.Fatalf("highlight = %d, want 2", p.highlight)
	}
	p.Update(key(tea.KeyUp))
	if p.highlight != 1 {
		t.Fatalf("highlight after up = %d, want 1", p.highlight)
	}
}

func TestPicklist_ViewShowsChipsAndDropdownOnlyWhenFocused(t *testing.T) {
	p := NewPicklist([]PicklistItem{{ID: "alpha"}, {ID: "beta"}}, 40)
	p.AddPicked("alpha")

	blurred := p.View()
	if !strings.Contains(blurred, "alpha") {
		t.Errorf("blurred view missing picked chip: %q", blurred)
	}
	if strings.Contains(blurred, "▶") {
		t.Errorf("blurred view unexpectedly shows dropdown selection: %q", blurred)
	}

	p.SetFocused(true)
	focused := p.View()
	if !strings.Contains(focused, "beta") {
		t.Errorf("focused view missing suggestion: %q", focused)
	}
}
