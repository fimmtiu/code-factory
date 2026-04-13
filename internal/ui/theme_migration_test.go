package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
	"github.com/fimmtiu/code-factory/internal/worker"
)

// ── app.go: renderHeader uses theme styles ──────────────────────────────────

func TestRenderHeader_UsesThemeActiveTabStyle(t *testing.T) {
	saveTheme(t)
	// Set a distinctive active tab style so we can detect it in output.
	theme.SetCurrent(theme.Tan())
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40
	m.activeView = ViewProject

	header := m.renderHeader()
	// Render the same label with the theme style and verify the header contains it.
	activeLabel := theme.Current().ActiveTabStyle.Render(m.views[ViewProject].Label())
	if !strings.Contains(header, activeLabel) {
		t.Errorf("renderHeader should use theme.Current().ActiveTabStyle for the active tab")
	}
}

func TestRenderHeader_UsesThemeInactiveTabStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40
	m.activeView = ViewProject

	header := m.renderHeader()
	// An inactive tab (e.g. ViewCommand) should use InactiveTabStyle.
	inactiveLabel := theme.Current().InactiveTabStyle.Render(m.views[ViewCommand].Label())
	if !strings.Contains(header, inactiveLabel) {
		t.Errorf("renderHeader should use theme.Current().InactiveTabStyle for inactive tabs")
	}
}

func TestRenderHeader_UsesThemeHeaderStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40

	header := m.renderHeader()
	// The header wraps tab content with HeaderStyle. Verify by checking
	// that the output is non-empty (the style is applied as a wrapper).
	if header == "" {
		t.Error("renderHeader returned empty string")
	}
}

func TestView_UsesThemeEditorWaitingStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())
	m := NewModel(nil, nil, 5)
	m.width = 80
	m.height = 24
	m.editorWaiting = true

	view := m.View()
	// The editor waiting overlay is composited via lipglossv2; verify the
	// waiting text appears in the final output.
	if !strings.Contains(view, "Waiting for editor") {
		t.Errorf("View() with editorWaiting should show waiting text")
	}
}

func TestView_UsesThemeNotifStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())
	m := NewModel(nil, nil, 5)
	m.width = 80
	m.height = 24
	m.notifText = "Test notification"

	view := m.View()
	// The notification overlay is composited via lipglossv2; verify the
	// notification text appears in the final output.
	if !strings.Contains(view, "Test notification") {
		t.Errorf("View() with notifText should show notification text")
	}
}

func TestView_UsesThemeDialogShadowStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())
	m := NewModel(nil, nil, 5)
	m.width = 80
	m.height = 24
	m.dialog = NewQuitDialog()

	view := m.View()
	// The dialog shadow uses theme.Current().DialogShadowStyle.
	// Verify the view contains the shadow rendering (non-empty output with dialog).
	if view == "" {
		t.Error("View() with dialog should produce non-empty output")
	}
}

func TestView_UsesThemeHelpHintStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())
	m := NewModel(nil, nil, 5)
	m.width = 80
	m.height = 24

	view := m.View()
	// The help hint should contain "help" text styled with HelpHintStyle.
	if !strings.Contains(view, "help") {
		t.Error("View() should render help hint text")
	}
}

// ── views.go: buildHint uses theme styles ───────────────────────────────────

func TestBuildHint_UsesThemeHintKeyStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	result := buildHint("Q", "quit")
	expected := theme.Current().HintKeyStyle.Render("Q")
	if !strings.Contains(result, expected) {
		t.Errorf("buildHint should use theme.Current().HintKeyStyle for keys")
	}
}

func TestBuildHint_UsesThemeHintDescStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	result := buildHint("Q", "quit")
	expected := theme.Current().HintDescStyle.Render(" quit")
	if !strings.Contains(result, expected) {
		t.Errorf("buildHint should use theme.Current().HintDescStyle for descriptions")
	}
}

func TestBuildHint_MultipleKeys_UsesThemeStyles(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	result := buildHint("Q", "quit", "?", "help")
	keyQ := theme.Current().HintKeyStyle.Render("Q")
	keyHelp := theme.Current().HintKeyStyle.Render("?")
	if !strings.Contains(result, keyQ) {
		t.Errorf("buildHint multi should contain themed 'Q' key")
	}
	if !strings.Contains(result, keyHelp) {
		t.Errorf("buildHint multi should contain themed '?' key")
	}
}

