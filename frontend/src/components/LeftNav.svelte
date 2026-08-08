<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { workspace, type NavItem, type SidebarView } from '../stores/workspace';
  import { audioMuted } from '../lib/audio';
  import { t } from '../stores/i18n';

  export let issueCount = 0;
  export let appVersion = '';

  const dispatch = createEventDispatcher<{
    openSettings: void;
  }>();

  // Main content views (replace pane grid)
  const mainViews: { id: NavItem; label: string; icon: string }[] = [
    { id: 'dashboard', label: 'Dashboard', icon: 'home' },
    { id: 'terminals', label: 'Terminals', icon: 'terminal' },
    { id: 'kanban', label: 'Kanban', icon: 'kanban' },
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

  <div class="nav-section bottom-actions">
    <button
      class="nav-item"
      class:muted={$audioMuted}
      title={$audioMuted ? $t('toolbar.audioOn') : $t('toolbar.audioOff')}
      on:click={() => $audioMuted = !$audioMuted}
    >
      <span class="icon icon-audio">{$audioMuted ? '🔇' : '🔊'}</span>
      {#if !$workspace.leftNavCollapsed}
        <span class="label">{$audioMuted ? 'Ton an' : 'Ton aus'}</span>
      {/if}
    </button>
    <button
      class="nav-item"
      title={$t('toolbar.settings')}
      on:click={() => dispatch('openSettings')}
    >
      <span class="icon icon-settings"></span>
      {#if !$workspace.leftNavCollapsed}
        <span class="label">Einstellungen</span>
      {/if}
    </button>
  </div>

  {#if appVersion && !$workspace.leftNavCollapsed}
    <div class="nav-version" title={`Version ${appVersion}`}>v{appVersion}</div>
  {/if}

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
    background: var(--status-running-tint, rgba(76, 197, 106, 0.12));
    color: var(--accent, #4cc56a);
  }
  .nav-item.active::before {
    content: '';
    position: absolute;
    left: 0; top: 4px; bottom: 4px;
    width: 2.5px;
    background: var(--accent, #4cc56a);
    border-radius: 0 2px 2px 0;
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
  .icon-home::before { content: '\2302'; }
  .icon-kanban::before { content: '\2630'; }
  .icon-explorer::before { content: '\1F4C1'; font-size: 0.75rem; }
  .icon-git::before { content: '\2387'; }
  .icon-issues::before { content: '\25CB'; }
  .icon-settings::before { content: '\2699'; }
  .icon-audio { font-size: 0.75rem; }
  .icon-collapse::before { content: '\276E'; font-size: 0.7rem; }
  .icon-collapse.rotated::before { content: '\276F'; }

  .bottom-actions {
    border-top: 1px solid var(--border, #45475a);
    padding-top: 0.4rem;
    margin-top: 0.25rem;
  }
  .nav-item.muted { opacity: 0.5; }

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
    background: var(--status-waiting-tint, rgba(214,168,92,.16));
    color: var(--status-waiting, #d6a85c);
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 700;
    flex-shrink: 0;
  }

  .nav-version {
    text-align: center;
    font-size: 10px;
    font-family: monospace;
    color: var(--fg-muted);
    padding: 2px 0 4px;
    user-select: none;
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
