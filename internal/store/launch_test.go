package store

import "testing"

func TestGetLaunchData(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Dashboard Writes", "add write ops to dashboard")

	// Add subtask with mixed checked/unchecked items
	st, _ := s.AddSubtask("dashboard-writes", "Hook changes", 0)
	item1, _ := s.AddTaskItem(st.ID, "Update SessionStart", 0)
	s.CompleteTaskItem(item1.ID, TaskItemCompletion{Outcome: "done"})
	s.AddTaskItem(st.ID, "Update Stop hook", 1)

	// Add mixed issues
	s.AddIssue("dashboard-writes", "Theme toggle broken", nil)
	resolved, _ := s.AddIssue("dashboard-writes", "Old bug", nil)
	s.ResolveIssue(resolved.ID, "abc123")

	// Add notes
	notes := "User wants Warp support"
	s.UpdateFeature("dashboard-writes", FeatureUpdate{Notes: &notes})

	data, err := s.GetLaunchData("dashboard-writes")
	if err != nil {
		t.Fatalf("GetLaunchData: %v", err)
	}

	if data.Feature.Title != "Dashboard Writes" {
		t.Errorf("Feature.Title = %q, want %q", data.Feature.Title, "Dashboard Writes")
	}
	if len(data.TaskItems) != 1 {
		t.Errorf("TaskItems count = %d, want 1 (unchecked only)", len(data.TaskItems))
	}
	if len(data.Issues) != 1 {
		t.Errorf("Issues count = %d, want 1 (open only)", len(data.Issues))
	}
	if data.Feature.Notes != "User wants Warp support" {
		t.Errorf("Notes = %q, want %q", data.Feature.Notes, "User wants Warp support")
	}
}

func TestGetLaunchData_NoTasks(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Empty Feature", "")

	data, err := s.GetLaunchData("empty-feature")
	if err != nil {
		t.Fatalf("GetLaunchData: %v", err)
	}
	if data.TaskItems == nil {
		t.Error("TaskItems should be empty slice, not nil")
	}
	if data.Issues == nil {
		t.Error("Issues should be empty slice, not nil")
	}
	if data.Subtasks == nil {
		t.Error("Subtasks should be empty slice, not nil")
	}
}

func TestGetLaunchData_AllComplete(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Done Feature", "")

	st, _ := s.AddSubtask("done-feature", "Work", 0)
	item, _ := s.AddTaskItem(st.ID, "Only task", 0)
	s.CompleteTaskItem(item.ID, TaskItemCompletion{Outcome: "done"})

	issue, _ := s.AddIssue("done-feature", "Bug", nil)
	s.ResolveIssue(issue.ID, "def456")

	data, err := s.GetLaunchData("done-feature")
	if err != nil {
		t.Fatalf("GetLaunchData: %v", err)
	}
	if len(data.TaskItems) != 0 {
		t.Errorf("TaskItems = %d, want 0", len(data.TaskItems))
	}
	if len(data.Issues) != 0 {
		t.Errorf("Issues = %d, want 0", len(data.Issues))
	}
}
