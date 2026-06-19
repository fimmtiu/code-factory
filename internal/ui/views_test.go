package ui

import "testing"

func TestNextView_CyclesThroughAllViews(t *testing.T) {
	// Starting from ViewProject, cycling viewCount times returns to ViewProject.
	v := ViewProject
	for i := 0; i < int(viewCount); i++ {
		v = nextView(v)
	}
	if v != ViewProject {
		t.Errorf("after %d nextView calls from ViewProject, got %d, want %d", viewCount, v, ViewProject)
	}
}

func TestNextView_Order(t *testing.T) {
	// Verify the full cycle order: Project → Command → Worker → Diffs → Memories → Log → Project.
	want := []ViewID{ViewCommand, ViewWorker, ViewDiff, ViewMemories, ViewLog, ViewProject}
	v := ViewProject
	for i, expected := range want {
		v = nextView(v)
		if v != expected {
			t.Errorf("step %d: nextView gave %d, want %d", i, v, expected)
		}
	}
}

func TestPrevView_CyclesThroughAllViews(t *testing.T) {
	v := ViewProject
	for i := 0; i < int(viewCount); i++ {
		v = prevView(v)
	}
	if v != ViewProject {
		t.Errorf("after %d prevView calls from ViewProject, got %d, want %d", viewCount, v, ViewProject)
	}
}

func TestPrevView_Order(t *testing.T) {
	// Reverse cycle: Project → Log → Memories → Diffs → Worker → Command → Project.
	want := []ViewID{ViewLog, ViewMemories, ViewDiff, ViewWorker, ViewCommand, ViewProject}
	v := ViewProject
	for i, expected := range want {
		v = prevView(v)
		if v != expected {
			t.Errorf("step %d: prevView gave %d, want %d", i, v, expected)
		}
	}
}

func TestViewLog_HasCorrectValue(t *testing.T) {
	if ViewLog != 5 {
		t.Errorf("ViewLog = %d, want 5", ViewLog)
	}
}

func TestViewMemories_HasCorrectValue(t *testing.T) {
	if ViewMemories != 4 {
		t.Errorf("ViewMemories = %d, want 4", ViewMemories)
	}
}

func TestViewCount_IsSix(t *testing.T) {
	if viewCount != 6 {
		t.Errorf("viewCount = %d, want 6", viewCount)
	}
}
