# Transcript-Based Session Context

Replace Docket's two-phase Stop flow with checkpoint-based context preservation that separates Claude runtime lifecycle from Docket work-session lifecycle.

## Problem

Docket currently blocks Claude at Stop and asks it to call `log_session` with a self-written summary. This has three problems:

1. **Reliability** — Claude sometimes skips `log_session` or writes thin summaries.
2. **Speed** — the two-phase stop blocks exit and forces an extra round-trip.
3. **Richness** — valuable context from the conversation (reasoning, dead ends, discoveries) is thrown away. The self-summary captures a fraction of what was actually discussed.

The transcript file contains everything, but mechanical extraction alone is insufficient. Files-edited and tests-run data is reconstructible from git; the real value is the semantic context — what worked, what didn't, what was learned. Extracting that requires an LLM.

## Terminology

**Claude session** — a runtime session managed by Claude Code hooks. Begins at SessionStart, ends at SessionEnd.

**Docket work session** — a Docket-defined span of work on a specific feature. May continue across many Claude turns. Ends when Claude exits or the user explicitly closes it.

**Checkpoint** — a persisted snapshot of meaningful semantic progress plus structured mechanical facts collected since the previous checkpoint.

**Handoff** — a rendered markdown summary for the next fresh Claude session, used to cold-start with status, progress, context, and next steps.

## Design Principles

- Stop means "Claude finished the current reply," not "session ended."
- PreCompact is the "protect semantic state before compression" event. Claude re-reads CLAUDE.md after compaction but conversation-only context is lost.
- SessionEnd is for final flush and cleanup, not heavy summarization. It cannot block termination and has a 1.5s default timeout.
- Docket supports user-declared boundaries (`/end-session`) separate from Claude exit.
- Hooks never call the model directly. They extract, enqueue, and return.
- A long-lived Docket worker (in the existing MCP server process) drains checkpoint jobs.

## Architecture

### Components

1. **Transcript filter** (`internal/transcript`) — reads JSONL transcript from a byte offset, strips tool results/inputs/system messages/thinking blocks, keeps assistant and user text blocks. Returns filtered text, new offset, and mechanical facts (files edited, tests run, errors). Knows nothing about docket or features.

2. **Checkpoint job queue** — `checkpoint_jobs` table in SQLite. Hooks insert rows, the background worker drains them.

3. **Background summarizer worker** — runs inside the existing long-lived Docket MCP server process. Polls `checkpoint_jobs`, calls the Anthropic Messages API with filtered transcript delta, writes results to `checkpoint_observations`.

4. **Revised hook handlers** — SessionStart opens work sessions, Stop/PreCompact enqueue checkpoints, SessionEnd does cheap finalization and handoff rendering.

5. **Handoff renderer** — assembles handoff markdown from DB state (feature status, checkpoints, mechanical facts, enrichment sections).

### Data Flow

```
Claude turn ends
       |
    Stop hook fires
       |
    Read transcript delta (from last offset)
    Extract mechanical facts
    Filter to user/assistant text
       |
    Delta meaningful? ----no----> allow stop, done
       |yes
    Insert checkpoint_job row
    Allow stop immediately
       |
    [Background worker picks up job]
       |
    Call Anthropic Messages API (haiku-tier)
    with filtered transcript delta
       |
    Write checkpoint_observations
    Update transcript offset
```

## State Machine

### States

| State | Meaning |
|-------|---------|
| NoActiveWorkSession | No feature bound to an open work session |
| ActiveClean | Work session open, no meaningful uncheckpointed delta |
| ActiveDirty | Work session open, new state not yet checkpointed |
| CheckpointQueued | Checkpoint job enqueued or running |
| Closing | Finalizing work session, writing handoff |

### Transitions

| From | To | Trigger |
|------|----|---------|
| NoActiveWorkSession | ActiveClean | SessionStart opens/resumes work session |
| ActiveClean | ActiveDirty | New meaningful transcript delta, file edits, commits, failures |
| ActiveDirty | CheckpointQueued | Stop, PreCompact, `/checkpoint`, `/end-session` |
| CheckpointQueued | ActiveClean | Checkpoint completes, no newer delta |
| CheckpointQueued | ActiveDirty | Checkpoint completes, newer dirty state arrived |
| ActiveClean | Closing | `/end-session` or SessionEnd |
| ActiveDirty | Closing | `/end-session` or SessionEnd (forces final cheap flush) |
| Closing | NoActiveWorkSession | Handoff written, work session closed |

## Hook Responsibilities

### SessionStart

Resolve active feature. Open or resume a Docket work session. Load latest handoff and recent durable context for injection. Transition to ActiveClean.

Does not create a checkpoint or write a handoff.

### Stop

Read transcript delta from last cursor. Extract semantic delta (user/assistant text only). Merge structured mechanical facts collected since last checkpoint.

If delta is trivial: leave state unchanged, allow stop.
If delta is meaningful: insert `checkpoint_jobs` row, transition to CheckpointQueued, allow stop immediately.

