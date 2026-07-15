# Tab Status Indicators — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show a small colored dot on inactive tabs to indicate the aggregate Claude activity state of all panes in that tab; clear the dot when the tab is clicked.

**Architecture:** Add `unreadActivity` field to the `Tab` interface. A pure helper function `computeTabActivity` aggregates pane activities (priority: needsInput > active > done > null). `updateActivity()` calls it for non-active tabs; `setActiveTab()` clears it. `TabBar.svelte` renders a CSS dot based on this field.

**Tech Stack:** TypeScript, Svelte 4, Vitest (tests in `frontend/src/stores/tabs.test.ts`)

---

## Task 1: Add `unreadActivity` to Tab interface + helper + initialize in `addTab`

**Files:**
- Modify: `frontend/src/stores/tabs.ts:24-31` (Tab interface)
- Modify: `frontend/src/stores/tabs.ts:48-63` (addTab)
- Test: `frontend/src/stores/tabs.test.ts`

### Step 1: Write the failing test

Add to `frontend/src/stores/tabs.test.ts`, inside the `describe('addTab', ...)` block:

```typescript
it('initializes unreadActivity as null', () => {
  const id = tabStore.addTab('ActivityInit');
  const tab = tabStore.getState().tabs.find((t) => t.id === id);
  expect(tab!.unreadActivity).toBeNull();
});
```

### Step 2: Run to confirm it fails

```bash
cd /d/repos/Multiterminal/frontend && npx vitest run src/stores/tabs.test.ts
```

