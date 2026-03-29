# Docket Auto-Tracking Hooks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add lifecycle hooks so docket automatically tracks session context and commits without any manual user action.

**Architecture:** New `docket hook` subcommand reads Claude Code hook JSON from stdin, branches on event type. Plugin declares hooks in `hooks/hooks.json`. SessionStart injects feature context, PostToolUse records git commits, Stop prompt hook triggers session logging and board-manager dispatch.

**Tech Stack:** Go (existing binary), Claude Code plugin hooks system (hooks.json)

---

## File Structure

| File | Responsibility |
|------|----------------|
| `cmd/docket/hook.go` | New. `runHook()` — reads stdin JSON, dispatches to SessionStart/PostToolUse handlers |
| `cmd/docket/hook_test.go` | New. Tests for hook handlers |
| `cmd/docket/main.go` | Modified. Add `hook` case to command switch |
| `plugin/hooks/hooks.json` | New. Hook declarations for SessionStart, PostToolUse, Stop |
| `install.sh` | Modified. Copy hooks dir, replace DOCKET_EXE_PATH placeholder |

---

### Task 1: Implement hook.go, tests, and wire into main.go

All the Go code in one shot — parsing, SessionStart handler, PostToolUse handler, tests, and the main.go wiring.

**Files:**
- Create: `cmd/docket/hook.go`
- Create: `cmd/docket/hook_test.go`
- Modify: `cmd/docket/main.go`

- [ ] **Step 1: Write hook.go**

Create `cmd/docket/hook.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sniffle6/claude-docket/internal/store"
)

type hookInput struct {
	SessionID     string    `json:"session_id"`
	CWD           string    `json:"cwd"`
	HookEventName string    `json:"hook_event_name"`
	ToolName      string    `json:"tool_name"`
	ToolInput     toolInput `json:"tool_input"`
}

type toolInput struct {
	Command string `json:"command"`
}

type hookOutput struct {
	Continue      bool   `json:"continue"`
	SystemMessage string `json:"systemMessage,omitempty"`
}

func runHook() {
	var h hookInput
	if err := json.NewDecoder(os.Stdin).Decode(&h); err != nil {
		fmt.Fprintf(os.Stderr, "docket hook: %v\n", err)
		os.Exit(1)
	}

	switch h.HookEventName {
	case "SessionStart":
		handleSessionStart(&h, os.Stdout)
	case "PostToolUse":
		handlePostToolUse(&h)
	}
}

func handleSessionStart(h *hookInput, w io.Writer) {
	s, err := store.Open(h.CWD)
	if err != nil {
		json.NewEncoder(w).Encode(hookOutput{Continue: true, SystemMessage: fmt.Sprintf("[docket] could not open store: %v", err)})
		return
	}
	defer s.Close()

	// Create/clear commits.log for this session
	logPath := filepath.Join(h.CWD, ".docket", "commits.log")
	os.WriteFile(logPath, []byte{}, 0644)

	features, err := s.ListFeatures("in_progress")
	if err != nil || len(features) == 0 {
		json.NewEncoder(w).Encode(hookOutput{Continue: true, SystemMessage: "[docket] No active features. Use docket MCP tools to create one."})
		return
	}

	var msg strings.Builder
	msg.WriteString("[docket] Active features:\n")
	for _, f := range features {
		msg.WriteString(fmt.Sprintf("- **%s** (%s)", f.Title, f.ID))
		if f.LeftOff != "" {
			msg.WriteString(fmt.Sprintf(" — %s", f.LeftOff))
		}
		msg.WriteString("\n")
	}

	// Get next unchecked task for the first feature
	subtasks, _ := s.GetSubtasksForFeature(features[0].ID, false)
	for _, st := range subtasks {
		for _, item := range st.Items {
			if !item.Checked {
				msg.WriteString(fmt.Sprintf("\nNext task: %s", item.Title))
				goto done
			}
		}
	}
done:
	json.NewEncoder(w).Encode(hookOutput{Continue: true, SystemMessage: msg.String()})
}

func handlePostToolUse(h *hookInput) {
	if !strings.Contains(h.ToolInput.Command, "git commit") {
		return
	}

	cmd := exec.Command("git", "log", "-1", "--format=%H|||%s")
	cmd.Dir = h.CWD
	out, err := cmd.Output()
	if err != nil {
		return
	}

	line := strings.TrimSpace(string(out))
	if line == "" {
		return
	}

	logPath := filepath.Join(h.CWD, ".docket", "commits.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}
```