Does not block Claude. Does not close the work session. Does not write handoff.

### PreCompact

Force checkpoint enqueue even if the normal threshold is not met. Uses the same summarizer pipeline as Stop — no special-case code. The checkpoint job includes transcript delta, trigger type (manual/auto), and structured facts since last checkpoint.

Does not close the work session. Does not write handoff.

PostCompact is deferred to v1.1+ (optional observer storing `compact_summary` as a non-canonical debugging observation).

### SessionEnd

Final cheap flush of already-collected mechanical state. Render handoff from completed checkpoints and structured facts. Mark work session closed. Transition through Closing to NoActiveWorkSession.

Does not wait for in-flight semantic checkpoint jobs. Does not perform LLM calls.

If an in-flight job finishes after SessionEnd, it writes to `checkpoint_observations` only. It must not reopen the closed work session. The next handoff rewrite or next session can consume that late observation.

## Meaningful Delta Threshold

A checkpoint is required if any of these are true:

- Filtered semantic text since last checkpoint >= 300 chars
- At least 1 non-trivial user message in the delta
- Any commit occurred
- Any test failure or tool failure occurred
- Any explicit `/checkpoint` or `/end-session`
- Any PreCompact trigger

A delta is trivial if all are true:

- Filtered semantic text < 300 chars
- No commit
- No failure
- No explicit checkpoint/end-session
- No PreCompact

"Non-trivial user message" in v1: not empty, not only acknowledgment/control text (ok, thanks, continue, go on, run it, yep). Deterministic filter, tunable later.

## Docket Commands

### /checkpoint

Both a plugin skill and an MCP tool. Forces transcript delta extraction and enqueues a checkpoint if anything meaningful exists. Keeps the work session open.

Safe for both manual and programmatic use.

### /end-session

Plugin skill only in v1. Not exposed as an MCP tool — Claude must not autonomously close a work session.

Forces a final checkpoint if needed. Renders and writes handoff. Closes the work session. Transitions to NoActiveWorkSession.

Exists because Claude's SessionEnd only covers real session termination, not user-declared work boundaries inside a still-open CLI session.

## Feature Binding

Checkpoints bind to the active Docket work session at enqueue time. The work session's `feature_id` is the source of truth.

- If no active work session exists, skip semantic checkpointing and log a debug event.
- If multiple features are in_progress but no work session is open, only SessionStart may pick a fallback. Normal hooks must not guess.
- Feature status (`in_progress`, `done`, etc.) answers "is this work ongoing overall?"
- Work session status (`open`, `closed`) answers "is there an active Docket session bound right now?"

A feature may remain `in_progress` after its work session closes.

## Checkpoint Model

### What a checkpoint contains

**Semantic state** (from LLM summarization of filtered transcript delta):
- Clarified goals
- Decisions discussed
- Blockers discovered
- Dead ends and what didn't work
- Next-step intent

**Mechanical state** (deterministic extraction):
- Files edited (path + action + count)
- Tests and commands run
- Commit hashes and messages
- Failures and error summaries

### Checkpoint identity

Each checkpoint is idempotent. Key: `session_id + transcript_start_offset + transcript_end_offset + feature_id`. Prevents duplicate observations if hooks retrigger or the process retries.

## Handoff Generation

### Inputs

- Current feature status and progress
- Left-off state
- Next tasks
- Deduplicated key files (auto-merged from checkpoint file edits)
- Recent checkpoint observations
- Open blockers
- Recent commits/tests/errors
- Preserved enrichment sections (Decisions & Context, Gotchas, Recommended Approach)

### Write points

**Write now:** `/end-session`, SessionEnd.

**Do not write:** normal Stop, PreCompact.

### Sections

- Status
- Current objective (from most recent checkpoint semantic state)
- Last Session (accumulated checkpoint observations + mechanical facts)
- Next Tasks
- Key Files
- Blockers / Gotchas
- Recent commits/tests/errors

## Summarizer Configuration

Reuse `ANTHROPIC_API_KEY` — no separate `DOCKET_API_KEY`.

| Variable | Purpose | Default |
|----------|---------|---------|
| `ANTHROPIC_API_KEY` | API authentication | required for semantic checkpoints |
| `DOCKET_SUMMARIZER_MODEL` | Model for checkpoint summarization | baked-in cheap default (haiku-tier) |
| `DOCKET_SUMMARIZER_ENABLED` | Toggle | `true` if API key exists |

If no API key exists, Docket falls back to mechanical-only checkpoints. All hook flows still work — checkpoints just lack semantic observations.

### Backend interface

```go
type SummarizerBackend interface {
    Summarize(ctx context.Context, input SummarizeInput) (*SummarizeOutput, error)
}
```

v1 ships with a direct Anthropic Messages API implementation in Go. No Agent SDK dependency.

## Schema Changes

### New tables

**work_sessions**

