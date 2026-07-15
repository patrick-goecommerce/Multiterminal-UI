# Session Dashboard Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a permanent "Home" dashboard view (house icon, always first in the tab bar) that shows ALL panes from ALL tabs as cards in swim-lane columns grouped by activity status — giving a global overview of every running agent/terminal across all projects.

**Architecture:** Pure frontend change, no backend required. A `groupPanesByActivity` helper (tested) groups pane data from the existing `allTabs` store. `App.svelte` gets a `showDashboard` boolean; the TabBar emits `on:showDashboard`. `DashboardView.svelte` reads `$allTabs` reactively and renders swim-lane cards. Clicking a card navigates to the tab + focuses the pane.

**Card design (inspired by SlayZone screenshot):**
- If pane has `issueNumber`: issue title is the **primary bold label** (e.g. `#42 Auth-Refactor`)
- Otherwise: pane name is the primary label
- Status dot + status text in same row (like SlayZone: `• working` / `• attention`)
- Tab/project name shown as secondary line
- Branch tag + cost in the card footer
- Currently focused pane gets accent border highlight
- Active card (from `$activeTab`) gets subtle background tint

**Dashboard header (like SlayZone "project: api-v2 — 14 tasks"):**
- Shows: active project name (`$activeTab?.name`) + dir snippet
- Shows: total session count + total cost across all tabs

**Tech Stack:** Svelte 4, TypeScript, existing `tabStore`/`allTabs`, Vitest (tests in `frontend/src/lib/dashboard.test.ts`)

**Branch context:** We are on `feat/tab-status-indicators`. The `tabs.ts` store already has the full activity type (`idle | active | done | waitingPermission | waitingAnswer | error`) and `computeTabActivity` — the dashboard builds directly on that work. Do NOT modify `tabs.ts` or `tabs.test.ts`.

---

## Task 1: `groupPanesByActivity` helper + tests

**Files:**
- Create: `frontend/src/lib/dashboard.ts`
- Create: `frontend/src/lib/dashboard.test.ts`

### Step 1: Write the failing tests

Create `frontend/src/lib/dashboard.test.ts`:

