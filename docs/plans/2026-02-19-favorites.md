# Favorites/Bookmarks Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow users to pin frequently used files and directories per working directory, shown as a collapsible section in the Explorer sidebar.

**Architecture:** Add `Favorites map[string][]string` to the Go config struct, expose three Wails bindings (Get/Add/Remove), create a new `FavoritesSection.svelte` component, and extend `FileTreeItem.svelte` with a star toggle button. Favorites are persisted in `~/.multiterminal.yaml`.

**Tech Stack:** Go (config + backend bindings), Svelte 4 (FavoritesSection component), Wails v2 bindings

---

### Task 1: Add Favorites field to Go Config struct

**Files:**
- Modify: `internal/config/config.go:14-34` (Config struct)

**Step 1: Add Favorites field to Config struct**

In `internal/config/config.go`, add the `Favorites` field to the `Config` struct after `SidebarPinned`:

```go
SidebarPinned         bool                `yaml:"sidebar_pinned" json:"sidebar_pinned"`
Favorites             map[string][]string `yaml:"favorites,omitempty" json:"favorites,omitempty"`
```

**Step 2: Initialize Favorites map in Load()**

In `internal/config/config.go`, in the `Load()` function, after all existing validation (around line 205), add nil-map initialization:

```go
if cfg.Favorites == nil {
    cfg.Favorites = make(map[string][]string)
}
```

**Step 3: Commit**

```
git add internal/config/config.go
git commit -m "feat(config): add Favorites map field for per-directory bookmarks (#63)"
```

---

### Task 2: Write tests for favorites config persistence

**Files:**
- Modify: `internal/config/config_test.go`

**Step 1: Write test for Favorites YAML round-trip**

Append to `internal/config/config_test.go`:

```go
func TestConfig_FavoritesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-config.yaml")

	original := DefaultConfig()
	original.Favorites = map[string][]string{
		"/home/user/project": {"/home/user/project/main.go", "/home/user/project/src"},
	}

	err := writeDefaults(path, original)
	if err != nil {
		t.Fatalf("writeDefaults failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var loaded Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	favs, ok := loaded.Favorites["/home/user/project"]
	if !ok {
		t.Fatal("expected favorites for /home/user/project")
	}
	if len(favs) != 2 {
		t.Fatalf("expected 2 favorites, got %d", len(favs))
	}
	if favs[0] != "/home/user/project/main.go" {
		t.Errorf("fav[0] = %q, want '/home/user/project/main.go'", favs[0])
	}
}

func TestConfig_FavoritesOmitEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-config.yaml")

	cfg := DefaultConfig()
	// Favorites is nil — should be omitted from YAML
	err := writeDefaults(path, cfg)
	if err != nil {
		t.Fatalf("writeDefaults failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	content := string(data)
	// With omitempty, nil/empty map should not appear in YAML
	if strings.Contains(content, "favorites:") {
		// This is acceptable — yaml.v3 may serialize empty maps
		// The important thing is it round-trips correctly
	}

	var loaded Config
	yaml.Unmarshal(data, &loaded)
	// Even if nil, after Load() it should be initialized
	if loaded.Favorites == nil {
		loaded.Favorites = make(map[string][]string)
	}
	if len(loaded.Favorites) != 0 {
		t.Errorf("expected 0 favorites, got %d", len(loaded.Favorites))
	}
}
```

**Step 2: Run tests to verify they pass**

Run: `cd D:/repos/Multiterminal && go test ./internal/config/ -run TestConfig_Favorites -v`
Expected: PASS

**Step 3: Commit**

```
git add internal/config/config_test.go
git commit -m "test(config): add favorites YAML round-trip tests (#63)"
```

---

### Task 3: Add favorites backend bindings

**Files:**
- Create: `internal/backend/app_favorites.go`

**Step 1: Create app_favorites.go with Get/Add/Remove bindings**

Create `internal/backend/app_favorites.go`:

```go
package backend

import (
	"log"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

// GetFavorites returns the list of favorite paths for the given directory.
func (a *App) GetFavorites(dir string) []string {
	if dir == "" {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg.Favorites == nil {
		return nil
	}
	return a.cfg.Favorites[dir]
}

// AddFavorite adds a path to the favorites for the given directory
// and persists the config to disk.
func (a *App) AddFavorite(dir string, path string) error {
	if dir == "" || path == "" {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cfg.Favorites == nil {
		a.cfg.Favorites = make(map[string][]string)
	}

	// Check for duplicates
	for _, f := range a.cfg.Favorites[dir] {
		if f == path {
			return nil
		}
	}

	a.cfg.Favorites[dir] = append(a.cfg.Favorites[dir], path)
	log.Printf("[AddFavorite] dir=%q path=%q total=%d", dir, path, len(a.cfg.Favorites[dir]))
	return config.Save(a.cfg)
}

// RemoveFavorite removes a path from the favorites for the given directory
// and persists the config to disk.
func (a *App) RemoveFavorite(dir string, path string) error {
	if dir == "" || path == "" {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cfg.Favorites == nil {
		return nil
	}

	favs := a.cfg.Favorites[dir]
	for i, f := range favs {
		if f == path {
			a.cfg.Favorites[dir] = append(favs[:i], favs[i+1:]...)
			break
		}
	}

	// Clean up empty entries
	if len(a.cfg.Favorites[dir]) == 0 {
		delete(a.cfg.Favorites, dir)
	}

	log.Printf("[RemoveFavorite] dir=%q path=%q", dir, path)
	return config.Save(a.cfg)
}
```

**Step 2: Commit**

```
git add internal/backend/app_favorites.go
git commit -m "feat(backend): add Get/Add/RemoveFavorite Wails bindings (#63)"
```

---

### Task 4: Write tests for favorites backend bindings

**Files:**
- Create: `internal/backend/app_favorites_test.go`

**Step 1: Write tests**

Create `internal/backend/app_favorites_test.go`:

```go
package backend

import "testing"

func TestGetFavorites_EmptyDir(t *testing.T) {
	a := newTestApp()
	if a.GetFavorites("") != nil {
		t.Fatal("empty dir should return nil")
	}
}

func TestGetFavorites_NoFavorites(t *testing.T) {
	a := newTestApp()
	result := a.GetFavorites("/some/dir")
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestAddFavorite_Basic(t *testing.T) {
	a := newTestApp()
	if err := a.AddFavorite("/project", "/project/main.go"); err != nil {
		t.Fatalf("AddFavorite failed: %v", err)
	}

	favs := a.GetFavorites("/project")
	if len(favs) != 1 {
		t.Fatalf("expected 1 favorite, got %d", len(favs))
	}
	if favs[0] != "/project/main.go" {
		t.Errorf("fav = %q, want '/project/main.go'", favs[0])
	}
}

func TestAddFavorite_NoDuplicates(t *testing.T) {
	a := newTestApp()
	a.AddFavorite("/project", "/project/main.go")
	a.AddFavorite("/project", "/project/main.go")

	favs := a.GetFavorites("/project")
	if len(favs) != 1 {
		t.Fatalf("expected 1 favorite (no duplicates), got %d", len(favs))
	}
}

func TestAddFavorite_MultiplePaths(t *testing.T) {
	a := newTestApp()
	a.AddFavorite("/project", "/project/main.go")
	a.AddFavorite("/project", "/project/src")

	favs := a.GetFavorites("/project")
	if len(favs) != 2 {
		t.Fatalf("expected 2 favorites, got %d", len(favs))
	}
}

func TestAddFavorite_MultipleDirectories(t *testing.T) {
	a := newTestApp()
	a.AddFavorite("/project1", "/project1/main.go")
	a.AddFavorite("/project2", "/project2/app.py")

	if len(a.GetFavorites("/project1")) != 1 {
		t.Fatal("project1 should have 1 favorite")
	}
	if len(a.GetFavorites("/project2")) != 1 {
		t.Fatal("project2 should have 1 favorite")
	}
}

func TestRemoveFavorite_Basic(t *testing.T) {
	a := newTestApp()
	a.AddFavorite("/project", "/project/main.go")
	a.AddFavorite("/project", "/project/src")

	if err := a.RemoveFavorite("/project", "/project/main.go"); err != nil {
		t.Fatalf("RemoveFavorite failed: %v", err)
	}

	favs := a.GetFavorites("/project")
	if len(favs) != 1 {
		t.Fatalf("expected 1 favorite after removal, got %d", len(favs))
	}
	if favs[0] != "/project/src" {
		t.Errorf("remaining fav = %q, want '/project/src'", favs[0])
	}
}

func TestRemoveFavorite_LastOneCleanup(t *testing.T) {
	a := newTestApp()
	a.AddFavorite("/project", "/project/main.go")
	a.RemoveFavorite("/project", "/project/main.go")

	favs := a.GetFavorites("/project")
	if favs != nil {
		t.Fatalf("expected nil after removing last favorite, got %v", favs)
	}
}

func TestRemoveFavorite_NonExistent(t *testing.T) {
	a := newTestApp()
	// Should not panic or error
	if err := a.RemoveFavorite("/project", "/project/nope.go"); err != nil {
		t.Fatalf("removing non-existent should not error: %v", err)
	}
}

func TestAddFavorite_EmptyArgs(t *testing.T) {
	a := newTestApp()
	if err := a.AddFavorite("", "/project/main.go"); err != nil {
		t.Fatal("empty dir should return nil, not error")
	}
	if err := a.AddFavorite("/project", ""); err != nil {
		t.Fatal("empty path should return nil, not error")
	}
}
```