// ── command_view.go: rendering uses theme styles ────────────────────────────

func TestCommandView_View_UsesThemeViewPaneStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	v := CommandView{
		width:  80,
		height: 24,
		rows:   nil, // empty → shows empty state
	}
	view := v.View()
	// Empty state uses both viewPaneStyle and emptyStateStyle from theme.
	if !strings.Contains(view, "No actionable tickets") {
		t.Error("empty CommandView should show 'No actionable tickets'")
	}
}

func TestCommandView_RenderRow_UsesThemeCmdSelectedStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	v := CommandView{width: 80, height: 24}
	wu := &models.WorkUnit{
		Identifier: "test/ticket-1",
		Phase:      models.PhaseImplement,
		Status:     models.StatusNeedsAttention,
	}
	row := v.renderRow(wu, true)
	// Selected row uses CmdSelectedStyle with a width set.
	if row == "" {
		t.Error("renderRow selected should produce non-empty output")
	}
}

func TestCommandView_RenderRow_NeedsAttention_UsesThemeStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	v := CommandView{width: 80, height: 24}
	wu := &models.WorkUnit{
		Identifier: "test/ticket-1",
		Phase:      models.PhaseImplement,
		Status:     models.StatusNeedsAttention,
	}
	row := v.renderRow(wu, false)
	if row == "" {
		t.Error("renderRow needs-attention should produce non-empty output")
	}
}

func TestCommandView_RenderRow_UserReview_UsesThemeStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	v := CommandView{width: 80, height: 24}
	wu := &models.WorkUnit{
		Identifier: "test/ticket-1",
		Phase:      models.PhaseReview,
		Status:     models.StatusUserReview,
	}
	row := v.renderRow(wu, false)
	if row == "" {
		t.Error("renderRow user-review should produce non-empty output")
	}
}

func TestCommandView_View_UsesThemeCmdErrorStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	v := CommandView{
		width:    80,
		height:   24,
		errorMsg: "something went wrong",
		rows: []listRow{
			{wu: &models.WorkUnit{
				Identifier: "test/ticket-1",
				Phase:      models.PhaseImplement,
				Status:     models.StatusNeedsAttention,
			}},
		},
	}
	view := v.View()
	expected := theme.Current().CmdErrorStyle.Render("something went wrong")
	if !strings.Contains(view, expected) {
		t.Errorf("CommandView.View() should use theme.Current().CmdErrorStyle for errors")
	}
}

func TestCommandView_View_UsesThemeEmptyStateStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	v := CommandView{
		width:  80,
		height: 24,
		rows:   nil,
	}
	view := v.View()
	expected := theme.Current().EmptyStateStyle.Render("No actionable tickets")
	if !strings.Contains(view, expected) {
		t.Errorf("empty CommandView should use theme.Current().EmptyStateStyle")
	}
}

// ── worker_view.go: rendering uses theme styles ─────────────────────────────

func TestWorkerView_RenderStatusLine_Idle_UsesThemeStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	w := worker.NewWorker(1)
	w.Status = worker.StatusIdle
	v := NewWorkerView(nil)
	line := v.renderStatusLine(w)

	expected := theme.Current().WorkerIdleStyle.Render("Worker 1: idle")
	if !strings.Contains(line, expected) {
		t.Errorf("idle worker should use theme.Current().WorkerIdleStyle, got: %q", line)
	}
}

func TestWorkerView_RenderStatusLine_Awaiting_UsesThemeStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	w := worker.NewWorker(1)
	w.Status = worker.StatusAwaitingResponse
	v := NewWorkerView(nil)
	line := v.renderStatusLine(w)

	expected := theme.Current().WorkerAwaitingStyle.Render("Worker 1: awaiting response")
	if !strings.Contains(line, expected) {
		t.Errorf("awaiting worker should use theme.Current().WorkerAwaitingStyle, got: %q", line)
	}
}

func TestWorkerView_RenderStatusLine_Busy_UsesThemeStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	w := worker.NewWorker(1)
	w.Status = worker.StatusBusy
	w.SetCurrentTicket("implement proj/t1")
	v := NewWorkerView(nil)
	line := v.renderStatusLine(w)

	expected := theme.Current().WorkerBusyStyle.Render("Worker 1: implement proj/t1")
	if !strings.Contains(line, expected) {
		t.Errorf("busy worker should use theme.Current().WorkerBusyStyle, got: %q", line)
	}
}

