package ui

import tea "github.com/charmbracelet/bubbletea"

// CommandView is a stub for the command view.
type CommandView struct{}

func NewCommandView() CommandView { return CommandView{} }

func (v CommandView) Init() tea.Cmd { return nil }

func (v CommandView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return v, nil }

func (v CommandView) View() string { return "Command View" }

func (v CommandView) KeyBindings() []KeyBinding { return nil }
