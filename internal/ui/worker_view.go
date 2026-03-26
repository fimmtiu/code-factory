package ui

import tea "github.com/charmbracelet/bubbletea"

// WorkerView is a stub for the worker view.
type WorkerView struct{}

func NewWorkerView() WorkerView { return WorkerView{} }

func (v WorkerView) Init() tea.Cmd { return nil }

func (v WorkerView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return v, nil }

func (v WorkerView) View() string { return "Worker View" }

func (v WorkerView) KeyBindings() []KeyBinding { return nil }
