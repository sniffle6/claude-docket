# Feature Templates & Completion Gate

## What It Does

Two features that make docket's feature tracking more consistent and reliable:

1. **Feature types with templates** — when creating a feature, pass a `type` (feature, bugfix, chore, spike) to auto-generate a standard subtask structure.
2. **Completion gate** — features can't be marked `done` until all task items are checked and all issues are resolved. Override with `force=true` (logs a decision).

## How to Use

### Creating a typed feature (MCP tool)

```json
{
    "title": "Fix login timeout",
    "type": "bugfix"
}
```

This creates the feature AND generates subtasks:
- Investigation: Reproduce the bug, Identify root cause
- Fix: Implement fix, Add regression test

### Available types and their templates

- **feature** — Planning, Implementation, Polish
- **bugfix** — Investigation, Fix
- **chore** — Work (single phase)
- **spike** — Research (single phase)

### Completion gate

Marking `status=done` checks:
- All task items on non-archived subtasks must be checked
- All issues must be resolved

If either fails, the update is rejected with a message listing what's outstanding.

To override: pass `force=true` and optionally `force_reason="why"`. This logs an accepted decision on the feature for audit.

## Gotchas

- Templates are fire-and-forget. Once created, subtasks are independent of the type.
- `import_plan` archives template-generated subtasks (same as any existing subtasks).
- Features with no subtasks pass the completion gate (nothing to check).
- `quick_track` calls `UpdateFeature` directly — but quick-tracked features rarely have subtasks so the gate passes.

## Key Files

- `internal/store/templates.go` — template definitions and ApplyTemplate
- `internal/store/store.go` — Feature struct (Type field), UpdateFeature (completion gate)
- `internal/store/migrate.go` — schema v7 (type column)
- `internal/mcp/tools.go` — add_feature (type param), update_feature (force/force_reason params)
- `dashboard/index.html` — type badge rendering
