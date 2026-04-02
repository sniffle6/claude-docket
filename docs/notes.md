# Notes

Append-only notes on feature cards for recording findings, context, and observations during work.

## What it does

The `add_note` MCP tool lets Claude append freeform text notes to a feature card. Unlike the `notes` field on features (which is a single text blob set via `update_feature` and gets overwritten), these are individual timestamped entries that accumulate over time.

## Why it exists

When Claude is told "add your findings" or "save this context", there was no clean append mechanism. The existing `notes` field on features overwrites on each update. Notes fill the gap between decisions (which are structured accept/reject) and issues (which track bugs) — they're for general observations, findings, and context that future sessions should know about.

## How to use it

MCP tool:
- `add_note(feature_id, content)` — appends a note to the feature card

Notes show up in:
- `get_feature` — full JSON output includes `notes` array
- `get_context` — compact briefing shows note snippets (truncated to 80 chars)
- `get_full_context` — complete dump includes all notes
- Dashboard detail panel — "Notes (N)" section between Decisions and Issues

## Design choices

- **Freeform** — no categories or labels. Claude can include context in the note text itself.
- **Append-only** — no edit or delete. Matches the decisions pattern and prevents accidental data loss.
- **Ordered by newest first** — `ORDER BY id DESC` for consistent display.

## Key files

- `internal/store/note.go` — Note struct, AddNote, GetNotesForFeature
- `internal/store/note_test.go` — store layer tests
- `internal/store/migrate.go` — schema v15 (notes table)
- `internal/mcp/tools_note.go` — add_note MCP handler
- `internal/mcp/tools.go` — tool registration
- `internal/mcp/tools_feature.go` — get_feature/get_context/get_full_context updated to include notes
- `internal/dashboard/dashboard.go` — API endpoint includes notes
- `dashboard/index.html` — frontend rendering