- [ ] **Step 2: Write hook_test.go**

Create `cmd/docket/hook_test.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sniffle6/claude-docket/internal/store"
)

func TestSessionStartWithFeature(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	f, _ := s.AddFeature("Test Feature", "desc")
	status := "in_progress"
	leftOff := "working on hooks"
	s.UpdateFeature(f.ID, store.FeatureUpdate{Status: &status, LeftOff: &leftOff})
	s.Close()

	var buf bytes.Buffer
	handleSessionStart(&hookInput{CWD: dir}, &buf)

	var out hookOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if !out.Continue {
		t.Error("continue should be true")
	}
	if !strings.Contains(out.SystemMessage, "Test Feature") {
		t.Errorf("should contain feature title, got: %s", out.SystemMessage)
	}
	if !strings.Contains(out.SystemMessage, "working on hooks") {
		t.Errorf("should contain left_off, got: %s", out.SystemMessage)
	}
	// commits.log should exist
	if _, err := os.Stat(filepath.Join(dir, ".docket", "commits.log")); err != nil {
		t.Errorf("commits.log not created: %v", err)
	}
}

func TestSessionStartNoFeatures(t *testing.T) {
	dir := t.TempDir()
	s, _ := store.Open(dir)
	s.Close()

	var buf bytes.Buffer
	handleSessionStart(&hookInput{CWD: dir}, &buf)

	var out hookOutput
	json.Unmarshal(buf.Bytes(), &out)
	if !strings.Contains(out.SystemMessage, "No active features") {
		t.Errorf("should say no active features, got: %s", out.SystemMessage)
	}
}

func TestPostToolUseIgnoresNonCommit(t *testing.T) {
	h := &hookInput{CWD: t.TempDir(), ToolInput: toolInput{Command: "go test ./..."}}
	// Should not panic or write anything
	handlePostToolUse(h)
}

func TestPostToolUseRecordsCommit(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".docket"), 0755)

	// Set up a git repo with one commit
	for _, args := range [][]string{
		{"init", dir},
		{"-C", dir, "config", "user.email", "test@test.com"},
		{"-C", dir, "config", "user.name", "Test"},
	} {
		exec.Command("git", args...).Run()
	}
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "initial commit").Run()

	handlePostToolUse(&hookInput{
		CWD:       dir,
		ToolInput: toolInput{Command: `git commit -m "initial commit"`},
	})

	data, err := os.ReadFile(filepath.Join(dir, ".docket", "commits.log"))
	if err != nil {
		t.Fatalf("read commits.log: %v", err)
	}
	if !strings.Contains(string(data), "initial commit") {
		t.Errorf("commits.log should contain commit message, got: %s", data)
	}
}
```

- [ ] **Step 3: Wire into main.go**

In `cmd/docket/main.go`, add `"hook"` case to the switch and update the usage line:

```go
fmt.Fprintln(os.Stderr, "commands: serve, init, hook, version")
```

```go
case "hook":
    runHook()
```

- [ ] **Step 4: Run tests and build**

Run: `cd "H:/claude code/tools/docket" && go test ./... -v`
Expected: all pass

Run: `cd "H:/claude code/tools/docket" && go build ./cmd/docket/`
Expected: success

- [ ] **Step 5: Commit**

