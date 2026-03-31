# Tags and Archival

## What it does

Two additions to feature tracking:

1. **Tags** — free-form string tags on features for categorization. Comma-separated input, stored as JSON array in SQLite. New-tag warnings help catch typos by listing existing tags when you add one that doesn't exist yet.

2. **Archival** — `archived` is a feature status. Done features auto-archive after 7 days (checked on SessionStart). Archived features are hidden from `list_features` and the dashboard by default. No special tool needed — use `update_feature(status="planned")` to unarchive.

## How to use

**Tags:**
- `add_feature(title="...", tags="auth,frontend")` — set tags on creation
- `update_feature(id="...", tags="backend,api")` — replace all tags
- `list_features(tag="auth")` — filter by tag

**Archival:**
- Features marked `done` for 7+ days auto-archive on next session start
- `list_features(status="archived")` — see archived features
- `update_feature(id="...", status="planned")` — unarchive
- Dashboard has an archive toggle button in the header

## Gotchas

- Tags are exact-match only. No fuzzy matching. The new-tag warning is your typo detector.
- `tags` param on `update_feature` replaces all tags, not appends. Send the full list.
- No `list_tags` or `delete_tag` tool. Tags are derived from what features have — unused tags disappear naturally.
- Auto-archive is hardcoded to 7 days. No config file.
- Archiving bypasses the completion gate — you can archive features with unchecked items.
- `get_feature` and `get_context` return any feature regardless of status, including archived.

## Key files

- `internal/store/migrate.go` — schemaV8 (tags column)
- `internal/store/store.go` — Tags field, GetKnownTags, CheckNewTags, ListFeaturesWithTag, AutoArchiveStale
- `internal/store/tags_test.go` — tag tests
- `internal/store/archive_test.go` — archival tests
- `internal/mcp/tools.go` — tags/tag params on MCP tools
- `internal/mcp/tools_feature.go` — tag handling in handlers
- `cmd/docket/hook.go` — auto-archive in SessionStart
- `dashboard/index.html` — tag pills, archived toggle
