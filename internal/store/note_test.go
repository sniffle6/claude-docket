package store

import (
	"testing"
)

func TestAddAndGetNotes(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	f, err := s.AddFeature("Test Feature", "desc")
	if err != nil {
		t.Fatalf("AddFeature: %v", err)
	}

	n1, err := s.AddNote(f.ID, "Found that the API returns 404 for deleted users")
	if err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	if n1.Content != "Found that the API returns 404 for deleted users" {
		t.Errorf("got content %q, want %q", n1.Content, "Found that the API returns 404 for deleted users")
	}
	if n1.FeatureID != f.ID {
		t.Errorf("got feature_id %q, want %q", n1.FeatureID, f.ID)
	}

	n2, err := s.AddNote(f.ID, "The config file is loaded at startup only")
	if err != nil {
		t.Fatalf("AddNote: %v", err)
	}

	notes, err := s.GetNotesForFeature(f.ID)
	if err != nil {
		t.Fatalf("GetNotesForFeature: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("got %d notes, want 2", len(notes))
	}
	// Ordered by id DESC, so n2 first
	if notes[0].ID != n2.ID {
		t.Errorf("expected newest note first, got id %d want %d", notes[0].ID, n2.ID)
	}
}

func TestAddNoteInvalidFeature(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	_, err = s.AddNote("nonexistent", "some content")
	if err == nil {
		t.Fatal("expected error for nonexistent feature, got nil")
	}
}

func TestGetNotesEmptyFeature(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	f, _ := s.AddFeature("Empty Feature", "")
	notes, err := s.GetNotesForFeature(f.ID)
	if err != nil {
		t.Fatalf("GetNotesForFeature: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("got %d notes, want 0", len(notes))
	}
}
