# docket plugin

Claude Code plugin for the docket feature tracker.

## What it provides

- **board-manager agent** — autonomous agent that handles all docket board operations (create features, update status, log sessions, auto-compact)
- **/docket skill** — opens the docket dashboard in the default browser
- **MCP server** — connects Claude Code to the docket binary for feature tracking tools

## Setup

Run `install.sh` from the docket repo root. It builds the binary and installs this plugin.

## Per-project setup

Add this to your project's CLAUDE.md:

    ## Feature Tracking (docket)

    This project uses `docket` for feature tracking. Run `/docket` to open the dashboard.

    Dispatch the `board-manager` agent (model: sonnet) at these points:
    1. **Start of implementation work** — skip for questions/reviews/lookups
    2. **After a commit** — pass commit hash, message, files, feature ID
    3. **Session ending** — pass summary, commits, files, feature ID

    Carry the feature ID the agent returns across dispatches. `get_ready` stays in main session.
