# Docket Auto-Tracking Hooks

## Problem

Docket relies on CLAUDE.md instructions telling Claude to manually dispatch the board-manager agent after commits, at session start, and at session end. This doesn't work — the audit log shows board-manager never gets dispatched during subagent workflows, and users forget too. Docket needs to track activity automatically without any manual steps.

## Solution

Add lifecycle hooks to the docket plugin via `hooks/hooks.json` and a new `docket hook` subcommand. Three hooks fire automatically:

1. **SessionStart** — injects active feature context into the conversation
2. **PostToolUse (Bash)** — detects git commits and records them
3. **Stop** — LLM summarizes session, logs it, dispatches board-manager

## Design

### `docket hook` subcommand

Single command: `docket.exe hook`. Reads Claude Code hook JSON from stdin, branches on `hook_event_name`.

**Stdin format** (from Claude Code):
```json
{
  "session_id": "abc123",
  "cwd": "/path/to/project",
  "hook_event_name": "SessionStart",
  "tool_name": "Bash",
  "tool_input": {"command": "git commit -m 'feat: thing'"},
  "tool_result": "..."
}
```

**SessionStart handler:**
- Opens `.docket/` DB from `cwd`
- Queries in_progress features
- Creates empty `.docket/commits.log`
- Outputs JSON to stdout with `systemMessage` containing: active feature title, status, left_off, next unchecked task item

**PostToolUse handler:**
- Checks `tool_input.command` for `git commit` — if not a commit, exit 0 silently
- Runs `git log -1 --format=%H|||%s` in `cwd` to get commit hash and message
- Appends line to `.docket/commits.log`
- Exit 0, no stdout

**Stop:** Not handled by the binary. This is a prompt hook (see below).

### hooks.json

```json
{
  "hooks": {
    "SessionStart": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "DOCKET_EXE_PATH hook",
        "timeout": 10
      }]
    }],
    "PostToolUse": [{
      "matcher": "Bash",
      "hooks": [{
        "type": "command",
        "command": "DOCKET_EXE_PATH hook",
        "timeout": 5
      }]
    }],
    "Stop": [{
      "matcher": "*",
      "hooks": [{
        "type": "prompt",
        "prompt": "Before ending: 1) Read .docket/commits.log for commits made this session. 2) Call log_session with a summary of what was accomplished, files touched, and commits. 3) Dispatch the board-manager agent with the session summary, commits, files, and active feature ID to update the board. 4) Delete .docket/commits.log."
      }]
    }]
  }
}
```

`DOCKET_EXE_PATH` is replaced by `install.sh` with the actual installed binary path.

### Commit detection

The PostToolUse hook checks `tool_input.command` for the substring `git commit`. This catches:
- `git commit -m "message"`
- `git commit -am "message"`
- `git add . && git commit -m "message"` (chained commands)

It does NOT catch commits made by subagents in worktrees (those have separate sessions). That's fine — subagent sessions get their own lifecycle.

After detecting a commit, the binary runs `git log -1 --format=%H|||%s` rather than parsing tool output. This is more reliable since git stdout format varies.

### Session lifecycle

```
Session starts
  -> SessionStart hook fires
  -> docket.exe hook reads stdin, outputs feature context as systemMessage
  -> Claude sees active feature, left_off, next task

User works, makes commits
  -> PostToolUse hook fires on each Bash call
  -> docket.exe hook checks for git commit, records to commits.log
  -> Non-commit bash calls ignored (exit 0)

Session ends
  -> Stop prompt hook fires
  -> Claude reads commits.log, summarizes session
  -> Claude calls log_session MCP tool
  -> Claude dispatches board-manager agent
  -> Board-manager updates feature status, matches commits to tasks, updates left_off
  -> commits.log deleted
```

## Files changed

| File | Change |
|------|--------|
| `cmd/docket/main.go` | Add `hook` case to command switch |
| `cmd/docket/hook.go` | New file — `runHook()` with SessionStart and PostToolUse handlers |
| `plugin/hooks/hooks.json` | New file — hook declarations |
| `install.sh` | Copy `hooks/` dir, replace `DOCKET_EXE_PATH` in hooks.json |

## What doesn't change

- No new DB tables or schema migrations
- No new HTTP endpoints
- No new MCP tools
- `get_ready` MCP tool stays (useful for mid-session manual checks)
- Board-manager agent unchanged — Stop hook dispatches it the same way
- Dashboard unchanged

## Gotchas

- **Hooks load at session start.** After install, user must restart Claude Code for hooks to take effect.
- **commits.log is a temp file.** If a session crashes without the Stop hook firing, stale commits.log may exist. SessionStart clears it.
- **Multiple in_progress features.** SessionStart outputs all of them; Claude picks the relevant one. Could add a `.docket/active-feature` file later if this gets noisy.

## Key files

- `cmd/docket/hook.go` — hook subcommand logic
- `plugin/hooks/hooks.json` — hook declarations
- `install.sh` — installs hooks into plugin directory
