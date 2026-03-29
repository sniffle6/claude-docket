# Issue Tracking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add issue/bug tracking to docket feature cards so humans and Claude can log, view, and resolve bugs during QA.

**Architecture:** New `issues` SQLite table linked to features (required) and task_items (optional). Three new MCP tools (add_issue, resolve_issue, list_issues). Dashboard gets issue count badge on cards and an Issues section in the detail panel.

**Tech Stack:** Go, SQLite, vanilla JS (dashboard)

**Spec:** `docs/superpowers/specs/2026-03-29-issue-tracking-design.md`

---

### Task 1: Schema Migration

**Files:**
- Modify: `internal/store/migrate.go`

- [ ] **Step 1: Add schemaV6 constant**

Add after the existing `schemaV5` constant in `internal/store/migrate.go`:

```go
const schemaV6 = `
CREATE TABLE IF NOT EXISTS issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feature_id TEXT NOT NULL REFERENCES features(id),
    task_item_id INTEGER REFERENCES task_items(id),
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open', 'resolved')),
    resolved_commit TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    resolved_at DATETIME
);
`
```

- [ ] **Step 2: Add migration call**

In the `migrate()` function, add after `db.Exec(schemaV5)`:

```go
// v6: add issues table (ignore error if already exists)
db.Exec(schemaV6)
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: compiles with no errors

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrate.go
git commit -m "feat: add issues table schema migration (v6)"
```

---

### Task 2: Store Layer — Issue Struct and AddIssue

**Files:**
- Create: `internal/store/issue.go`
- Create: `internal/store/issue_test.go`

- [ ] **Step 1: Write the failing test for AddIssue**

Create `internal/store/issue_test.go`:

```go
package store

import "testing"

func TestAddIssue(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "desc")

	issue, err := s.AddIssue("test-feature", "Button is broken", nil)
	if err != nil {
		t.Fatalf("AddIssue: %v", err)
	}
	if issue.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if issue.FeatureID != "test-feature" {
		t.Errorf("FeatureID = %q, want %q", issue.FeatureID, "test-feature")
	}
	if issue.Description != "Button is broken" {
		t.Errorf("Description = %q, want %q", issue.Description, "Button is broken")
	}
	if issue.Status != "open" {
		t.Errorf("Status = %q, want %q", issue.Status, "open")
	}
	if issue.TaskItemID != nil {
		t.Errorf("TaskItemID = %v, want nil", issue.TaskItemID)
	}
}

func TestAddIssueWithTaskItem(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "desc")
	st, _ := s.AddSubtask("test-feature", "Phase 1", 1)
	item, _ := s.AddTaskItem(st.ID, "Do thing", 1)

	taskItemID := item.ID
	issue, err := s.AddIssue("test-feature", "Thing is buggy", &taskItemID)
	if err != nil {
		t.Fatalf("AddIssue: %v", err)
	}
	if issue.TaskItemID == nil || *issue.TaskItemID != taskItemID {
		t.Errorf("TaskItemID = %v, want %d", issue.TaskItemID, taskItemID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestAddIssue -v`
Expected: FAIL — `AddIssue` not defined

- [ ] **Step 3: Write Issue struct and AddIssue method**

Create `internal/store/issue.go`:

```go
package store

import (
	"fmt"
	"time"
)

type Issue struct {
	ID             int64      `json:"id"`
	FeatureID      string     `json:"feature_id"`
	TaskItemID     *int64     `json:"task_item_id"`
	Description    string     `json:"description"`
	Status         string     `json:"status"`
	ResolvedCommit string     `json:"resolved_commit"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

