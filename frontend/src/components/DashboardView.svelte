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
