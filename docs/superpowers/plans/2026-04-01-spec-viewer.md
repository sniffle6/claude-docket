# Spec Viewer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Link design specs to features via a `spec_path` field, surface the path in MCP tool responses, and render specs in a dashboard modal.

**Architecture:** New `spec_path` TEXT column on `features` table. MCP tools (`add_feature`, `update_feature`, `get_feature`, `get_context`) accept and return it. Dashboard shows a teal "Spec" badge on cards; clicking it fetches the markdown via a new `/api/spec` endpoint and renders it in a modal using `marked` from CDN.

**Tech Stack:** Go (store, MCP handlers, HTTP endpoint), SQLite, HTML/CSS/JS (dashboard), marked.js via CDN.

**Docket feature:** `spec-viewer`

---

### Task 1: Add spec_path column to database

**Files:**
- Modify: `internal/store/migrate.go`
- Modify: `internal/store/store.go`

- [ ] **Step 1: Add migration V14**

In `internal/store/migrate.go`, add after `schemaV13`:

```go
const schemaV14 = `
ALTER TABLE features ADD COLUMN spec_path TEXT NOT NULL DEFAULT '';
`
```

And in `migrate()`, add after the `db.Exec(schemaV13)` line:

```go
// v14: add spec_path column to features
db.Exec(schemaV14)
```

- [ ] **Step 2: Add SpecPath to Feature struct and FeatureUpdate**

In `internal/store/store.go`, add `SpecPath` field to `Feature`:

```go
type Feature struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	Type         string    `json:"type"`
	LeftOff      string    `json:"left_off"`
	Notes        string    `json:"notes"`
	KeyFiles     []string  `json:"key_files"`
	Tags         []string  `json:"tags"`
	WorktreePath string    `json:"worktree_path"`
	SpecPath     string    `json:"spec_path,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
