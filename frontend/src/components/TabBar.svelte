<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { tabStore, allTabs } from '../stores/tabs';
  import * as App from '../../wailsjs/go/backend/App';
  import { EventsOn } from '../../wailsjs/runtime/runtime';
  import { getWindowId, isMainWindow } from '../lib/window';

  export let activeTabId: string;
  export let isDashboard: boolean = false;

  const dispatch = createEventDispatcher();

  const _isMain = isMainWindow();
  const _windowId = getWindowId();

  // Context menu state
  let contextMenuTabId: string | null = null;
  let contextMenuX = 0;
  let contextMenuY = 0;

  // Cross-window drag state
  let dropIndicator = false; // show drop zone highlight
  const claimedTabs = new Set<string>(); // tabs claimed by another window's drop

  onMount(() => {
    // Listen for our tabs being claimed by another window's drop
    EventsOn('window:tab-claimed', (event: any) => {
      if (event.data?.windowId !== _windowId) return;
      const tabId: string = event.data?.tabId;
      if (tabId) {
        claimedTabs.add(tabId);
        tabStore.forceCloseTab(tabId);
      }
    });
  });

  function handleTabClick(e: MouseEvent, tabId: string) {
    (e.currentTarget as HTMLElement).blur();
    tabStore.setActiveTab(tabId);
    // Always close the dashboard when a tab is clicked, even if it was
    // already the active tab (in that case the store doesn't emit a change).
    if (isDashboard) dispatch('closeDashboard');
  }

  function handleCloseTab(e: MouseEvent, tabId: string) {
    e.stopPropagation();
    tabStore.closeTab(tabId);
  }

  function handleAddTab() {
    dispatch('addTab');
  }

  function handleTabDblClick(tabId: string) {
    const name = prompt('Tab umbenennen:');
    if (name) tabStore.renameTab(tabId, name);
  }

  function handleDragStart(e: DragEvent, tabId: string) {
    e.dataTransfer?.setData('text/plain', tabId);
    // Tell backend so another window's drop handler can claim it
    const tab = $allTabs.find(t => t.id === tabId);
    App.SetDraggingTab(tabId, _windowId, tab ? JSON.stringify(tab) : '');
  }

  async function handleDragEnd(e: DragEvent, tabId: string) {
    // dropEffect 'move' = successfully dropped on a valid target (any window)
    // This is the fast path and avoids a race condition with cross-window events.
    if (e.dataTransfer?.dropEffect === 'move') {
      App.ClearDraggingTab();
      return;
    }
    const outside = e.clientX < 0 || e.clientX > window.innerWidth
                  || e.clientY < 0 || e.clientY > window.innerHeight;
    if (!outside) {
      App.ClearDraggingTab();
      return;
    }
    // Outside window: wait briefly for a cross-window claim event to arrive
    // before deciding to open a new window (IPC round-trip guard).
    await new Promise(r => setTimeout(r, 150));
    App.ClearDraggingTab();
    if (claimedTabs.has(tabId)) {
      claimedTabs.delete(tabId);
      return;
    }
    await detachTab(tabId);
  }

  async function detachTab(tabId: string) {
    try {
      const tab = $allTabs.find(t => t.id === tabId);
      const tabStateJSON = tab ? JSON.stringify(tab) : '';
      await App.DetachTab(tabId, _windowId, tabStateJSON);
      tabStore.forceCloseTab(tabId); // bypasses single-tab guard
    } catch (err) {
      console.error('[DetachTab] failed', err);
    }
  }

  // Drop zone: accept tabs dragged from other windows
  function handleTabBarDragOver(e: DragEvent) {
    if (e.dataTransfer?.types.includes('text/plain')) {
      e.preventDefault();
      e.dataTransfer.dropEffect = 'move';
      dropIndicator = true;
    }
  }

  function handleTabBarDragLeave() {
    dropIndicator = false;
  }

  async function handleTabBarDrop(e: DragEvent) {
    e.preventDefault();
    dropIndicator = false;
    const tabId = e.dataTransfer?.getData('text/plain') ?? '';
    // Skip same-window drops (tab is already in this window)
    if ($allTabs.some(t => t.id === tabId)) return;
    const tabStateJSON = await App.ClaimDraggedTab(tabId);
    if (tabStateJSON) {
      try {
        const tab = JSON.parse(tabStateJSON);
        tabStore.importTab(tab);
      } catch (err) {
        console.error('[TabBar drop] parse failed', err);
      }
    }
  }

  function handleContextMenu(e: MouseEvent, tabId: string) {
    e.preventDefault();
    e.stopPropagation();
    contextMenuTabId = tabId;
    contextMenuX = e.clientX;
    contextMenuY = e.clientY;
  }

  function closeContextMenu() {
    contextMenuTabId = null;
  }
</script>

