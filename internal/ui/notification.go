package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// startEditorMsg is dispatched by wrapEditorCmd; the root model uses it to
// set editorWaiting and then run fn in a goroutine.
type startEditorMsg struct{ fn func() tea.Msg }

// editorDoneMsg is delivered when the blocking editor goroutine exits.
type editorDoneMsg struct{ result tea.Msg }

// editorWaitingStyle is used for the "Waiting for editor..." overlay.
var editorWaitingStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("12")).
	BorderBackground(lipgloss.Color("238")).
	Background(lipgloss.Color("238")).
	Foreground(lipgloss.Color("230")).
	Bold(true).
	Padding(0, 2)

// wrapEditorCmd wraps fn so the root model can show "Waiting for editor..."
// while fn (which calls a blocking editor) runs in a goroutine.
func wrapEditorCmd(fn func() tea.Msg) tea.Cmd {
	return func() tea.Msg { return startEditorMsg{fn: fn} }
}

// notifStyle is the visual style for ephemeral pop-up notifications.
// Dark background with bright text and an amber border for high visibility.
var notifStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("214")).
	BorderBackground(lipgloss.Color("238")).
	Background(lipgloss.Color("238")).
	Foreground(lipgloss.Color("230")).
	Bold(true).
	Padding(0, 2)

// notifMsg triggers a new notification popup.
type notifMsg struct{ text string }

// clearNotifMsg dismisses the notification whose ID matches the current one.
type clearNotifMsg struct{ id int }

// ShowNotification returns a tea.Cmd that displays text in a transient popup
// in the bottom-right corner of the screen for 5 seconds. It does not steal
// focus and can be returned from any Update handler.
func ShowNotification(text string) tea.Cmd {
	return func() tea.Msg { return notifMsg{text: text} }
}

// workerNotifMsg is sent to the root model when a worker pushes a notification.
type workerNotifMsg struct{ text string }

// waitForWorkerNotif blocks until the next notification arrives on ch, then
// delivers it as a workerNotifMsg. The caller must re-arm this command after
// each message so the channel stays drained.
func waitForWorkerNotif(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		return workerNotifMsg{text: <-ch}
	}
}
