// Package ui provides a terminal user interface for the tickets application
// using the bubbletea framework.
package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/tickets/internal/db"
	"github.com/fimmtiu/tickets/internal/models"
)

// FocusedPane identifies which pane currently has keyboard focus.
type FocusedPane int

const (
	// NavigatorFocused means the tree navigator pane has focus.
	NavigatorFocused FocusedPane = iota
	// DetailFocused means the detail pane has focus.
	DetailFocused
)

const autoRefreshInterval = 60 * time.Second

// statusMsg is sent when a status fetch completes successfully.
type statusMsg struct {
	units []*models.WorkUnit
}

// errMsg is sent when a status fetch fails.
type errMsg struct {
	err error
}

// tickMsg is sent by the auto-refresh ticker.
type tickMsg struct{}

// Model is the top-level bubbletea model for the tickets UI.
type Model struct {
	width      int
	height     int
	focused    FocusedPane
	units      []*models.WorkUnit
	db         *db.DB
	navigator  NavigatorPane
	detail     DetailPane
	statusPane StatusPane
	err        error
}

// NewModel creates a new Model configured to use the given database.
func NewModel(d *db.DB, repoName string) Model {
	return Model{
		db:         d,
		focused:    NavigatorFocused,
		statusPane: StatusPane{repoName: repoName},
	}
}

// Init starts the auto-refresh ticker and fetches the initial status.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchStatus(m.db),
		startTicker(),
	)
}

// Update handles all incoming messages and updates the model accordingly.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case statusMsg:
		m.units = msg.units
		m.navigator.SetUnits(m.units)
		if sel := m.navigator.Selected(); sel != nil {
			m.detail.SetUnit(sel)
		}
		m.err = nil
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case tickMsg:
		return m, tea.Batch(fetchStatus(m.db), startTicker())

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey routes keyboard input based on which pane is focused.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyCtrlR:
		return m, fetchStatus(m.db)
	}

	if msg.Type == tea.KeyRunes {
		switch string(msg.Runes) {
		case "q":
			return m, tea.Quit
		}
	}

	switch m.focused {
	case NavigatorFocused:
		return m.handleNavigatorKey(msg)
	case DetailFocused:
		return m.handleDetailKey(msg)
	}

	return m, nil
}

// handleNavigatorKey handles key presses when the navigator pane is focused.
func (m Model) handleNavigatorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	navContentHeight := m.height/2 - 2
	switch msg.Type {
	case tea.KeyUp:
		m.navigator.MoveUp()
		if sel := m.navigator.Selected(); sel != nil {
			m.detail.SetUnit(sel)
		}
	case tea.KeyDown:
		m.navigator.MoveDown()
		if sel := m.navigator.Selected(); sel != nil {
			m.detail.SetUnit(sel)
		}
	case tea.KeyPgUp:
		m.navigator.PageUp(navContentHeight)
		if sel := m.navigator.Selected(); sel != nil {
			m.detail.SetUnit(sel)
		}
	case tea.KeyPgDown:
		m.navigator.PageDown(navContentHeight)
		if sel := m.navigator.Selected(); sel != nil {
			m.detail.SetUnit(sel)
		}
	case tea.KeyEnter:
		m.navigator.ToggleExpand()
	case tea.KeyTab, tea.KeySpace:
		m.focused = DetailFocused
	}
	return m, nil
}

// handleDetailKey handles key presses when the detail pane is focused.
func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	detailContentHeight := (m.height - m.height/2) - 2
	switch msg.Type {
	case tea.KeyUp:
		m.detail.ScrollUp()
	case tea.KeyDown:
		m.detail.ScrollDown()
	case tea.KeyPgUp:
		m.detail.PageUp(detailContentHeight)
	case tea.KeyPgDown:
		m.detail.PageDown(detailContentHeight)
	case tea.KeyTab, tea.KeySpace:
		m.focused = NavigatorFocused
	}
	return m, nil
}

// View renders the entire UI as a string.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	topHeight := m.height / 2
	bottomHeight := m.height - topHeight

	topLeftWidth := m.width / 3
	topRightWidth := m.width - topLeftWidth

	topSection := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.statusPane.View(m.units, topLeftWidth, topHeight),
		m.navigator.View(topRightWidth, topHeight, m.focused == NavigatorFocused),
	)

	bottom := m.detail.View(m.width, bottomHeight, m.focused == DetailFocused)

	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		errLine := errStyle.Render(fmt.Sprintf("Error: %v", m.err))
		return lipgloss.JoinVertical(lipgloss.Left, topSection, errLine, bottom)
	}

	return lipgloss.JoinVertical(lipgloss.Left, topSection, bottom)
}

// fetchStatus returns a tea.Cmd that fetches status from the database.
func fetchStatus(d *db.DB) tea.Cmd {
	return func() tea.Msg {
		if d == nil {
			return errMsg{err: fmt.Errorf("database not available")}
		}
		units, err := d.Status()
		if err != nil {
			return errMsg{err: err}
		}
		return statusMsg{units: units}
	}
}

// startTicker returns a tea.Cmd that fires a tickMsg after autoRefreshInterval.
func startTicker() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}
