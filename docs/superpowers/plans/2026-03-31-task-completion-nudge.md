# Task Completion Nudge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enrich the PostToolUse commit nudge with actual unchecked task items (IDs + titles) so the LLM can immediately act on them instead of ignoring a vague "if applicable" message.

**Architecture:** Single function addition (`formatUncheckedTasks`) in `hook.go` that queries subtasks for the active feature and formats unchecked items into the system message. Both the normal-commit and plan-import branches of `handlePostToolUse` call it. No new files, no schema changes.

**Tech Stack:** Go, SQLite (via existing store package)

---

### Task 1: Add `formatUncheckedTasks` helper and wire into normal-commit branch

**Files:**
- Modify: `cmd/docket/hook.go:426-435` (normal-commit system message)

- [ ] **Step 1: Write the failing test**

Add to `cmd/docket/hook_test.go`:

```go
func TestPostToolUseShowsUncheckedTasks(t *testing.T) {
	dir := t.TempDir()

	// Init git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}

	// Create a file and commit
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = dir
	addCmd.CombinedOutput()
	commitCmd := exec.Command("git", "commit", "-m", "feat: add main")
	commitCmd.Dir = dir
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}

	// Create feature with subtask + unchecked items
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := s.AddFeature("Auth System", "auth")
	s.UpdateFeature(f.ID, store.FeatureUpdate{Status: strPtr("in_progress")})
	st, _ := s.AddSubtask(f.ID, "Implementation", 0)
	item1, _ := s.AddTaskItem(st.ID, "Add validation to input handler", 0)
	item2, _ := s.AddTaskItem(st.ID, "Write unit tests for validator", 1)
	s.Close()

	h := &hookInput{
		SessionID:     "test-session",
		CWD:           dir,
		HookEventName: "PostToolUse",
		ToolName:      "Bash",
		ToolInput:     toolInput{Command: "git commit -m 'feat: add main'"},
	}

	var buf bytes.Buffer
	handlePostToolUse(h, &buf)

	var out hookOutput
	json.Unmarshal(buf.Bytes(), &out)

	// Should list unchecked task items with IDs
	if !strings.Contains(out.SystemMessage, fmt.Sprintf("#%d", item1.ID)) {
		t.Errorf("expected item1 ID in message, got: %s", out.SystemMessage)
	}
	if !strings.Contains(out.SystemMessage, "Add validation to input handler") {
		t.Errorf("expected item1 title in message, got: %s", out.SystemMessage)
	}
	if !strings.Contains(out.SystemMessage, fmt.Sprintf("#%d", item2.ID)) {
		t.Errorf("expected item2 ID in message, got: %s", out.SystemMessage)
	}
	if !strings.Contains(out.SystemMessage, "complete_task_item") {
		t.Errorf("expected complete_task_item instruction, got: %s", out.SystemMessage)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/docket/ -run TestPostToolUseShowsUncheckedTasks -v`
Expected: FAIL — system message still uses old vague format without task item IDs

- [ ] **Step 3: Add `formatUncheckedTasks` helper and update normal-commit branch**

In `cmd/docket/hook.go`, add the helper function before `handlePostToolUse`:

```go
// formatUncheckedTasks queries unchecked task items for a feature and formats
// them as lines for the system message. Returns empty string if no unchecked items.
func formatUncheckedTasks(s *store.Store, featureID string) string {
	subtasks, err := s.GetSubtasksForFeature(featureID, false)
	if err != nil {
		return ""
	}

	var unchecked []store.TaskItem
	for _, st := range subtasks {
		for _, item := range st.Items {
			if !item.Checked {
				unchecked = append(unchecked, item)
			}
		}
	}

	if len(unchecked) == 0 {
		return ""
	}

	var b strings.Builder
	cap := 10
	for i, item := range unchecked {
		if i >= cap {
			b.WriteString(fmt.Sprintf("\n  ... and %d more", len(unchecked)-cap))
			break
		}
		b.WriteString(fmt.Sprintf("\n  #%d: %s", item.ID, item.Title))
	}
	return b.String()
}
```

Then update the normal-commit branch in `handlePostToolUse` (the `else` block around line 430-433):

```go
	} else {
		// Normal commit — direct MCP calls only
		taskList := formatUncheckedTasks(s, features[0].ID)
		if taskList != "" {
			out.SystemMessage = fmt.Sprintf("[docket] Commit recorded: %s %s\nFeature %q — unchecked tasks:%s\nCall complete_task_item for any items this commit completes, then update_feature (left_off, key_files).",
				hash, msg, features[0].Title, taskList)
		} else {
			out.SystemMessage = fmt.Sprintf("[docket] Commit recorded: %s %s\nUpdate feature %q: update_feature (left_off, key_files).",
				hash, msg, features[0].ID)
		}
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/docket/ -run TestPostToolUseShowsUncheckedTasks -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/docket/hook.go cmd/docket/hook_test.go
git commit -m "feat: enrich PostToolUse commit nudge with unchecked task items"
```