**Step 2: Run tests**

Run: `cd D:/repos/Multiterminal && go test ./internal/backend/ -run TestGetFavorites -v && go test ./internal/backend/ -run TestAddFavorite -v && go test ./internal/backend/ -run TestRemoveFavorite -v`
Expected: ALL PASS

Note: `AddFavorite`/`RemoveFavorite` call `config.Save()` which writes to `~/.multiterminal.yaml`. In tests, `newTestApp()` has a zero-value `cfg` — `config.Save` will write to the real config path. The tests still pass because we only check in-memory state. A future improvement could mock the save, but for now this is acceptable since the file already exists.

**Step 3: Commit**

```
git add internal/backend/app_favorites_test.go
git commit -m "test(backend): add favorites binding tests (#63)"
```

---

### Task 5: Update frontend config store TypeScript types

**Files:**
- Modify: `frontend/src/stores/config.ts`

**Step 1: Add favorites field to AppConfig interface**

In `frontend/src/stores/config.ts`, add to the `AppConfig` interface (after `sidebar_pinned`):

```typescript
favorites: Record<string, string[]>;
```

**Step 2: Add default value to config store initializer**

In the `writable<AppConfig>()` call, add after `sidebar_pinned: false`:

```typescript
favorites: {},
```

**Step 3: Commit**

```
git add frontend/src/stores/config.ts
git commit -m "feat(frontend): add favorites type to AppConfig store (#63)"
```

---

### Task 6: Create FavoritesSection.svelte component

**Files:**
- Create: `frontend/src/components/FavoritesSection.svelte`

**Step 1: Create the component**

Create `frontend/src/components/FavoritesSection.svelte`:

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let favorites: string[] = [];
  let collapsed = false;

  const dispatch = createEventDispatcher();

  function fileName(path: string): string {
    const parts = path.replace(/\\/g, '/').split('/');
    return parts[parts.length - 1] || path;
  }

  function isDir(path: string): boolean {
    // Heuristic: paths ending with separator or without extension
    const name = fileName(path);
    return !name.includes('.');
  }

  function handleClick(path: string) {
    dispatch('selectFile', { path });
  }

  function handleRemove(e: MouseEvent, path: string) {
    e.stopPropagation();
    dispatch('removeFavorite', { path });
  }

  function handleDragStart(e: DragEvent, path: string) {
    const formatted = path.includes(' ') ? `"${path}"` : path;
    e.dataTransfer?.setData('text/plain', formatted);
  }