| Column | Type | Notes |
|--------|------|-------|
| id | INTEGER PRIMARY KEY | |
| feature_id | TEXT NOT NULL | FK to features |
| claude_session_id | TEXT | from hook input |
| status | TEXT | open, closed |
| started_at | DATETIME | |
| ended_at | DATETIME | nullable |
| handoff_stale | BOOLEAN | true if handoff render failed |

**checkpoint_jobs**

| Column | Type | Notes |
|--------|------|-------|
| id | INTEGER PRIMARY KEY | |
| work_session_id | INTEGER | FK to work_sessions |
| feature_id | TEXT NOT NULL | copied from work session at enqueue |
| reason | TEXT | stop, precompact, manual_checkpoint, manual_end_session |
| trigger | TEXT | auto, manual, null |
| transcript_start_offset | INTEGER | byte offset |
| transcript_end_offset | INTEGER | byte offset |
| status | TEXT | queued, running, done, failed, skipped |
| error | TEXT | nullable |
| created_at | DATETIME | |
| started_at | DATETIME | nullable |
| finished_at | DATETIME | nullable |

**checkpoint_observations**

| Column | Type | Notes |
|--------|------|-------|
| id | INTEGER PRIMARY KEY | |
| checkpoint_job_id | INTEGER | FK to checkpoint_jobs |
| work_session_id | INTEGER | FK to work_sessions |
| feature_id | TEXT NOT NULL | |
| kind | TEXT | summary, blocker, decision_candidate, dead_end, next_step, gotcha |
| payload_json | TEXT | structured data |
| summary_text | TEXT | human-readable text |
| created_at | DATETIME | |

### Existing tables

No changes to features, sessions, subtasks, decisions, issues. The existing `sessions` table remains for coarse session history. Checkpoint observations are the fine-grained semantic layer.

## Removals

- `log_session` MCP tool (registration in `tools.go`, handler in `tools_session.go`)
- Two-phase Stop logic (`stop_hook_active` branching for block/re-trigger)
- `session-logged` sentinel file (`.docket/session-logged`)
- `MarkSessionLogged()`, `WasSessionLogged()`, `ClearSessionLogged()` store methods
- The Stop hook block/reason prompting flow

## What Stays

- `commits.log` — fast append-only buffer for PostToolUse commit detection. Handoff rendering prefers normalized commit facts in DB if they exist. `commits.log` is an ingestion source, not long-term source of truth.
- `compact_sessions` MCP tool — manages historical session data, unrelated to this change.
- SessionStart context injection — loads handoff for new sessions.
- PostToolUse commit tracking — appends to `commits.log`.
- Enrichment section preservation in handoff files.

## Hooks Configuration Changes

Update `hooks.json`:

- Stop timeout: 15s → 30s (headroom for transcript extraction)
- Add PreCompact hook (same command, `docket.exe hook`)
- Add SessionEnd hook (same command, `docket.exe hook`)
- SessionEnd timeout: raise via config if default 1.5s is insufficient for handoff rendering

## Error Handling

- **Transcript missing/unreadable:** Fall back to `commits.log` data. Log mechanical-only checkpoint. Allow stop.
- **Malformed JSONL lines:** Skip individual lines, log to stderr, keep parsing.
- **No active work session:** Skip semantic checkpointing, log debug event.
- **Summarizer API failure:** Mark checkpoint job as failed. Mechanical facts still stored. Retry on next checkpoint if delta accumulates.
- **Handoff render failure during /end-session:** Set `handoff_stale=true`, log error, close work session anyway. Next SessionStart or explicit handoff regeneration repairs it.
- **SessionEnd timeout pressure:** Handoff rendering must be fast (read from DB, template, write file). No LLM calls. If still tight, the timeout can be raised via `CLAUDE_CODE_SESSIONEND_HOOKS_TIMEOUT_MS`.

## Invariants

1. A Stop hook must never be the only path to a durable handoff. Claude may continue for many turns after Stop.
2. SessionEnd must not depend on a slow model round-trip.
3. Any important instruction or conclusion that must survive compaction or fresh sessions must exist outside the conversation.
4. Mechanical facts come from deterministic event capture, not transcript inference.
5. Semantic checkpoints are idempotent and feature-bound at enqueue time.
6. Hooks never call the model directly. They enqueue.

## Key Files (post-implementation)

- `internal/transcript/` — JSONL parser, delta filter, mechanical fact extractor
- `internal/checkpoint/` — job queue, worker, summarizer backend interface
- `internal/checkpoint/anthropic.go` — Messages API summarizer implementation
- `internal/store/migrate.go` — schema v8 (work_sessions, checkpoint_jobs, checkpoint_observations)
- `cmd/docket/hook.go` — revised Stop, new PreCompact, new SessionEnd handlers
- `cmd/docket/handoff.go` — revised renderer consuming checkpoint observations
- `plugin/hooks/hooks.json` — updated hook declarations
- `plugin/skills/checkpoint/SKILL.md` — /checkpoint skill
- `plugin/skills/end-session/SKILL.md` — /end-session skill
