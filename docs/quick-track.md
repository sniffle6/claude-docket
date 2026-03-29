# quick_track

Lightweight tracking for small tasks that don't justify a full board-manager agent dispatch.

## What it does

Single MCP tool call that creates (or updates) a feature card with an optional commit hash and key files attached. Defaults to status `done` since the typical use case is "I just did a small thing."

## When to use it

- Cosmetic changes (add a logo, fix a typo, tweak colors)
- One-off fixes (config change, dependency bump)
- Any task where dispatching board-manager feels like overkill

Use board-manager instead when the work is multi-step, plan-driven, or spans multiple commits.

## How to use it

Call the `quick_track` MCP tool directly from the main session:

| Parameter | Required | Description |
|-----------|----------|-------------|
| `title` | yes | What was done — becomes the feature slug ID |
| `commit_hash` | no | Git commit SHA to attach (stored in a session) |
| `key_files` | no | Comma-separated file paths touched |
| `status` | no | `done` (default), `in_progress`, `planned` |

If a feature with the same slug already exists, it updates the existing one (merges key_files, updates status) instead of creating a duplicate.

## How it works internally

1. Slugifies the title, checks if the feature exists
2. Creates or updates the feature with status and key_files
3. If `commit_hash` is provided, logs a session with the commit attached
4. Returns the feature ID and whether it was created or updated

No schema migration needed — uses existing `features` and `sessions` tables.

## Key files

- `internal/store/store.go` — `QuickTrack` method and `QuickTrackInput`/`QuickTrackResult` types
- `internal/mcp/tools.go` — `quick_track` tool registration and handler
- `internal/store/quicktrack_test.go` — store-level tests
