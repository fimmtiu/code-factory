package ui

import "testing"

func TestNextView_CyclesThroughAllFiveViews(t *testing.T) {
	// Starting from ViewProject, cycling 5 times should return to ViewProject.
	v := ViewProject
	for i := 0; i < 5; i++ {
		v = nextView(v)
	}
	if v != ViewProject {
		t.Errorf("after 5 nextView calls from ViewProject, got %d, want %d", v, ViewProject)
	}
}

func TestNextView_Order(t *testing.T) {
	// Verify the full cycle order: Project → Command → Worker → Diffs → Log → Project.
	want := []ViewID{ViewCommand, ViewWorker, ViewDiff, ViewLog, ViewProject}
	v := ViewProject
	for i, expected := range want {
		v = nextView(v)
		if v != expected {
			t.Errorf("step %d: nextView gave %d, want %d", i, v, expected)
		}
	}
}

func TestPrevView_CyclesThroughAllFiveViews(t *testing.T) {
	v := ViewProject
	for i := 0; i < 5; i++ {
		v = prevView(v)
	}
	if v != ViewProject {
		t.Errorf("after 5 prevView calls from ViewProject, got %d, want %d", v, ViewProject)
	}
}

func TestPrevView_Order(t *testing.T) {
	// Reverse cycle: Project → Log → Diffs → Worker → Command → Project.
	want := []ViewID{ViewLog, ViewDiff, ViewWorker, ViewCommand, ViewProject}
	v := ViewProject
	for i, expected := range want {
		v = prevView(v)
		if v != expected {
			t.Errorf("step %d: prevView gave %d, want %d", i, v, expected)
		}
	}
}

func TestViewLog_HasCorrectValue(t *testing.T) {
	if ViewLog != 4 {
		t.Errorf("ViewLog = %d, want 4", ViewLog)
	}
}

func TestViewCount_IsFive(t *testing.T) {
	if viewCount != 5 {
		t.Errorf("viewCount = %d, want 5", viewCount)
	}
}
