package store

import (
	"fmt"
	"testing"
)

func TestGetHandoffData(t *testing.T) {
	s := openTestStore(t)

	// Create feature with subtasks, items, and sessions
	s.AddFeature("Auth System", "token-based auth")
	s.UpdateFeature("auth-system", FeatureUpdate{
		Status:   strPtr("in_progress"),
		LeftOff:  strPtr("implementing refresh tokens"),
		KeyFiles: &[]string{"internal/auth/token.go", "internal/auth/middleware.go"},
	})

	st, _ := s.AddSubtask("auth-system", "Token handling", 1)
	s.AddTaskItem(st.ID, "Create token struct", 1)
	s.AddTaskItem(st.ID, "Add signing logic", 2)
	s.AddTaskItem(st.ID, "Add refresh endpoint", 3)
	s.CompleteTaskItem(1, TaskItemCompletion{Outcome: "done", CommitHash: "abc123"})

	st2, _ := s.AddSubtask("auth-system", "Middleware", 2)
	s.AddTaskItem(st2.ID, "Auth middleware", 1)

	s.LogSession(SessionInput{FeatureID: "auth-system", Summary: "Set up token struct", Commits: []string{"abc123"}})
	s.LogSession(SessionInput{FeatureID: "auth-system", Summary: "Started signing logic"})

	data, err := s.GetHandoffData("auth-system")
	if err != nil {
		t.Fatalf("GetHandoffData: %v", err)
	}

	if data.Feature.ID != "auth-system" {
		t.Errorf("Feature.ID = %q, want %q", data.Feature.ID, "auth-system")
	}
	if data.Done != 1 || data.Total != 4 {
		t.Errorf("Progress = %d/%d, want 1/4", data.Done, data.Total)
	}
	if len(data.NextTasks) != 3 {
		t.Fatalf("NextTasks = %d, want 3", len(data.NextTasks))
	}
	if data.NextTasks[0] != "Add signing logic" {
		t.Errorf("NextTasks[0] = %q, want %q", data.NextTasks[0], "Add signing logic")
	}
	if data.NextTasks[2] != "Auth middleware" {
		t.Errorf("NextTasks[2] = %q, want %q", data.NextTasks[2], "Auth middleware")
	}
	if len(data.SubtaskSummary) != 2 {
		t.Fatalf("SubtaskSummary = %d, want 2", len(data.SubtaskSummary))
	}
	if data.SubtaskSummary[0].Done != 1 || data.SubtaskSummary[0].Total != 3 {
		t.Errorf("Subtask 0 progress = %d/%d, want 1/3", data.SubtaskSummary[0].Done, data.SubtaskSummary[0].Total)
	}
	if len(data.RecentSessions) != 2 {
		t.Errorf("RecentSessions = %d, want 2", len(data.RecentSessions))
	}
}

func TestGetHandoffDataNotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.GetHandoffData("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent feature")
	}
}

func TestGetHandoffDataNoSubtasks(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Simple Feature", "no subtasks")
	s.UpdateFeature("simple-feature", FeatureUpdate{Status: strPtr("in_progress")})

	data, err := s.GetHandoffData("simple-feature")
	if err != nil {
		t.Fatalf("GetHandoffData: %v", err)
	}
	if data.Done != 0 || data.Total != 0 {
		t.Errorf("Progress = %d/%d, want 0/0", data.Done, data.Total)
	}
	if len(data.NextTasks) != 0 {
		t.Errorf("NextTasks = %d, want 0", len(data.NextTasks))
	}
	if len(data.SubtaskSummary) != 0 {
		t.Errorf("SubtaskSummary = %d, want 0", len(data.SubtaskSummary))
	}
}

func TestGetHandoffDataCapsNextTasksAtThree(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Big Feature", "many items")
	s.UpdateFeature("big-feature", FeatureUpdate{Status: strPtr("in_progress")})
	st, _ := s.AddSubtask("big-feature", "Phase 1", 1)
	for i := 1; i <= 6; i++ {
		s.AddTaskItem(st.ID, fmt.Sprintf("Task %d", i), i)
	}

	data, err := s.GetHandoffData("big-feature")
	if err != nil {
		t.Fatalf("GetHandoffData: %v", err)
	}
	if len(data.NextTasks) != 3 {
		t.Errorf("NextTasks = %d, want 3 (capped)", len(data.NextTasks))
	}
}

func TestGetHandoffDataRecentSessionsCappedAtThree(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Chatty Feature", "many sessions")
	for i := 0; i < 5; i++ {
		s.LogSession(SessionInput{FeatureID: "chatty-feature", Summary: fmt.Sprintf("session %d", i)})
	}

	data, err := s.GetHandoffData("chatty-feature")
	if err != nil {
		t.Fatalf("GetHandoffData: %v", err)
	}
	if len(data.RecentSessions) != 3 {
		t.Errorf("RecentSessions = %d, want 3 (capped)", len(data.RecentSessions))
	}
}
