# Dashboard Session Control

Real-time session status indicators on feature cards and launch-with-context from the dashboard.

## What it does

When you're working on a feature in Claude Code, the dashboard shows your session state visually:
- **Working** (green pulsing dot) — Claude is actively working
- **Waiting** (yellow pulsing dot) — Claude needs input or encountered an issue
- **Subagent** (purple pulsing dot) — Claude dispatched a subagent
- **Launching** (blue pulsing dot) — Dashboard play button clicked; real session pending
- **Stale** (gray dot + time) — No heartbeat in 5+ minutes; session probably dead

The dashboard also provides a **Launch** button (▶) on each card that opens a new Claude Code session with full feature context, and a **Focus** button (↗) for active sessions.

## Session State Transitions

State changes automatically based on hook events:

1. **SessionStart** — `working` (session started; placeholder upgraded atomically)
2. **PostToolUse** — `working` (resumes from needs_attention/subagent via sentinel)
3. **Stop** — `needs_attention` or `subagent` (depending on agent-pending sentinel)
4. **SessionEnd** — `idle` → session closed

## Placeholder Lifecycle

When launching from the dashboard:

1. `CreatePlaceholderSession` creates a session with `session_state='launching'` and `claude_session_id='dashboard-launch'`
2. SessionStart hook calls `OpenWorkSession` which atomically upgrades the placeholder: sets `claude_session_id` to the real ID and `session_state` to `'working'`
3. `bind_session` MCP tool claims the session with `mcp_pid` for definitive liveness checking

Placeholders are auto-closed after 3 minutes if never upgraded (hook failed or terminal closed).

## Liveness Checks

The dashboard uses three-valued liveness (alive/dead/unknown):

| Condition | Check | Action |
|-----------|-------|--------|
| `mcp_pid` set | `isPIDRunning(pid)` | Alive → keep state; Dead → close session |
| Placeholder (`dashboard-launch`) | `isWindowAlive()` + age <3min | Alive → keep "launching"; Dead/old → close |
| No PID, not placeholder | Heartbeat age | Fresh (<5min) → trust DB state; Stale → mark unlinked |

## Force Close

Stale and launching cards show a close button (×) that calls `DELETE /api/sessions/{featureId}`. This immediately closes the work session and cleans up PID files. No more waiting for timeouts.

## Launch with Context

Clicking the **Launch** button (▶) on a feature card:

1. Checks for existing active sessions (focus or toast if found)
2. Gathers context: handoff file, feature data, remaining tasks, open issues
3. Writes a launch prompt to `.docket/launch/{feature-id}.md`
4. Generates a platform-specific launcher script (`.cmd` on Windows, shell script on Unix)
5. Opens a new terminal with Claude and the context injected via `--append-system-prompt-file`
6. Creates a placeholder session so the dashboard shows "Launching" immediately

## Key Files

- `internal/store/worksession.go` — `SetSessionState`, `GetActiveSessionStates`, `CreatePlaceholderSession`, `OpenWorkSession` (placeholder upgrade)
- `internal/dashboard/dashboard.go` — `POST /api/launch/{id}`, `DELETE /api/sessions/{featureId}`, liveness checks
- `internal/dashboard/launch.go` — `RenderLaunchPrompt`, `renderLaunchExtras`
- `internal/dashboard/launch_exec_windows.go` — `launchInTerminal`, `focusTerminal`, `isWindowAlive`, `isPIDRunning`
- `cmd/docket/hook.go` — state transitions, periodic heartbeat throttle
- `dashboard/index.html` — session state indicators, launch/focus/close buttons

## Gotchas

- Launch scripts are platform-specific (`.cmd` on Windows, shell on Unix). The `launch.toml` config allows custom launch/focus commands.
- Each launch overwrites the previous prompt file for that feature.
- The periodic heartbeat uses a file-timestamp throttle (`heartbeat-{sessionID}`) to avoid opening SQLite on every tool call — updates every 2 minutes.
