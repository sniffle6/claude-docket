# Decision Log Per Feature — Design Spec

## Problem

The biggest token waste in Claude Code sessions is re-exploring dead ends. Session N tries approach X, discovers it doesn't work, picks Y. Session N+1 starts fresh, tries X again, burns tokens rediscovering why it fails. Session logs capture what happened but not why something was rejected.

## Solution

Structured decision logging on feature cards. Each decision records what was considered, whether it was accepted or rejected, and why. Surfaced in context endpoints so Claude sees rejected approaches before starting work.

## Data Model

New `decisions` table via `schemaV5` in `internal/store/migrate.go`:

```sql
CREATE TABLE IF NOT EXISTS decisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feature_id TEXT NOT NULL REFERENCES features(id),
    approach TEXT NOT NULL,
    outcome TEXT NOT NULL CHECK(outcome IN ('accepted', 'rejected')),
    reason TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
```

New Go struct in `internal/store/decision.go`:

```go
type Decision struct {
    ID        int64     `json:"id"`
    FeatureID string    `json:"feature_id"`
    Approach  string    `json:"approach"`
    Outcome   string    `json:"outcome"`
    Reason    string    `json:"reason"`
    CreatedAt time.Time `json:"created_at"`
}
```

Store methods in `internal/store/decision.go`:
- `AddDecision(featureID, approach, outcome, reason string) (*Decision, error)`
- `GetDecisionsForFeature(featureID string) ([]Decision, error)` — returns decisions ordered by `created_at DESC`

## MCP Tool

One new tool: `add_decision`.

Parameters:
- `feature_id` (required): Feature slug ID
- `approach` (required): What was considered
- `outcome` (required): `accepted` or `rejected`
- `reason` (required): One-liner explanation

Returns confirmation with decision ID. Any caller (main session Claude, board-manager agent) can use it whenever a decision point occurs.

No standalone `get_decisions` tool. Decisions are retrieved through existing context endpoints.

## Context Integration

### `get_context` (token-efficient briefing)

Appends a "Rejected approaches:" section after progress, before recent sessions. Shows only rejected decisions — one line each with approach and reason. Accepted decisions are self-evident from the code; rejected ones prevent wasted re-exploration.

Example output addition:
```
Rejected approaches:
  - Use websockets for real-time updates — too complex for MVP, polling sufficient
  - Store decisions as JSON blob on feature row — no atomic appends, race conditions
```

### `get_full_context` and `get_feature`

Include all decisions (accepted and rejected) in the JSON response alongside subtasks and sessions.

## Dashboard

Add a "Decisions" section to the feature detail view in `dashboard/index.html`. Simple list with color to distinguish accepted (green) vs rejected (red). No new pages or routes — rendered inline on the existing feature card detail.

## Key Files (after implementation)

- `internal/store/decision.go` — Decision struct, AddDecision, GetDecisionsForFeature
- `internal/store/migrate.go` — schemaV5 with decisions table
- `internal/mcp/tools.go` — add_decision tool registration and handler
- `internal/store/store.go` — updated GetContext, FeatureContext struct
- `dashboard/index.html` — decisions section in feature detail
- `internal/store/decision_test.go` — store layer tests

## Testing

Tests in `internal/store/decision_test.go`:
- Add a decision, retrieve it
- Filter by outcome (accepted vs rejected via GetDecisionsForFeature)
- Verify FK constraint (decision on nonexistent feature fails)

MCP handler coverage follows existing patterns.

## What This Doesn't Do

- No batch `add_decisions` tool — one at a time is fine until proven otherwise
- No subtask-level decisions — feature-level only, timestamps provide session context
- No editing or deleting decisions — they're an append-only log
