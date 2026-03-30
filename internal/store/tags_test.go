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
