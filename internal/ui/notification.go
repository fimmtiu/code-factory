package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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
