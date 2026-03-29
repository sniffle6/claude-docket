# MCP Server Stability

## What this fixes

The docket MCP server process would randomly crash, killing both the MCP connection and the dashboard (they run in the same process).

## Root causes

### 1. Missing SQLite busy_timeout

Three things hit the same SQLite database concurrently:
- The MCP server (stdio) — tool calls from Claude
- The HTTP dashboard — API requests from the browser
- The hook process (`docket hook`) — opens a **separate** connection on every `git commit`

Without `PRAGMA busy_timeout`, any concurrent write attempt gets an immediate `SQLITE_BUSY` error instead of waiting. WAL mode helps with concurrent reads but writes still need serialization.

**Fix:** Added `PRAGMA busy_timeout=5000` in `store.Open()`. SQLite now waits up to 5 seconds for a lock before failing.

### 2. Unsafe type assertions in tool handlers

Every tool handler did bare type assertions like `args["id"].(string)`. If the MCP client sends a nil or non-string value (e.g. a JSON number for an ID field), this panics. Since `mcp-go` doesn't recover panics in tool handlers, the panic kills the entire process.

**Fix:** Added `argString()` helper that safely extracts string arguments with fallback for numeric types. All handlers now use it and return proper MCP errors instead of panicking.

## Key files

- `internal/store/store.go` — `busy_timeout` pragma
- `internal/mcp/tools.go` — `argString()` helper + all handler updates
