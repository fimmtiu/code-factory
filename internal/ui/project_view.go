package ui

import tea "github.com/charmbracelet/bubbletea"

// ProjectView is a stub for the project view.
type ProjectView struct{}

func NewProjectView() ProjectView { return ProjectView{} }

func (v ProjectView) Init() tea.Cmd { return nil }

func (v ProjectView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return v, nil }

func (v ProjectView) View() string { return "Project View" }

func (v ProjectView) KeyBindings() []KeyBinding { return nil }
