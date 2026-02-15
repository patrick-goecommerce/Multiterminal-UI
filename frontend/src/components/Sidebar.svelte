<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime';
  import FileTreeItem from './FileTreeItem.svelte';

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
  let gitStatuses: Record<string, string> = {};
  let sortModifiedFirst = false;
  let gitPollTimer: ReturnType<typeof setInterval> | null = null;

  onMount(() => {
    if (dir) {
      loadDir(dir);
      refreshGitStatus();
    }
    // Poll git status every 5 seconds
    gitPollTimer = setInterval(refreshGitStatus, 5000);
  });

  onDestroy(() => {
    if (gitPollTimer) clearInterval(gitPollTimer);
  });

  async function refreshGitStatus() {
    if (!dir) return;
    try {
      gitStatuses = await App.GetGitFileStatuses(dir);
    } catch {
      gitStatuses = {};
    }
  }

  async function loadDir(path: string) {
    try {
      const result = await App.ListDirectory(path);
      entries = (result || []).map((e: any) => ({ ...e, expanded: false, children: [], loaded: false }));
    } catch {}
  }

  let copiedPath = '';
  let copiedTimer: ReturnType<typeof setTimeout> | null = null;

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

  function sortEntries(items: FileEntry[]): FileEntry[] {
    if (!sortModifiedFirst) return items;
    return [...items].sort((a, b) => {
      const aChanged = (gitStatuses[a.path] || '') !== '';
      const bChanged = (gitStatuses[b.path] || '') !== '';
      if (aChanged !== bChanged) return aChanged ? -1 : 1;
      if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
      return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
    });
  }

  $: sortedEntries = sortEntries(entries);

  $: if (dir) {
    loadDir(dir);
    refreshGitStatus();
  }
</script>

{#if visible}
  <div class="sidebar" style="width: {width}px">
    <div class="sidebar-header">
      <span class="sidebar-title">Files</span>
      <button
        class="sort-btn"
        class:sort-active={sortModifiedFirst}
        on:click={() => (sortModifiedFirst = !sortModifiedFirst)}
        title={sortModifiedFirst ? 'Sortierung: Geänderte zuerst' : 'Sortierung: Standard'}
      >
        &#x25B2;&#x25BC;
      </button>
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

    <div class="shift-hint">Shift+Click = Pfad kopieren</div>
    <div class="file-list">
      {#if searching && searchResults.length > 0}
        {#each searchResults as entry (entry.path)}
          <FileTreeItem
            {entry}
            {gitStatuses}
            {copiedPath}
            {sortModifiedFirst}
            on:selectFile
            on:shiftclick={(e) => {
              ClipboardSetText(e.detail.path);
              copiedPath = e.detail.path;
              if (copiedTimer) clearTimeout(copiedTimer);
              copiedTimer = setTimeout(() => { copiedPath = ''; }, 1500);
            }}
          />
        {/each}
      {:else if searching && searchQuery}
        <div class="no-results">Keine Ergebnisse</div>
      {:else}
        {#each sortedEntries as entry (entry.path)}
          <FileTreeItem
            {entry}
            {gitStatuses}
            {copiedPath}
            {sortModifiedFirst}
            on:selectFile
            on:shiftclick={(e) => {
              ClipboardSetText(e.detail.path);
              copiedPath = e.detail.path;
              if (copiedTimer) clearTimeout(copiedTimer);
              copiedTimer = setTimeout(() => { copiedPath = ''; }, 1500);
            }}
          />
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
    gap: 4px;
  }

  .sidebar-title { font-size: 12px; font-weight: 600; color: var(--fg); flex: 1; }

  .sort-btn {
    background: none; border: none; color: var(--fg-muted);
    cursor: pointer; font-size: 9px; padding: 2px 4px;
    border-radius: 3px; letter-spacing: -2px;
  }
  .sort-btn:hover { background: var(--bg-tertiary); color: var(--fg); }
  .sort-btn.sort-active { color: var(--accent); background: var(--bg-tertiary); }

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

  .no-results { padding: 12px; text-align: center; color: var(--fg-muted); font-size: 12px; }

  .shift-hint {
    padding: 3px 10px;
    font-size: 10px;
    color: var(--fg-muted);
    border-bottom: 1px solid var(--border);
    opacity: 0.7;
  }
</style>
