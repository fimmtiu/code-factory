package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// PicklistItem is one candidate in a Picklist's suggestion pool.
// ID is the value returned by PickedIDs; Label is what the user sees and types
// against. When Label is empty, ID is used for display.
type PicklistItem struct {
	ID    string
	Label string
}

func (it PicklistItem) display() string {
	if it.Label != "" {
		return it.Label
	}
	return it.ID
}

// Picklist is a reusable "chip input" widget: a single-line text input with
// an autocomplete dropdown, plus a list of chips for the items already picked.
// The Picklist does not own focus or decide when to render itself — callers
// drive it via Update/View and decide when to forward key messages.
type Picklist struct {
	items    []PicklistItem
	byID     map[string]PicklistItem
	picked   []string // order of insertion; each entry is an item ID
	pickedOK map[string]struct{}

	query       []rune
	queryCursor int
	highlight   int // selected row in the filtered suggestions; clamped on every filter
	maxDropdown int // max rows in the dropdown; 0 = default 5
	width       int // display width for the input and dropdown

	focused bool
}

// NewPicklist builds an empty Picklist over the given candidate items. Items
// are stored in the order provided; suggestions are ranked by simple substring
// match against Label (falling back to ID).
func NewPicklist(items []PicklistItem, width int) Picklist {
	byID := make(map[string]PicklistItem, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}
	if width < 10 {
		width = 10
	}
	return Picklist{
		items:       items,
		byID:        byID,
		pickedOK:    map[string]struct{}{},
		width:       width,
		maxDropdown: 5,
	}
}

// SetFocused toggles the widget's focus state. The widget renders differently
// when focused (cursor block visible, suggestions shown).
func (p *Picklist) SetFocused(f bool) { p.focused = f }

// SetWidth updates the display width for the input field and dropdown.
func (p *Picklist) SetWidth(w int) {
	if w < 10 {
		w = 10
	}
	p.width = w
}

// SetMaxDropdownRows bounds the number of suggestion rows shown.
func (p *Picklist) SetMaxDropdownRows(n int) {
	if n < 1 {
		n = 1
	}
	p.maxDropdown = n
}

// PickedIDs returns the IDs of the currently picked items in insertion order.
func (p Picklist) PickedIDs() []string {
	out := make([]string, len(p.picked))
	copy(out, p.picked)
	return out
}

// AddPicked marks an item as already picked, in insertion order. Unknown IDs
// are silently ignored.
func (p *Picklist) AddPicked(id string) {
	if _, ok := p.byID[id]; !ok {
		return
	}
	if _, dup := p.pickedOK[id]; dup {
		return
	}
	p.picked = append(p.picked, id)
	p.pickedOK[id] = struct{}{}
}

// filtered returns the items matching the current query, excluding ones
// already picked.
func (p Picklist) filtered() []PicklistItem {
	q := strings.ToLower(string(p.query))
	var out []PicklistItem
	for _, it := range p.items {
		if _, taken := p.pickedOK[it.ID]; taken {
			continue
		}
		if q == "" {
			out = append(out, it)
			continue
		}
		if strings.Contains(strings.ToLower(it.display()), q) ||
			strings.Contains(strings.ToLower(it.ID), q) {
			out = append(out, it)
		}
	}
	return out
}

