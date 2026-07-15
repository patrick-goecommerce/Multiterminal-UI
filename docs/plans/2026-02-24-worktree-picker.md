# Worktree Picker Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a per-pane worktree picker button in the titlebar that lets users choose or create git worktrees at session start, replacing the broken auto-worktree behavior.

**Architecture:** A clickable `⎇ branch` button in `PaneTitlebar` opens a dropdown (`WorktreeDropdown`) showing all existing worktrees (categorized: Terminal / Issue). Selecting one opens a **new pane** in that worktree's dir. `+ Neuer Worktree` opens `WorktreeCreateDialog` which calls `App.CreateNamedWorktree`. The automatic `use_worktrees` setting is removed from Settings UI. At pane launch, the branch is read once and stored in `Pane.branch`.

**Tech Stack:** Go 1.21 (backend), Svelte 4 + TypeScript (frontend), Wails v2 bindings (auto-generated in `wailsjs/go/backend/`), git CLI.

---

### Task 1: Backend – Extend WorktreeInfo + ListAllWorktrees

**Files:**
- Modify: `internal/backend/app_worktree.go`
- Test: `internal/backend/app_worktree_test.go` (new file)

**Step 1: Write failing test**

```go
// internal/backend/app_worktree_test.go
package backend

import (
    "strings"
    "testing"
)

func TestParseAllWorktrees(t *testing.T) {
    root := "/repo"
    output := `worktree /repo
HEAD abc1234
branch refs/heads/main

worktree /repo/.mt-worktrees/issue-42
HEAD def5678
branch refs/heads/fix/bug-42

worktree /repo/.mt-worktrees/login
HEAD aaa9999
branch refs/heads/terminal/login

`
    result := parseAllWorktreeList(output, root)
    if len(result) != 3 {
        t.Fatalf("expected 3 worktrees, got %d", len(result))
    }
    if result[0].Category != "main" {
        t.Errorf("expected main, got %s", result[0].Category)
    }
    if result[1].Category != "issue" || result[1].Issue != 42 {
        t.Errorf("expected issue 42, got %+v", result[1])
    }
    if result[2].Category != "terminal" || result[2].Name != "login" {
        t.Errorf("expected terminal/login, got %+v", result[2])
    }
    if !strings.HasSuffix(result[2].Branch, "terminal/login") {
        t.Errorf("unexpected branch: %s", result[2].Branch)
    }
}
```

**Step 2: Run test to verify it fails**

```
go test ./internal/backend/... -run TestParseAllWorktrees -v
```
Expected: FAIL – `parseAllWorktreeList undefined`

**Step 3: Implement**

In `internal/backend/app_worktree.go`, extend `WorktreeInfo` and add the new functions:

```go
// WorktreeInfo describes an active git worktree.
type WorktreeInfo struct {
    Path     string `json:"path" yaml:"path"`
    Branch   string `json:"branch" yaml:"branch"`
    Issue    int    `json:"issue" yaml:"issue"`
    Category string `json:"category" yaml:"category"` // "main", "terminal", "issue"
    Name     string `json:"name" yaml:"name"`
}

// ListAllWorktrees returns ALL git worktrees for the repo containing dir,
// categorized as "main", "terminal", or "issue".
func (a *App) ListAllWorktrees(dir string) []WorktreeInfo {
    root, err := repoRoot(dir)
    if err != nil {
        return nil
    }
    cmd := exec.Command("git", "worktree", "list", "--porcelain")
    cmd.Dir = root
    hideConsole(cmd)
    out, err := cmd.Output()
    if err != nil {
        log.Printf("[ListAllWorktrees] error: %v", err)
        return nil
    }
    return parseAllWorktreeList(string(out), root)
}

// parseAllWorktreeList parses all worktrees without filtering.
func parseAllWorktreeList(output string, root string) []WorktreeInfo {
    var result []WorktreeInfo
    var current WorktreeInfo
    mtPrefix := filepath.Join(root, worktreeDir) + string(filepath.Separator)

    for _, line := range strings.Split(output, "\n") {
        line = strings.TrimSpace(line)
        if line == "" {
            if current.Path != "" {
                categorize(&current, root, mtPrefix)
                result = append(result, current)
            }
            current = WorktreeInfo{}
            continue
        }
        if strings.HasPrefix(line, "worktree ") {
            current.Path = filepath.FromSlash(strings.TrimPrefix(line, "worktree "))
        }
        if strings.HasPrefix(line, "branch refs/heads/") {
            current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
        }
    }
    if current.Path != "" {
        categorize(&current, root, mtPrefix)
        result = append(result, current)
    }
    return result
}

// categorize fills Category, Name, Issue based on path.
func categorize(wt *WorktreeInfo, root, mtPrefix string) {
    if wt.Path == root {
        wt.Category = "main"
        wt.Name = "main"
        return
    }
    if strings.HasPrefix(wt.Path, mtPrefix) {
        base := filepath.Base(wt.Path)
        if strings.HasPrefix(base, "issue-") {
            wt.Category = "issue"
            num, _ := strconv.Atoi(strings.TrimPrefix(base, "issue-"))
            wt.Issue = num
            wt.Name = base
        } else {
            wt.Category = "terminal"
            wt.Name = base
        }
        return
    }
    wt.Category = "terminal"
    wt.Name = filepath.Base(wt.Path)
}
```

