# Decision Log Per Feature — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add structured decision logging to feature cards so Claude can see rejected approaches and avoid re-exploring dead ends across sessions.

**Architecture:** New `decisions` table with feature FK, new store methods in `decision.go`, one new MCP tool (`add_decision`), decisions surfaced in existing context endpoints and dashboard panel.

**Tech Stack:** Go, SQLite, vanilla JS (dashboard)

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/store/decision.go` | Create | Decision struct, AddDecision, GetDecisionsForFeature |
| `internal/store/decision_test.go` | Create | Store-layer tests for decisions |
| `internal/store/migrate.go` | Modify | Add schemaV5 with decisions table |
| `internal/store/store.go` | Modify | Update GetContext, FeatureContext to include decisions |
| `internal/mcp/tools.go` | Modify | Register add_decision tool and handler |
| `internal/dashboard/dashboard.go` | Modify | Include decisions in feature detail API response |
| `dashboard/index.html` | Modify | Render decisions section in slide-out panel |

---

### Task 1: Schema Migration

**Files:**
- Modify: `internal/store/migrate.go`

- [ ] **Step 1: Add schemaV5 constant**

Add after the existing `schemaV4` constant in `internal/store/migrate.go`:

```go
const schemaV5 = `
CREATE TABLE IF NOT EXISTS decisions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	feature_id TEXT NOT NULL REFERENCES features(id),
	approach TEXT NOT NULL,
	outcome TEXT NOT NULL CHECK(outcome IN ('accepted', 'rejected')),
	reason TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
`
```

- [ ] **Step 2: Apply migration in migrate function**

Add after the `db.Exec(schemaV4)` line in the `migrate` function:

```go
// v5: add decisions table (ignore error if already exists)
db.Exec(schemaV5)
```

- [ ] **Step 3: Run tests to verify migration doesn't break anything**

Run: `go test ./internal/store/ -v`
Expected: All existing tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrate.go
git commit -m "feat: add schemaV5 with decisions table"
```

---

### Task 2: Decision Store Layer

**Files:**
- Create: `internal/store/decision.go`
- Create: `internal/store/decision_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/store/decision_test.go`:

```go
package store

import (
	"testing"
)

func TestAddAndGetDecisions(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// Create a feature to attach decisions to
	f, err := s.AddFeature("Test Feature", "desc")
	if err != nil {
		t.Fatalf("AddFeature: %v", err)
	}

	// Add a rejected decision
	d1, err := s.AddDecision(f.ID, "Use websockets", "rejected", "Too complex for MVP")
	if err != nil {
		t.Fatalf("AddDecision: %v", err)
	}
	if d1.Approach != "Use websockets" {
		t.Errorf("got approach %q, want %q", d1.Approach, "Use websockets")
	}
	if d1.Outcome != "rejected" {
		t.Errorf("got outcome %q, want %q", d1.Outcome, "rejected")
	}
	if d1.Reason != "Too complex for MVP" {
		t.Errorf("got reason %q, want %q", d1.Reason, "Too complex for MVP")
	}

	// Add an accepted decision
	d2, err := s.AddDecision(f.ID, "Use polling", "accepted", "Simple and sufficient")
	if err != nil {
		t.Fatalf("AddDecision: %v", err)
	}

	// Retrieve all decisions
	decisions, err := s.GetDecisionsForFeature(f.ID)
	if err != nil {
		t.Fatalf("GetDecisionsForFeature: %v", err)
	}
	if len(decisions) != 2 {
		t.Fatalf("got %d decisions, want 2", len(decisions))
	}
	// Ordered by created_at DESC, so d2 first
	if decisions[0].ID != d2.ID {
		t.Errorf("expected newest decision first, got id %d want %d", decisions[0].ID, d2.ID)
	}
}

func TestAddDecisionInvalidOutcome(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	f, _ := s.AddFeature("Test Feature", "desc")

	_, err = s.AddDecision(f.ID, "Something", "maybe", "dunno")
	if err == nil {
		t.Fatal("expected error for invalid outcome, got nil")
	}
}

func TestAddDecisionInvalidFeature(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	_, err = s.AddDecision("nonexistent", "Something", "rejected", "reason")
	if err == nil {
		t.Fatal("expected error for nonexistent feature, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -run TestAdd -v`
