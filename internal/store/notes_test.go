package store

import "testing"

func TestFeatureNotesField(t *testing.T) {
	s := openTestStore(t)
	f, err := s.AddFeature("Notes Test", "testing notes")
	if err != nil {
		t.Fatalf("AddFeature: %v", err)
	}
	if f.Notes != "" {
		t.Errorf("new feature Notes = %q, want empty", f.Notes)
	}

	err = s.UpdateFeature(f.ID, FeatureUpdate{Notes: strPtr("user thoughts here")})
	if err != nil {
		t.Fatalf("UpdateFeature: %v", err)
	}

	f, _ = s.GetFeature(f.ID)
	if f.Notes != "user thoughts here" {
		t.Errorf("Notes = %q, want %q", f.Notes, "user thoughts here")
	}
}

func TestNotesInListFeatures(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("With Notes", "")
	s.UpdateFeature("with-notes", FeatureUpdate{Notes: strPtr("my notes")})

	features, err := s.ListFeatures("")
	if err != nil {
		t.Fatalf("ListFeatures: %v", err)
	}
	if len(features) != 1 {
		t.Fatalf("got %d features", len(features))
	}
	if features[0].Notes != "my notes" {
		t.Errorf("Notes = %q, want %q", features[0].Notes, "my notes")
	}
}

func TestNotesInGetContext(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Context Notes", "")
	s.UpdateFeature("context-notes", FeatureUpdate{
		Notes:  strPtr("important context"),
		Status: strPtr("in_progress"),
	})

	ctx, err := s.GetContext("context-notes")
	if err != nil {
		t.Fatalf("GetContext: %v", err)
	}
	if ctx.Feature.Notes != "important context" {
		t.Errorf("Notes = %q", ctx.Feature.Notes)
	}
}

func TestDevCompleteStatus(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Dev Done", "")
	err := s.UpdateFeature("dev-done", FeatureUpdate{Status: strPtr("dev_complete")})
	if err != nil {
		t.Fatalf("UpdateFeature: %v", err)
	}
	f, _ := s.GetFeature("dev-done")
	if f.Status != "dev_complete" {
		t.Errorf("Status = %q, want dev_complete", f.Status)
	}
}

func TestDevCompleteExcludedFromReady(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Dev Complete Feature", "")
	s.UpdateFeature("dev-complete-feature", FeatureUpdate{Status: strPtr("dev_complete")})
	s.AddFeature("Active Feature", "")
	s.UpdateFeature("active-feature", FeatureUpdate{Status: strPtr("in_progress")})

	ready, err := s.GetReadyFeatures()
	if err != nil {
		t.Fatalf("GetReadyFeatures: %v", err)
	}
	for _, f := range ready {
		if f.Status == "dev_complete" {
			t.Errorf("dev_complete feature should not appear in ready list")
		}
	}
}
