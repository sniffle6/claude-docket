# Session Heartbeat & Staleness Detection

Detects crashed or killed Claude Code sessions and shows them as "Stale" on the dashboard instead of falsely showing "Working."

## How It Works

Heartbeat is updated from two sources:

1. **State-change hooks** — Stop, PreCompact, and PostToolUse (on state flip or git commit) update `last_heartbeat` in SQLite directly.
2. **Periodic PostToolUse heartbeat** — Every PostToolUse checks a file timestamp (`heartbeat-{sessionID}`). If >2 minutes since last update, it opens SQLite and touches the heartbeat. This keeps heartbeats fresh during normal tool use without opening SQLite on every call.

The dashboard compares heartbeat age against a 5-minute threshold. If the heartbeat is older than 5 minutes and the session is still marked "working," "needs_attention," or "subagent," the dashboard shows a gray "Stale (Xm)" indicator.

Staleness is a display-only concern — the DB stays in its last known state. The MCP server dies with the Claude session, so it can't update its own state when it crashes.

## Dashboard Indicators

| State | Indicator | Action |
|-------|-----------|--------|
| Working | Green pulsing dot | Session is live |
| Waiting | Yellow pulsing dot | Claude needs input |
| Subagent | Purple pulsing dot | Claude dispatched a subagent |
| Launching | Blue pulsing dot | Dashboard launch placeholder; real session pending |
| Stale | Gray static dot + "(Xm)" | Session probably dead |
| Idle | No indicator | No active session |

Stale and launching cards show a force-close button (×) that immediately closes the work session via `DELETE /api/sessions/{featureId}`.

## Cleanup

- **Stale sessions** auto-resolve when a new session starts — `OpenWorkSession` closes other open sessions for the same feature.
- **Placeholder sessions** are auto-closed after 3 minutes if the hook hasn't upgraded them (terminal alive or not).
- **Sessions without mcp_pid** are reclaimable after 5 minutes of stale heartbeat in both the features list and launch endpoint.
- **Force-close** — dashboard × button closes any stale or launching session immediately.

## Heartbeat File Lifecycle

- `SessionStart` creates `heartbeat-{sessionID}` (fresh file).
- `PostToolUse` touches the file every 2+ minutes (throttle gate).
- `SessionEnd` removes the file.

## Key Files

- `internal/store/worksession.go` — `TouchHeartbeat`, `SessionStateInfo`, `GetActiveSessionStates`
- `cmd/docket/hook.go` — heartbeat calls in Stop, PreCompact, PostToolUse hooks; periodic heartbeat throttle
- `dashboard/index.html` — staleness evaluation, rendering, force-close button
- `internal/dashboard/dashboard.go` — liveness checks (PID, placeholder age, heartbeat staleness), `DELETE /api/sessions` endpoint