<div class="tab-bar">
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div
    class="tabs"
    class:drop-target={dropIndicator}
    on:dragover={handleTabBarDragOver}
    on:dragleave={handleTabBarDragLeave}
    on:drop={handleTabBarDrop}
  >
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
    {#each $allTabs as tab (tab.id)}
      <button
        class="tab"
        class:active={tab.id === activeTabId && !isDashboard}
        class:highlight={tab._highlight}
        draggable="true"
        on:click={(e) => handleTabClick(e, tab.id)}
        on:dblclick={() => handleTabDblClick(tab.id)}
        on:dragstart={(e) => handleDragStart(e, tab.id)}
        on:dragend={(e) => handleDragEnd(e, tab.id)}
        on:contextmenu={(e) => handleContextMenu(e, tab.id)}
      >
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
      </button>
    {/each}
  </div>
  <button class="tab-add" on:click={handleAddTab} title="Neuer Tab (Ctrl+T)">
    +
  </button>
</div>

{#if contextMenuTabId}
  {@const _ctxTabId = contextMenuTabId}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="ctx-overlay" on:click={closeContextMenu}>
    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
    <div class="ctx-menu" style="left:{contextMenuX}px; top:{contextMenuY}px"
         on:click|stopPropagation>
      <button class="ctx-item" on:click={() => { detachTab(_ctxTabId); closeContextMenu(); }}>
        In neuem Fenster öffnen
      </button>
      <div class="ctx-separator"></div>
      <button class="ctx-item ctx-item-danger"
              on:click={() => { tabStore.closeTab(_ctxTabId); closeContextMenu(); }}>
        Tab schließen
      </button>
    </div>
  </div>
{/if}

<style>
  .tab-bar {
    display: flex;
    align-items: center;
    background: var(--tab-bg);
    border-bottom: 1px solid var(--border);
    height: 50px;
    padding: 0 6px;
    user-select: none;
    -webkit-app-region: drag;
  }

  .tabs {
    display: flex;
    gap: 3px;
    overflow-x: auto;
    flex: 1;
    -webkit-app-region: no-drag;
    border-radius: 6px 6px 0 0;
    transition: background 0.15s, outline 0.15s;
  }

  .tabs.drop-target {
    background: color-mix(in srgb, var(--accent) 15%, transparent);
    outline: 2px dashed var(--accent);
    outline-offset: -2px;
  }

  .tab {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 22px;
    background: transparent;
    border: none;
    border-radius: 6px 6px 0 0;
    color: var(--fg-muted);
    font-size: 14px;
    cursor: pointer;
    white-space: nowrap;
    transition: all 0.15s;
    min-width: 100px;
  }

  .tab-home {
    min-width: unset;
    padding: 12px 14px;
  }

  .tab:hover {
    background: var(--bg-tertiary);
    color: var(--fg);
  }

  .tab.active {
    background: var(--bg);
    color: var(--tab-active-fg);
    border-bottom: 2px solid var(--accent);
  }

  .tab.highlight {
    animation: tab-arrive 0.5s ease-out;
  }
  @keyframes tab-arrive {
    from { background: var(--accent); color: var(--bg); }
    to   { background: transparent; }
  }

  .tab-name {
    max-width: 180px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .tab-count {
    font-size: 10px;
    background: var(--bg-tertiary);
    color: var(--fg-muted);
    padding: 1px 5px;
    border-radius: 8px;
  }

  .tab-close {
    background: none;
    border: none;
    color: var(--fg-muted);
    cursor: pointer;
    padding: 0 2px;
    font-size: 16px;
    line-height: 1;
    border-radius: 4px;
  }

  .tab-close:hover {
    background: var(--error);
    color: white;
  }

  .tab-add {
    background: none;
    border: none;
    color: var(--fg-muted);
    font-size: 22px;
    cursor: pointer;
    padding: 6px 14px;
    border-radius: 4px;
    -webkit-app-region: no-drag;
  }

  .tab-add:hover {
    background: var(--bg-tertiary);
    color: var(--fg);
  }

  .ctx-overlay {
    position: fixed; inset: 0; z-index: 1000;
  }

  .ctx-menu {
    position: fixed;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 4px;
    min-width: 180px;
    box-shadow: 0 4px 16px rgba(0,0,0,0.4);
    z-index: 1001;
  }

  .ctx-item {
    display: block; width: 100%;
    padding: 7px 12px; text-align: left;
    background: none; border: none;
    color: var(--fg); font-size: 13px;
    border-radius: 4px; cursor: pointer;
  }

  .ctx-item:hover { background: var(--bg-tertiary); }

  .ctx-item-danger { color: #f87171; }

  .ctx-separator { height: 1px; background: var(--border); margin: 4px 0; }

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

  .tab-dot-waitingPermission {
    background: #f5a623;
    animation: tab-dot-pulse 1s ease-in-out infinite;
  }

  .tab-dot-waitingAnswer {
    background: #e8875a;
    animation: tab-dot-pulse 1s ease-in-out infinite;
  }

  .tab-dot-error {
    background: #e05252;
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
</style>
