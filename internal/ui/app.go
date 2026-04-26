package ui

import (
	"strconv"
	"strings"
	"time"

	lipglossv2 "charm.land/lipgloss/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
	"github.com/fimmtiu/code-factory/internal/worker"
)

// ── Messages ─────────────────────────────────────────────────────────────────

// openDiffViewMsg is emitted by ProjectView, CommandView, and LogView when the
// user presses 'g' to open the diff viewer for a work unit.
type openDiffViewMsg struct {
	identifier string
	phase      string
	isProject  bool
}

// ── Model ────────────────────────────────────────────────────────────────────

// Model is the root bubbletea model for the code-factory TUI.
type Model struct {
	pool   *worker.Pool
	db     *db.DB
	width  int
	height int

	activeView ViewID
	views      [viewCount]viewModel

	dialog        dialog // nil when no dialog is open
	editorWaiting bool   // true while a blocking editor is open

	notifText string // current notification text; empty = none visible
	notifID   int    // incremented on each new notification to expire old timers

	// OS notification batching: when a "needs attention" notification arrives,
	// we buffer it and fire terminal-notifier after a 3-second delay. If
	// multiple arrive in that window, the message becomes "Multiple tickets
	// need attention".
	osNotifPending []string // buffered notification texts; nil = no timer running
	osNotifID      int      // incremented on each batch to expire stale timers
}

