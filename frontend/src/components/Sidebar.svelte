<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime';
  import FileTreeItem from './FileTreeItem.svelte';
  import IssuesView from './IssuesView.svelte';

  export let visible: boolean = false;
  export let dir: string = '';
  export let width: number = 260;
  export let issueCount: number = 0;
  export let paneIssues: Record<number, { activity: string; cost: string }> = {};
  export let initialView: 'explorer' | 'source-control' | 'issues' = 'explorer';

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
  let gitPollTimer: ReturnType<typeof setInterval> | null = null;
  let activeView: 'explorer' | 'source-control' | 'issues' = initialView || 'explorer';

  // React to external view changes (e.g. Ctrl+I)
  $: if (initialView && visible) activeView = initialView;

  let copiedPath = '';
  let copiedTimer: ReturnType<typeof setTimeout> | null = null;

  function setCopied(path: string) {
    copiedPath = path;
    if (copiedTimer) clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => { copiedPath = ''; }, 1500);
  }

  onMount(() => {
    if (dir) {
      loadDir(dir);
      refreshGitStatus();
    }
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

  interface ScEntry { path: string; name: string; relPath: string; status: string; }
  interface ScGroup { label: string; code: string; entries: ScEntry[]; }

  const statusGroups = [
    { label: 'Modified', code: 'M' },
    { label: 'Added', code: 'A' },
    { label: 'Untracked', code: '?' },
    { label: 'Deleted', code: 'D' },
    { label: 'Renamed', code: 'R' },
  ];

  function getGroupedChanges(statuses: Record<string, string>): ScGroup[] {
    const groups: ScGroup[] = statusGroups.map(g => ({ ...g, entries: [] }));
    for (const [path, status] of Object.entries(statuses)) {
      const group = groups.find(g => g.code === status);
      if (!group) continue;
      const relPath = dir ? path.replace(dir.replace(/\\/g, '/'), '').replace(/^[\\/]/, '') : path;
      const name = relPath.split(/[\\/]/).pop() || relPath;
      group.entries.push({ path, name, relPath, status });
    }
    for (const g of groups) {
      g.entries.sort((a, b) => a.relPath.localeCompare(b.relPath, undefined, { sensitivity: 'base' }));
    }
    return groups.filter(g => g.entries.length > 0);
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

  function handleScClick(path: string) {
    dispatch('selectFile', { path });
  }

  function handleScCopy(e: MouseEvent, path: string) {
    e.stopPropagation();
    ClipboardSetText(path);
    setCopied(path);
  }

  $: groupedChanges = getGroupedChanges(gitStatuses);
  $: changeCount = Object.keys(gitStatuses).length;

  $: if (dir) {
    loadDir(dir);
    refreshGitStatus();
  }
</script>

{#if visible}
  <div class="sidebar" style="width: {width}px">
    <div class="sidebar-header">
      <span class="sidebar-title">Files</span>
      <button class="sidebar-close" on:click={() => dispatch('close')}>&times;</button>
    </div>

    <div class="view-toggle">
      <button
        class="toggle-btn"
        class:active={activeView === 'explorer'}
        on:click={() => (activeView = 'explorer')}
      >Explorer</button>
      <button
        class="toggle-btn"
        class:active={activeView === 'source-control'}
        on:click={() => (activeView = 'source-control')}
      >
        Source Control
        {#if changeCount > 0}
          <span class="change-count">{changeCount}</span>
        {/if}
      </button>
      <button
        class="toggle-btn"
        class:active={activeView === 'issues'}
        on:click={() => (activeView = 'issues')}
      >
        Issues
        {#if issueCount > 0}
          <span class="change-count">{issueCount}</span>
        {/if}
      </button>
    </div>

    {#if activeView === 'explorer'}
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
          {#each searchResults as entry (entry.path)}
            <FileTreeItem
              {entry}
              {gitStatuses}
              {copiedPath}
              on:selectFile
              on:copied={(e) => setCopied(e.detail.path)}
            />
          {/each}
        {:else if searching && searchQuery}
          <div class="no-results">Keine Ergebnisse</div>
        {:else}
          {#each entries as entry (entry.path)}
            <FileTreeItem
              {entry}
              {gitStatuses}
              {copiedPath}
              on:selectFile
              on:copied={(e) => setCopied(e.detail.path)}
            />
          {/each}
        {/if}
      </div>
    {:else if activeView === 'source-control'}
      <div class="file-list">
        {#if groupedChanges.length === 0}
          <div class="no-results">Keine Änderungen</div>
        {:else}
          {#each groupedChanges as group}
            <div class="sc-group-header">{group.label}</div>
            {#each group.entries as entry (entry.path)}
              <div
                class="sc-entry {getStatusClass(entry.status)}"
                on:click={() => handleScClick(entry.path)}
                on:keydown
                role="button"
                tabindex="-1"
                title={entry.path}
              >
                <span class="sc-name">{entry.name}</span>
                <span class="sc-relpath">{entry.relPath}</span>
                {#if copiedPath === entry.path}
                  <span class="copied-badge">kopiert!</span>
                {:else}
                  <span class="sc-badge {getStatusClass(entry.status)}">{entry.status === '?' ? 'N' : entry.status}</span>
                {/if}
                <button class="copy-btn" on:click={(e) => handleScCopy(e, entry.path)} title="Pfad kopieren">
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
                    <path d="M4 4v-2a2 2 0 0 1 2-2h6a2 2 0 0 1 2 2v6a2 2 0 0 1-2 2h-2v2a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2zm2-2v2h2a2 2 0 0 1 2 2v2h2V2H6zM2 6v6h6V6H2z"/>
                  </svg>
                </button>
              </div>
            {/each}
          {/each}
        {/if}
      </div>
    {:else if activeView === 'issues'}
      <div class="file-list">
        <IssuesView {dir} {paneIssues} on:createIssue on:editIssue on:launchForIssue />
      </div>
    {/if}
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

  .sidebar-close {
    background: none; border: none; color: var(--fg-muted);
    cursor: pointer; font-size: 16px; padding: 0 4px;
  }
  .sidebar-close:hover { color: var(--fg); }

  .view-toggle {
    display: flex; padding: 6px 8px; gap: 2px;
    border-bottom: 1px solid var(--border);
    background: var(--bg-secondary);
  }

  .toggle-btn {
    flex: 1; padding: 4px 8px; font-size: 11px; font-weight: 600;
    border: none; border-radius: 4px; cursor: pointer;
    background: transparent; color: var(--fg-muted);
    transition: background 0.15s, color 0.15s;
    display: flex; align-items: center; justify-content: center; gap: 4px;
  }
  .toggle-btn:hover { background: var(--bg-tertiary); color: var(--fg); }
  .toggle-btn.active { background: var(--accent); color: #fff; }

  .change-count {
    font-size: 10px; font-weight: 700; background: rgba(255,255,255,0.2);
    border-radius: 8px; padding: 0 5px; line-height: 16px; min-width: 16px;
    text-align: center;
  }
  .toggle-btn:not(.active) .change-count {
    background: var(--bg-tertiary); color: var(--fg-muted);
  }

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

  /* Source Control View */
  .sc-group-header {
    font-size: 11px; font-weight: 600; color: var(--fg-muted);
    padding: 8px 10px 4px; text-transform: uppercase; letter-spacing: 0.5px;
  }

  .sc-entry {
    display: flex; align-items: center; gap: 6px;
    padding: 3px 10px; cursor: pointer; font-size: 12px;
  }
  .sc-entry:hover { background: var(--bg-tertiary); }

  .sc-entry.git-modified { color: #e2b93d; }
  .sc-entry.git-new { color: #73c991; }
  .sc-entry.git-added { color: #73c991; }
  .sc-entry.git-deleted { color: #f87171; }
  .sc-entry.git-renamed { color: #6bc5d2; }

  .sc-name {
    font-weight: 500; white-space: nowrap; overflow: hidden;
    text-overflow: ellipsis; flex-shrink: 1; min-width: 0;
  }
  .sc-relpath {
    font-size: 10px; color: var(--fg-muted); white-space: nowrap;
    overflow: hidden; text-overflow: ellipsis; flex: 1; min-width: 0;
    opacity: 0.7;
  }

  .sc-badge {
    font-size: 10px; font-weight: 700; padding: 0 4px;
    border-radius: 3px; flex-shrink: 0; line-height: 16px;
  }
  .sc-badge.git-modified { background: #e2b93d22; color: #e2b93d; }
  .sc-badge.git-new { background: #73c99122; color: #73c991; }
  .sc-badge.git-added { background: #73c99122; color: #73c991; }
  .sc-badge.git-deleted { background: #f8717122; color: #f87171; }
  .sc-badge.git-renamed { background: #6bc5d222; color: #6bc5d2; }

  .copied-badge {
    font-size: 10px; font-weight: 600; padding: 0 4px;
    border-radius: 3px; flex-shrink: 0; line-height: 16px;
    background: #22c55e33; color: #22c55e;
    animation: fade-in 0.15s ease;
  }

  .copy-btn {
    opacity: 0; background: none; border: none; color: var(--fg-muted);
    cursor: pointer; padding: 1px 3px; border-radius: 3px; flex-shrink: 0;
    display: flex; align-items: center; transition: opacity 0.15s;
  }
  .sc-entry:hover .copy-btn { opacity: 1; }
  .copy-btn:hover { color: var(--fg); background: var(--bg-secondary); }

  @keyframes fade-in {
    from { opacity: 0; transform: scale(0.8); }
    to { opacity: 1; transform: scale(1); }
  }
</style>