```bash
git add cmd/docket/hook.go cmd/docket/hook_test.go cmd/docket/main.go
git commit -m "feat: add docket hook subcommand — SessionStart context injection and PostToolUse commit tracking"
```

---

### Task 2: Create hooks.json and update install.sh

**Files:**
- Create: `plugin/hooks/hooks.json`
- Modify: `install.sh`

- [ ] **Step 1: Create hooks.json**

Create `plugin/hooks/hooks.json`:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "DOCKET_EXE_PATH hook",
            "timeout": 10
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "DOCKET_EXE_PATH hook",
            "timeout": 5
          }
        ]
      }
    ],
    "Stop": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "prompt",
            "prompt": "Before ending: 1) Read .docket/commits.log for commits made this session. 2) Call log_session with a summary of what was accomplished, files touched, and commits. 3) Dispatch the board-manager agent with the session summary, commits, files, and active feature ID to update the board. 4) Delete .docket/commits.log."
          }
        ]
      }
    ]
  }
}
```

- [ ] **Step 2: Update install.sh**

After the `.mcp.json` generation block (after the `echo "Generated..."` line around line 69), add:

```bash
# Copy hooks and replace binary path placeholder
if [ -d "$SOURCE_DIR/plugin/hooks" ]; then
    cp -r "$SOURCE_DIR/plugin/hooks" "$PLUGIN_INSTALL/"
    sed -i "s|DOCKET_EXE_PATH|$DOCKET_BIN_JSON|g" "$PLUGIN_INSTALL/hooks/hooks.json"
    echo "Installed hooks with binary path: $DOCKET_BIN_JSON"
fi
```

- [ ] **Step 3: Verify**

Run: `python3 -c "import json; json.load(open('plugin/hooks/hooks.json'))"` — valid JSON
Run: `bash -n install.sh` — valid syntax

- [ ] **Step 4: Commit**

```bash
git add plugin/hooks/hooks.json install.sh
git commit -m "feat: add hooks.json and install.sh hooks support"
```

---

### Task 3: Write doc and final verification

**Files:**
- Create: `docs/docket-hooks.md`

- [ ] **Step 1: Write feature doc**

Create `docs/docket-hooks.md`:

```markdown
# Docket Auto-Tracking Hooks

Docket automatically tracks your session activity using Claude Code lifecycle hooks. No manual steps needed.

## What it does

- **Session start**: Injects active feature context (title, status, left_off, next task) into the conversation
- **After git commits**: Records each commit hash and message to `.docket/commits.log`
- **Session end**: Claude summarizes the session, logs it via `log_session`, and dispatches `board-manager` to update the board

## How it works

The plugin declares hooks in `plugin/hooks/hooks.json`. Claude Code fires these automatically:

1. `SessionStart` → runs `docket.exe hook` → outputs feature context as systemMessage
2. `PostToolUse` (Bash only) → runs `docket.exe hook` → checks for `git commit`, appends to commits.log
3. `Stop` → prompt hook → Claude summarizes, calls log_session, dispatches board-manager

## Key files

- `cmd/docket/hook.go` — hook subcommand logic
- `plugin/hooks/hooks.json` — hook declarations
- `install.sh` — installs hooks and replaces binary path placeholder

## Gotchas

- Hooks load at session start. After updating docket, restart Claude Code.
- If a session crashes, stale `commits.log` may exist. SessionStart clears it.
- The Stop prompt hook costs tokens (LLM summarization). This is intentional — summaries are the most useful part of session logs.
```

- [ ] **Step 2: Run full test suite and build**

Run: `cd "H:/claude code/tools/docket" && go test ./... -v`
Expected: all pass

Run: `cd "H:/claude code/tools/docket" && go build -ldflags="-s -w" -o docket.exe ./cmd/docket/`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add docs/docket-hooks.md
git commit -m "docs: add docket auto-tracking hooks documentation"
```
