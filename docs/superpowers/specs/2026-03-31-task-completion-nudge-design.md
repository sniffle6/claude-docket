# Task Completion Nudge — Design Spec

## Problem

Docket never checks off its own completed tasks. After a commit, the PostToolUse hook sends a vague system message: "complete_task_item if applicable." The LLM regularly ignores this because it doesn't know *which* tasks exist or which ones the commit might satisfy. By the time the completion gate catches unchecked items (when marking a feature `done`), the LLM has lost context.

## Solution

Enrich the PostToolUse commit nudge with actual unchecked task items (IDs and titles) so the LLM has everything it needs to act immediately.

No auto-completion. No fuzzy matching. The LLM still decides which tasks a commit satisfies — it just can't claim ignorance anymore.

## Design

### Enrich PostToolUse commit nudge

**File**: `cmd/docket/hook.go`, `handlePostToolUse` function (line ~359)

**Current behavior** (line 432):
```
[docket] Commit recorded: abc123 fix input validation
Update feature "Auth System": update_feature (left_off, key_files) and complete_task_item if applicable.
```

**New behavior**:
```
[docket] Commit recorded: abc123 fix input validation
Feature "Auth System" — unchecked tasks:
  #14: Add validation to input handler
  #15: Write unit tests for validator
  #16: Update API docs
Call complete_task_item for any items this commit completes, then update_feature (left_off, key_files).
```

**Implementation**:

After the commit is logged to `commits.log` and the active feature is found, query unchecked task items:

```go
subtasks, err := s.GetSubtasksForFeature(features[0].ID, false)
// iterate subtasks → items where !item.Checked
// format as "  #ID: Title" lines
```

- If unchecked items exist: include the task list in the system message
- If zero unchecked items: skip the task list, just prompt for `update_feature`
- Cap the list at 10 items to avoid bloating the system message (append "... and N more" if truncated)

Also applies to the plan-import branch (line ~428): when a plan file is auto-imported, the nudge should still include the unchecked task list after the import message.

## What this does NOT do

- **No fuzzy matching** — the LLM decides which task a commit satisfies
- **No auto-completion** — tasks require explicit `complete_task_item` calls with outcome metadata
- **No new schema** — uses existing data from `GetSubtasksForFeature`
- **No new MCP tools** — purely hook-side change
- **No PreToolUse gate** — deferred to a future iteration if needed

## Edge cases

| Scenario | Behavior |
|---|---|
| Feature with no task items | Skip task list, just show `update_feature` prompt (existing behavior) |
| All tasks already checked | Skip task list, confirm commit recorded + prompt `update_feature` |
| 10+ unchecked items | Truncate list, append "... and N more" |
| Subtasks with no items | Skipped (no items to list) |
| Plan-import commit | Import message first, then unchecked task list |

## Files to modify

- `cmd/docket/hook.go` — enrich `handlePostToolUse` system message
- `cmd/docket/hook_test.go` — test the new nudge formatting

## Test plan

1. **PostToolUse with unchecked tasks**: Commit fires → system message includes task item IDs and titles
2. **PostToolUse with all tasks checked**: Commit fires → system message omits task list
3. **PostToolUse with no tasks**: Commit fires → existing behavior unchanged
4. **PostToolUse with 10+ unchecked tasks**: List is capped at 10, shows "... and N more"

## Future work

**PreToolUse Agent gate**: If Change 1 doesn't sufficiently improve task completion rates, add a nudge on Agent dispatch that checks for unchecked tasks + recent commits. Keep it simple — no commit cross-referencing (produces false positives), just "you have unchecked tasks, handle them before dispatching."
