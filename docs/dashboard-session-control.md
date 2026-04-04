# Dashboard Session Control

Real-time session status indicators on feature cards and launch-with-context from the dashboard.

## What it does

When you're working on a feature in Claude Code, the dashboard shows your session state visually:
- **Idle** (grey dot) — No active session
- **Working** (red dot) — Claude is actively working (tool use, editing, etc.)
- **Needs Attention** (yellow dot) — Claude encountered an error, test failure, or stopped unexpectedly
- **Subagent** — Claude dispatched a subagent and is waiting for it to return
- **Launching** — Dashboard play button was clicked; placeholder session pending real Claude session

The dashboard also provides a **Launch** button on each card that opens a new Claude Code session in Windows Terminal with full feature context injected via `--append-system-prompt-file` and an initial prompt telling Claude which feature to resume.

## How it works

### Session State Transitions

Session state changes automatically based on hook events:

1. **SessionStart hook** — Set to `working` (Claude session started)
2. **PreToolUse hook** — Set to `working` (any tool used, including when resuming from `needs_attention`)
3. **Stop hook** — Set to `needs_attention` (Claude paused, waiting for user input)
4. **SessionEnd hook** — Set to `idle` (session ended, handoff logged)

### Launch with Context

Clicking the **Launch** button on a feature card:

1. Checks if the feature already has an active session (prevents duplicate launches)
2. Gathers current context: feature title, status, description, notes, unchecked tasks, open issues, key files
3. Renders a launch prompt file (markdown) at `.docket/launch/{feature-id}.md`
4. Generates a `.cmd` launcher script at `.docket/launch/{feature-id}.cmd` that runs:
   ```
   claude --dangerously-skip-permissions --append-system-prompt-file "<prompt-file>" "Resume work on: <title> (feature_id: <id>). Check get_ready for current status."
   ```
5. Opens the `.cmd` in Windows Terminal via `start "" wt cmd /k "<script>"`

The launched Claude session gets:
- **System prompt** — full feature context (handoff, remaining tasks, open issues, key files)
- **Initial user message** — tells Claude which feature to resume and to call `get_ready`
- **Skip permissions** — runs with `--dangerously-skip-permissions` for unattended operation

## Status Indicators on Dashboard

Feature cards display a colored dot in the top-right corner:

| State | Color | Meaning |
|---|---|---|
| **idle** | Grey | No active work session |
| **working** | Red (primary) | Claude is actively working |
| **needs_attention** | Yellow (accent) | Work stopped or encountered an issue — check Claude Code output |
| **subagent** | — | Claude dispatched a subagent; waiting for it to return |
| **launching** | — | Dashboard launch initiated; real session hasn't started yet |

## Key Files

- `internal/store/worksession.go` — `SetSessionState`, `GetActiveSessionStates` — session state CRUD
- `internal/store/handoff.go` — `LaunchData` struct, `GetLaunchData` — gather context for launch
- `internal/dashboard/dashboard.go` — `POST /api/launch/{id}` endpoint, `.cmd` script generation, `session_state` in API responses
- `internal/dashboard/launch.go` — `RenderLaunchPrompt`, `renderLaunchExtras` — launch prompt rendering
- `cmd/docket/hook.go` — State transitions (SessionStart→working, PreToolUse→working, Stop→needs_attention, SessionEnd→idle)
- `dashboard/index.html` — Session state indicator UI, launch button, toast notifications

## Gotchas

### Windows-Only Launch

The `.cmd` script and `start "" wt cmd /k` approach is Windows-specific. For macOS/Linux support, the launch mechanism would need platform detection.

### Active Session Check

Clicking Launch on a feature that already has an open work session fails with a toast error. This prevents accidental duplicate sessions. Close the existing session first (wait for the Stop hook to run), then launch again.

### Prompt File Overwrite

Each launch overwrites the previous prompt file for that feature. If you want to preserve a prompt, rename it before launching again.
