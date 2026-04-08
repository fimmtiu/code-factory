package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// dialog is an interface satisfied by all modal dialogs.
type dialog interface {
	tea.Model
}

// ── Styles ───────────────────────────────────────────────────────────────────

var (
	dialogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colourPrimary).
			Padding(1, 2)

	dialogTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Background(colourPrimary).
				Foreground(colourOnPrimary).
				Padding(0, 1).
				MarginBottom(1)

	buttonBaseStyle = lipgloss.NewStyle().Padding(0, 2)

	buttonNormalStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("250")).
				Foreground(lipgloss.Color("255")).
				Inherit(buttonBaseStyle)

	buttonFocusedStyle = lipgloss.NewStyle().
				Background(colourPrimary).
				Foreground(colourOnPrimary).
				Inherit(buttonBaseStyle)
)

// ── Quit dialog ──────────────────────────────────────────────────────────────

// quitDialogFocused tracks which button has focus.
type quitDialogFocused int

const (
	quitFocusCancel quitDialogFocused = iota
	quitFocusQuit
)

// QuitDialog is the "really quit?" modal shown when workers are busy.
type QuitDialog struct {
	focused quitDialogFocused
}

func NewQuitDialog() QuitDialog {
	return QuitDialog{focused: quitFocusCancel}
}

func (d QuitDialog) Init() tea.Cmd { return nil }

func (d QuitDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			if d.focused == quitFocusCancel {
				d.focused = quitFocusQuit
			} else {
				d.focused = quitFocusCancel
			}
		case "shift+tab":
			if d.focused == quitFocusQuit {
				d.focused = quitFocusCancel
			} else {
				d.focused = quitFocusQuit
			}
		case "left", "h":
			d.focused = quitFocusCancel
		case "right", "l":
			d.focused = quitFocusQuit
		case "enter":
			if d.focused == quitFocusQuit {
				return d, tea.Quit
			}
			// Cancel — caller will clear the dialog.
			return d, dismissDialogCmd()
		case "esc":
			return d, dismissDialogCmd()
		}
	}
	return d, nil
}

func (d QuitDialog) View() string {
	cancelBtn := buttonNormalStyle.Render("Cancel")
	quitBtn := buttonNormalStyle.Render("Quit")
	if d.focused == quitFocusCancel {
		cancelBtn = buttonFocusedStyle.Render("Cancel")
	} else {
		quitBtn = buttonFocusedStyle.Render("Quit")
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		dialogTitleStyle.Render("Really quit?"),
		"There are still active workers. Really quit?",
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, cancelBtn, "  ", quitBtn),
	)
	return dialogBoxStyle.Render(body)
}

// ── Help dialog ───────────────────────────────────────────────────────────────

// HelpDialog shows the key bindings for the current view.
type HelpDialog struct {
	bindings []KeyBinding
}

func NewHelpDialog(viewBindings []KeyBinding) HelpDialog {
	all := make([]KeyBinding, 0, len(globalKeyBindings)+len(viewBindings))
	all = append(all, viewBindings...)
	if len(viewBindings) > 0 {
		// separator represented as an empty binding
		all = append(all, KeyBinding{})
	}
	all = append(all, globalKeyBindings...)
	return HelpDialog{bindings: all}
}

func (d HelpDialog) Init() tea.Cmd { return nil }

func (d HelpDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "esc":
			return d, dismissDialogCmd()
		}
	}
	return d, nil
}

func (d HelpDialog) View() string {
	var sb strings.Builder
	for _, kb := range d.bindings {
		if kb.Hidden {
			continue
		}
		if kb.Key == "" {
			sb.WriteString("\n")
			continue
		}
		sb.WriteString(detailLabelStyle.Render(kb.Key))
		sb.WriteString("  ")
		sb.WriteString(kb.Description)
		sb.WriteString("\n")
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		dialogTitleStyle.Render("Help"),
		strings.TrimRight(sb.String(), "\n"),
		"",
		buttonFocusedStyle.Render("Okay"),
	)
	return dialogBoxStyle.Render(body)
}

// ── Merge conflict dialog ─────────────────────────────────────────────────────

type mergeConflictFocused int

const (
	mergeFocusFix mergeConflictFocused = iota
	mergeFocusIgnore
)

// MergeConflictDialog is shown when a git merge fails. It offers to open a
// terminal in the conflicted worktree so the user can resolve the conflict.
type MergeConflictDialog struct {
	worktreePath string
	branch       string
	focused      mergeConflictFocused
}

func NewMergeConflictDialog(worktreePath, branch string) MergeConflictDialog {
	return MergeConflictDialog{worktreePath: worktreePath, branch: branch, focused: mergeFocusFix}
}

func (d MergeConflictDialog) Init() tea.Cmd { return nil }

func (d MergeConflictDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "left", "right", "h", "l":
			if d.focused == mergeFocusFix {
				d.focused = mergeFocusIgnore
			} else {
				d.focused = mergeFocusFix
			}
		case "enter":
			if d.focused == mergeFocusFix {
				openTerminalWithCommand(d.worktreePath, "git status")
			}
			return d, dismissDialogCmd()
		case "esc":
			return d, dismissDialogCmd()
		}
	}
	return d, nil
}

func (d MergeConflictDialog) View() string {
	fixBtn := buttonNormalStyle.Render("Fix")
	ignoreBtn := buttonNormalStyle.Render("Ignore")
	if d.focused == mergeFocusFix {
		fixBtn = buttonFocusedStyle.Render("Fix")
	} else {
		ignoreBtn = buttonFocusedStyle.Render("Ignore")
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		dialogTitleStyle.Render("Merge Conflict"),
		"Merging "+detailLabelStyle.Render(d.branch)+" failed in:",
		detailLabelStyle.Render(d.worktreePath),
		"",
		"Resolve the conflict, then try approving again.",
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, fixBtn, "  ", ignoreBtn),
	)
	return dialogBoxStyle.Render(body)
}

// ── dismissDialogMsg ──────────────────────────────────────────────────────────

// dismissDialogMsg is sent by dialogs when they want to be closed.
type dismissDialogMsg struct{}

func dismissDialogCmd() tea.Cmd {
	return func() tea.Msg { return dismissDialogMsg{} }
}
