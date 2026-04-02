---
name: docket
description: Open the docket dashboard in the default browser. Use when the user says /docket, mentions the docket dashboard, or wants to check feature tracking status.
---

# docket: Open Dashboard

Open the docket feature-tracking dashboard in the default browser.

## Steps

1. **Check MCP server health** — try calling any lightweight docket MCP tool (e.g., `mcp__plugin_docket_docket__list_features`).
   - If it succeeds, the server is up. Continue to step 2.
   - If it fails or the tool is unavailable, tell the user: "The docket MCP server isn't running. Run `/reload-plugins` to start it, then try `/docket` again." **Stop here.**

2. **Read the port file** to get the dashboard port:
   ```bash
   cat .docket/port
   ```
   If the file doesn't exist, fall back to port `7890`.

3. **Verify the dashboard is reachable** — run:
   ```bash
   curl -s -o /dev/null -w "%{http_code}" http://localhost:<port>/api/project
   ```
   - If it returns `200`, continue to step 4.
   - If it fails or returns non-200, tell the user: "The docket MCP server is running but the dashboard isn't responding on port `<port>`. Try `/reload-plugins` to restart it." **Stop here.**

4. **Open the dashboard** using the discovered port. Detect the platform and use the appropriate command:
   - **Windows**: `start http://localhost:<port>`
   - **macOS**: `open http://localhost:<port>`
   - **Linux**: `xdg-open http://localhost:<port>`

5. **Confirm** to the user that the dashboard is opening, and mention the URL.

## Notes

- The docket MCP server is managed by Claude Code's plugin system (stdio-based). It cannot be started manually — `/reload-plugins` restarts it.
- Each project gets its own port (derived from the project path), so multiple projects can run simultaneously.
- For quick status without leaving the terminal, call `mcp__plugin_docket_docket__list_features` or `mcp__plugin_docket_docket__get_ready` directly.