```typescript
import { describe, it, expect } from 'vitest';
import { groupPanesByActivity } from './dashboard';
import type { Tab, Pane } from '../stores/tabs';

function makePane(overrides: Partial<Pane>): Pane {
  return {
    id: 'p1', sessionId: 1, name: 'Claude', mode: 'claude', model: '',
    focused: false, activity: 'idle', cost: '', running: true, maximized: false,
    issueNumber: null, issueTitle: '', issueBranch: '', worktreePath: '',
    branch: 'main', zoomDelta: 0,
    ...overrides,
  };
}

function makeTab(id: string, name: string, panes: Pane[]): Tab {
  return { id, name, dir: '/proj', panes, focusedPaneId: '', unreadActivity: null };
}

describe('groupPanesByActivity', () => {
  it('returns empty groups for no tabs', () => {
    const groups = groupPanesByActivity([]);
    expect(groups.needsAttention).toEqual([]);
    expect(groups.active).toEqual([]);
    expect(groups.done).toEqual([]);
    expect(groups.idle).toEqual([]);
  });

  it('groups waitingPermission into needsAttention', () => {
    const tab = makeTab('t1', 'Auth', [makePane({ id: 'p1', activity: 'waitingPermission' })]);
    const groups = groupPanesByActivity([tab]);
    expect(groups.needsAttention).toHaveLength(1);
    expect(groups.needsAttention[0].tabName).toBe('Auth');
  });

  it('groups waitingAnswer into needsAttention', () => {
    const tab = makeTab('t1', 'API', [makePane({ id: 'p1', activity: 'waitingAnswer' })]);
    const groups = groupPanesByActivity([tab]);
    expect(groups.needsAttention).toHaveLength(1);
  });

  it('groups error into needsAttention', () => {
    const tab = makeTab('t1', 'Test', [makePane({ id: 'p1', activity: 'error' })]);
    const groups = groupPanesByActivity([tab]);
    expect(groups.needsAttention).toHaveLength(1);
  });

  it('groups active into active', () => {
    const tab = makeTab('t1', 'Frontend', [makePane({ id: 'p1', activity: 'active' })]);
    const groups = groupPanesByActivity([tab]);
    expect(groups.active).toHaveLength(1);
    expect(groups.active[0].tabId).toBe('t1');
  });

  it('groups done into done', () => {
    const tab = makeTab('t1', 'Backend', [makePane({ id: 'p1', activity: 'done' })]);
    const groups = groupPanesByActivity([tab]);
    expect(groups.done).toHaveLength(1);
  });

  it('groups idle into idle', () => {
    const tab = makeTab('t1', 'Shell', [makePane({ id: 'p1', activity: 'idle', mode: 'shell' })]);
    const groups = groupPanesByActivity([tab]);
    expect(groups.idle).toHaveLength(1);
  });

  it('attaches tabId and tabName to each pane', () => {
    const tab = makeTab('tab-42', 'My Project', [makePane({ id: 'p99', activity: 'done' })]);
    const groups = groupPanesByActivity([tab]);
    expect(groups.done[0].tabId).toBe('tab-42');
    expect(groups.done[0].tabName).toBe('My Project');
    expect(groups.done[0].id).toBe('p99');
  });

  it('handles multiple tabs with mixed panes', () => {
    const tabs = [
      makeTab('t1', 'Auth', [
        makePane({ id: 'p1', activity: 'waitingPermission' }),
        makePane({ id: 'p2', activity: 'active' }),
      ]),
      makeTab('t2', 'API', [
        makePane({ id: 'p3', activity: 'done' }),
        makePane({ id: 'p4', activity: 'idle' }),
      ]),
    ];
    const groups = groupPanesByActivity(tabs);
    expect(groups.needsAttention).toHaveLength(1);
    expect(groups.active).toHaveLength(1);
    expect(groups.done).toHaveLength(1);
    expect(groups.idle).toHaveLength(1);
  });
});
```

### Step 2: Run to confirm it fails

```bash
cd /d/repos/Multiterminal/frontend && npx vitest run src/lib/dashboard.test.ts
```

Expected: FAIL — `groupPanesByActivity` not found.

### Step 3: Implement the helper

Create `frontend/src/lib/dashboard.ts`:

```typescript
import type { Tab, Pane } from '../stores/tabs';

export type PaneWithContext = Pane & {
  tabId: string;
  tabName: string;
};

export type ActivityGroups = {
  needsAttention: PaneWithContext[]; // waitingPermission | waitingAnswer | error
  active: PaneWithContext[];         // active
  done: PaneWithContext[];           // done
  idle: PaneWithContext[];           // idle (including shell panes, stopped)
};

export function groupPanesByActivity(tabs: Tab[]): ActivityGroups {
  const groups: ActivityGroups = {
    needsAttention: [],
    active: [],
    done: [],
    idle: [],
  };

  for (const tab of tabs) {
    for (const pane of tab.panes) {
      const ctx: PaneWithContext = { ...pane, tabId: tab.id, tabName: tab.name };
      if (
        pane.activity === 'waitingPermission' ||
        pane.activity === 'waitingAnswer' ||
        pane.activity === 'error'
      ) {
        groups.needsAttention.push(ctx);
      } else if (pane.activity === 'active') {
        groups.active.push(ctx);
      } else if (pane.activity === 'done') {
        groups.done.push(ctx);
      } else {
        groups.idle.push(ctx);
      }
    }
  }

  return groups;
}
```

### Step 4: Run to confirm it passes

```bash
cd /d/repos/Multiterminal/frontend && npx vitest run src/lib/dashboard.test.ts
```

Expected: all PASS.

### Step 5: Commit

