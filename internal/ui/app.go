package ui

import (
	"strings"

	lipglossv2 "charm.land/lipgloss/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/worker"
)

// ── Styles ───────────────────────────────────────────────────────────────────

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)

	tabBaseStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("62")).
			Inherit(tabBaseStyle)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Inherit(tabBaseStyle)

	helpHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(0, 1)
)

// ── Model ────────────────────────────────────────────────────────────────────

// Model is the root bubbletea model for the code-factory TUI.
type Model struct {
	pool     *worker.Pool
	db       *db.DB
	width    int
	height   int

	activeView ViewID
	views      [4]viewModel

	dialog dialog // nil when no dialog is open
}

// NewModel creates a new root Model with the given pool, database, poll
// interval (in seconds), and settings.
func NewModel(pool *worker.Pool, database *db.DB, waitSecs int) Model {
	return Model{
		pool:       pool,
		db:         database,
		activeView: ViewProject,
		views: [4]viewModel{
			ViewProject: NewProjectView(database, waitSecs),
			ViewCommand: NewCommandView(database, pool, waitSecs),
			ViewWorker:  NewWorkerView(pool),
			ViewLog:     NewLogView(database),
		},
	}
}

// Init returns the initial command batch, including init commands from all views.
func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, v := range m.views {
		if cmd := v.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// Update handles all incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Forward to all views so they can recalculate layouts.
		var cmds []tea.Cmd
		for i, v := range m.views {
			updated, cmd := v.Update(msg)
			m.views[i] = updated.(viewModel)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	case dismissDialogMsg:
		m.dialog = nil
		return m, nil

	case openChangeRequestDialogMsg:
		m.dialog = NewChangeRequestDialog(m.db, msg.wu, m.width, m.height)
		return m, nil

	case tea.KeyMsg:
		// If a dialog is open, route all keys to it.
		if m.dialog != nil {
			updated, cmd := m.dialog.Update(msg)
			m.dialog = updated.(dialog)
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m.handleQuit()

		case "?", "H":
			m.dialog = NewHelpDialog(m.views[m.activeView].KeyBindings())
			return m, nil

		case "f1":
			m.activeView = ViewProject
			return m, nil
		case "f2":
			m.activeView = ViewCommand
			return m, nil
		case "f3":
			m.activeView = ViewWorker
			return m, nil
		case "f4":
			m.activeView = ViewLog
			return m, func() tea.Msg { return logActivatedMsg{} }

		case "shift+tab":
			m.activeView = nextView(m.activeView)
			return m, m.activateViewCmd()

		case "ctrl+tab":
			m.activeView = prevView(m.activeView)
			return m, m.activateViewCmd()
		}

		// Pass remaining keys to the active view.
		updated, cmd := m.views[m.activeView].Update(msg)
		m.views[m.activeView] = updated.(viewModel)
		return m, cmd
	}

	// If a dialog is open, route non-key messages to it first.
	if m.dialog != nil {
		updated, cmd := m.dialog.Update(msg)
		m.dialog = updated.(dialog)
		return m, cmd
	}

	// Broadcast non-key messages to all views so that each view receives its
	// own async results (e.g. commandRefreshMsg) regardless of which view is
	// currently active.
	var cmds []tea.Cmd
	for i, v := range m.views {
		updated, cmd := v.Update(msg)
		m.views[i] = updated.(viewModel)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

// View renders the current state of the TUI.
func (m Model) View() string {
	header := m.renderHeader()
	content := m.views[m.activeView].View()
	hint := helpHintStyle.Render("? help  Q quit")

	// Compute the body area height.
	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(hint)
	if bodyHeight < 0 {
		bodyHeight = 0
	}

	// Pad content to fill the body area.
	contentLines := strings.Split(content, "\n")
	for len(contentLines) < bodyHeight {
		contentLines = append(contentLines, "")
	}
	// Truncate if too tall.
	if len(contentLines) > bodyHeight {
		contentLines = contentLines[:bodyHeight]
	}
	body := strings.Join(contentLines, "\n")

	full := lipgloss.JoinVertical(lipgloss.Left, header, body, hint)

	if m.dialog != nil {
		dialogStr := m.dialog.View()
		dialogW := lipgloss.Width(dialogStr)
		dialogH := strings.Count(dialogStr, "\n") + 1
		x := (m.width - dialogW) / 2
		y := (m.height - dialogH) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		bg := lipglossv2.NewLayer(full)
		fg := lipglossv2.NewLayer(dialogStr).X(x).Y(y).Z(1)
		full = lipglossv2.NewCompositor(bg, fg).Render()
	}

	return full
}

// renderHeader returns the tab bar showing the active view.
func (m Model) renderHeader() string {
	tabs := make([]string, 4)
	for i, name := range []string{
		viewNames[ViewProject],
		viewNames[ViewCommand],
		viewNames[ViewWorker],
		viewNames[ViewLog],
	} {
		label := name
		switch ViewID(i) {
		case ViewProject:
			label = "F1:" + name
		case ViewCommand:
			label = "F2:" + name
		case ViewWorker:
			label = "F3:" + name
		case ViewLog:
			label = "F4:" + name
		}
		if ViewID(i) == m.activeView {
			tabs[i] = activeTabStyle.Render(label)
		} else {
			tabs[i] = inactiveTabStyle.Render(label)
		}
	}
	return headerStyle.Render(strings.Join(tabs, "  "))
}

// activateViewCmd returns a command that sends an activation message for the
// newly-active view (currently only used by LogView for immediate refresh).
func (m Model) activateViewCmd() tea.Cmd {
	if m.activeView == ViewLog {
		return func() tea.Msg { return logActivatedMsg{} }
	}
	return nil
}

// handleQuit implements the quit flow: immediate exit if all workers are idle,
// otherwise show the confirmation dialog.
func (m Model) handleQuit() (tea.Model, tea.Cmd) {
	if m.allWorkersIdle() {
		return m, tea.Quit
	}
	m.dialog = NewQuitDialog()
	return m, nil
}

// allWorkersIdle returns true if every worker in the pool is StatusIdle.
func (m Model) allWorkersIdle() bool {
	if m.pool == nil {
		return true
	}
	for _, w := range m.pool.Workers {
		if w.Status != worker.StatusIdle {
			return false
		}
	}
	return true
}
