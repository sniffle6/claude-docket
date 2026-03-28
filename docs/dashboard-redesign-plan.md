# Dashboard Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the docket dashboard with Memphis Dark theme, slide-out panel detail view, per-task progress cards, user notes field, and dev_complete status.

**Architecture:** Coordinated backend + frontend change. Backend: schema v4 migration adds `notes` column, store/API/MCP tools updated to read/write it, `dev_complete` added as valid status. Frontend: full rewrite of `dashboard/index.html` with 5-column kanban, per-task progress cards, slide-out panel, Memphis Dark theme.

**Tech Stack:** Go (store, API, MCP), SQLite, vanilla HTML/CSS/JS (single-file dashboard)

**Spec:** `docs/dashboard-redesign.md`
**Mockup:** `.superpowers/brainstorm/7163-1774717832/content/memphis-dashboard-mockup.html`
**Theme:** `themes/memphis-dark.json`

---

### Task 1: Schema v4 Migration — Add `notes` Column

**Files:**
- Modify: `internal/store/migrate.go`

- [ ] **Step 1: Add schemaV4 constant and apply it in migrate()**

In `internal/store/migrate.go`, add after the `schemaV3` constant:

```go
const schemaV4 = `
ALTER TABLE features ADD COLUMN notes TEXT NOT NULL DEFAULT '';
`
```

And add at the end of the `migrate()` function, after the v3 line:

```go
// v4: add notes column (ignore error if already exists)
db.Exec(schemaV4)
```

- [ ] **Step 2: Run tests to verify migration doesn't break anything**

Run: `go test ./internal/store/ -v -run TestOpenCreatesDB`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/store/migrate.go
git commit -m "feat: add schema v4 migration — notes column on features"
```

---

### Task 2: Store Layer — Add `notes` to Feature CRUD

**Files:**
- Modify: `internal/store/store.go`
- Create: `internal/store/notes_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/store/notes_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -v -run "TestFeatureNotesField|TestNotesInListFeatures|TestNotesInGetContext|TestDevCompleteStatus|TestDevCompleteExcludedFromReady"`
Expected: FAIL — `Notes` field doesn't exist on Feature struct

- [ ] **Step 3: Add Notes field to Feature and FeatureUpdate structs**

In `internal/store/store.go`, add `Notes` field to the `Feature` struct after `LeftOff`:

```go
type Feature struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	LeftOff      string    `json:"left_off"`
	Notes        string    `json:"notes"`
	KeyFiles     []string  `json:"key_files"`
	WorktreePath string    `json:"worktree_path"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
```

Add `Notes` field to `FeatureUpdate`:

```go
type FeatureUpdate struct {
	Title        *string   `json:"title,omitempty"`
	Description  *string   `json:"description,omitempty"`
	Status       *string   `json:"status,omitempty"`
	LeftOff      *string   `json:"left_off,omitempty"`
	Notes        *string   `json:"notes,omitempty"`
	KeyFiles     *[]string `json:"key_files,omitempty"`
	WorktreePath *string   `json:"worktree_path,omitempty"`
}
```

- [ ] **Step 4: Update GetFeature to scan notes column**

In `GetFeature`, update the query and Scan to include `notes`:

```go
func (s *Store) GetFeature(id string) (*Feature, error) {
	row := s.db.QueryRow(
		`SELECT id, title, description, status, left_off, notes, key_files, worktree_path, created_at, updated_at FROM features WHERE id = ?`,
		id,
	)
	var f Feature
	var keyFilesJSON string
	err := row.Scan(&f.ID, &f.Title, &f.Description, &f.Status, &f.LeftOff, &f.Notes, &keyFilesJSON, &f.WorktreePath, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get feature %q: %w", id, err)
	}
	json.Unmarshal([]byte(keyFilesJSON), &f.KeyFiles)
	if f.KeyFiles == nil {
		f.KeyFiles = []string{}
	}
	return &f, nil
}
```

- [ ] **Step 5: Update ListFeatures to scan notes column**

In `ListFeatures`, update the query and Scan:

```go
func (s *Store) ListFeatures(status string) ([]Feature, error) {
	query := `SELECT id, title, description, status, left_off, notes, key_files, worktree_path, created_at, updated_at FROM features`
	var args []any
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY updated_at DESC"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list features: %w", err)
	}
	defer rows.Close()
	var features []Feature
	for rows.Next() {
		var f Feature
		var keyFilesJSON string
		if err := rows.Scan(&f.ID, &f.Title, &f.Description, &f.Status, &f.LeftOff, &f.Notes, &keyFilesJSON, &f.WorktreePath, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan feature: %w", err)
		}
		json.Unmarshal([]byte(keyFilesJSON), &f.KeyFiles)
		if f.KeyFiles == nil {
			f.KeyFiles = []string{}
		}
		features = append(features, f)
	}
	return features, nil
}
```

- [ ] **Step 6: Update GetReadyFeatures to scan notes column**

In `GetReadyFeatures`, update the query and Scan:

```go
func (s *Store) GetReadyFeatures() ([]Feature, error) {
	rows, err := s.db.Query(
		`SELECT id, title, description, status, left_off, notes, key_files, worktree_path, created_at, updated_at FROM features WHERE status IN ('in_progress', 'planned') ORDER BY CASE WHEN status='in_progress' THEN 0 ELSE 1 END, updated_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("get ready features: %w", err)
	}
	defer rows.Close()
	var features []Feature
	for rows.Next() {
		var f Feature
		var keyFilesJSON string
		if err := rows.Scan(&f.ID, &f.Title, &f.Description, &f.Status, &f.LeftOff, &f.Notes, &keyFilesJSON, &f.WorktreePath, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan feature: %w", err)
		}
		json.Unmarshal([]byte(keyFilesJSON), &f.KeyFiles)
		if f.KeyFiles == nil {
			f.KeyFiles = []string{}
		}
		features = append(features, f)
	}
	return features, nil
}
```

- [ ] **Step 7: Update UpdateFeature to handle notes**

In `UpdateFeature`, add after the `LeftOff` block:

```go
if u.Notes != nil {
	sets = append(sets, "notes = ?")
	args = append(args, *u.Notes)
}
```

- [ ] **Step 8: Run all store tests**

Run: `go test ./internal/store/ -v`
Expected: ALL PASS (including the 5 new tests)

- [ ] **Step 9: Commit**

```bash
git add internal/store/store.go internal/store/notes_test.go
git commit -m "feat: add notes field to Feature CRUD and dev_complete status"
```

---

### Task 3: MCP Tools — Add `notes` Param and `dev_complete` Status

**Files:**
- Modify: `internal/mcp/tools.go`
- Modify: `internal/mcp/tools_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/mcp/tools_test.go`:

```go
func TestNotesInWorkflow(t *testing.T) {
	s := testStore(t)

	f, err := s.AddFeature("Notes Feature", "testing notes in MCP")
	if err != nil {
		t.Fatalf("AddFeature: %v", err)
	}

	err = s.UpdateFeature(f.ID, store.FeatureUpdate{Notes: strPtr("user notes here")})
	if err != nil {
		t.Fatalf("UpdateFeature: %v", err)
	}

	f, _ = s.GetFeature(f.ID)
	if f.Notes != "user notes here" {
		t.Errorf("Notes = %q, want %q", f.Notes, "user notes here")
	}
}