Expected: FAIL — `AddDecision` and `GetDecisionsForFeature` not defined.

- [ ] **Step 3: Write the implementation**

Create `internal/store/decision.go`:

```go
package store

import (
	"fmt"
	"time"
)

type Decision struct {
	ID        int64     `json:"id"`
	FeatureID string    `json:"feature_id"`
	Approach  string    `json:"approach"`
	Outcome   string    `json:"outcome"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) AddDecision(featureID, approach, outcome, reason string) (*Decision, error) {
	res, err := s.db.Exec(
		`INSERT INTO decisions (feature_id, approach, outcome, reason) VALUES (?, ?, ?, ?)`,
		featureID, approach, outcome, reason,
	)
	if err != nil {
		return nil, fmt.Errorf("insert decision: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.getDecision(id)
}

func (s *Store) getDecision(id int64) (*Decision, error) {
	var d Decision
	err := s.db.QueryRow(
		`SELECT id, feature_id, approach, outcome, reason, created_at FROM decisions WHERE id = ?`, id,
	).Scan(&d.ID, &d.FeatureID, &d.Approach, &d.Outcome, &d.Reason, &d.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get decision %d: %w", id, err)
	}
	return &d, nil
}

func (s *Store) GetDecisionsForFeature(featureID string) ([]Decision, error) {
	rows, err := s.db.Query(
		`SELECT id, feature_id, approach, outcome, reason, created_at FROM decisions WHERE feature_id = ? ORDER BY created_at DESC`,
		featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("get decisions: %w", err)
	}
	defer rows.Close()

	var decisions []Decision
	for rows.Next() {
		var d Decision
		if err := rows.Scan(&d.ID, &d.FeatureID, &d.Approach, &d.Outcome, &d.Reason, &d.CreatedAt); err != nil {
			return nil, err
		}
		decisions = append(decisions, d)
	}
	return decisions, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -run TestAdd -v`
Expected: All 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/decision.go internal/store/decision_test.go
git commit -m "feat: add Decision store layer with tests"
```

---

### Task 3: MCP Tool — add_decision

**Files:**
- Modify: `internal/mcp/tools.go`

- [ ] **Step 1: Register the add_decision tool**

Add this tool registration inside `registerTools()` in `internal/mcp/tools.go`, after the `get_full_context` tool registration:

```go
srv.AddTool(mcp.NewTool("add_decision",
	mcp.WithDescription("Log a decision on a feature. Records what approach was considered, whether it was accepted or rejected, and why. Prevents re-exploring dead ends across sessions."),
	mcp.WithString("feature_id", mcp.Required(), mcp.Description("Feature slug ID")),
	mcp.WithString("approach", mcp.Required(), mcp.Description("What was considered (e.g., 'Use websockets for real-time updates')")),
	mcp.WithString("outcome", mcp.Required(), mcp.Description("accepted or rejected")),
	mcp.WithString("reason", mcp.Required(), mcp.Description("Why — one-liner (e.g., 'Too complex for MVP, polling sufficient')")),
), addDecisionHandler(s))
```

- [ ] **Step 2: Write the handler function**

Add this handler function in `internal/mcp/tools.go`, after the `getFullContextHandler` function:

```go
func addDecisionHandler(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		featureID := args["feature_id"].(string)
		approach := args["approach"].(string)
		outcome := args["outcome"].(string)
		reason := args["reason"].(string)

		if outcome != "accepted" && outcome != "rejected" {
			return mcp.NewToolResultError("outcome must be 'accepted' or 'rejected'"), nil
		}

		d, err := s.AddDecision(featureID, approach, outcome, reason)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Decision #%d logged: %s %s — %s", d.ID, outcome, approach, reason)), nil
	}
}
```

- [ ] **Step 3: Build to verify compilation**

Run: `go build ./cmd/docket/`
Expected: Compiles without errors.

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/tools.go
git commit -m "feat: add add_decision MCP tool"
```

---

### Task 4: Surface Decisions in Context Endpoints

**Files:**
- Modify: `internal/store/store.go` (GetContext, FeatureContext)
- Modify: `internal/mcp/tools.go` (getContextHandler, getFeatureHandler, getFullContextHandler)

- [ ] **Step 1: Add Decisions field to FeatureContext**

In `internal/store/store.go`, update the `FeatureContext` struct:

```go
type FeatureContext struct {
	Feature        Feature    `json:"feature"`
	RecentSessions []Session  `json:"recent_sessions"`
	Decisions      []Decision `json:"decisions"`
}
```

- [ ] **Step 2: Update GetContext to load rejected decisions**

In `internal/store/store.go`, update the `GetContext` method. Add this before the `return` statement at the end of `GetContext`:

```go
decisions, err := s.GetDecisionsForFeature(id)
if err != nil {
	return nil, err
}
if decisions == nil {
	decisions = []Decision{}
}
```

And update the return to include decisions:

```go
return &FeatureContext{Feature: *f, RecentSessions: sessions, Decisions: decisions}, nil
```

- [ ] **Step 3: Update getContextHandler to render rejected decisions**

In `internal/mcp/tools.go`, in the `getContextHandler` function, add this block after the progress section and before the recent sessions section (before `if len(fc.RecentSessions) > 0 {`):

```go
var rejected []store.Decision
for _, d := range fc.Decisions {
	if d.Outcome == "rejected" {
		rejected = append(rejected, d)
	}
}
if len(rejected) > 0 {
	b.WriteString("Rejected approaches:\n")
	for _, d := range rejected {
		fmt.Fprintf(&b, "  - %s — %s\n", d.Approach, d.Reason)
	}
}
```

- [ ] **Step 4: Update getFeatureHandler to include decisions**

In `internal/mcp/tools.go`, in the `getFeatureHandler` function, update the anonymous struct and its population. Replace the existing `fullFeature` struct and `full` assignment:

```go
type fullFeature struct {
	store.Feature
	Subtasks  []store.Subtask  `json:"subtasks"`
	Sessions  []store.Session  `json:"sessions"`
	Decisions []store.Decision `json:"decisions"`
}
decisions, _ := s.GetDecisionsForFeature(id)
if decisions == nil {
	decisions = []store.Decision{}
}
full := fullFeature{Feature: *f, Subtasks: subtasks, Sessions: sessions, Decisions: decisions}
```

- [ ] **Step 5: Update getFullContextHandler to include decisions**

In `internal/mcp/tools.go`, in the `getFullContextHandler` function, update the anonymous struct and its population. Replace the existing `fullDump` struct and data assignment:

```go
type fullDump struct {
	Feature   store.Feature    `json:"feature"`
	Subtasks  []store.Subtask  `json:"subtasks"`
	Sessions  []store.Session  `json:"sessions"`
	Decisions []store.Decision `json:"decisions"`
}
decisions, _ := s.GetDecisionsForFeature(id)
if decisions == nil {
	decisions = []store.Decision{}
}
data, _ := json.MarshalIndent(fullDump{
	Feature:   *f,
	Subtasks:  subtasks,
	Sessions:  sessions,
	Decisions: decisions,
}, "", "  ")
```

- [ ] **Step 6: Build to verify compilation**

Run: `go build ./cmd/docket/`
Expected: Compiles without errors.

- [ ] **Step 7: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass.

- [ ] **Step 8: Commit**

```bash
git add internal/store/store.go internal/mcp/tools.go
git commit -m "feat: surface decisions in get_context, get_feature, get_full_context"
```

---

### Task 5: Dashboard API — Include Decisions in Feature Detail

**Files:**
- Modify: `internal/dashboard/dashboard.go`

- [ ] **Step 1: Update the GET /api/features/{id} handler**

In `internal/dashboard/dashboard.go`, update the feature detail endpoint to include decisions. Replace the `writeJSON` line in the `GET /api/features/{id}` handler:

```go
decisions, _ := s.GetDecisionsForFeature(id)
if decisions == nil {
	decisions = []store.Decision{}
}
writeJSON(w, map[string]any{"feature": f, "subtasks": subtasks, "sessions": sessions, "decisions": decisions})
```

- [ ] **Step 2: Build to verify compilation**

Run: `go build ./cmd/docket/`
Expected: Compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add internal/dashboard/dashboard.go
git commit -m "feat: include decisions in feature detail API response"
```

---

### Task 6: Dashboard UI — Render Decisions

**Files:**
- Modify: `dashboard/index.html`

- [ ] **Step 1: Add CSS styles for decisions**

In `dashboard/index.html`, add these styles before the closing `</style>` tag:

```css
/* Decisions */
.decision-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 8px 0;
  border-top: 1px solid var(--border);
  font-size: 13px;
}
.decision-item:first-child { border-top: none; }
.decision-outcome {
  flex-shrink: 0;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  padding: 2px 6px;
  border-radius: 3px;
  margin-top: 1px;
}
.decision-accepted { background: #95E06C20; color: var(--done); }
.decision-rejected { background: #C73A3A20; color: var(--blocked); }
html.light .decision-accepted { background: #D5F0D0; color: #2A6A22; }
html.light .decision-rejected { background: #FFD0D0; color: #8A2020; }
.decision-text { flex: 1; }
.decision-approach { color: var(--text); }
.decision-reason { color: var(--muted); font-size: 12px; margin-top: 2px; }
```

- [ ] **Step 2: Add decisions rendering in showDetail function**

In `dashboard/index.html`, in the `showDetail` function, add this block after the "Key Files" section (after the `panel.appendChild(kfWrap)` closing brace) and before the "Activity" section:

```javascript
// Decisions
var decisions = data.decisions || [];
if (decisions.length > 0) {
  var decLabel = document.createElement('div');
  decLabel.className = 'panel-section-label';
  decLabel.textContent = 'Decisions';
  panel.appendChild(decLabel);

  for (var di = 0; di < decisions.length; di++) {
    var dec = decisions[di];
    var decDiv = document.createElement('div');
    decDiv.className = 'decision-item';

    var outSpan = document.createElement('span');
    outSpan.className = 'decision-outcome decision-' + dec.outcome;
    outSpan.textContent = dec.outcome;
    decDiv.appendChild(outSpan);

    var textDiv = document.createElement('div');
    textDiv.className = 'decision-text';
    var appDiv = document.createElement('div');
    appDiv.className = 'decision-approach';
    appDiv.textContent = dec.approach;
    textDiv.appendChild(appDiv);
    if (dec.reason) {
      var reasonDiv = document.createElement('div');
      reasonDiv.className = 'decision-reason';
      reasonDiv.textContent = dec.reason;
      textDiv.appendChild(reasonDiv);
    }
    decDiv.appendChild(textDiv);
    panel.appendChild(decDiv);
  }
}
```

- [ ] **Step 3: Build and verify**

Run: `go build ./cmd/docket/`
Expected: Compiles (dashboard HTML is embedded at build time).

- [ ] **Step 4: Commit**

```bash
git add dashboard/index.html
git commit -m "feat: render decisions in dashboard feature detail panel"
```

---

### Task 7: Update Documentation

**Files:**
- Modify: `CLAUDE.md`
- Create or modify: `docs/decision-log.md`

- [ ] **Step 1: Update CLAUDE.md key files list**

Add `internal/store/decision.go` to the Key Files section in `CLAUDE.md`:

```
- `internal/store/decision.go` — decision log operations
```

- [ ] **Step 2: Write feature doc**

Create `docs/decision-log.md`:

```markdown
# Decision Log

Structured decision tracking per feature. Prevents Claude from re-exploring dead ends across sessions.

## What It Does

Each feature can have decisions logged against it. A decision records:
- **Approach**: what was considered (e.g., "Use websockets for real-time updates")
- **Outcome**: `accepted` or `rejected`
- **Reason**: why (e.g., "Too complex for MVP, polling sufficient")

## How It Works

- `add_decision` MCP tool logs a decision on a feature
- `get_context` shows rejected decisions in the briefing so Claude avoids dead ends
- `get_feature` and `get_full_context` include all decisions (accepted + rejected)
- Dashboard shows decisions in the feature detail panel

## Key Files

- `internal/store/decision.go` — Decision struct, AddDecision, GetDecisionsForFeature
- `internal/store/migrate.go` — schemaV5 with decisions table
- `internal/mcp/tools.go` — add_decision tool handler
- `dashboard/index.html` — decisions section in feature detail panel
```

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md docs/decision-log.md
git commit -m "docs: add decision log documentation"
```

---

### Task 8: Full Integration Test

- [ ] **Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass including new decision tests.

- [ ] **Step 2: Build final binary**

Run: `go build -ldflags="-s -w" -o docket.exe ./cmd/docket/`
Expected: Binary builds successfully.

- [ ] **Step 3: Verify no regressions**

Run: `go vet ./...`
Expected: No issues.