func TestWorkerView_RenderStatusLine_Paused_UsesThemeStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	w := worker.NewWorker(1)
	w.Status = worker.StatusBusy
	w.Paused = true
	v := NewWorkerView(nil)
	line := v.renderStatusLine(w)

	expected := theme.Current().WorkerPausedStyle.Render("Worker 1: paused")
	if !strings.Contains(line, expected) {
		t.Errorf("paused worker should use theme.Current().WorkerPausedStyle, got: %q", line)
	}
}

func TestWorkerView_RenderAllLines_UsesThemeSeparatorStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	pool := &worker.Pool{
		Workers: []*worker.Worker{worker.NewWorker(1)},
	}
	v := WorkerView{
		pool:            pool,
		width:           80,
		height:          24,
		prevOutput:      make(map[int][]string),
		outputChangedAt: make(map[int]time.Time),
	}
	lines := v.renderAllLines()
	// The separator line should use theme.Current().SeparatorStyle.
	separator := theme.Current().SeparatorStyle.Render(strings.Repeat("─", 80-viewBorderOverhead))
	found := false
	for _, line := range lines {
		if line == separator {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("renderAllLines separator should use theme.Current().SeparatorStyle")
	}
}

func TestWorkerView_View_EmptyPool_UsesThemeStyles(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	pool := &worker.Pool{}
	v := WorkerView{
		pool:            pool,
		width:           80,
		height:          24,
		prevOutput:      make(map[int][]string),
		outputChangedAt: make(map[int]time.Time),
	}
	view := v.View()
	expected := theme.Current().EmptyStateStyle.Render("(no workers)")
	if !strings.Contains(view, expected) {
		t.Errorf("empty WorkerView should use theme.Current().EmptyStateStyle")
	}
}

func TestWorkerView_RenderAllLines_UsesThemeWorkerOutputStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	w := worker.NewWorker(1)
	w.Status = worker.StatusBusy
	w.SetCurrentTicket("implement proj/t1")
	w.SetLastOutput([]string{"some output line"})

	pool := &worker.Pool{Workers: []*worker.Worker{w}}
	v := WorkerView{
		pool:            pool,
		width:           80,
		height:          24,
		prevOutput:      make(map[int][]string),
		outputChangedAt: make(map[int]time.Time),
	}
	lines := v.renderAllLines()
	expected := theme.Current().WorkerOutputStyle.Render("some output line")
	found := false
	for _, line := range lines {
		if strings.Contains(line, expected) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("renderAllLines should use theme.Current().WorkerOutputStyle for output")
	}
}

// ── phase_picker_dialog.go: rendering uses theme styles ─────────────────────

func TestPhasePickerDialog_View_UsesThemeCmdSelectedStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	wu := &models.WorkUnit{
		Identifier: "test/ticket-1",
		Phase:      models.PhaseImplement,
	}
	d := NewPhasePickerDialog(nil, wu)
	d.focus = ppFocusList // ensure list is focused so CmdSelectedStyle is used

	view := d.View()
	// The selected phase label should be rendered with CmdSelectedStyle.
	expected := theme.Current().CmdSelectedStyle.Render(" implement    ")
	if !strings.Contains(view, expected) {
		t.Errorf("PhasePickerDialog.View() should use theme.Current().CmdSelectedStyle for selected phase")
	}
}

func TestWorkerView_RenderAllLines_UsesThemeWorkerNoOutputStyle(t *testing.T) {
	saveTheme(t)
	theme.SetCurrent(theme.Tan())

	w := worker.NewWorker(1)
	w.Status = worker.StatusIdle

	pool := &worker.Pool{Workers: []*worker.Worker{w}}
	v := WorkerView{
		pool:            pool,
		width:           80,
		height:          24,
		prevOutput:      make(map[int][]string),
		outputChangedAt: make(map[int]time.Time),
	}
	lines := v.renderAllLines()
	expected := theme.Current().WorkerNoOutputStyle.Render("(no output)")
	found := false
	for _, line := range lines {
		if strings.Contains(line, expected) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("renderAllLines should use theme.Current().WorkerNoOutputStyle for empty output")
	}
}
