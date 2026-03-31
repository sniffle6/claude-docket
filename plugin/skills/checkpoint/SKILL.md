---
name: checkpoint
description: Force a checkpoint of the current session's context. Preserves what was discussed, decisions made, and work done since the last checkpoint. Use when you want to save progress without ending the session.
---

# checkpoint: Save Session Context

Force a semantic checkpoint of the current Docket work session.

## Steps

1. **Call the checkpoint MCP tool:**
   Call `mcp__plugin_docket_docket__checkpoint` with no parameters.

2. **Report the result** to the user — how many chars of semantic text and files were captured.

## Notes

- Checkpoints are processed in the background by the Docket summarizer worker.
- If no API key is configured, only mechanical facts (files, tests, commits) are captured.
- The checkpoint is bound to the currently active work session and feature.
- If no work session is active, the tool will return an error.