func TestDevCompleteStatusInWorkflow(t *testing.T) {
	s := testStore(t)

	s.AddFeature("Dev Done Feature", "")
	err := s.UpdateFeature("dev-done-feature", store.FeatureUpdate{Status: strPtr("dev_complete")})
	if err != nil {
		t.Fatalf("UpdateFeature: %v", err)
	}

	f, _ := s.GetFeature("dev-done-feature")
	if f.Status != "dev_complete" {
		t.Errorf("Status = %q, want dev_complete", f.Status)
	}
}
```

- [ ] **Step 2: Run tests to verify they pass** (these use store directly, which we already updated)

Run: `go test ./internal/mcp/ -v -run "TestNotesInWorkflow|TestDevCompleteStatusInWorkflow"`
Expected: PASS

- [ ] **Step 3: Update add_feature tool to accept notes param**

In `internal/mcp/tools.go`, in the `registerTools` function, update the `add_feature` tool registration to add a notes param:

```go
srv.AddTool(mcp.NewTool("add_feature",
	mcp.WithDescription("Create a new feature to track. Returns the generated slug ID."),
	mcp.WithString("title", mcp.Required(), mcp.Description("Feature title (e.g., 'Bluetooth Panel')")),
	mcp.WithString("description", mcp.Description("What the feature is")),
	mcp.WithString("status", mcp.Description("Initial status: planned (default), in_progress, blocked, dev_complete")),
	mcp.WithString("notes", mcp.Description("User notes — thoughts, ideas, context for Claude to read when picking up this feature")),
), addFeatureHandler(s))
```

- [ ] **Step 4: Update addFeatureHandler to write notes**

In `addFeatureHandler`, after the status update block, add notes handling:

```go
func addFeatureHandler(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		title := args["title"].(string)
		desc, _ := args["description"].(string)
		status, _ := args["status"].(string)
		notes, _ := args["notes"].(string)

		f, err := s.AddFeature(title, desc)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if status != "" && status != "planned" {
			s.UpdateFeature(f.ID, store.FeatureUpdate{Status: &status})
		}
		if notes != "" {
			s.UpdateFeature(f.ID, store.FeatureUpdate{Notes: &notes})
		}
		f, _ = s.GetFeature(f.ID)

		data, _ := json.MarshalIndent(f, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}
```

- [ ] **Step 5: Update update_feature tool to accept notes param**

In `registerTools`, update the `update_feature` tool registration:

```go
srv.AddTool(mcp.NewTool("update_feature",
	mcp.WithDescription("Update a feature's status, description, left_off note, notes, worktree_path, or key_files."),
	mcp.WithString("id", mcp.Required(), mcp.Description("Feature slug ID")),
	mcp.WithString("status", mcp.Description("New status: planned, in_progress, done, blocked, dev_complete")),
	mcp.WithString("title", mcp.Description("New title")),
	mcp.WithString("description", mcp.Description("New description")),
	mcp.WithString("left_off", mcp.Description("Where work stopped — free text")),
	mcp.WithString("notes", mcp.Description("User notes — thoughts, ideas, context for Claude")),
	mcp.WithString("worktree_path", mcp.Description("Absolute path to git worktree")),
	mcp.WithString("key_files", mcp.Description("Comma-separated list of key file paths for this feature")),
), updateFeatureHandler(s))
```

- [ ] **Step 6: Update updateFeatureHandler to write notes**

In `updateFeatureHandler`, add after the `left_off` block:

```go
if v, ok := args["notes"].(string); ok {
	u.Notes = &v
}
```

- [ ] **Step 7: Update list_features tool description to include dev_complete**

In `registerTools`, update the `list_features` tool:

```go
mcp.WithString("status", mcp.Description("Filter by status: planned, in_progress, done, blocked, dev_complete. Omit for all.")),
```

- [ ] **Step 8: Update getContextHandler to output notes**

In `getContextHandler`, add after the `KeyFiles` output block:

```go
if f.Notes != "" {
	fmt.Fprintf(&b, "User notes: %s\n", f.Notes)
}
```

- [ ] **Step 9: Run all MCP tests**

Run: `go test ./internal/mcp/ -v`
Expected: ALL PASS

- [ ] **Step 10: Run all tests**

Run: `go test ./... -v`
Expected: ALL PASS

- [ ] **Step 11: Commit**

```bash
git add internal/mcp/tools.go internal/mcp/tools_test.go
git commit -m "feat: add notes param and dev_complete status to MCP tools"
```

---

### Task 4: Dashboard API — Include `notes` in Responses

**Files:**
- Modify: `internal/dashboard/dashboard.go`
- Modify: `internal/dashboard/dashboard_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/dashboard/dashboard_test.go`:

```go
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

	// Verify notes appears in JSON response
	if !strings.Contains(w.Body.String(), "important") {
		t.Errorf("response doesn't contain notes: %s", w.Body.String())
	}
}

