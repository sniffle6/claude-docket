package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sniffle6/claude-docket/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestListFeaturesAPI(t *testing.T) {
	s := testStore(t)
	s.AddFeature("Feature A", "desc a")
	s.AddFeature("Feature B", "desc b")

	handler := NewHandler(s, nil)
	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var features []store.Feature
	json.NewDecoder(w.Body).Decode(&features)
	if len(features) != 2 {
		t.Fatalf("got %d features", len(features))
	}
}

func TestGetFeatureAPI(t *testing.T) {
	s := testStore(t)
	s.AddFeature("Web Browser", "w3m")

	handler := NewHandler(s, nil)
	req := httptest.NewRequest("GET", "/api/features/web-browser", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestPatchFeatureAPI(t *testing.T) {
	s := testStore(t)
	s.AddFeature("Settings", "prefs")

	handler := NewHandler(s, nil)
	body := `{"status":"in_progress","left_off":"need save button"}`
	req := httptest.NewRequest("PATCH", "/api/features/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	f, _ := s.GetFeature("settings")
	if f.Status != "in_progress" {
		t.Errorf("Status = %q", f.Status)
	}
}

func TestGetUnlinkedSessionsAPI(t *testing.T) {
	s := testStore(t)
	s.LogSession(store.SessionInput{Summary: "orphan"})

	handler := NewHandler(s, nil)
	req := httptest.NewRequest("GET", "/api/sessions?unlinked=true", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var sessions []store.Session
	json.NewDecoder(w.Body).Decode(&sessions)
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions", len(sessions))
	}
}

func strPtr(s string) *string { return &s }

func TestPatchFeatureNotesAPI(t *testing.T) {
	s := testStore(t)
	s.AddFeature("Notes Feature", "")

	handler := NewHandler(s, nil)
	body := `{"notes":"my thoughts on this feature"}`
	req := httptest.NewRequest("PATCH", "/api/features/notes-feature", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	f, _ := s.GetFeature("notes-feature")
	if f.Notes != "my thoughts on this feature" {
		t.Errorf("Notes = %q, want %q", f.Notes, "my thoughts on this feature")
	}
}

func TestListFeaturesIncludesNotes(t *testing.T) {
	s := testStore(t)
	s.AddFeature("Feature With Notes", "")
	s.UpdateFeature("feature-with-notes", store.FeatureUpdate{Notes: strPtr("important")})

	handler := NewHandler(s, nil)
	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "important") {
		t.Errorf("response doesn't contain notes: %s", w.Body.String())
	}
}

func TestListFeaturesIncludesSubtaskProgress(t *testing.T) {
	s := testStore(t)
	s.AddFeature("Progress Test", "")
	st, _ := s.AddSubtask("progress-test", "Phase 1", 1)
	s.AddTaskItem(st.ID, "Item A", 1)
	s.AddTaskItem(st.ID, "Item B", 2)
	s.CompleteTaskItem(1, store.TaskItemCompletion{Outcome: "done"})

	handler := NewHandler(s, nil)
	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "subtask_progress") {
		t.Errorf("response missing subtask_progress: %s", body)
	}
	if !strings.Contains(body, "Phase 1") {
		t.Errorf("response missing subtask title: %s", body)
	}
}

func TestDevCompleteStatusAPI(t *testing.T) {
	s := testStore(t)
	s.AddFeature("DC Feature", "")
	s.UpdateFeature("dc-feature", store.FeatureUpdate{Status: strPtr("dev_complete")})

	handler := NewHandler(s, nil)
	req := httptest.NewRequest("GET", "/api/features?status=dev_complete", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	var features []json.RawMessage
	json.NewDecoder(w.Body).Decode(&features)
	if len(features) != 1 {
		t.Fatalf("got %d features, want 1", len(features))
	}
}

func TestReassignSessionAPI(t *testing.T) {
	s := testStore(t)
	s.AddFeature("Feature A", "")
	sess, _ := s.LogSession(store.SessionInput{Summary: "orphan"})

	handler := NewHandler(s, nil)
	body := fmt.Sprintf(`{"feature_id":"feature-a"}`)
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/sessions/%d", sess.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}