**Step 4: Run test to verify it passes**

```
go test ./internal/backend/... -run TestParseAllWorktrees -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/backend/app_worktree.go internal/backend/app_worktree_test.go
git commit -m "feat: add ListAllWorktrees with category support"
```

---

### Task 2: Backend – CreateNamedWorktree + GetLocalBranches

**Files:**
- Modify: `internal/backend/app_worktree.go`
- Modify: `internal/backend/app_git.go`
- Test: `internal/backend/app_worktree_test.go`

**Step 1: Write failing tests**

```go
// add to app_worktree_test.go

func TestSanitizeWorktreeName(t *testing.T) {
    cases := []struct{ in, want string }{
        {"my feature", "my-feature"},
        {"Fix/Bug 42", "fix-bug-42"},
        {"hello--world", "hello-world"},
        {"-start", "start"},
        {"end-", "end"},
    }
    for _, c := range cases {
        got := sanitizeWorktreeName(c.in)
        if got != c.want {
            t.Errorf("sanitize(%q) = %q, want %q", c.in, got, c.want)
        }
    }
}
```

**Step 2: Run test to verify it fails**

```
go test ./internal/backend/... -run TestSanitizeWorktreeName -v
```

**Step 3: Implement in `app_worktree.go`**

```go
// sanitizeWorktreeName converts a display name to a safe directory/branch name.
func sanitizeWorktreeName(name string) string {
    s := strings.ToLower(name)
    // Replace spaces and slashes with dashes
    s = strings.NewReplacer(" ", "-", "/", "-", "\\", "-").Replace(s)
    // Remove non-alphanumeric chars except dashes and underscores
    var b strings.Builder
    for _, r := range s {
        if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
            b.WriteRune(r)
        }
    }
    s = b.String()
    // Collapse multiple dashes
    for strings.Contains(s, "--") {
        s = strings.ReplaceAll(s, "--", "-")
    }
    // Trim leading/trailing dashes
    s = strings.Trim(s, "-")
    if s == "" {
        s = "worktree"
    }
    return s
}

// CreateNamedWorktree creates a general-purpose worktree (not tied to an issue).
// name is a display name (e.g. "my-feature"), baseBranch is the branch to fork from.
// The worktree is created at .mt-worktrees/<sanitized-name>/.
// The new branch is named "terminal/<sanitized-name>".
func (a *App) CreateNamedWorktree(dir, name, baseBranch string) (*WorktreeInfo, error) {
    root, err := repoRoot(dir)
    if err != nil {
        return nil, err
    }

    safeName := sanitizeWorktreeName(name)
    branch := "terminal/" + safeName
    wtPath := filepath.Join(root, worktreeDir, safeName)

    // Check if already exists
    if info, err := os.Stat(wtPath); err == nil && info.IsDir() {
        return &WorktreeInfo{Path: wtPath, Branch: branch, Category: "terminal", Name: safeName}, nil
    }

    // Ensure parent dir
    if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
        return nil, fmt.Errorf("mkdir failed: %w", err)
    }

    // Create worktree with new branch from baseBranch
    var cmd *exec.Cmd
    if branchExists(root, branch) {
        cmd = exec.Command("git", "worktree", "add", wtPath, branch)
    } else if baseBranch != "" && baseBranch != "HEAD" {
        cmd = exec.Command("git", "worktree", "add", "-b", branch, wtPath, baseBranch)
    } else {
        cmd = exec.Command("git", "worktree", "add", "-b", branch, wtPath)
    }
    cmd.Dir = root
    hideConsole(cmd)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("worktree add failed: %s – %w", strings.TrimSpace(string(out)), err)
    }

    log.Printf("[CreateNamedWorktree] created %s on branch %s", wtPath, branch)
    return &WorktreeInfo{Path: wtPath, Branch: branch, Category: "terminal", Name: safeName}, nil
}
```

