<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime';

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

  let copiedPath = '';
  let copiedTimer: ReturnType<typeof setTimeout> | null = null;

  function handleFileClick(e: MouseEvent, entry: FileEntry) {
    if (e.shiftKey) {
      // Shift+Click → copy path to clipboard
      e.preventDefault();
      ClipboardSetText(entry.path);
      copiedPath = entry.path;
      if (copiedTimer) clearTimeout(copiedTimer);
      copiedTimer = setTimeout(() => { copiedPath = ''; }, 1500);
      return;
    }
    selectFile(entry);
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

  function getGitStatus(path: string): string {
    return gitStatuses[path] || '';
  }

  function getStatusLabel(status: string): string {
    switch (status) {
      case 'M': return 'M';
      case '?': return 'N';
      case 'A': return 'A';
      case 'D': return 'D';
      case 'R': return 'R';
      default: return '';
    }
  }

  function getStatusClass(status: string): string {
    switch (status) {
      case 'M': return 'git-modified';
      case '?': return 'git-new';
      case 'A': return 'git-added';
      case 'D': return 'git-deleted';
      case 'R': return 'git-renamed';
      default: return '';
    }
  }

  function sortEntries(items: FileEntry[]): FileEntry[] {
    if (!sortModifiedFirst) return items;
    return [...items].sort((a, b) => {
      const aStatus = getGitStatus(a.path);
      const bStatus = getGitStatus(b.path);
      const aChanged = aStatus !== '';
      const bChanged = bStatus !== '';
      if (aChanged !== bChanged) return aChanged ? -1 : 1;
      // Keep dirs-first within same group
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
        {#each searchResults as entry}
          {@const status = getGitStatus(entry.path)}
          <button class="file-entry {getStatusClass(status)}" on:click={(e) => handleFileClick(e, entry)} title={entry.path}>
            <span class="file-icon">{entry.isDir ? '\u{1F4C1}' : '\u{1F4C4}'}</span>
            <span class="file-name">{entry.name}</span>
            {#if copiedPath === entry.path}
              <span class="copied-badge">kopiert!</span>
            {:else if status}
              <span class="git-badge {getStatusClass(status)}">{getStatusLabel(status)}</span>
            {/if}
          </button>
        {/each}
      {:else if searching && searchQuery}
        <div class="no-results">Keine Ergebnisse</div>
      {:else}
        {#each sortedEntries as entry}
          {@const status = getGitStatus(entry.path)}
          <button class="file-entry {getStatusClass(status)}" on:click={(e) => handleFileClick(e, entry)} title={entry.path}>
            <span class="file-icon">
              {#if entry.isDir}
                {entry.expanded ? '\u{1F4C2}' : '\u{1F4C1}'}
              {:else}
                {'\u{1F4C4}'}
              {/if}
            </span>
            <span class="file-name">{entry.name}</span>
            {#if copiedPath === entry.path}
              <span class="copied-badge">kopiert!</span>
            {:else if status}
              <span class="git-badge {getStatusClass(status)}">{getStatusLabel(status)}</span>
            {/if}
          </button>
          {#if entry.expanded && entry.children}
            {#each sortEntries(entry.children) as child}
              {@const childStatus = getGitStatus(child.path)}
              <button class="file-entry nested {getStatusClass(childStatus)}" on:click={(e) => handleFileClick(e, child)} title={child.path}>
                <span class="file-icon">{child.isDir ? '\u{1F4C1}' : '\u{1F4C4}'}</span>
                <span class="file-name">{child.name}</span>
                {#if copiedPath === child.path}
                  <span class="copied-badge">kopiert!</span>
                {:else if childStatus}
                  <span class="git-badge {getStatusClass(childStatus)}">{getStatusLabel(childStatus)}</span>
                {/if}
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

  .file-entry {
    display: flex; align-items: center; gap: 6px; width: 100%;
    padding: 3px 10px; background: none; border: none;
    color: var(--fg); font-size: 12px; cursor: pointer; text-align: left;
  }
  .file-entry:hover { background: var(--bg-tertiary); }
  .file-entry.nested { padding-left: 26px; }

  /* Git status file name coloring */
  .file-entry.git-modified { color: #e2b93d; }
  .file-entry.git-new { color: #73c991; }
  .file-entry.git-added { color: #73c991; }
  .file-entry.git-deleted { color: #f87171; }
  .file-entry.git-renamed { color: #6bc5d2; }

  .file-icon { font-size: 12px; flex-shrink: 0; }

  .file-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }

  .git-badge {
    font-size: 10px; font-weight: 700; padding: 0 4px;
    border-radius: 3px; flex-shrink: 0; line-height: 16px;
  }
  .git-badge.git-modified { background: #e2b93d22; color: #e2b93d; }
  .git-badge.git-new { background: #73c99122; color: #73c991; }
  .git-badge.git-added { background: #73c99122; color: #73c991; }
  .git-badge.git-deleted { background: #f8717122; color: #f87171; }
  .git-badge.git-renamed { background: #6bc5d222; color: #6bc5d2; }

  .no-results { padding: 12px; text-align: center; color: var(--fg-muted); font-size: 12px; }

  .shift-hint {
    padding: 3px 10px;
    font-size: 10px;
    color: var(--fg-muted);
    border-bottom: 1px solid var(--border);
    opacity: 0.7;
  }

  .copied-badge {
    font-size: 10px; font-weight: 600; padding: 0 4px;
    border-radius: 3px; flex-shrink: 0; line-height: 16px;
    background: #22c55e33; color: #22c55e;
    animation: fade-in 0.15s ease;
  }

  @keyframes fade-in {
    from { opacity: 0; transform: scale(0.8); }
    to { opacity: 1; transform: scale(1); }
  }
</style>
