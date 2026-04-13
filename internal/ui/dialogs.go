package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// dialog is an interface satisfied by all modal dialogs.
type dialog interface {
	tea.Model
}

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
	cancelBtn := theme.Current().ButtonNormalStyle.Render("Cancel")
	quitBtn := theme.Current().ButtonNormalStyle.Render("Quit")
	if d.focused == quitFocusCancel {
		cancelBtn = theme.Current().ButtonFocusedStyle.Render("Cancel")
	} else {
		quitBtn = theme.Current().ButtonFocusedStyle.Render("Quit")
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		theme.Current().DialogTitleStyle.Render("Really quit?"),
		"There are still active workers. Really quit?",
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, cancelBtn, "  ", quitBtn),
	)
	return theme.Current().DialogBoxStyle.Render(body)
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
		sb.WriteString(theme.Current().DetailLabelStyle.Render(kb.Key))
		sb.WriteString("  ")
		sb.WriteString(kb.Description)
		sb.WriteString("\n")
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		theme.Current().DialogTitleStyle.Render("Help"),
		strings.TrimRight(sb.String(), "\n"),
		"",
		theme.Current().ButtonFocusedStyle.Render("Okay"),
	)
	return theme.Current().DialogBoxStyle.Render(body)
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
	fixBtn := theme.Current().ButtonNormalStyle.Render("Fix")
	ignoreBtn := theme.Current().ButtonNormalStyle.Render("Ignore")
	if d.focused == mergeFocusFix {
		fixBtn = theme.Current().ButtonFocusedStyle.Render("Fix")
	} else {
		ignoreBtn = theme.Current().ButtonFocusedStyle.Render("Ignore")
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		theme.Current().DialogTitleStyle.Render("Merge Conflict"),
		"Merging "+theme.Current().DetailLabelStyle.Render(d.branch)+" failed in:",
		theme.Current().DetailLabelStyle.Render(d.worktreePath),
		"",
		"Resolve the conflict, then try approving again.",
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, fixBtn, "  ", ignoreBtn),
	)
	return theme.Current().DialogBoxStyle.Render(body)
}

// ── dismissDialogMsg ──────────────────────────────────────────────────────────

// dismissDialogMsg is sent by dialogs when they want to be closed.
type dismissDialogMsg struct{}

func dismissDialogCmd() tea.Cmd {
	return func() tea.Msg { return dismissDialogMsg{} }
}