```

Add `SpecPath` to `FeatureUpdate`:

```go
type FeatureUpdate struct {
	// ... existing fields ...
	SpecPath     *string   `json:"spec_path,omitempty"`
	Force        *bool     `json:"force,omitempty"`
	ForceReason  *string   `json:"force_reason,omitempty"`
}
```

- [ ] **Step 3: Update all SQL queries and Scan calls that touch features**

Every `SELECT` on the features table needs `spec_path` added. Every `Scan` call needs `&f.SpecPath`. There are several locations:

**`GetFeature`** — change the SELECT and Scan:

```go
func (s *Store) GetFeature(id string) (*Feature, error) {
	row := s.db.QueryRow(
		`SELECT id, title, description, status, type, left_off, notes, key_files, tags, worktree_path, spec_path, created_at, updated_at FROM features WHERE id = ?`,
		id,
	)
	var f Feature
	var keyFilesJSON, tagsJSON string
	err := row.Scan(&f.ID, &f.Title, &f.Description, &f.Status, &f.Type, &f.LeftOff, &f.Notes, &keyFilesJSON, &tagsJSON, &f.WorktreePath, &f.SpecPath, &f.CreatedAt, &f.UpdatedAt)
```

**`ListFeatures`** — same pattern, update SELECT and Scan:

```go
query := `SELECT id, title, description, status, type, left_off, notes, key_files, tags, worktree_path, spec_path, created_at, updated_at FROM features`
```

And in the scan loop:

```go
if err := rows.Scan(&f.ID, &f.Title, &f.Description, &f.Status, &f.Type, &f.LeftOff, &f.Notes, &keyFilesJSON, &tagsJSON, &f.WorktreePath, &f.SpecPath, &f.CreatedAt, &f.UpdatedAt); err != nil {
```

**`ListFeaturesWithTag`** — same SELECT and Scan pattern as `ListFeatures`.

**`GetReadyFeatures`** — find this function and apply the same SELECT/Scan change.

**`AutoArchiveStale`** — this only selects `id`, so no change needed.

Search the entire `store.go` file for every `SELECT.*FROM features` query and every `rows.Scan` / `row.Scan` that reads feature columns. Update all of them.

- [ ] **Step 4: Update UpdateFeature to handle SpecPath**

In `UpdateFeature`, add the SpecPath setter alongside the existing field setters:

```go
if u.SpecPath != nil {
	sets = append(sets, "spec_path = ?")
	args = append(args, *u.SpecPath)
}
```

Add this after the `WorktreePath` setter block (before the `if len(sets) == 0` check).

- [ ] **Step 5: Run tests**

Run: `go test ./internal/store/...`
Expected: All existing tests pass (migration is additive, new column has a default).

- [ ] **Step 6: Write test for spec_path round-trip**

Add to `internal/store/store_test.go` (or create it if there isn't one — check first):

```go
func TestSpecPath(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	f, err := s.AddFeature("Spec Test", "testing spec_path")
	if err != nil {
		t.Fatalf("AddFeature: %v", err)
	}
	if f.SpecPath != "" {
		t.Fatalf("expected empty spec_path, got %q", f.SpecPath)
	}

	path := "docs/superpowers/specs/2026-04-01-test-design.md"
	err = s.UpdateFeature(f.ID, FeatureUpdate{SpecPath: &path})
	if err != nil {
		t.Fatalf("UpdateFeature: %v", err)
	}

	got, err := s.GetFeature(f.ID)
	if err != nil {
		t.Fatalf("GetFeature: %v", err)
	}
	if got.SpecPath != path {
		t.Fatalf("expected spec_path %q, got %q", path, got.SpecPath)
	}
}
```

- [ ] **Step 7: Run tests**

Run: `go test ./internal/store/...`
Expected: All tests pass including the new `TestSpecPath`.

- [ ] **Step 8: Commit**

```bash
git add internal/store/migrate.go internal/store/store.go internal/store/store_test.go
git commit -m "feat: add spec_path column to features table"
```

---

### Task 2: Wire spec_path through MCP tools

**Files:**
- Modify: `internal/mcp/tools.go`
- Modify: `internal/mcp/tools_feature.go`

- [ ] **Step 1: Add spec_path parameter to add_feature and update_feature tool definitions**

In `internal/mcp/tools.go`, add to the `add_feature` tool definition (after the `tags` parameter):

```go
mcp.WithString("spec_path", mcp.Description("Relative path to the design spec file (e.g., 'docs/superpowers/specs/2026-04-01-foo-design.md')")),
```

Add to the `update_feature` tool definition (after the `tags` parameter):

```go
mcp.WithString("spec_path", mcp.Description("Relative path to the design spec file (e.g., 'docs/superpowers/specs/2026-04-01-foo-design.md')")),
```

- [ ] **Step 2: Handle spec_path in addFeatureHandler**

In `internal/mcp/tools_feature.go`, in `addFeatureHandler`, after the tag handling block and before `f, _ = s.GetFeature(f.ID)`:

```go
if specPath, ok := argString(args, "spec_path"); ok && specPath != "" {
	s.UpdateFeature(f.ID, store.FeatureUpdate{SpecPath: &specPath})
}
```

- [ ] **Step 3: Handle spec_path in updateFeatureHandler**

In `updateFeatureHandler`, add alongside the other field extractors (after the `worktree_path` block):

```go
if v, ok := argString(args, "spec_path"); ok {
	u.SpecPath = &v
}
```

- [ ] **Step 4: Surface spec_path in getContextHandler**

In `getContextHandler`, after the `User notes` line:

```go
if f.SpecPath != "" {
	fmt.Fprintf(&b, "Spec: %s\n", f.SpecPath)
}
```

Note: `getFeatureHandler` already returns the full `Feature` struct as JSON, which now includes `spec_path` from Task 1. No change needed there.

- [ ] **Step 5: Build and verify**

Run: `go build ./...`
Expected: Clean build, no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/tools.go internal/mcp/tools_feature.go
git commit -m "feat: wire spec_path through MCP tools"
```

---

### Task 3: Add /api/spec HTTP endpoint

**Files:**
- Modify: `internal/dashboard/dashboard.go`

- [ ] **Step 1: Add the spec endpoint**

In `internal/dashboard/dashboard.go`, in `NewHandler`, add before the `// Serve dashboard files` comment:

```go
mux.HandleFunc("GET /api/spec", func(w http.ResponseWriter, r *http.Request) {
	specPath := r.URL.Query().Get("path")
	if specPath == "" {
		http.Error(w, "missing path parameter", 400)
		return
	}
	// Safety: reject directory traversal and absolute paths
	if strings.Contains(specPath, "..") || filepath.IsAbs(specPath) {
		http.Error(w, "invalid path", 400)
		return
	}

	baseDir := devDir
	if baseDir == "" {
		baseDir, _ = os.Getwd()
	}
	fullPath := filepath.Join(baseDir, filepath.Clean(specPath))

	data, err := os.ReadFile(fullPath)
	if err != nil {
		http.Error(w, "spec not found", 404)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
})
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: Clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/dashboard/dashboard.go
git commit -m "feat: add /api/spec endpoint for spec file serving"
```

---

### Task 4: Dashboard — spec badge on feature cards

**Files:**
- Modify: `dashboard/index.html`

- [ ] **Step 1: Add spec-badge CSS**

In `dashboard/index.html`, add after the `.issue-badge` and `html.light .issue-badge` CSS rules (around line 209):

```css
.spec-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: #4ECDC420;
  color: var(--teal);
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
  cursor: pointer;
  border: none;
  transition: background 0.15s;
}
.spec-badge:hover { background: #4ECDC440; }
html.light .spec-badge { background: #E0F5F3; color: #1A7A72; }
html.light .spec-badge:hover { background: #C0EBE8; }
```

- [ ] **Step 2: Render spec badge on cards**

In the card rendering JS (the `renderBoard` function), find the issue badge block that starts with `if (f.issue_count > 0)`. Right before that block, add:

```javascript
// Spec badge
if (f.spec_path) {
  var specBadge = document.createElement('button');
  specBadge.className = 'spec-badge';
  specBadge.textContent = '\uD83D\uDCC4 Spec';
  specBadge.title = f.spec_path;
  (function(path, title) {
    specBadge.onclick = function(e) { e.stopPropagation(); showSpec(path, title); };
  })(f.spec_path, f.title);
  card.appendChild(specBadge);
}
```

- [ ] **Step 3: Build (dev-build.sh) and visually verify**

Run: `bash dev-build.sh`

Open dashboard in browser. Features with `spec_path` set should show a teal "Spec" badge. Features without should look unchanged.

- [ ] **Step 4: Commit**

```bash
git add dashboard/index.html
git commit -m "feat: add spec badge to dashboard feature cards"
```

---

### Task 5: Dashboard — spec viewer modal

**Files:**
- Modify: `dashboard/index.html`

- [ ] **Step 1: Add modal CSS**

In `dashboard/index.html`, add after the spec-badge CSS from Task 4:

```css
/* Spec modal */
.spec-overlay {
  display: none;
  position: fixed;
  inset: 0;
  background: var(--overlay-bg);
  z-index: 100;
  justify-content: center;
  align-items: flex-start;
  padding-top: 5vh;
}
.spec-overlay.open { display: flex; }
.spec-modal {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  width: 90%;
  max-width: 720px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
}
.spec-modal-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  gap: 12px;
}
.spec-modal-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
  flex: 1;
}
.spec-modal-path {
  font-size: 11px;
  color: var(--muted);
  font-family: monospace;
  background: var(--bg);
  padding: 2px 8px;
  border-radius: 4px;
}
.spec-modal-close {
  background: none;
  border: 1px solid var(--border);
  color: var(--muted);
  width: 28px;
  height: 28px;
  border-radius: 6px;
  font-size: 16px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
}
.spec-modal-close:hover { border-color: var(--primary); color: var(--primary); }
.spec-modal-body {
  padding: 20px;
  overflow-y: auto;
  font-size: 14px;
  color: var(--text);
  line-height: 1.6;
}
/* Markdown rendering styles */
.spec-modal-body h1, .spec-modal-body h2, .spec-modal-body h3 { color: var(--primary); margin: 16px 0 8px; }
.spec-modal-body h1 { font-size: 20px; }
.spec-modal-body h2 { font-size: 18px; }
.spec-modal-body h3 { font-size: 16px; }
.spec-modal-body p { margin-bottom: 12px; color: var(--secondary); }
.spec-modal-body ul, .spec-modal-body ol { margin-left: 20px; margin-bottom: 12px; color: var(--secondary); }
.spec-modal-body li { margin-bottom: 4px; }
.spec-modal-body code { font-size: 13px; background: var(--bg); padding: 1px 6px; border-radius: 3px; }
.spec-modal-body pre { background: var(--bg); padding: 12px; border-radius: 6px; overflow-x: auto; margin-bottom: 12px; }
.spec-modal-body pre code { padding: 0; background: none; }
.spec-modal-body a { color: var(--teal); }
.spec-modal-body blockquote { border-left: 3px solid var(--border); padding-left: 12px; color: var(--muted); margin-bottom: 12px; }
```

- [ ] **Step 2: Add modal HTML and marked.js script**

At the very end of `<body>`, just before `</body>`, add the modal container and the CDN script:

```html
<!-- Spec viewer modal -->
<div class="spec-overlay" id="specOverlay" onclick="closeSpec(event)">
  <div class="spec-modal" onclick="event.stopPropagation()">
    <div class="spec-modal-header">
      <span class="spec-modal-title" id="specTitle"></span>
      <span class="spec-modal-path" id="specPath"></span>
      <button class="spec-modal-close" onclick="closeSpec()">&times;</button>
    </div>
    <div class="spec-modal-body" id="specBody"></div>
  </div>
</div>
<script src="https://cdn.jsdelivr.net/npm/marked/marked.min.js"></script>
```

- [ ] **Step 3: Add showSpec and closeSpec functions**

In the `<script>` block, add before the closing `</script>`:

```javascript
async function showSpec(path, title) {
  var overlay = document.getElementById('specOverlay');
  var specTitle = document.getElementById('specTitle');
  var specPath = document.getElementById('specPath');
  var specBody = document.getElementById('specBody');

  specTitle.textContent = title;
  specPath.textContent = path;
  specBody.innerHTML = '<p style="color:var(--muted)">Loading...</p>';
  overlay.classList.add('open');

  try {
    var resp = await fetch('/api/spec?path=' + encodeURIComponent(path));
    if (!resp.ok) throw new Error('Failed to load spec');
    var md = await resp.text();
    specBody.innerHTML = marked.parse(md);
  } catch (e) {
    specBody.innerHTML = '<p style="color:var(--blocked)">Could not load spec: ' + e.message + '</p>';
  }
}

function closeSpec(e) {
  if (e && e.target !== document.getElementById('specOverlay')) return;
  document.getElementById('specOverlay').classList.remove('open');
}

document.addEventListener('keydown', function(e) {
  if (e.key === 'Escape') closeSpec();
});
```

- [ ] **Step 4: Build and manually test**

Run: `bash dev-build.sh`

Test the full flow:
1. Set `spec_path` on a feature (via MCP or direct DB edit)
2. Open dashboard — verify teal badge appears
3. Click badge — verify modal opens with rendered markdown
4. Close via X, Escape, and backdrop click
5. Verify features without `spec_path` show no badge

- [ ] **Step 5: Commit**

```bash
git add dashboard/index.html
git commit -m "feat: add spec viewer modal to dashboard"
```

---

### Task 6: Final integration test and cleanup

**Files:**
- Modify: `docs/superpowers/specs/2026-04-01-spec-viewer-design.md` (update key files if needed)

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 2: Build clean binary**

Run: `go build -ldflags="-s -w" -o docket.exe ./cmd/docket/`
Expected: Clean build.

- [ ] **Step 3: End-to-end test**

1. Start docket, open dashboard
2. Use MCP `update_feature` to set `spec_path` on the `spec-viewer` feature itself:
   - `spec_path: "docs/superpowers/specs/2026-04-01-spec-viewer-design.md"`
3. Verify badge shows on dashboard card
4. Click badge, verify spec renders in modal
5. Use `get_context` on the feature, verify `Spec:` line appears
6. Use `get_feature`, verify `spec_path` in JSON response

- [ ] **Step 4: Commit any final fixes**

If anything needed fixing in steps 1-3, commit those fixes.

- [ ] **Step 5: Set spec_path on the spec-viewer feature card**

Use `update_feature` MCP call to link the spec to its own feature:

```
update_feature(id: "spec-viewer", spec_path: "docs/superpowers/specs/2026-04-01-spec-viewer-design.md")
```
