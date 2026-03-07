<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { workspace, type NavItem, type SidebarView } from '../stores/workspace';

  export let issueCount = 0;
  export let queueCount = 0;
  export let chatUnread = 0;

  const dispatch = createEventDispatcher();

  // Main content views (replace pane grid)
  const mainViews: { id: NavItem; label: string; icon: string }[] = [
    { id: 'terminals', label: 'Terminals', icon: 'terminal' },
    { id: 'dashboard', label: 'Dashboard', icon: 'dashboard' },
    { id: 'kanban', label: 'Kanban', icon: 'kanban' },
    { id: 'chat', label: 'Chat', icon: 'chat' },
    { id: 'queue', label: 'Queue', icon: 'queue' },
  ];

  // Sidebar views (open as side panel)
  const sidebarViews: { id: SidebarView; label: string; icon: string }[] = [
    { id: 'explorer', label: 'Explorer', icon: 'explorer' },
    { id: 'source-control', label: 'Source Control', icon: 'git' },
    { id: 'issues', label: 'Issues', icon: 'issues' },
  ];

  function handleMainClick(id: NavItem) {
    workspace.setView(id);
  }

  function handleSidebarClick(id: SidebarView) {
    workspace.toggleSidebar(id);
  }

  function getBadge(id: string): number {
    if (id === 'issues') return issueCount;
    if (id === 'queue') return queueCount;
    if (id === 'chat') return chatUnread;
    return 0;
  }
</script>

<nav class="left-nav" class:collapsed={$workspace.leftNavCollapsed}>
  <div class="nav-section">
    {#each mainViews as view}
      <button
        class="nav-item"
        class:active={$workspace.activeView === view.id}
        title={view.label}
        on:click={() => handleMainClick(view.id)}
      >
        <span class="icon icon-{view.icon}"></span>
        {#if !$workspace.leftNavCollapsed}
          <span class="label">{view.label}</span>
        {/if}
        {#if getBadge(view.id) > 0}
          <span class="badge">{getBadge(view.id)}</span>
        {/if}
      </button>
    {/each}
  </div>

  <div class="nav-divider"></div>

  <div class="nav-section">
    {#each sidebarViews as view}
      <button
        class="nav-item"
        class:active={$workspace.activeView === 'terminals' && $workspace.sidebarView === view.id}
        title={view.label}
        on:click={() => handleSidebarClick(view.id)}
      >
        <span class="icon icon-{view.icon}"></span>
        {#if !$workspace.leftNavCollapsed}
          <span class="label">{view.label}</span>
        {/if}
        {#if getBadge(view.id) > 0}
          <span class="badge">{getBadge(view.id)}</span>
        {/if}
      </button>
    {/each}
  </div>

  <div class="nav-spacer"></div>

  <button
    class="nav-item collapse-toggle"
    title={$workspace.leftNavCollapsed ? 'Erweitern' : 'Einklappen'}
    on:click={() => workspace.toggleCollapsed()}
  >
    <span class="icon icon-collapse" class:rotated={$workspace.leftNavCollapsed}></span>
  </button>
</nav>

<style>
  .left-nav {
    display: flex;
    flex-direction: column;
    width: 180px;
    min-width: 180px;
    background: var(--surface, #1e1e2e);
    border-right: 1px solid var(--border, #45475a);
    padding: 0.25rem 0;
    overflow: hidden;
    transition: width 0.15s ease, min-width 0.15s ease;
  }
  .left-nav.collapsed {
    width: 44px;
    min-width: 44px;
  }

  .nav-section {
    display: flex;
    flex-direction: column;
    gap: 1px;
    padding: 0 0.25rem;
  }

  .nav-divider {
    height: 1px;
    background: var(--border, #45475a);
    margin: 0.5rem 0.5rem;
  }

  .nav-spacer { flex: 1; }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.4rem 0.5rem;
    background: none;
    border: none;
    border-radius: 6px;
    color: var(--fg-muted, #a6adc8);
    cursor: pointer;
    font-size: 0.8rem;
    text-align: left;
    position: relative;
    transition: background 0.1s, color 0.1s;
    white-space: nowrap;
    overflow: hidden;
  }
  .nav-item:hover {
    background: rgba(255,255,255,0.05);
    color: var(--fg, #cdd6f4);
  }
  .nav-item.active {
    background: rgba(57, 255, 20, 0.08);
    color: var(--accent, #39ff14);
  }
  .nav-item.active::before {
    content: '';
    position: absolute;
    left: 0; top: 4px; bottom: 4px;
    width: 2px;
    background: var(--accent, #39ff14);
    border-radius: 1px;
  }

  .icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    flex-shrink: 0;
    font-size: 0.85rem;
  }
  /* SVG icons as pseudo-content using unicode/emoji placeholders */
  .icon-terminal::before { content: '>_'; font-family: monospace; font-size: 0.7rem; font-weight: 700; }
  .icon-dashboard::before { content: '\25A3'; }
  .icon-kanban::before { content: '\2630'; }
  .icon-chat::before { content: '\1F4AC'; font-size: 0.75rem; }
  .icon-queue::before { content: '\2261'; font-size: 1.1rem; }
  .icon-explorer::before { content: '\1F4C1'; font-size: 0.75rem; }
  .icon-git::before { content: '\2387'; }
  .icon-issues::before { content: '\25CB'; }
  .icon-collapse::before { content: '\276E'; font-size: 0.7rem; }
  .icon-collapse.rotated::before { content: '\276F'; }

  .label {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .badge {
    font-size: 0.6rem;
    min-width: 16px;
    height: 16px;
    padding: 0 4px;
    border-radius: 8px;
    background: var(--accent, #39ff14);
    color: #000;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 700;
    flex-shrink: 0;
  }

  .collapse-toggle {
    margin: 0.25rem;
    justify-content: center;
  }
  .collapsed .collapse-toggle { justify-content: center; }
  .collapsed .nav-item { justify-content: center; padding: 0.4rem; }
  .collapsed .badge {
    position: absolute;
    top: 2px; right: 2px;
    min-width: 12px; height: 12px;
    font-size: 0.5rem; padding: 0 2px;
  }
</style>
