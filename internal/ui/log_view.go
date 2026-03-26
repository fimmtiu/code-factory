package ui

import tea "github.com/charmbracelet/bubbletea"

// LogView is a stub for the log view.
type LogView struct{}

func NewLogView() LogView { return LogView{} }

func (v LogView) Init() tea.Cmd { return nil }

func (v LogView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return v, nil }

func (v LogView) View() string { return "Log View" }

func (v LogView) KeyBindings() []KeyBinding { return nil }
