package ui

import tea "github.com/charmbracelet/bubbletea"

// diffScreen identifies which sub-screen the DiffView is showing.
type diffScreen int

const (
	screenCommitSelector diffScreen = iota
	screenDiffViewer
)

// DiffView is the view model for the Diffs tab (F5). It has two internal
// sub-screens: a commit selector and a diff viewer.
type DiffView struct {
	width  int
	height int

	screen diffScreen

	currentTicket string
	worktreePath  string
	forkPoint     string
	startCommit   int
	endCommit     int
}

// NewDiffView creates a DiffView with default (empty) state.
func NewDiffView() DiffView {
	return DiffView{
		screen: screenCommitSelector,
	}
}

// Init returns nil; the DiffView has no startup commands yet.
func (v DiffView) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages for the DiffView.
func (v DiffView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return v, nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}

	return v, nil
}

func (v DiffView) handleKey(_ tea.KeyMsg) (tea.Model, tea.Cmd) {
	return v, nil
}

// View renders the DiffView content.
func (v DiffView) View() string {
	vh := v.viewHeight()
	innerW := v.width - viewBorderOverhead
	if innerW < 1 {
		innerW = 1
	}

	var content string
	if v.currentTicket == "" {
		content = emptyStateStyle.Render("No ticket selected")
	}

	return viewPaneStyle.
		Width(innerW).
		Height(vh).
		Render(clipLines(content, vh))
}

// viewHeight returns the number of usable content lines inside the border.
func (v DiffView) viewHeight() int {
	h := v.height - chromeHeight - viewBorderOverhead
	if h < 1 {
		h = 1
	}
	return h
}

// KeyBindings returns the keybindings specific to the DiffView.
func (v DiffView) KeyBindings() []KeyBinding {
	return []KeyBinding{}
}