Add `GetLocalBranches` to `internal/backend/app_git.go`:

```go
// GetLocalBranches returns local branch names for the repo containing dir.
func (a *App) GetLocalBranches(dir string) []string {
    cmd := exec.Command("git", "branch", "--format=%(refname:short)")
    cmd.Dir = dir
    hideConsole(cmd)
    out, err := cmd.Output()
    if err != nil {
        return nil
    }
    var branches []string
    for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
        if line = strings.TrimSpace(line); line != "" {
            branches = append(branches, line)
        }
    }
    return branches
}
```

**Step 4: Run tests**

```
go test ./internal/backend/... -run TestSanitizeWorktreeName -v
go vet ./internal/backend/...
```
Expected: PASS, no vet errors

**Step 5: Regenerate Wails bindings**

```bash
# Run wails dev briefly to regenerate wailsjs/ bindings, then stop with Ctrl+C
# Alternatively: wails generate module
wails build -debug 2>&1 | head -20
```

The new methods `ListAllWorktrees`, `CreateNamedWorktree`, `GetLocalBranches` will appear in `wailsjs/go/backend/App.js` and `App.d.ts`.

**Step 6: Commit**

```bash
git add internal/backend/app_worktree.go internal/backend/app_git.go internal/backend/app_worktree_test.go
git commit -m "feat: add CreateNamedWorktree and GetLocalBranches backend"
```

---

### Task 3: Frontend – Add `branch` field to Pane

**Files:**
- Modify: `frontend/src/stores/tabs.ts`

**Step 1: Update `Pane` interface and `addPane`**

In `frontend/src/stores/tabs.ts`, add `branch` to `Pane`:

```typescript
export interface Pane {
  id: string;
  sessionId: number;
  name: string;
  mode: PaneMode;
  model: string;
  focused: boolean;
  activity: 'idle' | 'active' | 'done' | 'needsInput';
  cost: string;
  running: boolean;
  maximized: boolean;
  issueNumber: number | null;
  issueTitle: string;
  issueBranch: string;
  worktreePath: string;
  branch: string;        // ← new: current git branch of this pane's dir
  zoomDelta: number;
}
```

Update `addPane` signature and initialization (add `branch?: string` parameter after `worktreePath`):

```typescript
addPane(
  tabId: string,
  sessionId: number,
  name: string,
  mode: PaneMode,
  model: string,
  issueNumber?: number | null,
  issueTitle?: string,
  issueBranch?: string,
  worktreePath?: string,
  branch?: string,       // ← new
): string {
  // ...inside the push:
  branch: branch ?? issueBranch ?? '',
```

**Step 2: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -30
```
Expected: only errors in files that call `addPane` with the old signature (will be fixed in Task 7).

**Step 3: Commit**

```bash
git add frontend/src/stores/tabs.ts
git commit -m "feat: add branch field to Pane store"
```

---

### Task 4: Frontend – WorktreeDropdown component

**Files:**
- Create: `frontend/src/components/WorktreeDropdown.svelte`

This dropdown shows categorized worktrees and dispatches selection/create events.

**Step 1: Create component**

```svelte
<!-- frontend/src/components/WorktreeDropdown.svelte -->
<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';

  // WorktreeInfo matches the Go struct
  export interface WorktreeInfo {
    path: string;
    branch: string;
    issue: number;
    category: string; // "main" | "terminal" | "issue"
    name: string;
  }

  export let worktrees: WorktreeInfo[] = [];
  export let currentBranch: string = '';

  const dispatch = createEventDispatcher();

  $: mainWorktrees = worktrees.filter(w => w.category === 'main');
  $: terminalWorktrees = worktrees.filter(w => w.category === 'terminal');
  $: issueWorktrees = worktrees.filter(w => w.category === 'issue');

  function select(wt: WorktreeInfo) {
    dispatch('select', wt);
  }

  function createNew() {
    dispatch('createNew');
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('close');
    e.stopPropagation();
  }
