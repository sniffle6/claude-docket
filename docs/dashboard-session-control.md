# Dashboard Session Control

Real-time session status indicators on feature cards and launch-with-context from the dashboard.

## What it does

When you're working on a feature in Claude Code, the dashboard shows your session state visually:
- **Idle** (grey dot) — No active session
- **Working** (red dot) — Claude is actively working (tool use, editing, etc.)
- **Needs Attention** (yellow dot) — Claude encountered an error, test failure, or stopped unexpectedly

The dashboard also provides a **Launch** button on each card that opens a new Claude session with full feature context (status, notes, unchecked tasks, open issues, key files).

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
3. Renders a launch prompt file (markdown) with all that context
4. Substitutes template variables into your launch command
5. Executes the command (terminal opens, text editor loads, etc.)

The generated prompt file lives in `.docket/launch/{feature-id}.md` and includes:
- Feature title and description
- Current status and left-off note
- Remaining unchecked tasks
- Open issues with IDs
- User notes (if any)
- Key files list

## Configuration

### Option 1: Environment Variable (Highest Priority)

Set `DOCKET_LAUNCH_CMD` in your shell:

```bash
# Windows Terminal example
$env:DOCKET_LAUNCH_CMD = 'wt new-tab -d "{{project_dir}}" -e powershell -NoExit -Command "code {{handoff_file}}"'

# Bash example
export DOCKET_LAUNCH_CMD='warp --background-mode --window "{{project_dir}}" -- code "{{handoff_file}}"'
```

### Option 2: Per-Project Config File

Create `.docket/launch.toml` in your project root. The first non-empty, non-comment line is the command template:

```toml
# Launch Claude in Windows Terminal with VS Code
wt new-tab -d "{{project_dir}}" -e pwsh -NoExit -Command "code '{{handoff_file}}'"
```

If both are set, the environment variable takes precedence.

### Template Variables

- `{{handoff_file}}` — Absolute path to the generated launch prompt (e.g., `/home/user/project/.docket/launch/feature-123.md`)
- `{{feature_title}}` — Feature title (e.g., "Add dark mode toggle")
- `{{feature_id}}` — Feature slug ID (e.g., "feature-123")
- `{{project_dir}}` — Project root directory (absolute path)

## Example Configurations

### Windows Terminal + VS Code (PowerShell)

```toml
wt new-tab -d "{{project_dir}}" -e pwsh -NoExit -Command "code '{{handoff_file}}' ; Read-Host 'Press Enter to exit'"
```

This:
- Opens a new Terminal tab in the project directory
- Launches VS Code with the launch prompt file open
- Pauses before closing (useful for reading output)

### Warp Terminal + VS Code

```toml
warp --background-mode --window "{{project_dir}}" -- code "{{handoff_file}}"
```

Warp opens the project in the specified directory and launches VS Code.

### iTerm2 + Vim (macOS)

```toml
open -a iTerm "{{handoff_file}}" ; sleep 0.5 ; osascript -e 'tell application "iTerm" to activate'
```

Creates a new iTerm window and opens the prompt in vim, then focuses iTerm.

## Status Indicators on Dashboard

Feature cards display a colored dot in the top-right corner:

| State | Color | Meaning |
|---|---|---|
| **idle** | Grey | No active work session |
| **working** | Red (primary) | Claude is actively working |
| **needs_attention** | Yellow (accent) | Work stopped or encountered an issue — check Claude Code output |

Hovering over a card with `needs_attention` shows a tooltip: "Session stopped unexpectedly — check Claude Code for details."

## Key Files

- `internal/store/worksession.go` — `SetSessionState`, `GetActiveSessionStates` — session state CRUD
- `internal/store/handoff.go` — `LaunchData` struct, `GetLaunchData` — gather context for launch
- `internal/dashboard/dashboard.go` — `POST /api/launch/{id}` endpoint, `session_state` in API responses
- `internal/dashboard/launch.go` — `RenderLaunchPrompt`, `GetLaunchCmd`, `SubstituteLaunchCmd` — launch prompt generation and command templating
- `cmd/docket/hook.go` — State transitions (SessionStart→working, PreToolUse→working, Stop→needs_attention, SessionEnd→idle)
- `plugin/hooks/hooks.json` — Wildcard PreToolUse matcher to catch all tool uses
- `dashboard/index.html` — Session state indicator UI, launch button, toast notifications

## Gotchas

### Launch Command Not Set

If `DOCKET_LAUNCH_CMD` is empty and `.docket/launch.toml` doesn't exist, the Launch button shows an error toast: "No launch command configured."

Set one of the above before using the launch feature.

### Special Characters in Paths

If your project path contains spaces or special characters, quote it in the command template:

```toml
wt new-tab -d "{{project_dir}}" -e pwsh -NoExit -Command "code '{{handoff_file}}'"
```

The path substitution preserves quotes — don't double-quote.

### Active Session Check

Clicking Launch on a feature that already has an open work session fails with a toast error. This prevents accidental duplicate sessions. Close the existing session first (wait for the Stop hook to run), then launch again.

### Prompt File Overwrite

Each launch overwrites the previous prompt file for that feature. If you want to preserve a prompt, rename it before launching again.
