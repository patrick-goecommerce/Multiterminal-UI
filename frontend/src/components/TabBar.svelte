<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { tabStore, allTabs } from '../stores/tabs';
  import * as App from '../../wailsjs/go/backend/App';
  import { getWindowId, isMainWindow } from '../lib/window';

  export let activeTabId: string;

  const dispatch = createEventDispatcher();

  const _isMain = isMainWindow();
  const _windowId = getWindowId();

  // Context menu state
  let contextMenuTabId: string | null = null;
  let contextMenuX = 0;
  let contextMenuY = 0;

  function handleTabClick(e: MouseEvent, tabId: string) {
    (e.currentTarget as HTMLElement).blur();
    tabStore.setActiveTab(tabId);
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
  }

  async function handleDragEnd(e: DragEvent, tabId: string) {
    // Detect drop outside window bounds
    const outside = e.clientX < 0 || e.clientX > window.innerWidth
                  || e.clientY < 0 || e.clientY > window.innerHeight;
    if (outside) {
      await detachTab(tabId);
    }
  }

  async function detachTab(tabId: string) {
    try {
      await App.DetachTab(tabId, _windowId);
      tabStore.forceCloseTab(tabId); // bypasses single-tab guard
    } catch (err) {
      console.error('[DetachTab] failed', err);
    }
  }

  function handleContextMenu(e: MouseEvent, tabId: string) {
    if (!_isMain) return; // no "new window" on secondary windows
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
  <div class="tabs">
    {#each $allTabs as tab (tab.id)}
      <button
        class="tab"
        class:active={tab.id === activeTabId}
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

{#if contextMenuTabId && _isMain}
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
</style>
