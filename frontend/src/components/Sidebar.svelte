<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';

  export let visible: boolean = false;
  export let dir: string = '';
  export let width: number = 260;

  const dispatch = createEventDispatcher();

  interface FileEntry {
    name: string;
    path: string;
    isDir: boolean;
    expanded?: boolean;
    children?: FileEntry[];
    loaded?: boolean;
  }

  let entries: FileEntry[] = [];
  let searchQuery = '';
  let searchResults: FileEntry[] = [];
  let searching = false;

  onMount(() => {
    if (dir) loadDir(dir);
  });

  async function loadDir(path: string) {
    try {
      const result = await App.ListDirectory(path);
      entries = (result || []).map((e: any) => ({ ...e, expanded: false, children: [], loaded: false }));
    } catch {}
  }

  async function toggleDir(entry: FileEntry) {
    if (!entry.isDir) return;
    if (entry.expanded) {
      entry.expanded = false;
      entries = entries;
      return;
    }
    if (!entry.loaded) {
      try {
        const children = await App.ListDirectory(entry.path);
        entry.children = (children || []).map((e: any) => ({ ...e, expanded: false, children: [], loaded: false }));
        entry.loaded = true;
      } catch {
        entry.children = [];
      }
    }
    entry.expanded = true;
    entries = entries;
  }

  function selectFile(entry: FileEntry) {
    if (entry.isDir) {
      toggleDir(entry);
    } else {
      dispatch('selectFile', { path: entry.path });
    }
  }

  async function search() {
    if (!searchQuery.trim()) {
      searchResults = [];
      searching = false;
      return;
    }
    searching = true;
    try {
      searchResults = (await App.SearchFiles(dir, searchQuery)) || [];
    } catch {
      searchResults = [];
    }
  }

  function clearSearch() {
    searchQuery = '';
    searchResults = [];
    searching = false;
  }

  $: if (dir) loadDir(dir);
</script>

{#if visible}
  <div class="sidebar" style="width: {width}px">
    <div class="sidebar-header">
      <span class="sidebar-title">Files</span>
      <button class="sidebar-close" on:click={() => dispatch('close')}>&times;</button>
    </div>

    <div class="search-box">
      <input
        type="text"
        placeholder="Suchen..."
        bind:value={searchQuery}
        on:input={search}
      />
      {#if searchQuery}
        <button class="search-clear" on:click={clearSearch}>&times;</button>
      {/if}
    </div>

    <div class="file-list">
      {#if searching && searchResults.length > 0}
        {#each searchResults as entry}
          <button class="file-entry" on:click={() => selectFile(entry)}>
            <span class="file-icon">{entry.isDir ? '\u{1F4C1}' : '\u{1F4C4}'}</span>
            <span class="file-name">{entry.name}</span>
          </button>
        {/each}
      {:else if searching && searchQuery}
        <div class="no-results">Keine Ergebnisse</div>
      {:else}
        {#each entries as entry}
          <button class="file-entry" on:click={() => selectFile(entry)}>
            <span class="file-icon">
              {#if entry.isDir}
                {entry.expanded ? '\u{1F4C2}' : '\u{1F4C1}'}
              {:else}
                {'\u{1F4C4}'}
              {/if}
            </span>
            <span class="file-name">{entry.name}</span>
          </button>
          {#if entry.expanded && entry.children}
            {#each entry.children as child}
              <button class="file-entry nested" on:click={() => selectFile(child)}>
                <span class="file-icon">{child.isDir ? '\u{1F4C1}' : '\u{1F4C4}'}</span>
                <span class="file-name">{child.name}</span>
              </button>
            {/each}
          {/if}
        {/each}
      {/if}
    </div>
  </div>
{/if}

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    background: var(--bg-secondary);
    border-right: 1px solid var(--border);
    overflow: hidden;
    flex-shrink: 0;
  }

  .sidebar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 10px;
    border-bottom: 1px solid var(--border);
  }

  .sidebar-title { font-size: 12px; font-weight: 600; color: var(--fg); }

  .sidebar-close {
    background: none; border: none; color: var(--fg-muted);
    cursor: pointer; font-size: 16px; padding: 0 4px;
  }
  .sidebar-close:hover { color: var(--fg); }

  .search-box {
    position: relative; padding: 6px 8px;
    border-bottom: 1px solid var(--border);
  }

  .search-box input {
    width: 100%; padding: 5px 24px 5px 8px;
    background: var(--bg-tertiary); border: 1px solid var(--border);
    border-radius: 4px; color: var(--fg); font-size: 12px; box-sizing: border-box;
  }
  .search-box input::placeholder { color: var(--fg-muted); }

  .search-clear {
    position: absolute; right: 14px; top: 50%; transform: translateY(-50%);
    background: none; border: none; color: var(--fg-muted); cursor: pointer; font-size: 14px;
  }

  .file-list { flex: 1; overflow-y: auto; padding: 4px 0; }

  .file-entry {
    display: flex; align-items: center; gap: 6px; width: 100%;
    padding: 3px 10px; background: none; border: none;
    color: var(--fg); font-size: 12px; cursor: pointer; text-align: left;
  }
  .file-entry:hover { background: var(--bg-tertiary); }
  .file-entry.nested { padding-left: 26px; }

  .file-icon { font-size: 12px; flex-shrink: 0; }

  .file-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  .no-results { padding: 12px; text-align: center; color: var(--fg-muted); font-size: 12px; }
</style>