---

### Task 2: Test that checked items are excluded from the nudge

**Files:**
- Modify: `cmd/docket/hook_test.go`

- [ ] **Step 1: Write the failing test**

Add to `cmd/docket/hook_test.go`:

```go
func TestPostToolUseOmitsCheckedTasks(t *testing.T) {
	dir := t.TempDir()

	// Init git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = dir
	addCmd.CombinedOutput()
	commitCmd := exec.Command("git", "commit", "-m", "feat: done")
	commitCmd.Dir = dir
	commitCmd.CombinedOutput()

	// Create feature — all tasks checked
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := s.AddFeature("Done Feature", "done")
	s.UpdateFeature(f.ID, store.FeatureUpdate{Status: strPtr("in_progress")})
	st, _ := s.AddSubtask(f.ID, "Implementation", 0)
	item, _ := s.AddTaskItem(st.ID, "Already done task", 0)
	s.CompleteTaskItem(item.ID, store.TaskItemCompletion{Outcome: "done"})
	s.Close()

	h := &hookInput{
		SessionID:     "test-session",
		CWD:           dir,
		HookEventName: "PostToolUse",
		ToolName:      "Bash",
		ToolInput:     toolInput{Command: "git commit -m 'feat: done'"},
	}

	var buf bytes.Buffer
	handlePostToolUse(h, &buf)

	var out hookOutput
	json.Unmarshal(buf.Bytes(), &out)

	// Should NOT contain task list — all checked
	if strings.Contains(out.SystemMessage, "unchecked tasks") {
		t.Errorf("expected no task list when all checked, got: %s", out.SystemMessage)
	}
	if strings.Contains(out.SystemMessage, "Already done task") {
		t.Errorf("checked task should not appear in message, got: %s", out.SystemMessage)
	}
	// Should still prompt for update_feature
	if !strings.Contains(out.SystemMessage, "update_feature") {
		t.Errorf("expected update_feature prompt, got: %s", out.SystemMessage)
	}
}
```

