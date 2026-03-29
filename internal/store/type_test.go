package store

import "testing"

func TestFeatureTypeField(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	f, err := s.AddFeature("My Feature", "desc")
	if err != nil {
		t.Fatalf("AddFeature: %v", err)
	}
	if f.Type != "" {
		t.Fatalf("expected empty type, got %q", f.Type)
	}

	typ := "bugfix"
	s.UpdateFeature(f.ID, FeatureUpdate{Type: &typ})
	f, _ = s.GetFeature(f.ID)
	if f.Type != "bugfix" {
		t.Fatalf("expected bugfix, got %q", f.Type)
	}
}

func TestFeatureTypeInList(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	s.AddFeature("Typed Feature", "desc")
	typ := "feature"
	s.UpdateFeature("typed-feature", FeatureUpdate{Type: &typ})

	features, _ := s.ListFeatures("")
	if len(features) != 1 || features[0].Type != "feature" {
		t.Fatalf("expected type=feature in list, got %q", features[0].Type)
	}
}

func TestApplyTemplate(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	f, _ := s.AddFeature("Login Bug", "broken")
	err := s.ApplyTemplate(f.ID, "bugfix")
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}

	subtasks, _ := s.GetSubtasksForFeature(f.ID, false)
	if len(subtasks) != 2 {
		t.Fatalf("expected 2 subtasks, got %d", len(subtasks))
	}
	if subtasks[0].Title != "Investigation" {
		t.Fatalf("expected Investigation, got %q", subtasks[0].Title)
	}
	if len(subtasks[0].Items) != 2 {
		t.Fatalf("expected 2 items in Investigation, got %d", len(subtasks[0].Items))
	}
	if subtasks[0].Items[0].Title != "Reproduce the bug" {
		t.Fatalf("expected 'Reproduce the bug', got %q", subtasks[0].Items[0].Title)
	}
	if subtasks[1].Title != "Fix" {
		t.Fatalf("expected Fix, got %q", subtasks[1].Title)
	}
}

func TestApplyTemplateUnknownType(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	f, _ := s.AddFeature("Something", "desc")
	err := s.ApplyTemplate(f.ID, "unknown")
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestApplyTemplateAllTypes(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	for _, typ := range []string{"feature", "bugfix", "chore", "spike"} {
		f, _ := s.AddFeature("Test "+typ, "desc")
		err := s.ApplyTemplate(f.ID, typ)
		if err != nil {
			t.Fatalf("ApplyTemplate(%s): %v", typ, err)
		}
		subtasks, _ := s.GetSubtasksForFeature(f.ID, false)
		if len(subtasks) == 0 {
			t.Fatalf("type %s: expected subtasks, got 0", typ)
		}
		for _, st := range subtasks {
			if len(st.Items) == 0 {
				t.Fatalf("type %s, subtask %q: expected items, got 0", typ, st.Title)
			}
		}
	}
}

func TestValidFeatureTypes(t *testing.T) {
	valid := ValidFeatureTypes()
	if len(valid) != 4 {
		t.Fatalf("expected 4 valid types, got %d", len(valid))
	}
}

func TestCompletionGateBlocksDone(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	f, _ := s.AddFeature("Gated Feature", "desc")
	s.ApplyTemplate(f.ID, "bugfix")

	done := "done"
	err := s.UpdateFeature(f.ID, FeatureUpdate{Status: &done})
	if err == nil {
		t.Fatal("expected error when marking done with unchecked items")
	}

	f, _ = s.GetFeature(f.ID)
	if f.Status == "done" {
		t.Fatal("feature should not be done")
	}
}

func TestCompletionGateAllowsForce(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	f, _ := s.AddFeature("Force Feature", "desc")
	s.ApplyTemplate(f.ID, "chore")

	done := "done"
	force := true
	reason := "Decided items are not needed"
	err := s.UpdateFeature(f.ID, FeatureUpdate{Status: &done, Force: &force, ForceReason: &reason})
	if err != nil {
		t.Fatalf("force completion should succeed: %v", err)
	}

	f, _ = s.GetFeature(f.ID)
	if f.Status != "done" {
		t.Fatalf("expected done, got %q", f.Status)
	}

	decisions, _ := s.GetDecisionsForFeature(f.ID)
	if len(decisions) == 0 {
		t.Fatal("expected a decision logged for force completion")
	}
	if decisions[0].Outcome != "accepted" {
		t.Fatalf("expected accepted, got %q", decisions[0].Outcome)
	}
}

func TestCompletionGatePassesWhenAllDone(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	f, _ := s.AddFeature("Complete Feature", "desc")
	s.ApplyTemplate(f.ID, "chore")

	subtasks, _ := s.GetSubtasksForFeature(f.ID, false)
	for _, st := range subtasks {
		for _, item := range st.Items {
			s.CompleteTaskItem(item.ID, TaskItemCompletion{Outcome: "done"})
		}
	}

	done := "done"
	err := s.UpdateFeature(f.ID, FeatureUpdate{Status: &done})
	if err != nil {
		t.Fatalf("should pass gate when all items checked: %v", err)
	}
}

func TestCompletionGateNoSubtasks(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	f, _ := s.AddFeature("Empty Feature", "desc")

	done := "done"
	err := s.UpdateFeature(f.ID, FeatureUpdate{Status: &done})
	if err != nil {
		t.Fatalf("should pass gate with no subtasks: %v", err)
	}
}

func TestCompletionGateOpenIssues(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	f, _ := s.AddFeature("Issue Feature", "desc")
	s.AddIssue(f.ID, "something is broken", nil)

	done := "done"
	err := s.UpdateFeature(f.ID, FeatureUpdate{Status: &done})
	if err == nil {
		t.Fatal("expected error when marking done with open issues")
	}
}

func TestCompletionGateNonDoneStatusSkipsCheck(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	f, _ := s.AddFeature("Status Feature", "desc")
	s.ApplyTemplate(f.ID, "bugfix")

	inProgress := "in_progress"
	err := s.UpdateFeature(f.ID, FeatureUpdate{Status: &inProgress})
	if err != nil {
		t.Fatalf("non-done status should skip gate: %v", err)
	}
}

func TestCompletionGateForceNoReason(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	f, _ := s.AddFeature("No Reason", "desc")
	s.ApplyTemplate(f.ID, "spike")

	done := "done"
	force := true
	err := s.UpdateFeature(f.ID, FeatureUpdate{Status: &done, Force: &force})
	if err != nil {
		t.Fatalf("force without reason should succeed: %v", err)
	}

	decisions, _ := s.GetDecisionsForFeature(f.ID)
	if len(decisions) == 0 {
		t.Fatal("expected decision logged")
	}
	if decisions[0].Reason != "No reason given" {
		t.Fatalf("expected 'No reason given', got %q", decisions[0].Reason)
	}
}
