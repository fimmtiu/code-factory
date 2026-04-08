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
	// Verify the full cycle order: Project → Command → Worker → Log → Diffs → Project.
	want := []ViewID{ViewCommand, ViewWorker, ViewLog, ViewDiffs, ViewProject}
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
	// Reverse cycle: Project → Diffs → Log → Worker → Command → Project.
	want := []ViewID{ViewDiffs, ViewLog, ViewWorker, ViewCommand, ViewProject}
	v := ViewProject
	for i, expected := range want {
		v = prevView(v)
		if v != expected {
			t.Errorf("step %d: prevView gave %d, want %d", i, v, expected)
		}
	}
}

func TestViewDiffs_HasCorrectValue(t *testing.T) {
	if ViewDiffs != 4 {
		t.Errorf("ViewDiffs = %d, want 4", ViewDiffs)
	}
}

func TestViewNames_ContainsDiffs(t *testing.T) {
	name, ok := viewNames[ViewDiffs]
	if !ok {
		t.Fatal("viewNames does not contain ViewDiffs")
	}
	if name != "Diffs" {
		t.Errorf("viewNames[ViewDiffs] = %q, want %q", name, "Diffs")
	}
}

func TestViewNames_HasFiveEntries(t *testing.T) {
	if len(viewNames) != 5 {
		t.Errorf("viewNames has %d entries, want 5", len(viewNames))
	}
}