- [ ] **Step 2: Run test to verify it passes** (this should already pass from Task 1's implementation)

Run: `go test ./cmd/docket/ -run TestPostToolUseOmitsCheckedTasks -v`
Expected: PASS — the `formatUncheckedTasks` helper returns empty string when all items are checked, so the old-style message is used

- [ ] **Step 3: Commit**

```bash
git add cmd/docket/hook_test.go
git commit -m "test: verify checked tasks excluded from commit nudge"
```

---

### Task 3: Test the 10-item cap

**Files:**
- Modify: `cmd/docket/hook_test.go`

- [ ] **Step 1: Write the test**

Add to `cmd/docket/hook_test.go`:

```go
func TestFormatUncheckedTasksCapsAtTen(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	f, _ := s.AddFeature("Big Feature", "lots of tasks")
	st, _ := s.AddSubtask(f.ID, "Subtask", 0)
	for i := 0; i < 15; i++ {
		s.AddTaskItem(st.ID, fmt.Sprintf("Task item %d", i+1), i)
	}

	result := formatUncheckedTasks(s, f.ID)

	// Should contain exactly 10 numbered items
	count := strings.Count(result, "\n  #")
	if count != 10 {
		t.Errorf("expected 10 items listed, got %d in: %s", count, result)
	}

	// Should contain truncation message
	if !strings.Contains(result, "... and 5 more") {
		t.Errorf("expected '... and 5 more', got: %s", result)
	}

	// Item 1 should be present, item 15 should not
	if !strings.Contains(result, "Task item 1") {
		t.Errorf("expected first item present, got: %s", result)
	}
	if strings.Contains(result, "Task item 15") {
		t.Errorf("item 15 should be truncated, got: %s", result)
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./cmd/docket/ -run TestFormatUncheckedTasksCapsAtTen -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/docket/hook_test.go
git commit -m "test: verify unchecked task list caps at 10 items"
```

---

### Task 4: Wire unchecked tasks into the plan-import branch

**Files:**
- Modify: `cmd/docket/hook.go:426-429` (plan-import system message)
- Modify: `cmd/docket/hook_test.go`

- [ ] **Step 1: Write the failing test**

Add to `cmd/docket/hook_test.go`:

```go
func TestPostToolUsePlanImportShowsUncheckedTasks(t *testing.T) {
	dir := t.TempDir()

	// Init git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}

	// Create a plan file and commit
	planDir := filepath.Join(dir, "docs", "superpowers", "plans")
	os.MkdirAll(planDir, 0755)
	planContent := "# Plan\n\n### Task 1: Do stuff\n\n- [ ] **Step 1: Write code**\n"
	os.WriteFile(filepath.Join(planDir, "2026-03-31-test-plan.md"), []byte(planContent), 0644)

	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = dir
	addCmd.CombinedOutput()
	commitCmd := exec.Command("git", "commit", "-m", "docs: add plan")
	commitCmd.Dir = dir
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}

	// Create feature with existing unchecked items
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := s.AddFeature("Plan Feature", "plan test")
	s.UpdateFeature(f.ID, store.FeatureUpdate{Status: strPtr("in_progress")})
	st, _ := s.AddSubtask(f.ID, "Pre-existing", 0)
	s.AddTaskItem(st.ID, "Existing unchecked task", 0)
	s.Close()

	h := &hookInput{
		SessionID:     "test-session",
		CWD:           dir,
		HookEventName: "PostToolUse",
		ToolName:      "Bash",
		ToolInput:     toolInput{Command: "git commit -m 'docs: add plan'"},
	}

	var buf bytes.Buffer
	handlePostToolUse(h, &buf)

	var out hookOutput
	json.Unmarshal(buf.Bytes(), &out)

	// Should mention the import
	if !strings.Contains(out.SystemMessage, "imported") {
		t.Errorf("expected import message, got: %s", out.SystemMessage)
	}
	// Should also list unchecked tasks (includes both pre-existing and newly imported)
	if !strings.Contains(out.SystemMessage, "unchecked tasks") {
		t.Errorf("expected unchecked task list after plan import, got: %s", out.SystemMessage)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/docket/ -run TestPostToolUsePlanImportShowsUncheckedTasks -v`
Expected: FAIL — plan-import branch doesn't include unchecked task list

- [ ] **Step 3: Update the plan-import branch**

In `cmd/docket/hook.go`, update the plan-import `if` block (around line 426-429):

```go
	if importMsg != "" {
		// Plan file imported — show unchecked tasks (includes newly imported ones)
		taskList := formatUncheckedTasks(s, features[0].ID)
		if taskList != "" {
			out.SystemMessage = fmt.Sprintf("[docket] Commit recorded: %s %s%s\nFeature %q — unchecked tasks:%s\nDispatch board-manager agent (model: sonnet) to structure imported plan: feature_id=\"%s\", commit %s.",
				hash, msg, importMsg, features[0].Title, taskList, features[0].ID, hash)
		} else {
			out.SystemMessage = fmt.Sprintf("[docket] Commit recorded: %s %s%s\nDispatch board-manager agent (model: sonnet) to structure imported plan: feature_id=\"%s\", commit %s.",
				hash, msg, importMsg, features[0].ID, hash)
		}
	} else {
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/docket/ -run TestPostToolUsePlanImportShowsUncheckedTasks -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./... -v`
Expected: All tests pass, including existing `TestPostToolUseRecordsCommit` and `TestPostToolUseAutoImportsPlan`

- [ ] **Step 6: Commit**

```bash
git add cmd/docket/hook.go cmd/docket/hook_test.go
git commit -m "feat: include unchecked tasks in plan-import commit nudge"
```

---

### Task 5: Verify existing tests still pass and update docs

**Files:**
- Modify: `docs/task-completion-nudge.md` (new doc)

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: All tests pass

- [ ] **Step 2: Write feature doc**

Create `docs/task-completion-nudge.md`:

```markdown
# Task Completion Nudge

## What it does

After every git commit, the PostToolUse hook includes a list of unchecked task items (with IDs) in the system message. This gives the LLM everything it needs to call `complete_task_item` immediately instead of ignoring a vague "if applicable" prompt.

## Why it exists

The LLM was consistently ignoring the old commit nudge ("complete_task_item if applicable") because it didn't know which tasks existed or what their IDs were. Tasks would pile up unchecked until the completion gate blocked marking the feature as done.

## How it works

1. PostToolUse hook fires after a `git commit` command
2. Hook queries `GetSubtasksForFeature` for the active feature
3. Unchecked items are formatted as `#ID: Title` lines (capped at 10)
4. System message includes the list, prompting the LLM to call `complete_task_item`

If all tasks are already checked, the task list is omitted and only `update_feature` is prompted.

## Gotchas

- The list is capped at 10 items to avoid bloating the system message. If there are more, it shows "... and N more".
- The nudge is advisory — the LLM decides which tasks a commit satisfies. Not every commit maps to a task.
- Works for both normal commits and plan-import commits.

## Key files

- `cmd/docket/hook.go` — `formatUncheckedTasks` helper, `handlePostToolUse` function
- `cmd/docket/hook_test.go` — tests for the nudge behavior
```

- [ ] **Step 3: Commit**

```bash
git add docs/task-completion-nudge.md
git commit -m "docs: add task completion nudge feature doc"
```
