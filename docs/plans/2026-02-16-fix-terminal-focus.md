# Fix Terminal Focus Problems — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Eliminate all focus-stealing bugs so users can interact with sidebar search, pane rename, terminal search, queue panel, and dialogs without the terminal yanking focus back.

**Architecture:** The root cause is a reactive statement in `TerminalPane.svelte:308` that calls `terminal.focus()` on every Svelte store change — not just focus changes. Every `updateActivity` event (every few seconds) re-triggers it. The fix adds a guard that skips `focus()` when an interactive element (input/textarea/select) already has focus. Secondary fixes address dialog focus management, session restore, and store consistency.

**Tech Stack:** Svelte 4, xterm.js, TypeScript

---

### Task 1: Guard the reactive focus statement in TerminalPane

**Files:**
- Modify: `frontend/src/components/TerminalPane.svelte:307-310`

**Step 1: Replace the reactive focus block**

Replace lines 307-310:
```svelte
  // Re-focus terminal when its tab becomes active again
  $: if (active && pane.focused && termInstance) {
    termInstance.terminal.focus();
  }
```

With:
```svelte
  // Re-focus terminal when its tab becomes active or pane gets focused.
  // Guard: skip if an interactive element (input, textarea, select) already
  // has focus — prevents stealing focus from sidebar search, pane rename,
  // terminal search, queue panel, etc.
  $: if (active && pane.focused && termInstance) {
    const ae = document.activeElement;
    const isInteractive = ae instanceof HTMLInputElement ||
                          ae instanceof HTMLTextAreaElement ||
                          ae instanceof HTMLSelectElement;
    if (!isInteractive) {
      termInstance.terminal.focus();
    }
  }
```

**Why this works:** The reactive block still fires on store changes, but now only calls `terminal.focus()` when no input field has focus. Tab switches still work (buttons are not interactive elements). Pane clicks still work (click target is a div). Activity updates no longer steal focus from inputs.

**Step 2: Manual test**

Build with `wails dev`, open sidebar search, type — focus should stay in search input even when activity updates arrive.

---

### Task 2: Fix dialog focus management (LaunchDialog + SettingsDialog)

**Files:**
- Modify: `frontend/src/components/LaunchDialog.svelte`
- Modify: `frontend/src/components/SettingsDialog.svelte`

**Problem:** When these dialogs open, the terminal still has focus. Keyboard events (e.g. pressing "1", "2", "3") go to BOTH the dialog's `svelte:window` handler AND the terminal. The dialog needs to grab focus to blur the terminal.

**Step 1: Fix LaunchDialog**

Add a `dialogEl` binding and auto-focus when visible. Change from `<svelte:window>` to dialog-level keydown.

In `<script>`:
```typescript
let dialogEl: HTMLDivElement;

$: if (visible) {
  requestAnimationFrame(() => dialogEl?.focus());
}
```

In template, change:
```svelte
<svelte:window on:keydown={visible ? handleKeydown : undefined} />
```
to remove the `<svelte:window>` line entirely.

Change the dialog div:
```svelte
<div class="dialog" on:click|stopPropagation>
```
to:
```svelte
<div class="dialog" on:click|stopPropagation bind:this={dialogEl} tabindex="-1" on:keydown={handleKeydown}>
```

Add to `.dialog` CSS:
```css
outline: none;
```

**Step 2: Fix SettingsDialog**

Same pattern. Add `dialogEl` binding, auto-focus, move keydown to dialog element.

In `<script>`:
```typescript
let dialogEl: HTMLDivElement;

$: if (visible) {
  requestAnimationFrame(() => dialogEl?.focus());
}
```

Remove `<svelte:window on:keydown={visible ? handleKeydown : undefined} />`.

Change dialog div to:
```svelte
<div class="dialog" on:click|stopPropagation bind:this={dialogEl} tabindex="-1" on:keydown={handleKeydown}>
```

Add `outline: none;` to `.dialog` CSS.

**Step 3: Manual test**

Open LaunchDialog (Ctrl+N) — pressing "1" should only trigger the dialog action, not send "1" to the terminal.

---

### Task 3: Fix closePane focus consistency

**Files:**
- Modify: `frontend/src/stores/tabs.ts:139-153`

**Problem:** When the focused pane is closed, `closePane()` sets `focused = true` on the next pane but does NOT set `focused = false` on all other panes. If any pane had an inconsistent `focused = true` state, it persists.

**Step 1: Fix closePane**

Replace the focus reassignment block inside `closePane`:
```typescript
if (tab.focusedPaneId === paneId && tab.panes.length > 0) {
  const newIdx = Math.min(idx, tab.panes.length - 1);
  tab.panes[newIdx].focused = true;
  tab.focusedPaneId = tab.panes[newIdx].id;
}
```

With:
```typescript
if (tab.focusedPaneId === paneId && tab.panes.length > 0) {
  const newIdx = Math.min(idx, tab.panes.length - 1);
  tab.panes.forEach((p) => (p.focused = false));
  tab.panes[newIdx].focused = true;
  tab.focusedPaneId = tab.panes[newIdx].id;
}
```

---

### Task 4: Restore focus_idx on session restore

**Files:**
- Modify: `frontend/src/lib/session.ts:10-28`

**Problem:** `focus_idx` is saved per tab (line 49) but never used during restore. The last-added pane always gets focus instead of the previously focused one.

**Step 1: Add focus restoration after pane loop**

After the inner `for` loop that adds panes, add:
```typescript
// Restore focused pane (addPane always focuses the last-added pane)
if (savedTab.focus_idx >= 0) {
  const state = tabStore.getState();
  const tab = state.tabs.find(t => t.id === tabId);
  if (tab && savedTab.focus_idx < tab.panes.length) {
    tabStore.focusPane(tabId, tab.panes[savedTab.focus_idx].id);
  }
}
```

This goes right after the closing `}` of `for (const savedPane of savedTab.panes)` and before the closing `}` of `for (const savedTab of saved.tabs)`.

---

### Task 5: Build, verify, and commit

**Step 1: Build**

```bash
cd D:/repos/Multiterminal && wails build
```

Expected: Build succeeds with no errors.

**Step 2: Commit**

```bash
git add frontend/src/components/TerminalPane.svelte \
       frontend/src/components/LaunchDialog.svelte \
       frontend/src/components/SettingsDialog.svelte \
       frontend/src/stores/tabs.ts \
       frontend/src/lib/session.ts
git commit -m "fix: prevent terminal from stealing focus from UI elements

Guard the reactive focus statement in TerminalPane to skip terminal.focus()
when an interactive element (input/textarea/select) already has focus.
Also fix dialog focus management, closePane consistency, and session
restore focus_idx."
```
