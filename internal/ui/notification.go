package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// notifStyle is the visual style for ephemeral pop-up notifications.
var notifStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("214")).
	Padding(0, 1)

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
