package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/models"
)

// ── Styles (bold/reverse only — works on light and dark terminals) ─────────────

var (
	wizBold     = lipgloss.NewStyle().Bold(true)
	wizDim      = lipgloss.NewStyle().Faint(true)
	wizSelected = lipgloss.NewStyle().Bold(true).Reverse(true)
	wizError    = lipgloss.NewStyle().Bold(true)
)

// ── Step enum ─────────────────────────────────────────────────────────────────

type wizardStep int

const (
	stepName wizardStep = iota
	stepParent
	stepDescription
)

// ── Model ─────────────────────────────────────────────────────────────────────

type wizardModel struct {
	kind string // "ticket" or "project"
	step wizardStep

	// Step 1 — name
	nameText string
	nameCur  int
	nameErr  string

	// Step 2 — parent selection
	projects  []*models.WorkUnit
	filter    string
	filterCur int
	listSel   int // 0 = None; 1..N = index into filtered list
	listOff   int // scroll offset

	// Step 3 — description
	descText string
	descCur  int
	descOff  int // first visible line index

	// Terminal size
	width  int
	height int

	// Completion
	cancelled bool
	confirmed bool
}

func newWizard(kind string, projects []*models.WorkUnit) wizardModel {
	return wizardModel{kind: kind, projects: projects}
}

func (m wizardModel) Init() tea.Cmd { return nil }

// ── Rune-slice helpers ────────────────────────────────────────────────────────

func runesInsert(s string, pos int, r []rune) (string, int) {
	rs := []rune(s)
	rs = append(rs[:pos], append(r, rs[pos:]...)...)
	return string(rs), pos + len(r)
}

func runesBackspace(s string, pos int) (string, int) {
	rs := []rune(s)
	if pos <= 0 {
		return s, 0
	}
	rs = append(rs[:pos-1], rs[pos:]...)
	return string(rs), pos - 1
}

func runesDelete(s string, pos int) string {
	rs := []rune(s)
	if pos >= len(rs) {
		return s
	}
	return string(append(rs[:pos], rs[pos+1:]...))
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch m.step {
		case stepName:
			return m.updateName(msg)
		case stepParent:
			return m.updateParent(msg)
		case stepDescription:
			return m.updateDesc(msg)
		}
	}
	return m, nil
}

// ── Step 1: Name ──────────────────────────────────────────────────────────────

func (m wizardModel) updateName(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.cancelled = true
		return m, tea.Quit
	case "enter":
		name := strings.TrimSpace(m.nameText)
		if err := models.ValidateIdentifier(name); err != nil {
			m.nameErr = err.Error()
			return m, nil
		}
		m.nameText = name
		m.nameErr = ""
		m.step = stepParent
	case "backspace":
		m.nameText, m.nameCur = runesBackspace(m.nameText, m.nameCur)
	case "delete":
		m.nameText = runesDelete(m.nameText, m.nameCur)
	case "left":
		if m.nameCur > 0 {
			m.nameCur--
		}
	case "right":
		if m.nameCur < len([]rune(m.nameText)) {
			m.nameCur++
		}
	case "home", "ctrl+a":
		m.nameCur = 0
	case "end", "ctrl+e":
		m.nameCur = len([]rune(m.nameText))
	default:
		if len(msg.Runes) > 0 {
			m.nameText, m.nameCur = runesInsert(m.nameText, m.nameCur, msg.Runes)
		}
	}
	return m, nil
}

// ── Step 2: Parent ────────────────────────────────────────────────────────────

func (m wizardModel) filteredProjects() []*models.WorkUnit {
	if m.filter == "" {
		return m.projects
	}
	low := strings.ToLower(m.filter)
	var out []*models.WorkUnit
	for _, p := range m.projects {
		if strings.Contains(strings.ToLower(p.Identifier), low) {
			out = append(out, p)
		}
	}
	return out
}

func (m wizardModel) listLen() int { return 1 + len(m.filteredProjects()) }

func (m wizardModel) selectedParent() string {
	if m.listSel == 0 {
		return ""
	}
	fp := m.filteredProjects()
	if idx := m.listSel - 1; idx < len(fp) {
		return fp[idx].Identifier
	}
	return ""
}

func (m wizardModel) fullIdentifier() string {
	if p := m.selectedParent(); p != "" {
		return p + "/" + m.nameText
	}
	return m.nameText
}

