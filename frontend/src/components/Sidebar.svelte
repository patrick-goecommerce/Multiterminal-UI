<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';
  import FileTreeItem from './FileTreeItem.svelte';
  import FavoritesSection from './FavoritesSection.svelte';
  import IssuesView from './IssuesView.svelte';
  import SourceControlView from './SourceControlView.svelte';

  export let visible: boolean = false;
  export let dir: string = '';
  export let width: number = 260;
  export let issueCount: number = 0;
  export let paneIssues: Record<number, { activity: string; cost: string }> = {};
  export let conflictFiles: string[] = [];
  export let conflictOperation: string = '';
  export let initialView: 'explorer' | 'source-control' | 'issues' = 'explorer';
  export let pinned: boolean = false;

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
  let favorites: string[] = [];
  $: favoritePaths = new Set(favorites);

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
      loadFavorites();
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

  async function loadFavorites() {
    if (!dir) {
      favorites = [];
      return;
    }
    try {
      favorites = (await App.GetFavorites(dir)) || [];
    } catch {
      favorites = [];
    }
  }

  async function handleToggleFavorite(e: CustomEvent<{ path: string; isFavorite: boolean }>) {
    const { path, isFavorite } = e.detail;
    try {
      if (isFavorite) {
        await App.RemoveFavorite(dir, path);
      } else {
        await App.AddFavorite(dir, path);
      }
      await loadFavorites();
    } catch {}
  }

  async function handleRemoveFavorite(e: CustomEvent<{ path: string }>) {
    try {
      await App.RemoveFavorite(dir, e.detail.path);
      await loadFavorites();
    } catch {}
  }

  $: changeCount = (() => {
    const paths = Object.keys(gitStatuses);
    return paths.filter(p => {
      const normalized = p.replace(/\\/g, '/').replace(/\/$/, '');
      return !paths.some(other => other !== p && other.replace(/\\/g, '/').startsWith(normalized + '/'));
    }).length;
  })();

  $: if (dir) {
    loadDir(dir);
    refreshGitStatus();
    loadFavorites();
  }
</script>

{#if visible}
  <div class="sidebar" style="width: {width}px">
    <div class="sidebar-header">
      <span class="sidebar-title">Files</span>
      <button class="sidebar-pin" class:active={pinned} title={pinned ? 'Sidebar lösen' : 'Sidebar anpinnen'} on:click={() => dispatch('togglePin')}>
        <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
          {#if pinned}
            <path d="M10 1.5l4.5 4.5-1 1-1-.5L10 9l-.5 3.5L8 14l-1.5-3L3 14.5 1.5 13 5 9.5 2 8l1.5-1.5L7 6l2.5-2.5-.5-1z"/>
          {:else}
            <path d="M10.5 2L14 5.5l-1 1-.5-.5-3 3-.5 3.5-1 1L6.5 12 4 14.5 1.5 12 4 9.5 2.5 8l1-1L7 7.5l3-3-.5-.5zm0 1.4L7.4 6.5l.1.5-3.5.5-.5.5 2 2L2 13.5 3.5 12 7 8.5l2 2 .5-.5.5-3.5.5.1 3.1-3.1z"/>
          {/if}
        </svg>
      </button>
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
      <FavoritesSection
        {favorites}
        on:selectFile
        on:removeFavorite={handleRemoveFavorite}
      />

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
              {favoritePaths}
              on:selectFile
              on:copied={(e) => setCopied(e.detail.path)}
              on:toggleFavorite={handleToggleFavorite}
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
              {favoritePaths}
              on:selectFile
              on:copied={(e) => setCopied(e.detail.path)}
              on:toggleFavorite={handleToggleFavorite}
            />
          {/each}
        {/if}
      </div>
    {:else if activeView === 'source-control'}
      <SourceControlView {dir} {gitStatuses} {conflictFiles} {conflictOperation} on:selectFile />
    {:else if activeView === 'issues'}
      <div class="file-list">
        {#key dir}
          <IssuesView {dir} {paneIssues} on:createIssue on:editIssue on:launchForIssue />
        {/key}
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

  .sidebar-pin {
    background: none; border: none; color: var(--fg-muted);
    cursor: pointer; padding: 2px 4px; display: flex; align-items: center;
    border-radius: 3px; transition: color 0.15s, background 0.15s;
  }
  .sidebar-pin:hover { color: var(--fg); background: var(--bg-tertiary); }
  .sidebar-pin.active { color: var(--accent); }

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

</style>
