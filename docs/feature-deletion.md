# Feature Deletion

Permanently delete a feature card and all related data.

## What it does

Removes a feature and everything linked to it: subtasks, task items, decisions, notes, issues, sessions, work sessions, checkpoint jobs, and checkpoint observations. Search index entries are auto-cleaned by FTS5 triggers.

## How to use

### MCP tool

```
delete_feature(id: "my-feature", confirm: true)
```

The `confirm` parameter must be `true` — without it, the tool returns an error prompting confirmation.

### Dashboard

Open a feature's detail panel and click the "Delete Feature" button at the top. A browser confirmation dialog prevents accidental deletion.

## Gotchas

- Deletion is permanent — there's no undo. Use archiving (`status=archived`) if you want to hide but keep data.
- Deleting a feature with an active work session closes the session implicitly (the work_sessions row is deleted).

## Key files

- `internal/store/store.go` — `DeleteFeature` method (transactional cascading deletes)
- `internal/mcp/tools.go` — `delete_feature` tool registration
- `internal/mcp/tools_feature.go` — `deleteFeatureHandler`
- `internal/dashboard/dashboard.go` — `DELETE /api/features/{id}` endpoint
- `dashboard/index.html` — delete button in side panel