func (m wizardModel) updateParent(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancelled = true
		return m, tea.Quit
	case "esc":
		m.step = stepName
	case "enter":
		m.step = stepDescription
	case "up":
		if m.listSel > 0 {
			m.listSel--
			m.clampParentScroll()
		}
	case "down":
		if m.listSel < m.listLen()-1 {
			m.listSel++
			m.clampParentScroll()
		}
	case "backspace":
		if m.filterCur > 0 {
			m.filter, m.filterCur = runesBackspace(m.filter, m.filterCur)
			m.clampListSel()
		}
	case "delete":
		if m.filterCur < len([]rune(m.filter)) {
			m.filter = runesDelete(m.filter, m.filterCur)
			m.clampListSel()
		}
	case "left":
		if m.filterCur > 0 {
			m.filterCur--
		}
	case "right":
		if m.filterCur < len([]rune(m.filter)) {
			m.filterCur++
		}
	default:
		if len(msg.Runes) > 0 {
			m.filter, m.filterCur = runesInsert(m.filter, m.filterCur, msg.Runes)
			m.clampListSel()
		}
	}
	return m, nil
}

func (m *wizardModel) clampListSel() {
	if n := m.listLen(); m.listSel >= n {
		m.listSel = n - 1
	}
	m.clampParentScroll()
}

func (m *wizardModel) clampParentScroll() {
	vis := m.parentListVisible()
	if m.listSel < m.listOff {
		m.listOff = m.listSel
	}
	if m.listSel >= m.listOff+vis {
		m.listOff = m.listSel - vis + 1
	}
	if max := m.listLen() - vis; m.listOff > max {
		if max < 0 {
			max = 0
		}
		m.listOff = max
	}
	if m.listOff < 0 {
		m.listOff = 0
	}
}

// parentListVisible returns the number of list rows that fit in the terminal.
// Fixed lines above the list: title(1) + blank(1) + instruction(1) + hints(1)
// + blank(1) + filter(1) + blank(1) = 7.
func (m wizardModel) parentListVisible() int {
	h := m.height - 7
	if h < 3 {
		h = 3
	}
	return h
}

// ── Step 3: Description ───────────────────────────────────────────────────────

func (m wizardModel) updateDesc(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancelled = true
		return m, tea.Quit
	case "esc":
		m.step = stepParent
	case "ctrl+d":
		if strings.TrimSpace(m.descText) != "" {
			m.confirmed = true
			return m, tea.Quit
		}
	case "enter":
		m.descText, m.descCur = runesInsert(m.descText, m.descCur, []rune{'\n'})
		m.clampDescScroll()
	case "backspace":
		m.descText, m.descCur = runesBackspace(m.descText, m.descCur)
		m.clampDescScroll()
	case "delete":
		m.descText = runesDelete(m.descText, m.descCur)
	case "left":
		if m.descCur > 0 {
			m.descCur--
		}
	case "right":
		if m.descCur < len([]rune(m.descText)) {
			m.descCur++
		}
	case "up":
		m.descCur = descMoveLine(m.descText, m.descCur, -1)
		m.clampDescScroll()
	case "down":
		m.descCur = descMoveLine(m.descText, m.descCur, +1)
		m.clampDescScroll()
	case "home", "ctrl+a":
		m.descCur = descLineStart(m.descText, m.descCur)
	case "end", "ctrl+e":
		m.descCur = descLineEnd(m.descText, m.descCur)
	default:
		if len(msg.Runes) > 0 {
			m.descText, m.descCur = runesInsert(m.descText, m.descCur, msg.Runes)
			m.clampDescScroll()
		}
	}
	return m, nil
}