func (s *Store) AddIssue(featureID, description string, taskItemID *int64) (*Issue, error) {
	res, err := s.db.Exec(
		`INSERT INTO issues (feature_id, task_item_id, description) VALUES (?, ?, ?)`,
		featureID, taskItemID, description,
	)
	if err != nil {
		return nil, fmt.Errorf("insert issue: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.getIssue(id)
}

func (s *Store) getIssue(id int64) (*Issue, error) {
	var issue Issue
	var taskItemID *int64
	var resolvedAt *time.Time
	err := s.db.QueryRow(
		`SELECT id, feature_id, task_item_id, description, status, resolved_commit, created_at, resolved_at FROM issues WHERE id = ?`, id,
	).Scan(&issue.ID, &issue.FeatureID, &taskItemID, &issue.Description, &issue.Status, &issue.ResolvedCommit, &issue.CreatedAt, &resolvedAt)
	if err != nil {
		return nil, fmt.Errorf("get issue %d: %w", id, err)
	}
	issue.TaskItemID = taskItemID
	issue.ResolvedAt = resolvedAt
	return &issue, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestAddIssue -v`
Expected: PASS (both TestAddIssue and TestAddIssueWithTaskItem)

- [ ] **Step 5: Commit**

```bash
git add internal/store/issue.go internal/store/issue_test.go
git commit -m "feat: add Issue struct and AddIssue store method"
```

---

### Task 3: Store Layer — ResolveIssue

**Files:**
- Modify: `internal/store/issue.go`
- Modify: `internal/store/issue_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/store/issue_test.go`:

```go
func TestResolveIssue(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "desc")

	issue, _ := s.AddIssue("test-feature", "Bug found", nil)

	err := s.ResolveIssue(issue.ID, "abc123")
	if err != nil {
		t.Fatalf("ResolveIssue: %v", err)
	}

	resolved, _ := s.getIssue(issue.ID)
	if resolved.Status != "resolved" {
		t.Errorf("Status = %q, want %q", resolved.Status, "resolved")
	}
	if resolved.ResolvedCommit != "abc123" {
		t.Errorf("ResolvedCommit = %q, want %q", resolved.ResolvedCommit, "abc123")
	}
	if resolved.ResolvedAt == nil {
		t.Error("ResolvedAt should be set")
	}
}

func TestResolveIssueNoCommit(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "desc")

	issue, _ := s.AddIssue("test-feature", "Minor bug", nil)

	err := s.ResolveIssue(issue.ID, "")
	if err != nil {
		t.Fatalf("ResolveIssue: %v", err)
	}

	resolved, _ := s.getIssue(issue.ID)
	if resolved.Status != "resolved" {
		t.Errorf("Status = %q, want %q", resolved.Status, "resolved")
	}
	if resolved.ResolvedCommit != "" {
		t.Errorf("ResolvedCommit = %q, want empty", resolved.ResolvedCommit)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestResolveIssue -v`
Expected: FAIL — `ResolveIssue` not defined

- [ ] **Step 3: Implement ResolveIssue**

Add to `internal/store/issue.go`:

```go
func (s *Store) ResolveIssue(id int64, commitHash string) error {
	res, err := s.db.Exec(
		`UPDATE issues SET status = 'resolved', resolved_commit = ?, resolved_at = ? WHERE id = ?`,
		commitHash, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("resolve issue: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("issue %d not found", id)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestResolveIssue -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/issue.go internal/store/issue_test.go
git commit -m "feat: add ResolveIssue store method"
```

---

### Task 4: Store Layer — Query Methods

**Files:**
- Modify: `internal/store/issue.go`
- Modify: `internal/store/issue_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/store/issue_test.go`:

```go
func TestGetIssuesForFeature(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "desc")
	s.AddIssue("test-feature", "Bug 1", nil)
	s.AddIssue("test-feature", "Bug 2", nil)
	issue3, _ := s.AddIssue("test-feature", "Bug 3", nil)
	s.ResolveIssue(issue3.ID, "fix123")

	issues, err := s.GetIssuesForFeature("test-feature")
	if err != nil {
		t.Fatalf("GetIssuesForFeature: %v", err)
	}
	if len(issues) != 3 {
		t.Fatalf("len = %d, want 3", len(issues))
	}
	// Open issues first (newest first), then resolved
	if issues[0].Description != "Bug 2" {
		t.Errorf("first issue = %q, want %q", issues[0].Description, "Bug 2")
	}
	if issues[1].Description != "Bug 1" {
		t.Errorf("second issue = %q, want %q", issues[1].Description, "Bug 1")
	}
	if issues[2].Status != "resolved" {
		t.Errorf("third issue status = %q, want %q", issues[2].Status, "resolved")
	}
}

func TestGetOpenIssueCount(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Test Feature", "desc")
	s.AddIssue("test-feature", "Bug 1", nil)
	s.AddIssue("test-feature", "Bug 2", nil)
	issue3, _ := s.AddIssue("test-feature", "Bug 3", nil)
	s.ResolveIssue(issue3.ID, "")

	count, err := s.GetOpenIssueCount("test-feature")
	if err != nil {
		t.Fatalf("GetOpenIssueCount: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestGetAllOpenIssues(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Feature A", "desc")
	s.AddFeature("Feature B", "desc")
	s.AddIssue("feature-a", "Bug in A", nil)
	s.AddIssue("feature-b", "Bug in B", nil)
	resolved, _ := s.AddIssue("feature-a", "Fixed bug", nil)
	s.ResolveIssue(resolved.ID, "")

	issues, err := s.GetAllOpenIssues()
	if err != nil {
		t.Fatalf("GetAllOpenIssues: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("len = %d, want 2", len(issues))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -run "TestGetIssues|TestGetOpen|TestGetAll" -v`
Expected: FAIL — methods not defined

- [ ] **Step 3: Implement query methods**

Add to `internal/store/issue.go`:

```go
func (s *Store) GetIssuesForFeature(featureID string) ([]Issue, error) {
	rows, err := s.db.Query(
		`SELECT id, feature_id, task_item_id, description, status, resolved_commit, created_at, resolved_at
		 FROM issues WHERE feature_id = ?
		 ORDER BY CASE WHEN status = 'open' THEN 0 ELSE 1 END, id DESC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("get issues: %w", err)
	}
	defer rows.Close()
	return scanIssues(rows)
}

func (s *Store) GetOpenIssueCount(featureID string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM issues WHERE feature_id = ? AND status = 'open'`,
		featureID,
	).Scan(&count)
	return count, err
}

func (s *Store) GetAllOpenIssues() ([]Issue, error) {
	rows, err := s.db.Query(
		`SELECT id, feature_id, task_item_id, description, status, resolved_commit, created_at, resolved_at
		 FROM issues WHERE status = 'open' ORDER BY id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("get all open issues: %w", err)
	}
	defer rows.Close()
	return scanIssues(rows)
}

func scanIssues(rows *sql.Rows) ([]Issue, error) {
	var issues []Issue
	for rows.Next() {
		var issue Issue
		var taskItemID *int64
		var resolvedAt *time.Time
		if err := rows.Scan(&issue.ID, &issue.FeatureID, &taskItemID, &issue.Description, &issue.Status, &issue.ResolvedCommit, &issue.CreatedAt, &resolvedAt); err != nil {
			return nil, err
		}
		issue.TaskItemID = taskItemID
		issue.ResolvedAt = resolvedAt
		issues = append(issues, issue)
	}
	return issues, nil
}
```

Note: Add `"database/sql"` to the import block in `issue.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -run "TestGetIssues|TestGetOpen|TestGetAll" -v`
Expected: PASS (all 3 tests)

- [ ] **Step 5: Run full store test suite**

Run: `go test ./internal/store/ -v`
Expected: all tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/store/issue.go internal/store/issue_test.go
git commit -m "feat: add issue query methods (GetIssuesForFeature, GetOpenIssueCount, GetAllOpenIssues)"
```

---

### Task 5: MCP Tools — add_issue, resolve_issue, list_issues

**Files:**
- Modify: `internal/mcp/tools.go`

- [ ] **Step 1: Register the three tools**

In `registerTools()` in `internal/mcp/tools.go`, add after the `quick_track` tool registration (before `add_decision`):

```go
srv.AddTool(mcp.NewTool("add_issue",
    mcp.WithDescription("Log a bug or issue on a feature card. Issues are visible on the dashboard and in get_feature output."),
    mcp.WithString("feature_id", mcp.Required(), mcp.Description("Feature slug ID")),
    mcp.WithString("description", mcp.Required(), mcp.Description("What's wrong — describe the bug")),
    mcp.WithString("task_item_id", mcp.Description("Optional task item ID this issue relates to")),
), addIssueHandler(s))

srv.AddTool(mcp.NewTool("resolve_issue",
    mcp.WithDescription("Mark an issue as resolved. Optionally attach the commit that fixed it."),
    mcp.WithString("id", mcp.Required(), mcp.Description("Issue ID")),
    mcp.WithString("commit_hash", mcp.Description("Commit SHA that fixed the issue")),
), resolveIssueHandler(s))

srv.AddTool(mcp.NewTool("list_issues",
    mcp.WithDescription("List open issues. Filter by feature or list all open issues across all features."),
    mcp.WithString("feature_id", mcp.Description("Filter to one feature. Omit for all open issues.")),
), listIssuesHandler(s))
```

- [ ] **Step 2: Implement handler functions**

Add to `internal/mcp/tools.go` (before `addDecisionHandler`):

```go
func addIssueHandler(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		featureID := args["feature_id"].(string)
		description := args["description"].(string)

		var taskItemID *int64
		if v, ok := args["task_item_id"].(string); ok && v != "" {
			id := parseInt64(v)
			taskItemID = &id
		}

		issue, err := s.AddIssue(featureID, description, taskItemID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		msg := fmt.Sprintf("Issue #%d logged on %s: %s", issue.ID, featureID, description)
		if taskItemID != nil {
			msg += fmt.Sprintf(" (linked to task item #%d)", *taskItemID)
		}
		return mcp.NewToolResultText(msg), nil
	}
}

func resolveIssueHandler(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		id := parseInt64(args["id"].(string))
		commitHash, _ := args["commit_hash"].(string)

		if err := s.ResolveIssue(id, commitHash); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		msg := fmt.Sprintf("Issue #%d resolved.", id)
		if commitHash != "" {
			msg += fmt.Sprintf(" Fix: %s", commitHash)
		}
		return mcp.NewToolResultText(msg), nil
	}
}

func listIssuesHandler(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		featureID, _ := req.GetArguments()["feature_id"].(string)

		var issues []store.Issue
		var err error
		if featureID != "" {
			issues, err = s.GetIssuesForFeature(featureID)
		} else {
			issues, err = s.GetAllOpenIssues()
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if len(issues) == 0 {
			return mcp.NewToolResultText("No open issues."), nil
		}

		var lines []string
		for _, iss := range issues {
			line := fmt.Sprintf("- #%d [%s] %s", iss.ID, iss.FeatureID, iss.Description)
			if iss.Status == "resolved" {
				line += " (resolved"
				if iss.ResolvedCommit != "" {
					line += ": " + iss.ResolvedCommit
				}
				line += ")"
			}
			lines = append(lines, line)
		}
		return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
	}
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: compiles with no errors

- [ ] **Step 4: Run full test suite**

Run: `go test ./...`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/tools.go
git commit -m "feat: add add_issue, resolve_issue, list_issues MCP tools"
```

---

### Task 6: Dashboard API — Issue Count and Issues in Detail

**Files:**
- Modify: `internal/dashboard/dashboard.go`

- [ ] **Step 1: Add issue_count to board API response**

In the `GET /api/features` handler in `dashboard.go`, add an `IssueCount` field to the `featureWithProgress` struct at the top of the file:

```go
type featureWithProgress struct {
	store.Feature
	ProgressDone    int               `json:"progress_done"`
	ProgressTotal   int               `json:"progress_total"`
	NextTask        string            `json:"next_task"`
	SubtaskProgress []subtaskProgress `json:"subtask_progress"`
	IssueCount      int               `json:"issue_count"`
}
```

In the loop that builds `result`, add after the subtask progress block:

```go
fp.IssueCount, _ = s.GetOpenIssueCount(f.ID)
```

- [ ] **Step 2: Add issues to feature detail API response**

In the `GET /api/features/{id}` handler, add issues to the response map. After the `decisions` fetch:

```go
issues, _ := s.GetIssuesForFeature(id)
if issues == nil {
    issues = []store.Issue{}
}
writeJSON(w, map[string]any{"feature": f, "subtasks": subtasks, "sessions": sessions, "decisions": decisions, "issues": issues})
```

Replace the existing `writeJSON` line that doesn't include issues.

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: compiles with no errors

- [ ] **Step 4: Run full test suite**

Run: `go test ./...`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/dashboard/dashboard.go
git commit -m "feat: add issue_count to board API and issues to detail API"
```

---

### Task 7: Dashboard Frontend — Issue Badge on Cards

**Files:**
- Modify: `dashboard/index.html`

- [ ] **Step 1: Add CSS for the issue badge**

Add after the existing `.card-leftoff` styles (around line 193) in the `<style>` block:

```css
.issue-badge {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    background: #C73A3A20;
    color: var(--blocked);
    font-size: 11px;
    font-weight: 600;
    padding: 2px 8px;
    border-radius: 4px;
    margin-top: 6px;
}
html.light .issue-badge { background: #FFD0D0; color: #8A2020; }
```

- [ ] **Step 2: Render issue badge on cards**

In the `render()` function, after the left-off snippet block (after the `if (leftoff)` block, before `col.appendChild(card)`), add:

```javascript
if (f.issue_count > 0) {
    var issueBadge = document.createElement('div');
    issueBadge.className = 'issue-badge';
    issueBadge.textContent = '! ' + f.issue_count + ' issue' + (f.issue_count !== 1 ? 's' : '');
    card.appendChild(issueBadge);
}
```

- [ ] **Step 3: Verify in browser**

Open the dashboard and verify cards with issues show the red badge. Cards without issues should show no badge.

- [ ] **Step 4: Commit**

```bash
git add dashboard/index.html
git commit -m "feat: add issue count badge to dashboard feature cards"
```

---

### Task 8: Dashboard Frontend — Issues Section in Detail Panel

**Files:**
- Modify: `dashboard/index.html`

- [ ] **Step 1: Add CSS for issue items in the detail panel**

Add after the `.decision-reason` style in the `<style>` block:

```css
.issue-item {
    display: flex;
    gap: 10px;
    align-items: flex-start;
    padding: 8px 0;
    border-top: 1px solid var(--border);
}
.issue-item:first-child { border-top: none; }
.issue-status {
    font-size: 11px;
    font-weight: 600;
    padding: 2px 8px;
    border-radius: 4px;
    white-space: nowrap;
    flex-shrink: 0;
}
.issue-open { background: #C73A3A20; color: var(--blocked); }
.issue-resolved { background: #95E06C20; color: var(--done); }
html.light .issue-open { background: #FFD0D0; color: #8A2020; }
html.light .issue-resolved { background: #D5F0D0; color: #2A6A22; }
.issue-text { flex: 1; }
.issue-description { color: var(--text); }
.issue-meta { color: var(--muted); font-size: 12px; margin-top: 2px; }
```

- [ ] **Step 2: Render issues section in the detail panel**

In the `showDetail()` function, after the decisions rendering block (after the closing `}` of the `if (decisions.length > 0)` block) and before the Activity section, add:

```javascript
// Issues
var issues = data.issues || [];
if (issues.length > 0) {
    var issLabel = document.createElement('div');
    issLabel.className = 'panel-section-label';
    var openCount = issues.filter(function(i) { return i.status === 'open'; }).length;
    issLabel.textContent = 'Issues' + (openCount > 0 ? ' (' + openCount + ' open)' : '');
    panel.appendChild(issLabel);

    for (var ii = 0; ii < issues.length; ii++) {
        var iss = issues[ii];
        var issDiv = document.createElement('div');
        issDiv.className = 'issue-item';

        var statusSpan = document.createElement('span');
        statusSpan.className = 'issue-status issue-' + iss.status;
        statusSpan.textContent = iss.status;
        issDiv.appendChild(statusSpan);

        var textDiv = document.createElement('div');
        textDiv.className = 'issue-text';
        var descDiv = document.createElement('div');
        descDiv.className = 'issue-description';
        descDiv.textContent = iss.description;
        textDiv.appendChild(descDiv);

        var meta = [];
        if (iss.task_item_id) {
            meta.push('Task #' + iss.task_item_id);
        }
        if (iss.resolved_commit) {
            meta.push('Fix: ' + iss.resolved_commit.slice(0, 7));
        }
        if (meta.length > 0) {
            var metaDiv = document.createElement('div');
            metaDiv.className = 'issue-meta';
            metaDiv.textContent = meta.join(' \u00B7 ');
            textDiv.appendChild(metaDiv);
        }
        issDiv.appendChild(textDiv);
        panel.appendChild(issDiv);
    }
}
```

- [ ] **Step 3: Verify in browser**

Open a feature detail panel and verify issues appear with correct styling: open issues in red, resolved in green, with description, optional task item link, and commit hash.

- [ ] **Step 4: Commit**

```bash
git add dashboard/index.html
git commit -m "feat: add issues section to dashboard feature detail panel"
```

---

### Task 9: Update Board Manager Agent

**Files:**
- Modify: `plugin/agents/board-manager.md`

- [ ] **Step 1: Add issue tools to the agent's tool table**

In `plugin/agents/board-manager.md`, add these rows to the tool table:

```markdown
| `add_issue` | Logging a bug found during work. Params: `feature_id` (required), `description` (required), `task_item_id` (optional). |
| `resolve_issue` | Marking a bug as fixed. Params: `id` (required), `commit_hash` (optional). |
| `list_issues` | Checking open bugs. Params: `feature_id` (optional, omit for all). |
```

- [ ] **Step 2: Commit**

```bash
git add plugin/agents/board-manager.md
git commit -m "feat: add issue tools to board-manager agent"
```

---

### Task 10: Documentation

**Files:**
- Create: `docs/issue-tracking.md`

- [ ] **Step 1: Write the doc**

Create `docs/issue-tracking.md`:

```markdown
# Issue Tracking

Track bugs and issues found during QA or development on docket feature cards.

## What it does

Issues are bugs logged against a feature card. They have two states: open or resolved. When resolved, the fixing commit hash is recorded for traceability. Issues can optionally link to a specific task item.

## Why it exists

During QA or development, bugs get found. Without a place to log them, they get lost between sessions. Issues give Claude and humans a shared list of known bugs per feature card, visible on the dashboard.

## How to use it

### MCP tools

- `add_issue` — log a bug: `feature_id` (required), `description` (required), `task_item_id` (optional)
- `resolve_issue` — mark fixed: `id` (required), `commit_hash` (optional)
- `list_issues` — see open bugs: `feature_id` (optional, omit for all features)

### Dashboard

- Feature cards show a red `! N issues` badge when there are open issues
- Feature detail panel has an "Issues" section showing all issues (open first, then resolved)

### Workflow

1. Find a bug during QA or development
2. Call `add_issue` with the feature ID and a description of the bug
3. When the bug is fixed, call `resolve_issue` with the issue ID and the commit hash
4. Point Claude at the feature card to fix remaining open issues

## Key files

- `internal/store/issue.go` — Issue struct and store methods
- `internal/store/issue_test.go` — tests
- `internal/mcp/tools.go` — add_issue, resolve_issue, list_issues tool handlers
- `internal/dashboard/dashboard.go` — API endpoints (issue_count on board, issues in detail)
- `dashboard/index.html` — badge on cards, issues section in detail panel
```

- [ ] **Step 2: Commit**

```bash
git add docs/issue-tracking.md
git commit -m "docs: add issue tracking documentation"
```

---

### Task 11: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: all PASS

- [ ] **Step 2: Build binary**

Run: `go build -ldflags="-s -w" -o docket.exe ./cmd/docket/`
Expected: builds successfully

- [ ] **Step 3: Manual smoke test**

Start the server and verify:
1. `add_issue` creates an issue
2. `list_issues` returns it
3. `resolve_issue` marks it resolved
4. Dashboard card shows issue badge
5. Dashboard detail panel shows issues section