// NewModel creates a new root Model with the given pool, database, poll
// interval (in seconds), and settings.
func NewModel(pool *worker.Pool, database *db.DB, waitSecs int) Model {
	return Model{
		pool:       pool,
		db:         database,
		activeView: ViewProject,
		views: [viewCount]viewModel{
			ViewProject: NewProjectView(database, waitSecs),
			ViewCommand: NewCommandView(database, pool, waitSecs),
			ViewWorker:  NewWorkerView(pool),
			ViewLog:     NewLogView(database),
			ViewDiff:    NewDiffView(database),
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
	if m.pool != nil {
		cmds = append(cmds, waitForWorkerNotif(m.pool.NotifChannel))
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

	case workerNotifMsg:
		// Re-arm the listener so the next notification is also delivered.
		cmds := []tea.Cmd{
			ShowNotification(msg.text),
			waitForWorkerNotif(m.pool.NotifChannel),
		}
		// Buffer for a batched OS notification. If this is the first in
		// the batch, start a 3-second timer.
		startTimer := len(m.osNotifPending) == 0
		m.osNotifPending = append(m.osNotifPending, msg.text)
		if startTimer {
			m.osNotifID++
			id := m.osNotifID
			cmds = append(cmds, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
				return fireOSNotifMsg{id: id}
			}))
		}
		return m, tea.Batch(cmds...)

	case notifMsg:
		m.notifID++
		m.notifText = msg.text
		id := m.notifID
		return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg {
			return clearNotifMsg{id: id}
		})

	case clearNotifMsg:
		if msg.id == m.notifID {
			m.notifText = ""
		}
		return m, nil

	case fireOSNotifMsg:
		if msg.id == m.osNotifID && len(m.osNotifPending) > 0 {
			text := m.osNotifPending[0]
			if len(m.osNotifPending) > 1 {
				text = "Multiple tickets need attention"
			}
			m.osNotifPending = nil
			return m, fireOSNotification(text)
		}
		return m, nil

	case dismissDialogMsg:
		m.dialog = nil
		return m, nil

	case openMergeConflictDialogMsg:
		m.dialog = NewMergeConflictDialog(msg.worktreePath, msg.branch)
		return m, nil

	case openTicketDialogMsg:
		m.dialog = NewTicketDialog(m.db, msg.wu, m.width, m.height)
		return m, nil

	case openNewTicketDialogMsg:
		m.dialog = NewAddTicketDialog(m.db, msg.parent, msg.units, m.width)
		return m, nil

	case ticketCreatedMsg:
		cmds := []tea.Cmd{ticketCreatedNotificationCmd(msg)}
		// Refresh the Projects view so the new ticket appears.
		updated, cmd := m.views[ViewProject].Update(projectRefreshMsg{})
		m.views[ViewProject] = updated.(viewModel)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case openPhasePickerMsg:
		m.dialog = NewPhasePickerDialog(m.db, msg.wu)
		return m, nil

	case openEditChangeRequestDialogMsg:
		m.dialog = NewEditChangeRequestDialog(m.db, msg.location, msg.existingCR, m.width)
		return m, nil

	case openViewChangeRequestDialogMsg:
		m.dialog = NewViewChangeRequestDialog(m.db, msg.cr, msg.identifier, msg.worktreePath, m.width)
		return m, nil

	case crSavedMsg:
		var notif tea.Cmd
		switch {
		case msg.errMsg != "":
			notif = ShowNotification("CR failed: " + msg.errMsg)
		case msg.edited:
			notif = ShowNotification("Change request updated")
		default:
			notif = ShowNotification("Change request created")
		}
		// Forward to DiffView so its crMap (and any active viewer) can refresh.
		updated, cmd := m.views[ViewDiff].Update(msg)
		m.views[ViewDiff] = updated.(viewModel)
		return m, tea.Batch(notif, cmd)

	case openQuickResponseMsg:
		// If the worker has a structured permission request pending, show the
		// options chooser; otherwise fall back to the free-text input.
		if m.pool != nil && msg.wu.ClaimedBy != "" {
			if num, err := strconv.Atoi(msg.wu.ClaimedBy); err == nil {
				if w := m.pool.GetWorker(num); w != nil {
					if perm := w.GetPendingPermission(); perm != nil {
						m.dialog = NewPermissionDialog(m.db, m.pool, msg.wu, perm, m.width)
						return m, nil
					}
				}
			}
		}
		m.dialog = NewQuickResponseDialog(m.db, m.pool, msg.wu, m.width)
		return m, nil

	case startEditorMsg:
		m.editorWaiting = true
		fn := msg.fn
		return m, func() tea.Msg { return editorDoneMsg{result: fn()} }

	case editorDoneMsg:
		m.editorWaiting = false
		if msg.result != nil {
			return m, func() tea.Msg { return msg.result }
		}
		return m, nil

	case openDiffViewMsg:
		m.activeView = ViewDiff
		updated, cmd := m.views[ViewDiff].Update(msg)
		m.views[ViewDiff] = updated.(viewModel)
		return m, cmd

	case tea.KeyMsg:
		if m.editorWaiting {
			return m, nil
		}
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
		case "f5":
			m.activeView = ViewDiff
			return m, nil

		case "shift+tab":
			m.activeView = nextView(m.activeView)
			return m, m.activateViewCmd()

		case "ctrl+tab":
			m.activeView = prevView(m.activeView)
			return m, m.activateViewCmd()

		case "esc":
			// Esc on the Diffs commit list returns to Commands. The diff
			// viewer itself owns esc when it's open, so only intercept
			// here when no viewer is active.
			if m.activeView == ViewDiff {
				if dv, ok := m.views[ViewDiff].(DiffView); ok && dv.viewer == nil {
					m.activeView = ViewCommand
					return m, m.activateViewCmd()
				}
			}
		}

		// Pass remaining keys to the active view.
		updated, cmd := m.views[m.activeView].Update(msg)
		m.views[m.activeView] = updated.(viewModel)
		return m, cmd
	}

	// Broadcast non-key messages to all views so that each view receives its
	// own async results (e.g. respondToAgentDoneMsg, commandRefreshMsg)
	// regardless of which view is active or whether a dialog is open.
	// The dialog also receives the message, but views are never skipped.
	var cmds []tea.Cmd
	if m.dialog != nil {
		updated, cmd := m.dialog.Update(msg)
		m.dialog = updated.(dialog)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
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
	var leftHint string
	if m.dialog != nil {
		leftHint = theme.Current().HelpHintStyle.Render(buildHint("?", "help"))
	} else {
		leftHint = theme.Current().HelpHintStyle.Render(buildHint("?", "help", "Q", "quit"))
	}
	hint := leftHint
	if m.dialog == nil {
		var rightPairs []string
		switch m.activeView {
		case ViewProject:
			if pv, ok := m.views[ViewProject].(ProjectView); ok && pv.filtering {
				rightPairs = []string{"Esc", "clear filter"}
			} else {
				rightPairs = []string{"E", "edit", "g", "git-diff", "T", "open terminal", "Tab", "switch", "Enter", "view", "/", "filter"}
			}
		case ViewCommand:
			rightPairs = []string{"A", "approve", "g", "git-diff"}
			if isGitHubRepo() {
				rightPairs = append(rightPairs, "G", "github")
			}
			rightPairs = append(rightPairs, "E", "edit worktree", "T", "open terminal", "Enter", "respond/view")
		case ViewLog:
			if lv, ok := m.views[ViewLog].(LogView); ok && lv.filtering {
				rightPairs = []string{"Esc", "clear filter"}
			} else {
				rightPairs = []string{"E", "open in editor", "g", "git-diff"}
				if isGitHubRepo() {
					rightPairs = append(rightPairs, "G", "github")
				}
				rightPairs = append(rightPairs, "C", "copy path", "/", "filter")
			}
		case ViewDiff:
			if dv, ok := m.views[ViewDiff].(DiffView); ok {
				rightPairs = dv.HintPairs()
			}
		}
		if len(rightPairs) > 0 {
			right := theme.Current().HelpHintStyle.Render(buildHint(rightPairs...))
			spacer := m.width - lipgloss.Width(leftHint) - lipgloss.Width(right)
			if spacer < 2 {
				spacer = 2
			}
			hint = leftHint + strings.Repeat(" ", spacer) + right
		}
	}

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
		shadowLine := theme.Current().DialogShadowStyle.Render(strings.Repeat(" ", dialogW))
		shadowLines := make([]string, dialogH)
		for i := range shadowLines {
			shadowLines[i] = shadowLine
		}
		shadowStr := strings.Join(shadowLines, "\n")
		bg := lipglossv2.NewLayer(full)
		shadow := lipglossv2.NewLayer(shadowStr).X(x + 1).Y(y + 1).Z(1)
		fg := lipglossv2.NewLayer(dialogStr).X(x).Y(y).Z(2)
		full = lipglossv2.NewCompositor(bg, shadow, fg).Render()
	}

	if m.editorWaiting {
		waitStr := theme.Current().EditorWaitingStyle.Render("Waiting for editor…")
		waitW := lipgloss.Width(waitStr)
		waitH := strings.Count(waitStr, "\n") + 1
		x := (m.width - waitW) / 2
		y := (m.height - waitH) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		bg := lipglossv2.NewLayer(full)
		fg := lipglossv2.NewLayer(waitStr).X(x).Y(y).Z(4)
		full = lipglossv2.NewCompositor(bg, fg).Render()
	}

	if m.notifText != "" {
		notifStr := theme.Current().NotifStyle.Render(m.notifText)
		notifW := lipgloss.Width(notifStr)
		notifH := strings.Count(notifStr, "\n") + 1
		x := m.width - notifW
		y := m.height - notifH - 1 // leave the hint bar unobscured
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		bg := lipglossv2.NewLayer(full)
		fg := lipglossv2.NewLayer(notifStr).X(x).Y(y).Z(3)
		full = lipglossv2.NewCompositor(bg, fg).Render()
	}

	return full
}

// renderHeader returns the tab bar showing the active view.
func (m Model) renderHeader() string {
	tabs := make([]string, len(m.views))
	for i, v := range m.views {
		if ViewID(i) == m.activeView {
			tabs[i] = theme.Current().ActiveTabStyle.Render(v.Label())
		} else {
			tabs[i] = theme.Current().InactiveTabStyle.Render(v.Label())
		}
	}
	return theme.Current().HeaderStyle.Render(strings.Join(tabs, "  "))
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