// descCursorPos returns the (line, col) of cur in text (both zero-based).
func descCursorPos(text string, cur int) (line, col int) {
	for i, r := range []rune(text) {
		if i >= cur {
			break
		}
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return
}

// descMoveLine moves cur up (delta=-1) or down (delta=+1) by one line,
// preserving column as closely as possible.
func descMoveLine(text string, cur, delta int) int {
	lines := strings.Split(text, "\n")
	curLine, curCol := descCursorPos(text, cur)
	target := curLine + delta
	if target < 0 {
		target = 0
	}
	if target >= len(lines) {
		target = len(lines) - 1
	}
	col := curCol
	if col > len([]rune(lines[target])) {
		col = len([]rune(lines[target]))
	}
	pos := 0
	for i := 0; i < target; i++ {
		pos += len([]rune(lines[i])) + 1 // +1 for '\n'
	}
	return pos + col
}

func descLineStart(text string, cur int) int {
	rs := []rune(text)
	for i := cur - 1; i >= 0; i-- {
		if rs[i] == '\n' {
			return i + 1
		}
	}
	return 0
}

func descLineEnd(text string, cur int) int {
	rs := []rune(text)
	for i := cur; i < len(rs); i++ {
		if rs[i] == '\n' {
			return i
		}
	}
	return len(rs)
}

func (m *wizardModel) clampDescScroll() {
	line, _ := descCursorPos(m.descText, m.descCur)
	vis := m.descVisible()
	if line < m.descOff {
		m.descOff = line
	}
	if line >= m.descOff+vis {
		m.descOff = line - vis + 1
	}
	if m.descOff < 0 {
		m.descOff = 0
	}
}

// descVisible returns the number of description lines that fit in the terminal.
// Fixed lines above the text area: title(1) + blank(1) + creating(1) + hints(1)
// + blank(1) = 5.
func (m wizardModel) descVisible() int {
	h := m.height - 5
	if h < 3 {
		h = 3
	}
	return h
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m wizardModel) View() string {
	kind := "Ticket"
	if m.kind == "project" {
		kind = "Project"
	}

	var sb strings.Builder
	switch m.step {
	case stepName:
		sb.WriteString(wizBold.Render(fmt.Sprintf("New %s — Step 1 of 3: Name", kind)))
		sb.WriteString("\n\n")
		m.viewName(&sb)
	case stepParent:
		sb.WriteString(wizBold.Render(fmt.Sprintf("New %s — Step 2 of 3: Parent Project", kind)))
		sb.WriteString("\n\n")
		m.viewParent(&sb)
	case stepDescription:
		sb.WriteString(wizBold.Render(fmt.Sprintf("New %s — Step 3 of 3: Description", kind)))
		sb.WriteString("\n\n")
		m.viewDesc(&sb)
	}
	return sb.String()
}

func withCursor(s string, pos int) string {
	rs := []rune(s)
	if pos < len(rs) {
		return string(rs[:pos]) + "█" + string(rs[pos:])
	}
	return s + "█"
}

func (m wizardModel) viewName(sb *strings.Builder) {
	item := "ticket"
	if m.kind == "project" {
		item = "project"
	}
	sb.WriteString(fmt.Sprintf("Enter a name for this %s.\n\n", item))
	sb.WriteString("> " + withCursor(m.nameText, m.nameCur) + "\n")
	if m.nameErr != "" {
		sb.WriteString("\n")
		sb.WriteString(wizError.Render(m.nameErr))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(wizDim.Render("Enter to continue · Esc to quit"))
	sb.WriteString("\n")
}

func (m wizardModel) viewParent(sb *strings.Builder) {
	item := "ticket"
	if m.kind == "project" {
		item = "project"
	}
	sb.WriteString(fmt.Sprintf("Choose a parent project for this %s, or keep \"None\".\n", item))
	sb.WriteString(wizDim.Render("Type to filter · ↑↓ to navigate · Enter to continue · Esc to go back"))
	sb.WriteString("\n\n")
	sb.WriteString("Filter: " + withCursor(m.filter, m.filterCur) + "\n\n")

	fp := m.filteredProjects()
	total := 1 + len(fp)
	vis := m.parentListVisible()
	end := m.listOff + vis
	if end > total {
		end = total
	}

	for i := m.listOff; i < end; i++ {
		label := "None"
		if i > 0 {
			label = fp[i-1].Identifier
		}
		if i == m.listSel {
			sb.WriteString(wizSelected.Render("▶ " + label))
		} else {
			sb.WriteString("  " + label)
		}
		sb.WriteString("\n")
	}
}

func (m wizardModel) viewDesc(sb *strings.Builder) {
	sb.WriteString("Creating: " + wizBold.Render(m.fullIdentifier()) + "\n")
	sb.WriteString(wizDim.Render("Type the description below. Ctrl+D when done · Esc to go back"))
	sb.WriteString("\n\n")

	lines := strings.Split(m.descText, "\n")
	curLine, curCol := descCursorPos(m.descText, m.descCur)

	vis := m.descVisible()
	end := m.descOff + vis
	if end > len(lines) {
		end = len(lines)
	}

	for i := m.descOff; i < end; i++ {
		row := lines[i]
		if i == curLine {
			row = withCursor(row, curCol)
		}
		sb.WriteString(row + "\n")
	}
}