func strPtr(s string) *string { return &s }
```

Wait — there's already a `strPtr` in `dashboard_test.go`? No, checking the existing file, there isn't one. But it exists in `tools_test.go`. Add it to this test file.

- [ ] **Step 2: Run tests to verify they pass**

Run: `go test ./internal/dashboard/ -v -run "TestPatchFeatureNotesAPI|TestListFeaturesIncludesNotes"`
Expected: PASS — the store layer already handles notes, and the API uses `FeatureUpdate` directly from the JSON body which now includes `Notes`. The `GET /api/features` response embeds `store.Feature` which now has `Notes`.

This should already work because:
- The PATCH handler decodes into `store.FeatureUpdate` which has `Notes` field
- The GET handler returns `store.Feature` which now has `Notes` field
- Both were updated in Task 2

If they fail, it means the `featureWithProgress` struct in `dashboard.go` needs adjustment (it embeds `store.Feature` so it should inherit `Notes` automatically).

- [ ] **Step 3: Commit**

```bash
git add internal/dashboard/dashboard_test.go
git commit -m "test: add dashboard API tests for notes field"
```

---

### Task 5: Frontend — Full Dashboard Rewrite

**Files:**
- Modify: `dashboard/index.html` (full rewrite)

This is the largest task. The reference mockup is at `.superpowers/brainstorm/7163-1774717832/content/memphis-dashboard-mockup.html`. The production version needs to be data-driven (fetching from the API) rather than hardcoded.

- [ ] **Step 1: Read the mockup for reference**

Read `.superpowers/brainstorm/7163-1774717832/content/memphis-dashboard-mockup.html` to understand the exact CSS, HTML structure, and interaction patterns to replicate.

- [ ] **Step 2: Write the full dashboard HTML**

Rewrite `dashboard/index.html` with the complete Memphis Dark themed dashboard. Key differences from the mockup:

**CSS:** Copy all styles from the mockup verbatim. They are already finalized and theme-compliant.

**HTML structure:** Keep the same `<header>`, but the `<div class="board">` and `<div class="panel-overlay">` are generated dynamically by JavaScript.

**JavaScript — required functions:**

1. `initTheme()` / `toggleTheme()` — keep existing theme toggle logic, update CSS variables for light mode
2. `load()` — fetch `GET /api/features`, call `render()`, call `loadUnlinked()`
3. `render()` — build the 5-column kanban board:
   - Columns in order: `planned`, `in_progress`, `blocked`, `dev_complete`, `done`
   - Column labels: `Planned`, `In Progress`, `Blocked`, `Dev Complete`, `Done`
   - Each column gets the appropriate CSS class (`col-planned`, `col-in-progress`, etc.)
   - For each feature in a column, fetch subtasks from the feature list response (the API returns `progress_done` and `progress_total` already)
   - **Per-task progress rows:** The current API returns only overall progress. We need per-subtask progress. Update the `GET /api/features` endpoint to also return subtask-level progress (see Step 3).
   - Cards show: title, per-task progress rows, left-off snippet
   - Cards have `onclick` → `showDetail(f.id)`
   - No status dropdown
4. `showDetail(id)` — fetch `GET /api/features/{id}?include_archived=true`, populate and show the slide-out panel:
   - Panel structure matches the mockup exactly
   - Left-off is non-editable (just a div, not a textarea)
   - Notes textarea with save button
   - Collapsible task groups
   - Key files as tags
   - Activity section collapsed by default
5. `saveNotes(id)` — PATCH `/api/features/{id}` with `{notes: value}`
6. `closePanel()` — hide the panel overlay
7. `toggleSubtask(header)` — toggle subtask item visibility
8. `toggleActivity()` — toggle activity section visibility
9. Auto-refresh: `setInterval(load, 10000)`

- [ ] **Step 3: Update dashboard API to return per-subtask progress**

In `internal/dashboard/dashboard.go`, update the `GET /api/features` handler. The `featureWithProgress` struct needs to include subtask-level data for cards to render per-task progress rows.

Add a `SubtaskProgress` type and include it in the response:

```go
type subtaskProgress struct {
	Title string `json:"title"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

type featureWithProgress struct {
	store.Feature
	ProgressDone     int               `json:"progress_done"`
	ProgressTotal    int               `json:"progress_total"`
	NextTask         string            `json:"next_task"`
	SubtaskProgress  []subtaskProgress `json:"subtask_progress"`
}
```

In the features loop, populate `SubtaskProgress`:

```go
if total > 0 {
	subtasks, _ := s.GetSubtasksForFeature(f.ID, false)
	for _, st := range subtasks {
		stDone := 0
		for _, item := range st.Items {
			if item.Checked {
				stDone++
			} else if fp.NextTask == "" {
				fp.NextTask = item.Title
			}
		}
		fp.SubtaskProgress = append(fp.SubtaskProgress, subtaskProgress{
			Title: st.Title,
			Done:  stDone,
			Total: len(st.Items),
		})
	}
}
if fp.SubtaskProgress == nil {
	fp.SubtaskProgress = []subtaskProgress{}
}
```

- [ ] **Step 4: Write the complete dashboard/index.html**

Write the full file. The CSS section should be copied from the mockup (all styles are finalized). The JavaScript must:

- Define `STATUSES` as `['planned', 'in_progress', 'blocked', 'dev_complete', 'done']`
- Define `STATUS_LABELS` as `{planned:'Planned', in_progress:'In Progress', blocked:'Blocked', dev_complete:'Dev Complete', done:'Done'}`
- Define `STATUS_CLASSES` as `{planned:'col-planned', in_progress:'col-in-progress', blocked:'col-blocked', dev_complete:'col-dev-complete', done:'col-done'}`
- `render()` builds 5 columns, each with cards containing per-task progress rows from `subtask_progress` array
- `showDetail(id)` fetches the feature and builds the panel with all sections
- Panel notes textarea calls `saveNotes(id)` on button click
- ESC closes panel, clicking overlay closes panel
- `setInterval(load, 10000)` for auto-refresh
- Panel stays open during auto-refresh (don't close it, just refresh the board behind it)

- [ ] **Step 5: Test manually in the browser**

Run: `go build -o docket.exe ./cmd/docket/ && ./docket.exe serve`
Open the dashboard in a browser. Verify:
- 5 columns render correctly
- Cards show per-task progress bars
- Clicking a card opens the slide-out panel
- Left-off is non-editable
- Notes textarea saves on click
- Task groups are collapsible
- Activity section is collapsed by default
- ESC closes the panel
- Clicking another card while panel is open swaps content
- Auto-refresh works (check network tab for 10s polling)

- [ ] **Step 6: Commit**

```bash
git add dashboard/index.html internal/dashboard/dashboard.go
git commit -m "feat: redesign dashboard — Memphis Dark theme, slide-out panel, per-task progress"
```

---

### Task 6: Dashboard API Test Updates

**Files:**
- Modify: `internal/dashboard/dashboard_test.go`

- [ ] **Step 1: Add test for subtask progress in features list**

Add to `internal/dashboard/dashboard_test.go`:

```go
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

	// Verify subtask_progress appears in response
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
```

- [ ] **Step 2: Run all dashboard tests**

Run: `go test ./internal/dashboard/ -v`
Expected: ALL PASS

- [ ] **Step 3: Commit**

```bash
git add internal/dashboard/dashboard_test.go
git commit -m "test: add dashboard API tests for subtask progress and dev_complete"
```

---

### Task 7: Final Integration Test

**Files:**
- No new files — run existing tests

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: Build the binary**

Run: `go build -ldflags="-s -w" -o docket.exe ./cmd/docket/`
Expected: Build succeeds

- [ ] **Step 3: Commit any remaining changes**

If there are any unstaged changes from fixes during testing:

```bash
git add -A
git commit -m "fix: integration fixes from full test suite"
```

If no changes, skip this step.
