# .gitignore Context Menu Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a right-click context menu to Source Control view entries that appends the file's repo-relative path to `.gitignore` (creating it if missing).

**Architecture:** New `AddToGitignore(dir, relPath string) error` method on the Go backend; a custom Svelte context menu overlay triggered by `contextmenu` events on each file entry in `SourceControlView.svelte`.

**Tech Stack:** Go (backend, `internal/backend/app_git.go`), Svelte 4 + TypeScript (frontend, `SourceControlView.svelte`), Wails v3 auto-generated bindings.

---

### Task 1: Backend — `AddToGitignore`

**Files:**
- Modify: `internal/backend/app_git.go` (append at end, currently 225 lines → stays under 300)
- Modify: `internal/backend/app_git_test.go` (append new tests at end)

**Context:**
- `app_git.go` already has `GetGitBranch`, `GetGitFileStatuses`, etc.
- `newTestApp()` is defined in `app_queue_test.go` (same package `backend`)
- `gitAvailable()` and `gitTestEnv()` helper are already in `app_git_test.go`
- Tests use `t.TempDir()` + `exec.Command("git", ...)` to create real temp repos

---

**Step 1: Write failing tests in `internal/backend/app_git_test.go`**

Append these tests at the end of the file:

```go
// ---------------------------------------------------------------------------
// AddToGitignore – integration tests
// ---------------------------------------------------------------------------

func setupGitRepo(t *testing.T) (dir string, run func(...string)) {
	t.Helper()
	dir = t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	run = func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), gitTestEnv()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init")
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0644)
	run("add", ".")
	run("commit", "--no-gpg-sign", "-m", "initial")
	return
}

func TestAddToGitignore_CreatesFile(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir, _ := setupGitRepo(t)
	a := newTestApp()

	err := a.AddToGitignore(dir, "secret.env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gitignorePath := filepath.Join(dir, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf(".gitignore not created: %v", err)
	}
	if !strings.Contains(string(content), "secret.env") {
		t.Fatalf(".gitignore does not contain 'secret.env', got: %q", string(content))
	}
}

func TestAddToGitignore_AppendsToExisting(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir, _ := setupGitRepo(t)
	a := newTestApp()

	gitignorePath := filepath.Join(dir, ".gitignore")
	os.WriteFile(gitignorePath, []byte("node_modules/\n"), 0644)

	err := a.AddToGitignore(dir, "dist/output.js")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(gitignorePath)
	if !strings.Contains(string(content), "node_modules/") {
		t.Fatalf("existing entry was removed")
	}
	if !strings.Contains(string(content), "dist/output.js") {
		t.Fatalf("new entry not appended")
	}
}

func TestAddToGitignore_NoDuplicates(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir, _ := setupGitRepo(t)
	a := newTestApp()

	gitignorePath := filepath.Join(dir, ".gitignore")
	os.WriteFile(gitignorePath, []byte("secret.env\n"), 0644)

	err := a.AddToGitignore(dir, "secret.env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(gitignorePath)
	count := strings.Count(string(content), "secret.env")
	if count != 1 {
		t.Fatalf("expected exactly 1 occurrence of 'secret.env', got %d", count)
	}
}

func TestAddToGitignore_NormalizesBackslashes(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir, _ := setupGitRepo(t)
	a := newTestApp()

	// Windows-style path separator
	err := a.AddToGitignore(dir, `config\local.yaml`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gitignorePath := filepath.Join(dir, ".gitignore")
	content, _ := os.ReadFile(gitignorePath)
	if !strings.Contains(string(content), "config/local.yaml") {
		t.Fatalf("expected forward slashes in .gitignore, got: %q", string(content))
	}
}

func TestAddToGitignore_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	a := newTestApp()

	err := a.AddToGitignore(dir, "secret.env")
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
cd /d/repos/Multiterminal
go test ./internal/backend/... -run TestAddToGitignore -v
```

Expected: FAIL with `undefined: (*AppService).AddToGitignore` or similar.

**Step 3: Implement `AddToGitignore` in `internal/backend/app_git.go`**

Append at the end of the file (after the existing `markParentDirs` and helpers):

