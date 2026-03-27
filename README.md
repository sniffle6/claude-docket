# feat

Local feature tracker for AI coding agents. MCP server + web dashboard.

## Install

```bash
go install github.com/sniffyanimal/feat@latest
```

## Quick Start

```bash
cd your-project
feat init          # creates .feat/ directory
feat serve         # starts MCP server + dashboard on localhost:7890
```

## Claude Code Setup

Add to `.claude/settings.json`:

```json
{
  "mcpServers": {
    "feat": {
      "command": "feat",
      "args": ["serve"],
      "type": "stdio"
    }
  }
}
```

Add to your project's `CLAUDE.md`:

```markdown
## Feature Tracking (feat)

This project uses feat for feature tracking. When the user describes work:

1. Call list_features to check for a matching feature.
2. If one matches, call get_context to load it, then switch to its worktree.
3. If none match, ask if this is a new feature. If yes, call add_feature,
   create a worktree, and call update_feature with the worktree path.
```

Optional — add a Stop hook for auto session logging in `.claude/settings.json`:

```json
{
  "hooks": {
    "Stop": [{
      "type": "prompt",
      "prompt": "Before ending, call the feat.log_session tool with a brief summary of what was accomplished this session, files touched, commits made, and the feature ID if you were working on one."
    }]
  }
}
```

## MCP Tools

| Tool | Purpose |
|---|---|
| `add_feature` | Create a feature. Returns slug ID. |
| `update_feature` | Update status, description, left_off, worktree_path, key_files. |
| `list_features` | Compact feature list, filterable by status. |
| `get_feature` | Full feature detail with all sessions. |
| `log_session` | Record what happened in a session. |
| `get_context` | Token-efficient briefing (~15-20 lines). |

## Dashboard

Open `http://localhost:7890` while `feat serve` is running.

- Kanban board: Planned / In Progress / Blocked / Done
- Click a card for full detail, session history, key files
- Edit "left off" notes inline
- Reassign unlinked sessions to features
