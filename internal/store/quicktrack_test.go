package store

import "testing"

func TestQuickTrackCreatesFeature(t *testing.T) {
	s := openTestStore(t)

	result, err := s.QuickTrack(QuickTrackInput{
		Title:      "Add logo to README",
		CommitHash: "abc123",
		KeyFiles:   []string{"README.md", "assets/logo.png"},
	})
	if err != nil {
		t.Fatalf("QuickTrack: %v", err)
	}
	if !result.Created {
		t.Error("expected Created=true for new feature")
	}
	if result.Feature.ID != "add-logo-to-readme" {
		t.Errorf("ID = %q, want %q", result.Feature.ID, "add-logo-to-readme")
	}
	if result.Feature.Status != "done" {
		t.Errorf("Status = %q, want %q", result.Feature.Status, "done")
	}
	if len(result.Feature.KeyFiles) != 2 {
		t.Errorf("KeyFiles len = %d, want 2", len(result.Feature.KeyFiles))
	}

	// Verify session was logged with commit
	sessions, _ := s.GetSessionsForFeature("add-logo-to-readme")
	if len(sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(sessions))
	}
	if len(sessions[0].Commits) != 1 || sessions[0].Commits[0] != "abc123" {
		t.Errorf("session commits = %v, want [abc123]", sessions[0].Commits)
	}
}

func TestQuickTrackUpdatesExisting(t *testing.T) {
	s := openTestStore(t)

	// Create first
	s.QuickTrack(QuickTrackInput{
		Title:    "Fix typo in docs",
		KeyFiles: []string{"docs/readme.md"},
	})

	// Update with new commit
	result, err := s.QuickTrack(QuickTrackInput{
		Title:      "Fix typo in docs",
		CommitHash: "def456",
		KeyFiles:   []string{"docs/readme.md", "docs/install.md"},
	})
	if err != nil {
		t.Fatalf("QuickTrack update: %v", err)
	}
	if result.Created {
		t.Error("expected Created=false for existing feature")
	}
	// Should merge key_files without duplicates
	if len(result.Feature.KeyFiles) != 2 {
		t.Errorf("KeyFiles = %v, want 2 files", result.Feature.KeyFiles)
	}
}

func TestQuickTrackDefaultsDone(t *testing.T) {
	s := openTestStore(t)

	result, _ := s.QuickTrack(QuickTrackInput{Title: "Minor fix"})
	if result.Feature.Status != "done" {
		t.Errorf("Status = %q, want %q", result.Feature.Status, "done")
	}
}

func TestQuickTrackCustomStatus(t *testing.T) {
	s := openTestStore(t)

	result, _ := s.QuickTrack(QuickTrackInput{
		Title:  "Upcoming small task",
		Status: "in_progress",
	})
	if result.Feature.Status != "in_progress" {
		t.Errorf("Status = %q, want %q", result.Feature.Status, "in_progress")
	}
}

func TestQuickTrackNoCommitNoSession(t *testing.T) {
	s := openTestStore(t)

	s.QuickTrack(QuickTrackInput{Title: "No commit task"})

	sessions, _ := s.GetSessionsForFeature("no-commit-task")
	if len(sessions) != 0 {
		t.Errorf("sessions len = %d, want 0 (no commit = no session)", len(sessions))
	}
}
