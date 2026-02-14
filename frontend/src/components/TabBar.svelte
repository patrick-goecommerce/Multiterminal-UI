<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { tabStore, allTabs } from '../stores/tabs';

  export let activeTabId: string;

  const dispatch = createEventDispatcher();

  function handleTabClick(tabId: string) {
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
</script>

<div class="tab-bar">
  <div class="tabs">
    {#each $allTabs as tab (tab.id)}
      <button
        class="tab"
        class:active={tab.id === activeTabId}
        on:click={() => handleTabClick(tab.id)}
        on:dblclick={() => handleTabDblClick(tab.id)}
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
</style>