</script>

<!-- svelte-ignore a11y-no-static-element-interactions -->
<div class="dropdown" on:keydown={handleKeydown}>
  <button class="menu-item create-btn" on:click={createNew}>
    <span class="plus">+</span> Neuer Worktree
  </button>

  {#if mainWorktrees.length > 0}
    <div class="section-header">Haupt-Repository</div>
    {#each mainWorktrees as wt}
      <button
        class="menu-item"
        class:active={wt.branch === currentBranch}
        on:click={() => select(wt)}
        title={wt.path}
      >
        <span class="branch-icon">⎇</span>
        <span class="branch-name">{wt.branch || 'main'}</span>
        {#if wt.branch === currentBranch}
          <span class="current-dot">●</span>
        {/if}
      </button>
    {/each}
  {/if}

  {#if terminalWorktrees.length > 0}
    <div class="section-header">Terminal Worktrees</div>
    {#each terminalWorktrees as wt}
      <button
        class="menu-item"
        class:active={wt.branch === currentBranch}
        on:click={() => select(wt)}
        title={wt.path}
      >
        <span class="branch-icon">⎇</span>
        <span class="branch-name">{wt.branch}</span>
        <span class="wt-path">{wt.name}</span>
      </button>
    {/each}
  {/if}

  {#if issueWorktrees.length > 0}
    <div class="section-header">Issue Worktrees</div>
    {#each issueWorktrees as wt}
      <button
        class="menu-item"
        class:active={wt.branch === currentBranch}
        on:click={() => select(wt)}
        title={wt.path}
      >
        <span class="branch-icon">⎇</span>
        <span class="branch-name">{wt.branch}</span>
        <span class="wt-issue">#{wt.issue}</span>
      </button>
    {/each}
  {/if}

  {#if worktrees.length === 0}
    <div class="empty">Kein Git-Repository gefunden</div>
  {/if}
</div>

<style>
  .dropdown {
    position: absolute;
    top: 100%;
    left: 0;
    z-index: 200;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    box-shadow: 0 6px 24px rgba(0,0,0,0.4);
    min-width: 220px;
    max-height: 320px;
    overflow-y: auto;
    padding: 4px 0;
  }

  .section-header {
    font-size: 10px;
    font-weight: 700;
    color: var(--fg-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    padding: 8px 12px 4px;
  }

  .menu-item {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: 6px 12px;
    background: none;
    border: none;
    color: var(--fg);
    font-size: 12px;
    cursor: pointer;
    text-align: left;
    transition: background 0.1s;
    white-space: nowrap;
    overflow: hidden;
  }

  .menu-item:hover { background: var(--bg-tertiary); }
  .menu-item.active { color: var(--accent); }

  .create-btn {
    color: var(--accent);
    font-weight: 600;
    border-bottom: 1px solid var(--border);
    margin-bottom: 4px;
    padding-bottom: 8px;
  }

  .plus { font-size: 14px; font-weight: 700; }
  .branch-icon { color: var(--fg-muted); font-size: 10px; }
  .branch-name { flex: 1; overflow: hidden; text-overflow: ellipsis; }
  .wt-path { font-size: 10px; color: var(--fg-muted); }
  .wt-issue { font-size: 10px; color: #22c55e; font-weight: 600; }
  .current-dot { color: var(--accent); font-size: 8px; }
  .empty { padding: 12px; color: var(--fg-muted); font-size: 11px; text-align: center; }
</style>
```

**Step 2: Commit**

```bash
git add frontend/src/components/WorktreeDropdown.svelte
git commit -m "feat: add WorktreeDropdown component"
```

---

### Task 5: Frontend – WorktreeCreateDialog component

**Files:**
- Create: `frontend/src/components/WorktreeCreateDialog.svelte`

**Step 1: Create component**

```svelte
<!-- frontend/src/components/WorktreeCreateDialog.svelte -->
<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';

  export let visible: boolean = false;
  export let dir: string = '';

  const dispatch = createEventDispatcher();

  let name = '';
  let baseBranch = '';
  let branches: string[] = [];
  let creating = false;
  let error = '';
  let inputEl: HTMLInputElement;

  $: safeName = name
    .toLowerCase()
    .replace(/[\s/\\]+/g, '-')
    .replace(/[^a-z0-9\-_]/g, '')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '');
  $: branchPreview = safeName ? `terminal/${safeName}` : '';

  $: if (visible) {
    name = '';
    baseBranch = '';
    error = '';
    creating = false;
    loadBranches();
    requestAnimationFrame(() => inputEl?.focus());
  }

  async function loadBranches() {
    if (!dir) return;
    try {
      branches = await App.GetLocalBranches(dir);
      if (branches.length > 0) baseBranch = branches[0];
    } catch {}
  }

  async function create() {
    if (!safeName) { error = 'Name erforderlich'; return; }
    creating = true;
    error = '';
    try {
      const wt = await App.CreateNamedWorktree(dir, name, baseBranch);
      dispatch('created', wt);
      dispatch('close');
    } catch (err: any) {
      error = err?.message || String(err);
    } finally {
      creating = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('close');
    if (e.key === 'Enter' && !e.shiftKey) create();
    e.stopPropagation();
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="overlay" on:click={() => dispatch('close')}>
    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation on:keydown={handleKeydown}>
      <div class="dialog-header">
        <span class="dialog-icon">⎇</span>
        <h3>Neuer Terminal Worktree</h3>
      </div>
      <p class="dialog-desc">Erstellt ein isoliertes Arbeitsverzeichnis für diesen Branch.</p>

      <label class="field-label">Worktree-Name</label>
      <input
        class="field-input"
        type="text"
        placeholder="z.B. my-feature"
        bind:value={name}
        bind:this={inputEl}
      />
      {#if branchPreview}
        <div class="branch-preview">Branch: <code>{branchPreview}</code></div>
      {/if}

      <label class="field-label">Base Branch</label>
      {#if branches.length > 0}
        <select class="field-input" bind:value={baseBranch}>
          {#each branches as b}
            <option value={b}>{b}</option>
          {/each}
        </select>
      {:else}
        <input class="field-input" type="text" placeholder="main" bind:value={baseBranch} />
      {/if}
      <div class="field-hint">Branch, von dem der neue Worktree abzweigt</div>

      {#if error}
        <div class="error">{error}</div>
      {/if}

      <div class="dialog-footer">
        <button class="btn-cancel" on:click={() => dispatch('close')}>Abbrechen</button>
        <button class="btn-create" on:click={create} disabled={!safeName || creating}>
          {creating ? 'Erstelle...' : 'Erstellen'}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed; inset: 0;
    background: rgba(0,0,0,0.5);
    display: flex; align-items: center; justify-content: center;
    z-index: 300;
  }
  .dialog {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 20px;
    width: 380px;
    box-shadow: 0 8px 32px rgba(0,0,0,0.4);
  }
  .dialog-header {
    display: flex; align-items: center; gap: 8px; margin-bottom: 6px;
  }
  .dialog-icon { font-size: 18px; }
  h3 { margin: 0; font-size: 15px; color: var(--fg); }
  .dialog-desc { font-size: 12px; color: var(--fg-muted); margin: 0 0 16px; }

  .field-label {
    display: block; font-size: 12px; font-weight: 600;
    color: var(--fg); margin-bottom: 4px;
  }
  .field-input {
    width: 100%; padding: 8px 10px; box-sizing: border-box;
    background: var(--bg-secondary); border: 1px solid var(--border);
    border-radius: 6px; color: var(--fg); font-size: 13px; margin-bottom: 4px;
  }
  .field-input:focus { outline: none; border-color: var(--accent); }
  .branch-preview {
    font-size: 11px; color: var(--fg-muted); margin-bottom: 12px;
  }
  .branch-preview code {
    color: var(--accent); background: var(--bg-tertiary);
    padding: 1px 5px; border-radius: 3px;
  }
  .field-hint { font-size: 11px; color: var(--fg-muted); margin-bottom: 12px; }

  .error {
    background: rgba(239,68,68,0.1); border: 1px solid rgba(239,68,68,0.4);
    border-radius: 6px; padding: 8px 10px; font-size: 12px;
    color: #f87171; margin-bottom: 12px;
  }
  .dialog-footer { display: flex; justify-content: flex-end; gap: 8px; }
  .btn-cancel {
    padding: 7px 14px; background: var(--bg-tertiary); border: 1px solid var(--border);
    border-radius: 6px; color: var(--fg-muted); cursor: pointer; font-size: 12px;
  }
  .btn-create {
    padding: 7px 16px; background: var(--accent); border: none;
    border-radius: 6px; color: var(--bg); cursor: pointer; font-size: 12px; font-weight: 600;
  }
  .btn-create:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn-cancel:hover { color: var(--fg); }
</style>
```

**Step 2: Commit**

```bash
git add frontend/src/components/WorktreeCreateDialog.svelte
git commit -m "feat: add WorktreeCreateDialog component"
```

---

### Task 6: Frontend – PaneTitlebar: ⎇ branch button + dropdown

**Files:**
- Modify: `frontend/src/components/PaneTitlebar.svelte`

**Step 1: Replace static worktree badge with clickable button**

Replace the static `worktree-badge` block and add dropdown logic.

At the top of `<script>`, add imports:

```typescript
import WorktreeDropdown from './WorktreeDropdown.svelte';
import WorktreeCreateDialog from './WorktreeCreateDialog.svelte';

// New props:
export let worktrees: any[] = [];
export let tabDir: string = '';

let showWorktreeDropdown = false;
let showWorktreeCreate = false;

function toggleWorktreeDropdown() {
  showWorktreeDropdown = !showWorktreeDropdown;
}

function handleWorktreeSelect(e: CustomEvent) {
  showWorktreeDropdown = false;
  dispatch('openWorktreePane', { worktree: e.detail });
}

function handleWorktreeCreated(e: CustomEvent) {
  dispatch('openWorktreePane', { worktree: e.detail });
}

$: displayBranch = pane.branch || pane.issueBranch || '';
```

In the template, **replace** the existing static worktree badge:

```svelte
{#if pane.worktreePath}
  <span class="worktree-badge" title="Worktree: {pane.worktreePath}">worktree</span>
{/if}
```

**With** a clickable branch button + dropdown:

```svelte
<div class="worktree-wrap">
  <button
    class="branch-btn"
    class:has-worktree={!!pane.worktreePath}
    on:click|stopPropagation={toggleWorktreeDropdown}
    title={pane.worktreePath ? `Worktree: ${pane.worktreePath}` : 'Worktree auswählen'}
  >
    <span class="branch-icon">⎇</span>
    {displayBranch || '—'}
    <span class="dropdown-arrow">▾</span>
  </button>
  {#if showWorktreeDropdown}
    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
    <div class="dropdown-backdrop" on:click={() => showWorktreeDropdown = false}></div>
    <WorktreeDropdown
      {worktrees}
      currentBranch={displayBranch}
      on:select={handleWorktreeSelect}
      on:createNew={() => { showWorktreeDropdown = false; showWorktreeCreate = true; }}
      on:close={() => showWorktreeDropdown = false}
    />
  {/if}
</div>

<WorktreeCreateDialog
  visible={showWorktreeCreate}
  dir={tabDir}
  on:created={handleWorktreeCreated}
  on:close={() => showWorktreeCreate = false}
/>
```

Add CSS for the new elements (append to `<style>`):

```css
  .worktree-wrap { position: relative; }

  .branch-btn {
    display: flex; align-items: center; gap: 3px;
    font-size: 10px; padding: 1px 6px; border-radius: 4px;
    background: var(--bg-tertiary); border: 1px solid var(--border);
    color: var(--fg-muted); cursor: pointer; white-space: nowrap;
    transition: border-color 0.15s;
  }
  .branch-btn:hover { border-color: var(--accent); color: var(--fg); }
  .branch-btn.has-worktree { color: #60a5fa; border-color: #2563eb44; }

  .branch-icon { font-size: 9px; }
  .dropdown-arrow { font-size: 8px; opacity: 0.6; }

  .dropdown-backdrop {
    position: fixed; inset: 0; z-index: 199;
  }
```

**Step 2: Compile check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -30
```

**Step 3: Commit**

```bash
git add frontend/src/components/PaneTitlebar.svelte
git commit -m "feat: add worktree branch button to PaneTitlebar"
```

---

### Task 7: Frontend – App.svelte: wire events, pass branch to pane

**Files:**
- Modify: `frontend/src/App.svelte`
- Modify: `frontend/src/lib/launch.ts`

**Step 1: Remove auto-worktree from `setupIssueBranch` in `launch.ts`**

In `frontend/src/lib/launch.ts`, remove the `useWorktrees` code path.
Replace `setupIssueBranch`:

```typescript
/**
 * Set up branch for an issue before launching a session.
 * Worktrees are now managed explicitly via the WorktreeDropdown, not automatically.
 */
export async function setupIssueBranch(
  sessionDir: string,
  issue: IssueContext,
  autoBranch: boolean,
): Promise<BranchSetupResult> {
  const result: BranchSetupResult = { issueBranch: '', worktreePath: '', sessionDir };

  if (!autoBranch) return result;

  const branchInfo = await App.IsOnIssueBranch(sessionDir, issue.number);
  const isDefaultBranch = ['main', 'master', 'develop'].includes(branchInfo.branch_name);

  if (!isDefaultBranch && !branchInfo.is_same_issue) {
    const dirty = !(await App.HasCleanWorkingTree(sessionDir));
    result.conflict = {
      currentBranch: branchInfo.branch_name,
      currentIssueNumber: branchInfo.issue_number,
      dirtyWorkingTree: dirty,
    };
    return result;
  }

  if (branchInfo.is_same_issue) {
    result.issueBranch = branchInfo.branch_name;
  } else {
    try {
      result.issueBranch = await App.GetOrCreateIssueBranch(sessionDir, issue.number, issue.title);
    } catch (err: any) {
      const msg = err?.message || String(err);
      if (!confirm(`Branch-Erstellung fehlgeschlagen:\n${msg}\n\nTrotzdem ohne eigenen Branch starten?`)) {
        result.cancelled = true;
      }
    }
  }

  return result;
}
```

Also remove `useWorktrees` from `BranchSetupResult` (it wasn't there) — just verify `resolveBranchConflict` still works (the `'worktree'` action path in `resolveBranchConflict` can stay as a fallback for the BranchConflictDialog).

**Step 2: Update `App.svelte` – call site for `setupIssueBranch`**

Find the call (around line 230):
```typescript
const result = await setupIssueBranch(
  sessionDir, issueCtx,
  $config.use_worktrees === true,
  $config.auto_branch_on_issue !== false,
);
```

Change to:
```typescript
const result = await setupIssueBranch(
  sessionDir, issueCtx,
  $config.auto_branch_on_issue !== false,
);
```

**Step 3: In `App.svelte` – load branch at pane creation and pass to `addPane`**

In `handleLaunch`, after `const sessionId = await App.CreateSession(...)`, fetch the branch:

```typescript
if (sessionId > 0) {
  let paneBranch = issueBranch;
  if (!paneBranch) {
    try { paneBranch = await App.GetGitBranch(sessionDir); } catch {}
  }
  tabStore.addPane(tab.id, sessionId, name, type, model,
    issueCtx?.number, issueCtx?.title, issueBranch, worktreePath, paneBranch);
  // ... rest unchanged
}
```

Same fix in `handleBranchConflictChoice` (around line 284):

```typescript
if (sessionId > 0) {
  let paneBranch = resolved.issueBranch;
  if (!paneBranch) {
    try { paneBranch = await App.GetGitBranch(resolved.sessionDir); } catch {}
  }
  tabStore.addPane(tab.id, sessionId, name, type, model,
    issueCtx.number, issueCtx.title, resolved.issueBranch, resolved.worktreePath, paneBranch);
  // ... rest unchanged
}
```

**Step 4: In `App.svelte` – load worktrees and pass to PaneGrid/PaneTitlebar**

Add a reactive worktrees store:

```typescript
let allWorktrees: any[] = [];

async function loadWorktrees() {
  const tab = $activeTab;
  if (!tab?.dir) return;
  try { allWorktrees = await App.ListAllWorktrees(tab.dir); } catch {}
}

$: if ($activeTab) loadWorktrees();
```

**Step 5: In `App.svelte` – handle `openWorktreePane` event**

Add handler:

```typescript
async function handleOpenWorktreePane(e: CustomEvent<{ worktree: any }>) {
  const tab = $activeTab;
  if (!tab) return;
  const wt = e.detail.worktree;
  const claudeCmd = resolvedClaudePath;
  const argv = buildClaudeArgv('claude', '', claudeCmd);
  const name = `Claude – ⎇ ${wt.branch}`;
  try {
    const sessionId = await App.CreateSession(argv, wt.path, 24, 80);
    if (sessionId > 0) {
      tabStore.addPane(tab.id, sessionId, name, 'claude', '', null, '', wt.branch, wt.path, wt.branch);
    }
  } catch (err) { console.error('[handleOpenWorktreePane] failed:', err); }
}
```

**Step 6: Thread props/events through PaneGrid**

In `App.svelte` template, pass `worktrees` and `tabDir` to `PaneGrid`:

```svelte
<PaneGrid
  ...
  worktrees={allWorktrees}
  tabDir={$activeTab?.dir || ''}
  on:openWorktreePane={handleOpenWorktreePane}
  ...
/>
```

In `PaneGrid.svelte`, add props and forward:
```svelte
export let worktrees: any[] = [];
export let tabDir: string = '';
```
And pass to `TerminalPane`:
```svelte
<TerminalPane {worktrees} {tabDir} on:openWorktreePane />
```

In `TerminalPane.svelte`, similarly add props and forward to `PaneTitlebar`:
```svelte
export let worktrees: any[] = [];
export let tabDir: string = '';
```
```svelte
<PaneTitlebar {worktrees} {tabDir} on:openWorktreePane />
```

**Step 7: TypeScript compile check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -40
```
Expected: no errors.

**Step 8: Commit**

```bash
git add frontend/src/App.svelte frontend/src/lib/launch.ts \
        frontend/src/components/PaneGrid.svelte frontend/src/components/TerminalPane.svelte
git commit -m "feat: wire worktree events through component tree"
```

---

### Task 8: Cleanup – Remove use_worktrees from Settings UI

**Files:**
- Modify: `frontend/src/components/SettingsDialog.svelte`
- Modify: `frontend/src/stores/config.ts`

**Step 1: Remove from SettingsDialog**

In `SettingsDialog.svelte`, find and **remove** the `useWorktrees` variable declaration, the `$: if (visible)` reactive assignment, and the checkbox element for it in the template. Search for `use_worktrees` and `useWorktrees` and delete all occurrences.

**Step 2: Remove from config store type**

In `frontend/src/stores/config.ts`, remove:
```typescript
use_worktrees?: boolean;
```

**Step 3: Compile check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -20
```

**Step 4: Commit**

```bash
git add frontend/src/components/SettingsDialog.svelte frontend/src/stores/config.ts
git commit -m "feat: remove use_worktrees setting, worktrees are now explicit"
```

---

### Task 9: Build + Smoke Test

**Step 1: Run Go tests**

```bash
go test ./internal/backend/... -v 2>&1 | tail -20
go vet ./...
```
Expected: all PASS, no vet errors.

**Step 2: Build**

```bash
wails build -debug 2>&1 | tail -10
```
Expected: `Build successful`

**Step 3: Manual smoke test checklist**

- [ ] Öffne ein Tab mit einem Git-Repo
- [ ] `Ctrl+N` → Claude starten → Pane zeigt `⎇ main` Button in Titlebar
- [ ] Klick auf `⎇ main` → Dropdown öffnet, zeigt "Haupt-Repository" + evtl. bestehende Worktrees
- [ ] `+ Neuer Worktree` → Create-Dialog öffnet
- [ ] Name eingeben → Branch-Preview zeigt `terminal/<name>`
- [ ] Base Branch Dropdown zeigt lokale Branches
- [ ] "Erstellen" → neuer Pane öffnet sich im Worktree-Verzeichnis mit `⎇ terminal/<name>` im Titlebar
- [ ] Worktree erscheint beim nächsten Dropdown-Öffnen unter "Terminal Worktrees"
- [ ] Einstellungen: kein `use_worktrees` Toggle mehr vorhanden
- [ ] Issue-Flow (falls vorhanden): startet ohne auto-Worktree, Branch wird wie bisher angelegt

**Step 4: Final commit**

```bash
git add .
git commit -m "feat: worktree picker per pane – explicit control via titlebar dropdown"
```