// Update handles a key message. It returns true when the message was consumed;
// the caller should only forward further messages to the Picklist when it is
// focused and returns true.
func (p *Picklist) Update(msg tea.KeyMsg) bool {
	if !p.focused {
		return false
	}

	switch msg.String() {
	case "up":
		if p.highlight > 0 {
			p.highlight--
		}
		return true
	case "down":
		if max := len(p.filtered()); p.highlight < max-1 {
			p.highlight++
		}
		return true
	case "enter":
		fp := p.filtered()
		if len(fp) == 0 || p.highlight >= len(fp) {
			return true
		}
		p.picked = append(p.picked, fp[p.highlight].ID)
		p.pickedOK[fp[p.highlight].ID] = struct{}{}
		p.query = nil
		p.queryCursor = 0
		p.highlight = 0
		return true
	case "backspace":
		if p.queryCursor > 0 {
			p.query = append(p.query[:p.queryCursor-1], p.query[p.queryCursor:]...)
			p.queryCursor--
			p.clampHighlight()
			return true
		}
		if len(p.picked) > 0 {
			removed := p.picked[len(p.picked)-1]
			p.picked = p.picked[:len(p.picked)-1]
			delete(p.pickedOK, removed)
			p.clampHighlight()
		}
		return true
	case "delete":
		if p.queryCursor < len(p.query) {
			p.query = append(p.query[:p.queryCursor], p.query[p.queryCursor+1:]...)
			p.clampHighlight()
		}
		return true
	case "left":
		if p.queryCursor > 0 {
			p.queryCursor--
		}
		return true
	case "right":
		if p.queryCursor < len(p.query) {
			p.queryCursor++
		}
		return true
	case "home", "ctrl+a":
		p.queryCursor = 0
		return true
	case "end", "ctrl+e":
		p.queryCursor = len(p.query)
		return true
	default:
		if len(msg.Runes) > 0 {
			p.query = append(p.query[:p.queryCursor], append(append([]rune{}, msg.Runes...), p.query[p.queryCursor:]...)...)
			p.queryCursor += len(msg.Runes)
			p.highlight = 0
			return true
		}
	}
	return false
}

func (p *Picklist) clampHighlight() {
	if max := len(p.filtered()); p.highlight >= max {
		if max == 0 {
			p.highlight = 0
		} else {
			p.highlight = max - 1
		}
	}
	if p.highlight < 0 {
		p.highlight = 0
	}
}

// View renders the picklist: an input box on top, a suggestion dropdown
// (only while focused), and a list of chips below. Width is fixed to the
// value passed via SetWidth / NewPicklist; the caller can place it inside
// its own dialog frame.
func (p Picklist) View() string {
	var b strings.Builder

	b.WriteString(p.renderInput())
	b.WriteString("\n")
	if p.focused {
		b.WriteString(p.renderDropdown())
		b.WriteString("\n")
	}
	b.WriteString(p.renderChips())
	return strings.TrimRight(b.String(), "\n")
}

func (p Picklist) renderInput() string {
	style := theme.Current().QuickResponseInputStyle
	if p.focused {
		style = style.BorderForeground(lipgloss.Color("63"))
	}
	inner := p.width - style.GetHorizontalFrameSize()
	if inner < 4 {
		inner = 4
	}

	text := string(p.query)
	if p.focused {
		text = withPicklistCursor(text, p.queryCursor)
	}
	if lipgloss.Width(text) < inner {
		text += strings.Repeat(" ", inner-lipgloss.Width(text))
	}
	return style.Width(inner).Render(text)
}

func withPicklistCursor(s string, pos int) string {
	rs := []rune(s)
	if pos >= len(rs) {
		return s + cursorStyle.Render(" ")
	}
	return string(rs[:pos]) + cursorStyle.Render(string(rs[pos])) + string(rs[pos+1:])
}

func (p Picklist) renderDropdown() string {
	fp := p.filtered()
	if len(fp) == 0 {
		return theme.Current().DetailLabelStyle.Render("  (no matches)")
	}
	n := p.maxDropdown
	if n == 0 {
		n = 5
	}
	if n > len(fp) {
		n = len(fp)
	}

	// Scroll window around the highlight.
	start := 0
	if p.highlight >= n {
		start = p.highlight - n + 1
	}
	end := start + n
	if end > len(fp) {
		end = len(fp)
		start = end - n
		if start < 0 {
			start = 0
		}
	}

	var lines []string
	for i := start; i < end; i++ {
		label := "  " + fp[i].display()
		if lipgloss.Width(label) > p.width-2 {
			rr := []rune(label)
			label = string(rr[:p.width-3]) + "…"
		}
		if i == p.highlight {
			label = theme.Current().TreeSelectedStyle.Render("▶ " + fp[i].display())
		}
		lines = append(lines, label)
	}
	return strings.Join(lines, "\n")
}

func (p Picklist) renderChips() string {
	if len(p.picked) == 0 {
		return theme.Current().DetailLabelStyle.Render("(none)")
	}
	chipStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("240")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1)

	parts := make([]string, 0, len(p.picked))
	for _, id := range p.picked {
		it, ok := p.byID[id]
		if !ok {
			it = PicklistItem{ID: id}
		}
		parts = append(parts, chipStyle.Render(it.display()))
	}
	// Simple space-join; callers that need wrapping can set a small width.
	return strings.Join(parts, " ")
}
