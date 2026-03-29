# docket

Local feature tracker for Claude Code sessions. MCP server + SQLite + web dashboard.

## Build

```
go build -ldflags="-s -w" -o docket.exe ./cmd/docket/
```

## Test

```
go test ./...
```

## Install

```
bash install.sh
```

Builds binary to `~/.local/share/docket/docket.exe`, installs plugin to `~/.claude/plugins/marketplaces/local/docket/`.

## Key Files

- `cmd/docket/main.go` — entry point (serve, init, version commands)
- `internal/mcp/server.go` — MCP server setup
- `internal/mcp/tools.go` — MCP tool implementations
- `internal/store/store.go` — SQLite data layer, Feature/Session structs
- `internal/store/migrate.go` — schema migrations
- `internal/store/decision.go` — decision log operations
- `internal/store/subtask.go` — subtask/task item operations
- `internal/store/import.go` — plan file parser
- `internal/dashboard/dashboard.go` — HTTP handler for web UI
- `dashboard/index.html` — frontend (embedded in binary)
- `plugin/` — Claude Code plugin (agent, skill, MCP config)
- `cmd/docket/hook.go` — SessionStart/PostToolUse/Stop hook handlers
- `cmd/docket/handoff.go` — handoff file renderer and writer
- `cmd/docket/update.go` — CLAUDE.md snippet sync command

## Dashboard

http://localhost:<port> (port is per-project, see `.docket/port`) (runs while MCP server is active)

## Architecture

docket.exe runs two things in parallel:
1. MCP server on stdio (Claude Code talks to this)
2. HTTP dashboard on a per-project port (user opens in browser)

Both read/write the same SQLite database at `<project>/.docket/features.db`.

## SQLite Gotchas

- `datetime('now')` has second-level precision — use `ORDER BY id DESC` not `ORDER BY created_at DESC` when insertion order matters within the same second.

## Adding Schema Migrations

Add a new `const schemaVN` in `migrate.go`, then `db.Exec(schemaVN)` in `migrate()`. Use `CREATE TABLE IF NOT EXISTS` or `ALTER TABLE` — errors are ignored for idempotency. No version tracking table.

## Test Pattern

Store tests: `s, _ := Open(t.TempDir())` gives a fresh DB. No mocks, no cleanup needed.

## Feature Tracking (docket)

This project uses `docket` for feature tracking. Dashboard: http://localhost:7890 (or run `/docket`).

**Small tasks** (cosmetic changes, one-off fixes, config tweaks): call `quick_track` MCP tool directly — one call, no agent dispatch needed. Pass title, commit_hash, and key_files.

**Larger features** (multi-step, plan-driven, complex): dispatch the `board-manager` agent (model: sonnet) at these points:
1. **Before writing any code for a new task** — if the user asks to build, fix, or add something, dispatch board-manager FIRST to create or find a feature card. Do not write code until the card exists. Skip only for questions, reviews, and lookups.
2. **After a commit** — pass commit hash, message, files, feature ID
3. **After subagent implementation work** — subagent commits bypass PostToolUse hooks. After an implementer subagent returns with commits, dispatch board-manager with all new commit hashes, messages, and files. Don't wait for per-commit dispatches — batch them.

Session logging is handled automatically by the Stop hook (no agent dispatch needed).

Carry the feature ID the agent returns across dispatches. `get_ready` stays in main session.

**If user rejects a board-manager dispatch**, fix the issue (e.g., missing context) and retry — don't silently drop the dispatch for the rest of the session.