Expected: FAIL — `tab.unreadActivity` is `undefined` (field doesn't exist yet).

### Step 3: Implement

**In `frontend/src/stores/tabs.ts`:**

1. Add `unreadActivity` to the `Tab` interface (after `_highlight?: boolean`):

```typescript
export interface Tab {
  id: string;
  name: string;
  dir: string;
  panes: Pane[];
  focusedPaneId: string;
  _highlight?: boolean;
  unreadActivity: 'needsInput' | 'active' | 'done' | null;
}
```

2. Add the helper function **before** `function createTabStore()`:

```typescript
export function computeTabActivity(panes: Pane[]): Tab['unreadActivity'] {
  let result: Tab['unreadActivity'] = null;
  for (const pane of panes) {
    if (pane.activity === 'needsInput') return 'needsInput';
    if (pane.activity === 'active') result = 'active';
    else if (pane.activity === 'done' && result === null) result = 'done';
  }
  return result;
}
```

3. In `addTab()`, add `unreadActivity: null` to the pushed object:

```typescript
state.tabs.push({
  id,
  name: tabName,
  dir: dir || '',
  panes: [],
  focusedPaneId: '',
  unreadActivity: null,     // ← add this line
});
```

### Step 4: Run to confirm it passes

```bash
cd /d/repos/Multiterminal/frontend && npx vitest run src/stores/tabs.test.ts
```

Expected: all tests PASS.

### Step 5: Commit

```bash
cd /d/repos/Multiterminal
git add frontend/src/stores/tabs.ts frontend/src/stores/tabs.test.ts
git commit -m "feat(tabs): add unreadActivity field to Tab interface"
```

---

## Task 2: Implement aggregate logic in `updateActivity()`

**Files:**
- Modify: `frontend/src/stores/tabs.ts:191-204`
- Test: `frontend/src/stores/tabs.test.ts`

### Step 1: Write the failing tests

Add a new `describe` block to `tabs.test.ts`:

```typescript
describe('computeTabActivity', () => {
  it('returns null for all-idle panes', () => {
    expect(computeTabActivity([])).toBeNull();
  });

  it('returns done when all panes are done', () => {
    const panes = [
      { activity: 'done' } as any,
      { activity: 'idle' } as any,
    ];
    expect(computeTabActivity(panes)).toBe('done');
  });

  it('returns active when any pane is active', () => {
    const panes = [
      { activity: 'done' } as any,
      { activity: 'active' } as any,
    ];
    expect(computeTabActivity(panes)).toBe('active');
  });

  it('returns needsInput when any pane needs input (highest priority)', () => {
    const panes = [
      { activity: 'active' } as any,
      { activity: 'needsInput' } as any,
      { activity: 'done' } as any,
    ];
    expect(computeTabActivity(panes)).toBe('needsInput');
  });
});

describe('updateActivity — tab unreadActivity', () => {
  it('sets unreadActivity on non-active tab when pane becomes done', () => {
    const tab1 = tabStore.addTab('UAActive');
    const tab2 = tabStore.addTab('UABackground');  // becomes active
    tabStore.setActiveTab(tab1);                    // tab1 is now active

    tabStore.addPane(tab2, 3001, 'Claude', 'claude', '');
    tabStore.updateActivity(3001, 'done', '$0.10');

    const t2 = tabStore.getState().tabs.find((t) => t.id === tab2);
    expect(t2!.unreadActivity).toBe('done');
  });

  it('does not set unreadActivity on the currently active tab', () => {
    const tabId = tabStore.addTab('UAActiveTab');
    tabStore.setActiveTab(tabId);
    tabStore.addPane(tabId, 3002, 'Claude', 'claude', '');

    tabStore.updateActivity(3002, 'done', '');

    const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
    expect(tab!.unreadActivity).toBeNull();
  });

  it('escalates to needsInput when one pane needs input', () => {
    const bgTab = tabStore.addTab('UAEscalate');
    const fgTab = tabStore.addTab('UAForeground');
    tabStore.setActiveTab(fgTab);

    tabStore.addPane(bgTab, 3003, 'C1', 'claude', '');
    tabStore.addPane(bgTab, 3004, 'C2', 'claude', '');

    tabStore.updateActivity(3003, 'done', '');
    tabStore.updateActivity(3004, 'needsInput', '');

    const tab = tabStore.getState().tabs.find((t) => t.id === bgTab);
    expect(tab!.unreadActivity).toBe('needsInput');
  });
});
```

Also add the import at the top of `tabs.test.ts`:
```typescript
import { tabStore, activeTab, allTabs, computeTabActivity } from './tabs';
```

### Step 2: Run to confirm tests fail

```bash
cd /d/repos/Multiterminal/frontend && npx vitest run src/stores/tabs.test.ts
```

Expected: `computeTabActivity` is not exported / `unreadActivity` not updated.

### Step 3: Implement

Replace `updateActivity()` in `frontend/src/stores/tabs.ts`:

```typescript
updateActivity(sessionId: number, activity: string, cost: string) {
  update((state) => {
    for (const tab of state.tabs) {
      for (const pane of tab.panes) {
        if (pane.sessionId === sessionId) {
          pane.activity = activity as Pane['activity'];
          if (cost) pane.cost = cost;
          if (tab.id !== state.activeTabId) {
            tab.unreadActivity = computeTabActivity(tab.panes);
          }
          return state;
        }
      }
    }
    return state;
  });
},
```

### Step 4: Run to confirm all tests pass

```bash
cd /d/repos/Multiterminal/frontend && npx vitest run src/stores/tabs.test.ts
```

Expected: all tests PASS.

### Step 5: Commit

```bash
cd /d/repos/Multiterminal
git add frontend/src/stores/tabs.ts frontend/src/stores/tabs.test.ts
git commit -m "feat(tabs): compute unreadActivity on background tab activity change"
```

---

## Task 3: Clear `unreadActivity` on tab click in `setActiveTab()`

**Files:**
- Modify: `frontend/src/stores/tabs.ts:78-83`
- Test: `frontend/src/stores/tabs.test.ts`

### Step 1: Write the failing test

Add to the `describe('setActiveTab', ...)` block:

```typescript
it('clears unreadActivity when tab becomes active', () => {
  const bgTab = tabStore.addTab('ClearBg');
  const fgTab = tabStore.addTab('ClearFg');
  tabStore.setActiveTab(fgTab);  // fgTab active, bgTab background

  tabStore.addPane(bgTab, 4001, 'Claude', 'claude', '');
  tabStore.updateActivity(4001, 'needsInput', '');

  // bgTab should now have unreadActivity set
  let tab = tabStore.getState().tabs.find((t) => t.id === bgTab);
  expect(tab!.unreadActivity).toBe('needsInput');

  // Switch to bgTab → should clear
  tabStore.setActiveTab(bgTab);
  tab = tabStore.getState().tabs.find((t) => t.id === bgTab);
  expect(tab!.unreadActivity).toBeNull();
});
```

### Step 2: Run to confirm it fails

```bash
cd /d/repos/Multiterminal/frontend && npx vitest run src/stores/tabs.test.ts
```

Expected: FAIL — `unreadActivity` is still `'needsInput'` after `setActiveTab`.

### Step 3: Implement

Replace `setActiveTab()` in `frontend/src/stores/tabs.ts`:

```typescript
setActiveTab(tabId: string) {
  update((state) => {
    state.activeTabId = tabId;
    const tab = state.tabs.find((t) => t.id === tabId);
    if (tab) tab.unreadActivity = null;
    return state;
  });
},
```

### Step 4: Run to confirm all tests pass

```bash
cd /d/repos/Multiterminal/frontend && npx vitest run src/stores/tabs.test.ts
```

Expected: all tests PASS.

### Step 5: Commit

```bash
cd /d/repos/Multiterminal
git add frontend/src/stores/tabs.ts frontend/src/stores/tabs.test.ts
git commit -m "feat(tabs): clear unreadActivity when tab is activated"
```

---

## Task 4: Render the status dot in `TabBar.svelte`

**Files:**
- Modify: `frontend/src/components/TabBar.svelte`

No automated test for this task — verify visually with `wails dev`.

### Step 1: Add the dot element

In `TabBar.svelte`, after line 163 (`{#if tab.panes.length > 0} ... {/if}`), add:

```svelte
{#if tab.unreadActivity}
  <span class="tab-activity-dot tab-dot-{tab.unreadActivity}"></span>
{/if}
```

Full updated tab button content (lines 161–168 area):

```svelte
<span class="tab-name">{tab.name}</span>
{#if tab.panes.length > 0}
  <span class="tab-count">{tab.panes.length}</span>
{/if}
{#if tab.unreadActivity}
  <span class="tab-activity-dot tab-dot-{tab.unreadActivity}"></span>
{/if}
<button class="tab-close" on:click={(e) => handleCloseTab(e, tab.id)}>
  &times;
</button>
```

### Step 2: Add CSS

In the `<style>` section of `TabBar.svelte`, append after the last rule (`.ctx-separator`):

```css
.tab-activity-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
  display: inline-block;
}

.tab-dot-done {
  background: #22c55e;
  box-shadow: 0 0 4px #22c55e88;
}

.tab-dot-needsInput {
  background: #ef4444;
  animation: tab-dot-pulse 1s ease-in-out infinite;
}

.tab-dot-active {
  background: var(--accent);
  animation: tab-dot-slow-pulse 2s ease-in-out infinite;
}

@keyframes tab-dot-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50%       { opacity: 0.5; transform: scale(0.75); }
}

@keyframes tab-dot-slow-pulse {
  0%, 100% { opacity: 0.8; }
  50%       { opacity: 0.25; }
}
```

### Step 3: Visual verification

```bash
cd /d/repos/Multiterminal && wails dev
```

Checklist:
- [ ] Start Claude in Tab 2, switch to Tab 1 → Tab 2 shows accent-colored pulsing dot while active
- [ ] Claude finishes in Tab 2 → Tab 2 shows green static dot
- [ ] Claude waits for input in Tab 2 → Tab 2 shows red pulsing dot
- [ ] Click Tab 2 → dot disappears immediately
- [ ] Switch back to Tab 1 → Tab 2 dot reappears only when new activity happens

### Step 4: Commit

```bash
cd /d/repos/Multiterminal
git add frontend/src/components/TabBar.svelte
git commit -m "feat(tabbar): show unreadActivity dot on inactive tabs"
```

---

## Final: Full test run + push

```bash
cd /d/repos/Multiterminal/frontend && npm test
```

Expected: all tests pass.

```bash
cd /d/repos/Multiterminal && git push
```