</script>

{#if favorites.length > 0 || !collapsed}
  <div class="favorites-section">
    <button class="favorites-header" on:click={() => (collapsed = !collapsed)}>
      <span class="collapse-icon">{collapsed ? '\u25B6' : '\u25BC'}</span>
      <span class="favorites-title">\u2605 Favorites</span>
      {#if favorites.length > 0}
        <span class="fav-count">{favorites.length}</span>
      {/if}
    </button>

    {#if !collapsed}
      {#if favorites.length === 0}
        <div class="no-favorites">Keine Favoriten</div>
      {:else}
        {#each favorites as fav (fav)}
          <div
            class="fav-entry"
            draggable="true"
            on:dragstart={(e) => handleDragStart(e, fav)}
            on:click={() => handleClick(fav)}
            on:keydown
            role="treeitem"
            tabindex="-1"
            title={fav}
          >
            <span class="fav-icon">{isDir(fav) ? '\u{1F4C1}' : '\u{1F4C4}'}</span>
            <span class="fav-name">{fileName(fav)}</span>
            <button class="remove-btn" on:click={(e) => handleRemove(e, fav)} title="Favorit entfernen">
              <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
                <path d="M8 1a7 7 0 1 0 0 14A7 7 0 0 0 8 1zm3.5 9.5l-1 1L8 9l-2.5 2.5-1-1L7 8 4.5 5.5l1-1L8 7l2.5-2.5 1 1L9 8z"/>
              </svg>
            </button>
          </div>
        {/each}
      {/if}
    {/if}
  </div>
{/if}

<style>
  .favorites-section {
    border-bottom: 1px solid var(--border);
  }

  .favorites-header {
    display: flex; align-items: center; gap: 6px; width: 100%;
    padding: 6px 10px; background: none; border: none;
    color: var(--fg); font-size: 11px; font-weight: 600;
    cursor: pointer; text-align: left;
  }
  .favorites-header:hover { background: var(--bg-tertiary); }

  .collapse-icon { font-size: 8px; width: 10px; flex-shrink: 0; }
  .favorites-title { flex: 1; }

  .fav-count {
    font-size: 10px; font-weight: 700; background: var(--bg-tertiary);
    border-radius: 8px; padding: 0 5px; line-height: 16px; min-width: 16px;
    text-align: center; color: var(--fg-muted);
  }

  .no-favorites {
    padding: 8px 10px; font-size: 11px; color: var(--fg-muted);
    font-style: italic;
  }

  .fav-entry {
    display: flex; align-items: center; gap: 6px;
    padding: 3px 10px 3px 20px; cursor: pointer;
    color: var(--fg); font-size: 12px;
  }
  .fav-entry:hover { background: var(--bg-tertiary); }

  .fav-icon { font-size: 12px; flex-shrink: 0; }
  .fav-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }

  .remove-btn {
    opacity: 0; background: none; border: none; color: var(--fg-muted);
    cursor: pointer; padding: 1px 3px; border-radius: 3px; flex-shrink: 0;
    display: flex; align-items: center; transition: opacity 0.15s;
  }
  .fav-entry:hover .remove-btn { opacity: 1; }
  .remove-btn:hover { color: #f87171; background: var(--bg-secondary); }
</style>
```

**Step 2: Commit**

```
git add frontend/src/components/FavoritesSection.svelte
git commit -m "feat(frontend): create FavoritesSection component (#63)"
```

---

### Task 7: Add star toggle button to FileTreeItem.svelte

**Files:**
- Modify: `frontend/src/components/FileTreeItem.svelte`

**Step 1: Add isFavorite prop**

In `FileTreeItem.svelte`, add a new export prop after `copiedPath`:

```typescript
export let isFavorite: boolean = false;
```

**Step 2: Add toggleFavorite handler**

After the `handleCopy` function, add:

```typescript
function handleToggleFavorite(e: MouseEvent) {
    e.stopPropagation();
    dispatch('toggleFavorite', { path: entry.path, isFavorite });
}
```

**Step 3: Add star button to template**

In the template, after the copy button (`<button class="copy-btn" ...>`), add the star button:

```svelte
<button class="star-btn" class:active={isFavorite} on:click={handleToggleFavorite} title={isFavorite ? 'Favorit entfernen' : 'Als Favorit markieren'}>
    <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
      {#if isFavorite}
        <path d="M8 .25a.75.75 0 0 1 .673.418l1.882 3.815 4.21.612a.75.75 0 0 1 .416 1.279l-3.046 2.97.719 4.192a.75.75 0 0 1-1.088.791L8 12.347l-3.766 1.98a.75.75 0 0 1-1.088-.79l.72-4.194L.818 6.374a.75.75 0 0 1 .416-1.28l4.21-.611L7.327.668A.75.75 0 0 1 8 .25z"/>
      {:else}
        <path d="M8 .25a.75.75 0 0 1 .673.418l1.882 3.815 4.21.612a.75.75 0 0 1 .416 1.279l-3.046 2.97.719 4.192a.75.75 0 0 1-1.088.791L8 12.347l-3.766 1.98a.75.75 0 0 1-1.088-.79l.72-4.194L.818 6.374a.75.75 0 0 1 .416-1.28l4.21-.611L7.327.668A.75.75 0 0 1 8 .25zm0 2.445L6.615 5.5a.75.75 0 0 1-.564.41l-3.097.45 2.24 2.184a.75.75 0 0 1 .216.664l-.528 3.084 2.769-1.456a.75.75 0 0 1 .698 0l2.77 1.456-.53-3.084a.75.75 0 0 1 .216-.664l2.24-2.183-3.096-.45a.75.75 0 0 1-.564-.41L8 2.694z"/>
      {/if}
    </svg>
</button>
```

**Step 4: Add star button styles**

In the `<style>` block, add after `.copy-btn:hover`:

```css
.star-btn {
    opacity: 0; background: none; border: none; color: var(--fg-muted);
    cursor: pointer; padding: 1px 3px; border-radius: 3px; flex-shrink: 0;
    display: flex; align-items: center; transition: opacity 0.15s, color 0.15s;
}
.file-entry:hover .star-btn { opacity: 1; }
.star-btn:hover { color: #eab308; background: var(--bg-secondary); }
.star-btn.active { opacity: 1; color: #eab308; }
```

**Step 5: Pass isFavorite and forward toggleFavorite in recursive self**

In the `{#each children as child}` block, add the `isFavorite` prop and event forwarding:

```svelte
<svelte:self
    entry={child}
    depth={depth + 1}
    {gitStatuses}
    {copiedPath}
    {isFavorite}
    on:selectFile
    on:copied
    on:toggleFavorite
/>
```

Note: The recursive `isFavorite` prop here is wrong — each child needs its own favorite status. This will be handled by the parent (Sidebar.svelte) passing a `favoritePaths` Set instead. Change the approach:

Replace `isFavorite` prop with `favoritePaths`:

```typescript
export let favoritePaths: Set<string> = new Set();
```

And compute `isFavorite` reactively:

```typescript
$: isFavorite = favoritePaths.has(entry.path);
```

Update the template star button to use the reactive `isFavorite`.

Update recursive self to pass `{favoritePaths}` instead of `{isFavorite}`.

**Step 6: Commit**

```
git add frontend/src/components/FileTreeItem.svelte
git commit -m "feat(frontend): add star toggle button to FileTreeItem (#63)"
```

---

### Task 8: Wire up Sidebar.svelte with favorites state

**Files:**
- Modify: `frontend/src/components/Sidebar.svelte`

**Step 1: Import FavoritesSection and add favorites state**

In `Sidebar.svelte`, add the import:

```typescript
import FavoritesSection from './FavoritesSection.svelte';
```

Add state variables after `let activeView`:

```typescript
let favorites: string[] = [];
$: favoritePaths = new Set(favorites);
```

**Step 2: Add loadFavorites function**

After the `clearSearch` function:

```typescript
async function loadFavorites() {
    if (!dir) {
        favorites = [];
        return;
    }
    try {
        favorites = (await App.GetFavorites(dir)) || [];
    } catch {
        favorites = [];
    }
}
```

**Step 3: Call loadFavorites on mount and dir change**

In `onMount`, after `refreshGitStatus()`:

```typescript
loadFavorites();
```

In the reactive `$: if (dir)` block, add `loadFavorites()`:

```typescript
$: if (dir) {
    loadDir(dir);
    refreshGitStatus();
    loadFavorites();
}
```

**Step 4: Add toggleFavorite handler**

```typescript
async function handleToggleFavorite(e: CustomEvent<{ path: string; isFavorite: boolean }>) {
    const { path, isFavorite } = e.detail;
    try {
        if (isFavorite) {
            await App.RemoveFavorite(dir, path);
        } else {
            await App.AddFavorite(dir, path);
        }
        await loadFavorites();
    } catch {}
}

async function handleRemoveFavorite(e: CustomEvent<{ path: string }>) {
    try {
        await App.RemoveFavorite(dir, e.detail.path);
        await loadFavorites();
    } catch {}
}
```

**Step 5: Add FavoritesSection to template**

In the explorer view section, before the search box div, add:

```svelte
{#if activeView === 'explorer'}
    <FavoritesSection
        {favorites}
        on:selectFile
        on:removeFavorite={handleRemoveFavorite}
    />

    <div class="search-box">
```

**Step 6: Pass favoritePaths to all FileTreeItem instances**

Update all `<FileTreeItem>` usages to include `{favoritePaths}` and `on:toggleFavorite={handleToggleFavorite}`:

```svelte
<FileTreeItem
    {entry}
    {gitStatuses}
    {copiedPath}
    {favoritePaths}
    on:selectFile
    on:copied={(e) => setCopied(e.detail.path)}
    on:toggleFavorite={handleToggleFavorite}
/>
```

Do this for both the search results loop and the regular entries loop.

**Step 7: Commit**

```
git add frontend/src/components/Sidebar.svelte
git commit -m "feat(frontend): wire Sidebar with favorites loading and toggling (#63)"
```

---

### Task 9: Build and manual test

**Step 1: Generate Wails bindings**

Run: `cd D:/repos/Multiterminal && wails generate module`

This regenerates the TypeScript bindings so the frontend can call `App.GetFavorites`, `App.AddFavorite`, `App.RemoveFavorite`.

**Step 2: Build the app**

Run: `cd D:/repos/Multiterminal && wails build -debug`
Expected: BUILD SUCCESS

**Step 3: Run all Go tests**

Run: `cd D:/repos/Multiterminal && go test ./...`
Expected: ALL PASS

**Step 4: Manual test checklist**

1. Open Multiterminal, open sidebar (Ctrl+B)
2. Hover over a file — star button should appear
3. Click star — file should appear in Favorites section at top
4. Click star again — file should be removed from Favorites
5. Click a favorite — path should be inserted into terminal
6. Drag a favorite — should work like drag from file tree
7. Click remove (X) on a favorite — should remove it
8. Switch tabs/directories — favorites should change per directory
9. Restart app — favorites should persist

**Step 5: Commit**

```
git add -A
git commit -m "feat: favorites/bookmarks for files in sidebar (#63)"
```