```go
// AddToGitignore appends relPath to the .gitignore at the repo root of dir.
// Creates .gitignore if it does not exist. No-ops if the entry is already present.
// relPath must be relative to the repo root (forward slashes preferred; backslashes are normalized).
func (a *AppService) AddToGitignore(dir, relPath string) error {
	// Normalize path separator for .gitignore (always forward slashes)
	entry := strings.ReplaceAll(relPath, `\`, "/")

	// Find repo root
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}
	repoRoot := strings.TrimSpace(string(out))
	gitignorePath := filepath.Join(filepath.FromSlash(repoRoot), ".gitignore")

	// Check for existing entry
	if data, err := os.ReadFile(gitignorePath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == entry {
				return nil // already present
			}
		}
	}

	// Append entry (create file if missing)
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open .gitignore: %w", err)
	}
	defer f.Close()

	// Ensure new entry starts on its own line
	info, _ := f.Stat()
	if info != nil && info.Size() > 0 {
		// Read last byte to check if file ends with newline
		rf, _ := os.Open(gitignorePath)
		if rf != nil {
			rf.Seek(-1, 2)
			buf := make([]byte, 1)
			rf.Read(buf)
			rf.Close()
			if buf[0] != '\n' {
				f.WriteString("\n")
			}
		}
	}

	_, err = fmt.Fprintf(f, "%s\n", entry)
	return err
}
```

Also add `"fmt"` to the import block at the top of `app_git.go` if not already present.

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/backend/... -run TestAddToGitignore -v
```

Expected: All 5 tests PASS.

**Step 5: Run full backend test suite**

```bash
go test ./internal/backend/... -v 2>&1 | tail -20
```

Expected: All pass, no regressions.

**Step 6: Commit**

```bash
git add internal/backend/app_git.go internal/backend/app_git_test.go
git commit -m "feat(git): add AddToGitignore backend method with tests"
```

---

### Task 2: Frontend — Context Menu in SourceControlView

**Files:**
- Modify: `frontend/src/components/SourceControlView.svelte` (currently 194 lines)

**Context:**
- The component receives `dir: string` and `gitStatuses: Record<string, string>` as props
- `entry.relPath` is the relative path from repo root — this is what we pass to `AddToGitignore`
- Wails v3 bindings are in `wailsjs/go/backend/AppService.js` — use named import
- Existing feedback pattern: `copiedPath` + `setCopied()` timeout — replicate for "added" feedback
- `<style>` block is at the bottom — append new CSS rules there
- The component is used in `Sidebar.svelte` which handles `on:selectFile`

---

**Step 1: Add import and state variables**

In the `<script>` block, after the existing imports:

```typescript
import { AddToGitignore } from '../../wailsjs/go/backend/AppService';
```

After the existing `let copiedPath = '';` and `let copiedTimer`:

```typescript
let addedPath = '';
let addedTimer: ReturnType<typeof setTimeout> | null = null;

function setAdded(path: string) {
  addedPath = path;
  if (addedTimer) clearTimeout(addedTimer);
  addedTimer = setTimeout(() => { addedPath = ''; }, 1500);
}

let contextMenu: { x: number; y: number; entry: ScEntry } | null = null;

function openContextMenu(e: MouseEvent, entry: ScEntry) {
  e.preventDefault();
  contextMenu = { x: e.clientX, y: e.clientY, entry };
  window.addEventListener('keydown', onMenuKeydown);
}

function closeContextMenu() {
  contextMenu = null;
  window.removeEventListener('keydown', onMenuKeydown);
}

function onMenuKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') closeContextMenu();
}

async function handleAddToGitignore() {
  if (!contextMenu) return;
  const entry = contextMenu.entry;
  closeContextMenu();
  try {
    await AddToGitignore(dir, entry.relPath);
    setAdded(entry.path);
  } catch (err) {
    console.error('AddToGitignore failed:', err);
  }
}
```

**Step 2: Add `on:contextmenu` to `.sc-entry` div**

Find this line in the template (around line 99–106):
```svelte
<div
  class="sc-entry {getStatusClass(entry.status)}"
  on:click={() => handleScClick(entry.path)}
  on:keydown
  role="button"
  tabindex="-1"
  title={entry.path}
>
```

Change to:
```svelte
<div
  class="sc-entry {getStatusClass(entry.status)}"
  on:click={() => handleScClick(entry.path)}
  on:contextmenu={(e) => openContextMenu(e, entry)}
  on:keydown
  role="button"
  tabindex="-1"
  title={entry.path}