```bash
cd /d/repos/Multiterminal
git add frontend/src/lib/dashboard.ts frontend/src/lib/dashboard.test.ts
git commit -m "feat(dashboard): add groupPanesByActivity helper with tests"
```

---

## Task 2: TabBar — add house icon as the first item

**Files:**
- Modify: `frontend/src/components/TabBar.svelte`

The house button sits to the LEFT of all tabs. It is visually styled like a tab button but with a house icon (🏠 or SVG). It accepts an `isDashboard` prop so the parent can mark it as active.

### Step 1: Add prop + house button markup

Open `frontend/src/components/TabBar.svelte`.

**Add the prop** at the top of the `<script>` section (after existing exports):

```svelte
export let isDashboard: boolean = false;
```

**Add the house button** in the `<div class="tabs">` block, BEFORE the `{#each $allTabs ...}` loop:

```svelte
<button
  class="tab tab-home"
  class:active={isDashboard}
  title="Dashboard (Ctrl+Shift+H)"
  on:click={() => dispatch('showDashboard')}
>
  <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
    <path d="M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z"/>
  </svg>
</button>
```

### Step 2: Add CSS for the home tab

In the `<style>` block, add after `.tab` styles:

```css
.tab-home {
  min-width: unset;
  padding: 12px 14px;
}
```

### Step 3: TypeScript check

```bash
cd /d/repos/Multiterminal/frontend && npx tsc --noEmit 2>&1 | head -20
```

Expected: no errors.

### Step 4: Commit

```bash
cd /d/repos/Multiterminal
git add frontend/src/components/TabBar.svelte
git commit -m "feat(tabbar): add house icon dashboard button as first tab item"
```

---

## Task 3: DashboardView.svelte — the main component

**Files:**
- Create: `frontend/src/components/DashboardView.svelte`

This component reads `$allTabs` from the store, groups panes using `groupPanesByActivity`, and renders four swim-lane columns. Each card is clickable and dispatches `on:navigate`.

### Step 1: Create the component

The card design follows SlayZone's pattern: **issue title as primary label** when linked, minimal but informative layout, `• status` line at the bottom of each card.

Create `frontend/src/components/DashboardView.svelte`:

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { allTabs, activeTab } from '../stores/tabs';
  import { groupPanesByActivity } from '../lib/dashboard';
  import type { PaneWithContext } from '../lib/dashboard';

  const dispatch = createEventDispatcher<{
    navigate: { tabId: string; paneId: string };
  }>();

  $: groups = groupPanesByActivity($allTabs);
  $: totalPanes = $allTabs.reduce((n, t) => n + t.panes.length, 0);
  $: totalCost = (() => {
    let sum = 0;
    for (const tab of $allTabs) {
      for (const pane of tab.panes) {
        if (pane.cost) {
          const v = parseFloat(pane.cost.replace('$', ''));
          if (!isNaN(v)) sum += v;
        }
      }
    }
    return sum > 0 ? `$${sum.toFixed(2)}` : '';
  })();

  // Derive the "active project" label like SlayZone's "project: api-v2 — 14 tasks"
  $: activeProjectName = $activeTab?.name ?? '';
  $: activeProjectDir = (() => {
    const d = $activeTab?.dir ?? '';
    // Show only the last path segment
    return d.split(/[/\\]/).filter(Boolean).pop() ?? d;
  })();

  function handleCardClick(pane: PaneWithContext) {
    dispatch('navigate', { tabId: pane.tabId, paneId: pane.id });
  }

  // Returns the primary display label for a card (issue title if linked, else pane name)
  function cardTitle(pane: PaneWithContext): string {
    if (pane.issueNumber && pane.issueTitle) return `#${pane.issueNumber} ${pane.issueTitle}`;
    if (pane.issueNumber) return `#${pane.issueNumber}`;
    return pane.name;
  }

  function statusDotClass(activity: string): string {
    switch (activity) {
      case 'waitingPermission': return 'dot-attention';
      case 'waitingAnswer':     return 'dot-attention';
      case 'error':             return 'dot-error';
      case 'active':            return 'dot-active';
      case 'done':              return 'dot-done';
      default:                  return 'dot-idle';
    }
  }

  function statusLabel(activity: string): string {
    switch (activity) {
      case 'waitingPermission': return 'attention';
      case 'waitingAnswer':     return 'attention';
      case 'error':             return 'error';
      case 'active':            return 'working';
      case 'done':              return 'done';
      default:                  return 'idle';
    }
  }

  function modeLabel(mode: string): string {
    switch (mode) {
      case 'claude':      return 'claude';
      case 'claude-yolo': return 'yolo';
      default:            return 'shell';
    }
  }

  // A pane is "focused" if it's in the active tab AND is the focused pane of that tab
  function isFocused(pane: PaneWithContext): boolean {
    const tab = $allTabs.find(t => t.id === pane.tabId);
    return tab?.focusedPaneId === pane.id && tab?.id === $activeTab?.id;
  }
