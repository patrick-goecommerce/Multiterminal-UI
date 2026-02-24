<!-- frontend/src/components/WorktreeDropdown.svelte -->
<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export interface WorktreeInfo {
    path: string;
    branch: string;
    issue: number;
    category: string;
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