>
```

**Step 3: Replace the copy badge section with added feedback**

Find this block (around lines 109–113):
```svelte
{#if copiedPath === entry.path}
  <span class="copied-badge">kopiert!</span>
{:else}
  <span class="sc-badge {getStatusClass(entry.status)}">{entry.status === '?' ? 'N' : entry.status === 'U' ? 'C' : entry.status}</span>
{/if}
```

Change to:
```svelte
{#if copiedPath === entry.path}
  <span class="copied-badge">kopiert!</span>
{:else if addedPath === entry.path}
  <span class="copied-badge">hinzugefügt!</span>
{:else}
  <span class="sc-badge {getStatusClass(entry.status)}">{entry.status === '?' ? 'N' : entry.status === 'U' ? 'C' : entry.status}</span>
{/if}
```

**Step 4: Add context menu overlay to end of `.file-list` div**

Just before the closing `</div>` of the outer `.file-list` div (after the `{/if}` that ends `{#if groupedChanges.length === 0}`):

```svelte
{#if contextMenu}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="ctx-backdrop" on:click={closeContextMenu} on:contextmenu|preventDefault></div>
  <div class="ctx-menu" style="left:{contextMenu.x}px; top:{contextMenu.y}px">
    <button class="ctx-item" on:click={handleAddToGitignore}>
      Zu .gitignore hinzufügen
    </button>
  </div>
{/if}
```

**Step 5: Add CSS rules**

Append to the `<style>` block:

```css
  .ctx-backdrop {
    position: fixed; inset: 0; z-index: 100;
  }

  .ctx-menu {
    position: fixed; z-index: 101;
    background: var(--bg-secondary); border: 1px solid var(--border);
    border-radius: 6px; padding: 4px 0;
    box-shadow: 0 4px 16px rgba(0,0,0,0.3);
    min-width: 180px;
  }

  .ctx-item {
    display: block; width: 100%;
    padding: 6px 14px; text-align: left;
    background: none; border: none;
    color: var(--fg); font-size: 12px; cursor: pointer;
  }
  .ctx-item:hover { background: var(--bg-tertiary); }
```

**Step 6: Check Wails binding is generated**

Run dev build briefly to verify the binding was picked up:

```bash
cd /d/repos/Multiterminal
go build ./... 2>&1
```

Expected: No errors. (Wails regenerates `wailsjs/` on `wails dev`; for tests we just verify Go compiles.)

**Step 7: Run frontend type check**

```bash
cd /d/repos/Multiterminal/frontend
npm run check 2>&1 | tail -20
```

Expected: No TypeScript errors.

**Step 8: Commit**

```bash
git add frontend/src/components/SourceControlView.svelte
git commit -m "feat(ui): add right-click context menu to source control entries"
```

---

### Task 3: Regenerate Wails Bindings

**Files:**
- Modify: `wailsjs/go/backend/AppService.js` (auto-generated, do not edit manually)
- Modify: `wailsjs/go/backend/AppService.d.ts` (auto-generated, do not edit manually)

**Context:**
Wails v3 auto-generates JS bindings from Go methods tagged on `AppService`. The file
`wailsjs/go/backend/AppService.js` must contain `AddToGitignore` before the frontend
import will resolve at runtime.

**Step 1: Check current bindings**

```bash
grep -n "AddToGitignore" wailsjs/go/backend/AppService.js wailsjs/go/backend/AppService.d.ts
```

Expected (if already present from a previous `wails dev` run): lines found. If not present, proceed to Step 2.

**Step 2: Regenerate bindings**

```bash
cd /d/repos/Multiterminal
wails generate bindings 2>&1
```

Or start `wails dev` briefly (it auto-regenerates on startup), then Ctrl-C.

Expected: `AddToGitignore` now appears in `wailsjs/go/backend/AppService.js`.

**Step 3: Commit bindings**

```bash
git add wailsjs/go/backend/AppService.js wailsjs/go/backend/AppService.d.ts
git commit -m "chore: regenerate Wails bindings for AddToGitignore"
```

---

### Task 4: Full Test Run & Verification

**Step 1: Run all Go tests**

```bash
cd /d/repos/Multiterminal
go test ./internal/... -v 2>&1 | grep -E "^(ok|FAIL|---)"
```

Expected: All packages `ok`.

**Step 2: Run frontend tests**

```bash
cd /d/repos/Multiterminal/frontend
npm test -- --run 2>&1 | tail -10
```

Expected: All tests pass (currently 94).

**Step 3: Run go vet**

```bash
cd /d/repos/Multiterminal
go vet ./... 2>&1
```

Expected: No output (clean).

**Step 4: Commit if any fixes were needed, otherwise done.**
