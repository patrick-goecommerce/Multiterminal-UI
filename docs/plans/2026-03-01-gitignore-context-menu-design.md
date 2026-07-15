# .gitignore Context Menu Design

**Date:** 2026-03-01
**Branch:** alpha-main
**Status:** Approved

## Overview

Add a right-click context menu to the Source Control view's file entries.
The single action "Zu .gitignore hinzufügen" appends the file's repo-relative
path to the repo root's `.gitignore`, creating the file if it does not exist.

## Backend

New method in `internal/backend/app_git.go`:

```go
func (a *AppService) AddToGitignore(dir, relPath string) error
```

Steps:
1. `git rev-parse --show-toplevel` → repo root
2. `.gitignore` path = `<repoRoot>/.gitignore`
3. Normalize `relPath`: backslashes → forward slashes
4. If file exists: read lines, check for exact duplicate → return nil if already present
5. Append entry + trailing newline (create file if missing)

Error cases: no git repo, file write failure → return error to frontend.

`app_git.go` stays under 300 lines after addition (~260 lines total).

## Frontend

Changes in `frontend/src/components/SourceControlView.svelte`:

**State:**
```ts
let contextMenu: { x: number; y: number; entry: ScEntry } | null = null;
let addedPath = '';   // analogous to copiedPath — brief feedback
```

**Trigger on `.sc-entry`:**
```svelte
on:contextmenu={(e) => { e.preventDefault(); contextMenu = { x: e.clientX, y: e.clientY, entry }; }}
```

**Overlay (at end of `.file-list`):**
```svelte
{#if contextMenu}
  <div class="ctx-backdrop" on:click={() => contextMenu = null}
       on:contextmenu|preventDefault on:keydown />
  <div class="ctx-menu" style="left:{contextMenu.x}px; top:{contextMenu.y}px">
    <button on:click={handleAddToGitignore}>Zu .gitignore hinzufügen</button>
  </div>
{/if}
```

**Handler:**
- Calls `AddToGitignore(dir, contextMenu.entry.relPath)`
- On success: sets `addedPath = entry.relPath`, clears after 1500 ms
- On error: logs to console (no crash)
- Always closes context menu

**Keyboard:** Escape closes the menu via a `keydown` listener added when menu opens, removed when it closes.

**Edge case — screen boundary:** `.ctx-menu` uses `position: fixed` so it stays in viewport. If `x + menuWidth > window.innerWidth`, shift left. Same for bottom edge. Handled in `handleAddToGitignore` open logic.

## Wails Binding

`AddToGitignore` is auto-generated into `wailsjs/go/backend/AppService.js` by Wails.
Frontend imports: `import { AddToGitignore } from '../../wailsjs/go/backend/AppService';`

## New Files

None.

## Changed Files

| File | Change |
|---|---|
| `internal/backend/app_git.go` | Add `AddToGitignore()` method |
| `frontend/src/components/SourceControlView.svelte` | Context menu state, overlay, handler, CSS |
