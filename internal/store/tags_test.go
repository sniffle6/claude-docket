package store

import "testing"


func TestFeatureTagsField(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	f, err := s.AddFeature("Tagged Feature", "desc")
	if err != nil {
		t.Fatalf("AddFeature: %v", err)
	}
	if len(f.Tags) != 0 {
		t.Fatalf("expected empty tags, got %v", f.Tags)
	}

	tags := []string{"auth", "frontend"}
	s.UpdateFeature(f.ID, FeatureUpdate{Tags: &tags})
	f, _ = s.GetFeature(f.ID)
	if len(f.Tags) != 2 || f.Tags[0] != "auth" || f.Tags[1] != "frontend" {
		t.Fatalf("expected [auth frontend], got %v", f.Tags)
	}
}

func TestFeatureTagsInList(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	s.AddFeature("Auth Feature", "desc")
	s.AddFeature("UI Feature", "desc")

	authTags := []string{"auth", "backend"}
	uiTags := []string{"frontend", "ui"}
	s.UpdateFeature("auth-feature", FeatureUpdate{Tags: &authTags})
	s.UpdateFeature("ui-feature", FeatureUpdate{Tags: &uiTags})

	features, _ := s.ListFeatures("")
	if len(features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(features))
	}
	if len(features[0].Tags) == 0 {
		t.Fatal("expected tags in list results")
	}
}

func TestListFeaturesFilterByTag(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	s.AddFeature("Auth Feature", "")
	s.AddFeature("UI Feature", "")
	s.AddFeature("Mixed Feature", "")

	authTags := []string{"auth"}
	uiTags := []string{"ui"}
	mixedTags := []string{"auth", "ui"}
	s.UpdateFeature("auth-feature", FeatureUpdate{Tags: &authTags})
	s.UpdateFeature("ui-feature", FeatureUpdate{Tags: &uiTags})
	s.UpdateFeature("mixed-feature", FeatureUpdate{Tags: &mixedTags})

	features, err := s.ListFeaturesWithTag("", "auth")
	if err != nil {
		t.Fatalf("ListFeaturesWithTag: %v", err)
	}
	if len(features) != 2 {
		t.Fatalf("expected 2 features with auth tag, got %d", len(features))
	}

	// Combined with status filter
	ip := "in_progress"
	s.UpdateFeature("auth-feature", FeatureUpdate{Status: &ip})
	features, _ = s.ListFeaturesWithTag("in_progress", "auth")
	if len(features) != 1 || features[0].ID != "auth-feature" {
		t.Fatalf("expected 1 in_progress feature with auth tag, got %d", len(features))
	}
}

func TestGetKnownTags(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	// No features — no tags
	tags, _ := s.GetKnownTags()
	if len(tags) != 0 {
		t.Fatalf("expected 0 tags, got %d", len(tags))
	}

	s.AddFeature("Feature A", "")
	s.AddFeature("Feature B", "")
	tagsA := []string{"backend", "auth"}
	tagsB := []string{"auth", "frontend"}
	s.UpdateFeature("feature-a", FeatureUpdate{Tags: &tagsA})
	s.UpdateFeature("feature-b", FeatureUpdate{Tags: &tagsB})

	tags, err := s.GetKnownTags()
	if err != nil {
		t.Fatalf("GetKnownTags: %v", err)
	}
	// Should be sorted and deduplicated: auth, backend, frontend
	if len(tags) != 3 {
		t.Fatalf("expected 3 unique tags, got %d: %v", len(tags), tags)
	}
	if tags[0] != "auth" || tags[1] != "backend" || tags[2] != "frontend" {
		t.Fatalf("expected [auth backend frontend], got %v", tags)
	}
}

func TestCheckNewTags(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	s.AddFeature("Feature A", "")
	existing := []string{"auth", "frontend"}
	s.UpdateFeature("feature-a", FeatureUpdate{Tags: &existing})

	// "auth" exists, "frntend" is new
	newTags := s.CheckNewTags([]string{"auth", "frntend"})
	if len(newTags) != 1 || newTags[0] != "frntend" {
		t.Fatalf("expected [frntend], got %v", newTags)
	}

	// All existing — no new
	newTags = s.CheckNewTags([]string{"auth", "frontend"})
	if len(newTags) != 0 {
		t.Fatalf("expected no new tags, got %v", newTags)
	}
}

func TestTagsInUpdateFeature(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	s.AddFeature("Replaceable", "")
	tags1 := []string{"old", "stale"}
	s.UpdateFeature("replaceable", FeatureUpdate{Tags: &tags1})

	tags2 := []string{"new"}
	s.UpdateFeature("replaceable", FeatureUpdate{Tags: &tags2})
	f, _ := s.GetFeature("replaceable")
	if len(f.Tags) != 1 || f.Tags[0] != "new" {
		t.Fatalf("expected [new], got %v", f.Tags)
	}
}
