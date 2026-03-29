package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPlan = `# Test Plan

### Task 1: Project Scaffold

**Files:**
- Create: ` + "`cmd/feat/main.go`" + `
- Create: ` + "`go.mod`" + `

- [ ] **Step 1: Initialize Go module**
- [ ] **Step 2: Create main.go**
- [ ] **Step 3: Build and verify**
- [ ] **Step 4: Commit**

---

### Task 2: SQLite Store

**Files:**
- Create: ` + "`internal/store/store.go`" + `
- Modify: ` + "`internal/store/migrate.go`" + `

- [ ] **Step 1: Install SQLite**
- [ ] **Step 2: Write migrate.go**
- [ ] **Step 3: Write store.go**
`

func TestImportPlan(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "")

	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.md")
	os.WriteFile(planPath, []byte(testPlan), 0644)

	result, err := s.ImportPlan("test-feature", planPath)
	if err != nil {
		t.Fatalf("ImportPlan: %v", err)
	}
	if result.SubtaskCount != 2 {
		t.Errorf("SubtaskCount = %d, want 2", result.SubtaskCount)
	}
	if result.TaskItemCount != 7 {
		t.Errorf("TaskItemCount = %d, want 7", result.TaskItemCount)
	}

	subtasks, _ := s.GetSubtasksForFeature("test-feature", false)
	if len(subtasks) != 2 {
		t.Fatalf("subtasks = %d, want 2", len(subtasks))
	}
	if subtasks[0].Title != "Task 1: Project Scaffold" {
		t.Errorf("subtask 0 title = %q", subtasks[0].Title)
	}
	if len(subtasks[0].Items) != 4 {
		t.Errorf("subtask 0 items = %d, want 4", len(subtasks[0].Items))
	}
	if subtasks[0].Items[0].Title != "Initialize Go module" {
		t.Errorf("item 0 title = %q", subtasks[0].Items[0].Title)
	}
	if len(subtasks[0].Items[0].KeyFiles) != 2 {
		t.Errorf("item 0 key_files = %d, want 2", len(subtasks[0].Items[0].KeyFiles))
	}
}

func TestImportPlanRefusesDoneFeature(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "")
	done := "done"
	s.UpdateFeature("test-feature", FeatureUpdate{Status: &done})

	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.md")
	os.WriteFile(planPath, []byte(testPlan), 0644)

	_, err := s.ImportPlan("test-feature", planPath)
	if err == nil {
		t.Fatal("expected error importing plan on done feature, got nil")
	}
	if !strings.Contains(err.Error(), "done") {
		t.Errorf("error should mention done status, got: %v", err)
	}
}

func TestImportPlanArchivesExisting(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "")
	s.AddSubtask("test-feature", "Old Phase", 1)

	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.md")
	os.WriteFile(planPath, []byte(testPlan), 0644)

	s.ImportPlan("test-feature", planPath)

	all, _ := s.GetSubtasksForFeature("test-feature", true)
	archived := 0
	active := 0
	for _, st := range all {
		if st.Archived {
			archived++
		} else {
			active++
		}
	}
	if archived != 1 {
		t.Errorf("archived = %d, want 1", archived)
	}
	if active != 2 {
		t.Errorf("active = %d, want 2", active)
	}
}