</script>

<div class="dashboard">
  <!-- Header: like SlayZone "project: api-v2 — 14 tasks" -->
  <header class="dash-header">
    <div class="dash-left">
      <span class="dash-app">multiterminal</span>
      {#if activeProjectName}
        <span class="dash-sep">—</span>
        <span class="dash-project">
          {#if activeProjectDir && activeProjectDir !== activeProjectName}
            <span class="dash-project-dir">{activeProjectDir}</span>
          {:else}
            <span class="dash-project-dir">{activeProjectName}</span>
          {/if}
        </span>
      {/if}
      <span class="dash-sep">—</span>
      <span class="dash-stats">
        {totalPanes} session{totalPanes !== 1 ? 's' : ''}
        {#if totalCost}<span class="dash-cost">{totalCost}</span>{/if}
      </span>
    </div>
    <div class="dash-tabs-count">{$allTabs.length} tab{$allTabs.length !== 1 ? 's' : ''}</div>
  </header>

  <!-- Swim lanes: ATTENTION | IN PROGRESS | DONE | IDLE -->
  <div class="swim-lanes">

    <!-- Attention: waitingPermission + waitingAnswer + error -->
    <div class="lane">
      <div class="lane-header">
        <span class="lane-dot dot-attention-hdr"></span>
        ATTENTION
        <span class="lane-count">{groups.needsAttention.length}</span>
      </div>
      <div class="lane-cards">
        {#each groups.needsAttention as pane (pane.id)}
          {@const focused = isFocused(pane)}
          <button class="card card-attention" class:card-focused={focused} on:click={() => handleCardClick(pane)}>
            <div class="card-title">{cardTitle(pane)}</div>
            {#if pane.issueNumber}
              <div class="card-pane-name">{pane.name}</div>
            {/if}
            <div class="card-project">{pane.tabName}</div>
            <div class="card-footer">
              {#if pane.branch}
                <span class="card-branch">⎇ {pane.branch}</span>
              {/if}
              {#if pane.cost}
                <span class="card-cost">{pane.cost}</span>
              {/if}
              <span class="card-status-row">
                <span class="dot {statusDotClass(pane.activity)}"></span>
                <span class="status-text status-{pane.activity}">{statusLabel(pane.activity)}</span>
              </span>
            </div>
          </button>
        {/each}
        {#if groups.needsAttention.length === 0}
          <div class="lane-empty">–</div>
        {/if}
      </div>
    </div>

    <!-- In Progress: active -->
    <div class="lane">
      <div class="lane-header">
        <span class="lane-dot dot-active-hdr"></span>
        IN PROGRESS
        <span class="lane-count">{groups.active.length}</span>
      </div>
      <div class="lane-cards">
        {#each groups.active as pane (pane.id)}
          {@const focused = isFocused(pane)}
          <button class="card card-active" class:card-focused={focused} on:click={() => handleCardClick(pane)}>
            <div class="card-title">{cardTitle(pane)}</div>
            {#if pane.issueNumber}
              <div class="card-pane-name">{pane.name}</div>
            {/if}
            <div class="card-project">{pane.tabName}</div>
            <div class="card-footer">
              {#if pane.branch}
                <span class="card-branch">⎇ {pane.branch}</span>
              {/if}
              {#if pane.cost}
                <span class="card-cost">{pane.cost}</span>
              {/if}
              <span class="card-status-row">
                <span class="dot dot-active"></span>
                <span class="status-text status-active">working</span>
              </span>
            </div>
          </button>
        {/each}
        {#if groups.active.length === 0}
          <div class="lane-empty">–</div>
        {/if}
      </div>
    </div>

    <!-- Done -->
    <div class="lane">
      <div class="lane-header">
        <span class="lane-dot dot-done-hdr"></span>
        DONE
        <span class="lane-count">{groups.done.length}</span>
      </div>
      <div class="lane-cards">
        {#each groups.done as pane (pane.id)}
          {@const focused = isFocused(pane)}
          <button class="card card-done" class:card-focused={focused} on:click={() => handleCardClick(pane)}>
            <div class="card-title">{cardTitle(pane)}</div>
            {#if pane.issueNumber}
              <div class="card-pane-name">{pane.name}</div>
            {/if}
            <div class="card-project">{pane.tabName}</div>
            <div class="card-footer">
              {#if pane.branch}
                <span class="card-branch">⎇ {pane.branch}</span>
              {/if}
              {#if pane.cost}
                <span class="card-cost">{pane.cost}</span>
              {/if}
              <span class="card-status-row">
                <span class="dot dot-done"></span>
                <span class="status-text status-done">done</span>
              </span>
            </div>
          </button>
        {/each}
        {#if groups.done.length === 0}
          <div class="lane-empty">–</div>
        {/if}
      </div>
    </div>

    <!-- Idle -->
    <div class="lane">
      <div class="lane-header">
        <span class="lane-dot dot-idle-hdr"></span>
        IDLE
        <span class="lane-count">{groups.idle.length}</span>
      </div>
      <div class="lane-cards">
        {#each groups.idle as pane (pane.id)}
          {@const focused = isFocused(pane)}
          <button class="card card-idle" class:card-focused={focused} on:click={() => handleCardClick(pane)}>
            <div class="card-title card-title-muted">{cardTitle(pane)}</div>
            <div class="card-project">{pane.tabName}</div>
            <div class="card-footer">
              {#if pane.branch}
                <span class="card-branch">⎇ {pane.branch}</span>
              {/if}
              {#if pane.cost}
                <span class="card-cost">{pane.cost}</span>
              {/if}
              <span class="card-status-row">
                <span class="status-text status-idle">{modeLabel(pane.mode)}</span>
              </span>
            </div>
          </button>
        {/each}
        {#if groups.idle.length === 0}
          <div class="lane-empty">–</div>
        {/if}
      </div>
    </div>

  </div>
</div>

<style>
  .dashboard {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--bg);
    overflow: hidden;
  }

  /* ── Header ─────────────────────────────── */
  .dash-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 20px;
    border-bottom: 1px solid var(--border);
    background: var(--bg-secondary);
    font-size: 13px;
    color: var(--fg-muted);
    flex-shrink: 0;
  }

  .dash-left {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .dash-app {
    font-weight: 600;
    color: var(--fg);
    letter-spacing: 0.02em;
  }

  .dash-sep { opacity: 0.4; }

  .dash-project-dir {
    color: var(--accent);
    font-weight: 500;
  }

  .dash-stats { display: flex; align-items: center; gap: 6px; }

  .dash-cost {
    color: var(--fg-muted);
    background: var(--bg-tertiary);
    padding: 1px 6px;
    border-radius: 8px;
    font-size: 11px;
  }

  .dash-tabs-count { font-size: 12px; opacity: 0.5; }

  /* ── Swim lanes ──────────────────────────── */
  .swim-lanes {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1px;
    flex: 1;
    overflow: hidden;
    background: var(--border);
  }

  .lane {
    display: flex;
    flex-direction: column;
    background: var(--bg);
    overflow: hidden;
  }

  .lane-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 14px 9px;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: var(--fg-muted);
    border-bottom: 1px solid var(--border);
    background: var(--bg-secondary);
    flex-shrink: 0;
  }

  .lane-count {
    margin-left: auto;
    font-size: 11px;
    opacity: 0.6;
  }

  .lane-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .dot-attention-hdr { background: #f5a623; }
  .dot-active-hdr    { background: var(--accent); }
  .dot-done-hdr      { background: #22c55e; }
  .dot-idle-hdr      { background: var(--fg-muted); opacity: 0.3; }

  .lane-cards {
    flex: 1;
    overflow-y: auto;
    padding: 10px 10px 20px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .lane-empty {
    padding: 20px 14px;
    text-align: center;
    color: var(--fg-muted);
    font-size: 13px;
    opacity: 0.3;
  }

  /* ── Cards ───────────────────────────────── */
  .card {
    display: flex;
    flex-direction: column;
    gap: 5px;
    padding: 11px 13px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 7px;
    cursor: pointer;
    text-align: left;
    width: 100%;
    transition: background 0.1s, border-color 0.1s, transform 0.1s;
  }

  .card:hover {
    background: var(--bg-tertiary);
    transform: translateY(-1px);
  }

  /* Focused pane: accent top border like SlayZone's highlighted card */
  .card-focused {
    border-top: 2px solid var(--accent);
  }

  /* Left accent stripe per status */
  .card-attention { border-left: 3px solid #f5a623; }
  .card-active    { border-left: 3px solid var(--accent); }
  .card-done      { border-left: 3px solid #22c55e; }
  .card-idle      { border-left: 3px solid transparent; opacity: 0.7; }

  /* Primary label — issue title or pane name */
  .card-title {
    font-size: 13px;
    font-weight: 600;
    color: var(--fg);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    line-height: 1.3;
  }

  .card-title-muted { color: var(--fg-muted); font-weight: 500; }

  /* Secondary pane name (when issue title is primary) */
  .card-pane-name {
    font-size: 11px;
    color: var(--fg-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* Project / tab name — clearly visible, slightly highlighted */
  .card-project {
    font-size: 11px;
    font-weight: 500;
    color: var(--accent);
    opacity: 0.75;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* Footer row: branch · cost · status */
  .card-footer {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    margin-top: 2px;
  }

  .card-branch {
    font-size: 10px;
    color: var(--fg-muted);
    background: var(--bg-tertiary);
    padding: 1px 5px;
    border-radius: 4px;
    white-space: nowrap;
  }

  .card-cost {
    font-size: 11px;
    color: var(--fg-muted);
    font-variant-numeric: tabular-nums;
  }

  /* Status dot + text row (SlayZone style: "• working") */
  .card-status-row {
    display: flex;
    align-items: center;
    gap: 5px;
    margin-left: auto;
  }

  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .dot-attention { background: #f5a623; animation: pulse 1s ease-in-out infinite; }
  .dot-error     { background: #e05252; }
  .dot-active    { background: var(--accent); animation: slow-pulse 2s ease-in-out infinite; }
  .dot-done      { background: #22c55e; }
  .dot-idle      { background: var(--fg-muted); opacity: 0.3; }

  @keyframes pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50%       { opacity: 0.5; transform: scale(0.75); }
  }
  @keyframes slow-pulse {
    0%, 100% { opacity: 0.8; }
    50%       { opacity: 0.2; }
  }

  .status-text {
    font-size: 11px;
    font-weight: 500;
  }

  .status-waitingPermission,
  .status-waitingAnswer { color: #f5a623; }
  .status-error         { color: #e05252; }
  .status-active        { color: var(--accent); }
  .status-done          { color: #22c55e; }
  .status-idle          { color: var(--fg-muted); opacity: 0.6; }
</style>
```

### Step 2: TypeScript check

```bash
cd /d/repos/Multiterminal/frontend && npx tsc --noEmit 2>&1 | head -20
```

Expected: no errors.

### Step 3: Commit

```bash
cd /d/repos/Multiterminal
git add frontend/src/components/DashboardView.svelte
git commit -m "feat(dashboard): add DashboardView swim-lane component"
```

---

## Task 4: Wire everything into App.svelte

**Files:**
- Modify: `frontend/src/App.svelte`

### Step 1: Import DashboardView

At the top of `<script>`, add the import after the other component imports:

```typescript
import DashboardView from './components/DashboardView.svelte';
```

### Step 2: Add `showDashboard` state variable

In the `<script>` section, add after the other `let` declarations (near `showLaunchDialog`):

```typescript
let showDashboard = false;
```

### Step 3: Add dashboard navigation handler

Add this function in the `<script>` section:

```typescript
function handleDashboardNavigate(e: CustomEvent<{ tabId: string; paneId: string }>) {
  const { tabId, paneId } = e.detail;
  showDashboard = false;
  tabStore.setActiveTab(tabId);
  tabStore.focusPane(tabId, paneId);
}
```

### Step 4: Handle setActiveTab closing dashboard

The existing `tabStore.setActiveTab` calls should close the dashboard. Update the `handleTabClick` flow in App.svelte:

The TabBar already calls `tabStore.setActiveTab` internally. We need to close the dashboard whenever a regular tab is activated. Add a reactive statement:

```typescript
$: if ($activeTab) showDashboard = false;
```

Place this near the other reactive statements (`$: totalCost = ...`, etc.).

### Step 5: Update the TabBar binding in the template

Find the `<TabBar>` line in the template (currently line 553):

```svelte
<TabBar activeTabId={$activeTab?.id ?? ''} on:addTab={() => (showProjectDialog = true)} />
```

Replace with:

```svelte
<TabBar
  activeTabId={$activeTab?.id ?? ''}
  isDashboard={showDashboard}
  on:addTab={() => (showProjectDialog = true)}
  on:showDashboard={() => { showDashboard = true; }}
/>
```

### Step 6: Add DashboardView rendering

In the template, find the `<div class="content">` block. The current structure is:

```svelte
<div class="content">
  <Sidebar ... />
  <div class="tab-layers">
    {#each $allTabs as tab (tab.id)}
      ...
    {/each}
  </div>
</div>
```

Replace `<div class="tab-layers">...</div>` with:

```svelte
{#if showDashboard}
  <DashboardView on:navigate={handleDashboardNavigate} />
{:else}
  <div class="tab-layers">
    {#each $allTabs as tab (tab.id)}
      <div class="tab-layer" class:active={tab.id === $activeTab?.id}>
        <PaneGrid
          tabId={tab.id}
          panes={tab.panes}
          active={tab.id === $activeTab?.id}
          worktrees={allWorktrees}
          tabDir={$activeTab?.dir || ''}
          on:closePane={handleClosePane}
          on:maximizePane={handleMaximizePane}
          on:focusPane={handleFocusPane}
          on:renamePane={handleRenamePane}
          on:restartPane={handleRestartPane}
          on:issueAction={handleIssueAction}
          on:navigateFile={handleNavigateFile}
          on:splitPane={() => (showLaunchDialog = true)}
          on:openWorktreePane={handleOpenWorktreePane}
          on:worktreeListChanged={loadWorktrees}
        />
      </div>
    {/each}
  </div>
{/if}
```

### Step 7: Add keyboard shortcut for dashboard

In `frontend/src/lib/shortcuts.ts`, the `createGlobalKeyHandler` needs a new callback. First check the signature — it takes an options object. Add an optional `onToggleDashboard` callback:

Open `frontend/src/lib/shortcuts.ts` and add to the options interface:

```typescript
onToggleDashboard?: () => void;
```

And in the keydown handler, add (with the other `Ctrl+Shift+...` checks):

```typescript
if (e.ctrlKey && e.shiftKey && e.key === 'H') {
  e.preventDefault();
  opts.onToggleDashboard?.();
  return;
}
```

Back in `App.svelte`, update `createGlobalKeyHandler`:

```typescript
const handleGlobalKeydown = createGlobalKeyHandler({
  // ... existing options ...
  onToggleDashboard: () => { showDashboard = !showDashboard; },
});
```

### Step 8: TypeScript check

```bash
cd /d/repos/Multiterminal/frontend && npx tsc --noEmit 2>&1 | head -20
```

Expected: no errors.

### Step 9: Run all frontend tests

```bash
cd /d/repos/Multiterminal/frontend && npx vitest run 2>&1 | tail -20
```

Expected: all PASS (tabs.test.ts + dashboard.test.ts).

### Step 10: Commit

```bash
cd /d/repos/Multiterminal
git add frontend/src/App.svelte frontend/src/lib/shortcuts.ts
git commit -m "feat(dashboard): wire DashboardView into App.svelte with Ctrl+Shift+H shortcut"
```

---

## Task 5: Visual verification

### Step 1: Start dev server

```bash
cd /d/repos/Multiterminal && wails dev
```

### Step 2: Manual checklist

- [ ] House icon (⌂) appears as the FIRST item in the tab bar, to the left of all tabs
- [ ] Clicking house icon shows the Dashboard view — PaneGrid disappears
- [ ] House icon is highlighted/active when dashboard is open
- [ ] Dashboard header shows correct pane/tab counts
- [ ] Four swim-lane columns are visible: "Braucht Eingabe", "Aktiv", "Fertig", "Inaktiv"
- [ ] Each column shows "Nichts wartet" / empty state when empty
- [ ] Starting a Claude session creates a card in the "Inaktiv" column (idle start state)
- [ ] Card shows: pane name, mode icon, tab name, branch (if any)
- [ ] Clicking a card closes dashboard and jumps to that tab + focuses the pane
- [ ] Clicking any regular tab in the tab bar closes the dashboard
- [ ] Ctrl+Shift+H toggles the dashboard open/closed
- [ ] When Claude becomes active, card moves to "Aktiv" column (reactive)
- [ ] When Claude finishes, card moves to "Fertig" column (reactive)
- [ ] When Claude needs input, card moves to "Braucht Eingabe" column with orange dot

### Step 3: Final commit (if any tweaks needed)

```bash
cd /d/repos/Multiterminal
git add -A
git commit -m "fix(dashboard): visual tweaks from manual review"
```

---

## Quick Reference

| Pane Activity | Dashboard Column |
|---|---|
| `waitingPermission` | Braucht Eingabe (orange) |
| `waitingAnswer` | Braucht Eingabe (orange) |
| `error` | Braucht Eingabe (red) |
| `active` | Aktiv (blue pulse) |
| `done` | Fertig (green) |
| `idle` | Inaktiv (gray) |

**Test commands:**
```bash
cd /d/repos/Multiterminal/frontend && npx vitest run
cd /d/repos/Multiterminal/frontend && npx tsc --noEmit
```

**Files changed:**
- Create: `frontend/src/lib/dashboard.ts`
- Create: `frontend/src/lib/dashboard.test.ts`
- Create: `frontend/src/components/DashboardView.svelte`
- Modify: `frontend/src/components/TabBar.svelte` (house icon)
- Modify: `frontend/src/App.svelte` (showDashboard state + DashboardView)
- Modify: `frontend/src/lib/shortcuts.ts` (Ctrl+Shift+H)
