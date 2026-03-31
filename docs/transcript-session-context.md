# Transcript-Based Session Context

Docket captures what happened during a Claude Code session — not just commits and files, but reasoning, decisions, dead ends, and gotchas. This context carries over to the next session via handoff files.

## What it does

When Claude works on a feature, docket:
1. Opens a **work session** linking the Claude session to the active feature
2. On each Stop hook (fires every turn), checks if anything meaningful happened
3. If yes, enqueues a **checkpoint job** with the transcript delta
4. A background worker summarizes the delta using the Anthropic API
5. At session end, writes a **handoff file** with all accumulated observations

The next session gets the handoff file injected as context at startup.

## How it works

### Transcript parsing

Claude Code stores conversation transcripts as JSONL files. The parser (`internal/transcript/parse.go`) reads from a byte offset, extracts:
- **Semantic text**: user and assistant messages (no tool results, no file contents)
- **Mechanical facts**: files edited (with counts), test runs (pass/fail), commits (hash + message), errors

Trivial user messages ("ok", "thanks", "continue") are filtered out when deciding if content is meaningful.

### Checkpoint jobs

Each checkpoint is a row in `checkpoint_jobs` with the semantic text, mechanical facts, and transcript byte offsets. Jobs go through: queued → running → done/failed.

The worker polls every 5 seconds, dequeues one job at a time, calls the summarizer, and writes observations.

### Summarizer

The Anthropic Messages API (haiku-tier by default) extracts structured observations:
- Summary narrative
- Blockers, dead ends, decisions, next steps, gotchas

No API key = noop summarizer (only mechanical facts survive).

### Work sessions

A work session ties a Claude session ID to a feature ID. One active at a time. Opened at SessionStart, closed at SessionEnd. Checkpoint observations are grouped by work session.

## How to use it

It's automatic. Just work normally with docket features.

Manual controls:
- `/checkpoint` — force a checkpoint mid-session
- `/end-session` — close the work session without closing Claude (useful when switching features)

## Gotchas

- **API key required for summaries**: Set `ANTHROPIC_API_KEY`. Without it, only mechanical facts are captured (files, tests, commits). No semantic summary.
- **Transcript path**: Claude Code must provide the transcript path in hook input. If missing, transcript parsing is skipped (commits.log still works).
- **Meaningful delta threshold**: Stop only checkpoints if there are commits, errors, failed tests, 300+ chars of text, or non-trivial user input. This avoids noise from quick yes/no exchanges.
- **PreCompact always checkpoints**: No threshold — context compression means data loss, so we save everything before it happens.
- **SessionEnd is cheap**: Just enqueues remaining delta and writes handoffs. No blocking, no model calls.
- **Worker timeout**: 30 seconds per summarization. Failures are logged, job marked failed, and the worker moves on.

## Key files

- `internal/transcript/types.go` — Delta struct, trivial message map
- `internal/transcript/parse.go` — JSONL transcript parser
- `internal/checkpoint/summarizer.go` — SummarizerBackend interface
- `internal/checkpoint/anthropic.go` — Anthropic Messages API implementation
- `internal/checkpoint/noop.go` — No-op summarizer (no API key)
- `internal/checkpoint/config.go` — Config from env vars
- `internal/checkpoint/worker.go` — Background job queue worker
- `internal/store/worksession.go` — Work session CRUD
- `internal/store/checkpoint.go` — Checkpoint job + observation CRUD
- `internal/store/migrate.go` — Schema v9 (work_sessions, checkpoint_jobs, checkpoint_observations)
- `cmd/docket/hook.go` — Hook handlers (Stop, PreCompact, SessionEnd, SessionStart)
- `cmd/docket/handoff.go` — Handoff renderer with Last Session section
- `plugin/hooks/hooks.json` — Hook declarations
- `plugin/skills/checkpoint/SKILL.md` — /checkpoint skill
- `plugin/skills/end-session/SKILL.md` — /end-session skill
