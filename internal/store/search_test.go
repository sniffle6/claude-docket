package store

import (
	"testing"
)

func TestSearchBasicMatch(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Add a feature with searchable content
	s.AddFeature("Auth Middleware", "Implement JWT authentication for API endpoints")

	results, err := s.Search("authentication", SearchOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result for 'authentication'")
	}
	found := false
	for _, r := range results {
		if r.EntityType == "feature" && r.FeatureID == "auth-middleware" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected feature 'auth-middleware' in results, got: %+v", results)
	}
}
