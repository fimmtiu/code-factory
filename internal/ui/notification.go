package ui

import (
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// notifIconPath is the absolute path to the notification icon.
var notifIconPath = filepath.Join(os.Getenv("HOME"), "src", "code-factory", "img", "terminal_icon.png")

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
	BorderForeground(lipgloss.Color("166")).
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

// fireOSNotifMsg is sent after the 3-second batching window expires.
type fireOSNotifMsg struct{ id int }

// hasTerminalNotifier caches whether terminal-notifier is available.
var hasTerminalNotifier = func() bool {
	_, err := exec.LookPath("terminal-notifier")
	return err == nil
}()

// fireOSNotification sends an OS-level notification via terminal-notifier.
func fireOSNotification(text string) tea.Cmd {
	if !hasTerminalNotifier {
		return nil
	}
	return func() tea.Msg {
		_ = exec.Command("terminal-notifier",
			"-title", "Code Factory",
			"-message", text,
			"-appIcon", notifIconPath,
		).Run()
		return nil
	}
}
