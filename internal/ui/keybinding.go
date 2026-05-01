package ui

// KeyBinding associates a key name with a human-readable description.
// When Hidden is true, the binding is omitted from the help dialog.
type KeyBinding struct {
	Key         string
	Description string
	Hidden      bool
}

// globalKeyBindings are the key bindings that apply in every view.
var globalKeyBindings = []KeyBinding{
	{Key: "F1", Description: "Switch to project view"},
	{Key: "F2", Description: "Switch to command view"},
	{Key: "F3", Description: "Switch to worker view"},
	{Key: "F4", Description: "Switch to diffs view"},
	{Key: "F5", Description: "Switch to log view"},
	{Key: "Shift-Tab", Description: "Next view"},
	{Key: "Ctrl-Tab", Description: "Previous view"},
	{Key: "?, H", Description: "Show help"},
	{Key: "Q, Ctrl-C", Description: "Quit"},
}
