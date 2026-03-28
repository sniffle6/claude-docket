package store

import (
	"testing"
)

func TestAddSubtask(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "")

	st, err := s.AddSubtask("test-feature", "Store Layer", 1)
	if err != nil {
		t.Fatalf("AddSubtask: %v", err)
	}
	if st.Title != "Store Layer" {
		t.Errorf("Title = %q", st.Title)
	}
	if st.Position != 1 {
		t.Errorf("Position = %d", st.Position)
	}
}

func TestAddTaskItem(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "")
	st, _ := s.AddSubtask("test-feature", "Store Layer", 1)

	item, err := s.AddTaskItem(st.ID, "Create store.go", 1)
	if err != nil {
		t.Fatalf("AddTaskItem: %v", err)
	}
	if item.Title != "Create store.go" {
		t.Errorf("Title = %q", item.Title)
	}
	if item.Checked {
		t.Error("expected unchecked")
	}
}

func TestCompleteTaskItem(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "")
	st, _ := s.AddSubtask("test-feature", "Store Layer", 1)
	item, _ := s.AddTaskItem(st.ID, "Create store.go", 1)

	err := s.CompleteTaskItem(item.ID, TaskItemCompletion{
		Outcome:    "Store with Open/Close and migration",
		CommitHash: "abc1234",
		KeyFiles:   []string{"internal/store/store.go"},
	})
	if err != nil {
		t.Fatalf("CompleteTaskItem: %v", err)
	}

	updated, _ := s.GetTaskItem(item.ID)
	if !updated.Checked {
		t.Error("expected checked")
	}
	if updated.Outcome != "Store with Open/Close and migration" {
		t.Errorf("Outcome = %q", updated.Outcome)
	}
	if updated.CommitHash != "abc1234" {
		t.Errorf("CommitHash = %q", updated.CommitHash)
	}
}

func TestGetSubtasksForFeature(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "")
	s.AddSubtask("test-feature", "Phase 1", 1)
	s.AddSubtask("test-feature", "Phase 2", 2)

	subtasks, err := s.GetSubtasksForFeature("test-feature", false)
	if err != nil {
		t.Fatalf("GetSubtasksForFeature: %v", err)
	}
	if len(subtasks) != 2 {
		t.Fatalf("got %d subtasks, want 2", len(subtasks))
	}
	if subtasks[0].Title != "Phase 1" {
		t.Errorf("first subtask = %q", subtasks[0].Title)
	}
}

func TestArchiveSubtasks(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "")
	s.AddSubtask("test-feature", "Old Phase", 1)

	err := s.ArchiveSubtasks("test-feature")
	if err != nil {
		t.Fatalf("ArchiveSubtasks: %v", err)
	}

	active, _ := s.GetSubtasksForFeature("test-feature", false)
	if len(active) != 0 {
		t.Fatalf("active = %d, want 0", len(active))
	}

	all, _ := s.GetSubtasksForFeature("test-feature", true)
	if len(all) != 1 {
		t.Fatalf("all = %d, want 1", len(all))
	}
	if !all[0].Archived {
		t.Error("expected archived")
	}
}

func TestGetTaskItemsForSubtask(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "")
	st, _ := s.AddSubtask("test-feature", "Phase 1", 1)
	s.AddTaskItem(st.ID, "Task A", 1)
	s.AddTaskItem(st.ID, "Task B", 2)

	items, err := s.GetTaskItemsForSubtask(st.ID)
	if err != nil {
		t.Fatalf("GetTaskItemsForSubtask: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
}

func TestGetFeatureProgress(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "")
	st, _ := s.AddSubtask("test-feature", "Phase 1", 1)
	item1, _ := s.AddTaskItem(st.ID, "Task A", 1)
	s.AddTaskItem(st.ID, "Task B", 2)
	s.CompleteTaskItem(item1.ID, TaskItemCompletion{Outcome: "done"})

	done, total, err := s.GetFeatureProgress("test-feature")
	if err != nil {
		t.Fatalf("GetFeatureProgress: %v", err)
	}
	if done != 1 || total != 2 {
		t.Errorf("progress = %d/%d, want 1/2", done, total)
	}
}
