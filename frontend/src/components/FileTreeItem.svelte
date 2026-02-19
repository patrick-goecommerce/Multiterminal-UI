<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime';
  import * as App from '../../wailsjs/go/backend/App';

  interface FileEntry {
    name: string;
    path: string;
    isDir: boolean;
    expanded?: boolean;
    children?: FileEntry[];
    loaded?: boolean;
  }

  export let entry: FileEntry;
  export let depth: number = 0;
  export let gitStatuses: Record<string, string> = {};
  export let copiedPath: string = '';
  export let favoritePaths: Set<string> = new Set();

  const dispatch = createEventDispatcher();

  async function toggle() {
    if (!entry.isDir) return;
    if (entry.expanded) {
      entry.expanded = false;
      entry = entry;
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
    entry = entry;
  }

  function handleClick(e: MouseEvent) {
    if (entry.isDir) {
      toggle();
    } else {
      dispatch('selectFile', { path: entry.path });
    }
  }

  function handleCopy(e: MouseEvent) {
    e.stopPropagation();
    ClipboardSetText(entry.path);
    dispatch('copied', { path: entry.path });
  }

  function handleToggleFavorite(e: MouseEvent) {
    e.stopPropagation();
    dispatch('toggleFavorite', { path: entry.path, isFavorite });
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

  function handleDragStart(e: DragEvent) {
    const path = entry.path.includes(' ') ? `"${entry.path}"` : entry.path;
    e.dataTransfer?.setData('text/plain', path);
  }

  $: status = gitStatuses[entry.path] || '';
  $: children = entry.children || [];
  $: isFavorite = favoritePaths.has(entry.path);
</script>

<div
  class="file-entry {getStatusClass(status)}"
  style="padding-left: {10 + depth * 16}px"
  draggable="true"
  on:dragstart={handleDragStart}
  on:click={handleClick}
  on:keydown
  role="treeitem"
  tabindex="-1"
  title={entry.path}
>
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
  <button class="copy-btn" on:click={handleCopy} title="Pfad kopieren">
    <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
      <path d="M4 4v-2a2 2 0 0 1 2-2h6a2 2 0 0 1 2 2v6a2 2 0 0 1-2 2h-2v2a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2zm2-2v2h2a2 2 0 0 1 2 2v2h2V2H6zM2 6v6h6V6H2z"/>
    </svg>
  </button>
  <button class="star-btn" class:active={isFavorite} on:click={handleToggleFavorite} title={isFavorite ? 'Favorit entfernen' : 'Als Favorit markieren'}>
    <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
      {#if isFavorite}
        <path d="M8 .25a.75.75 0 0 1 .673.418l1.882 3.815 4.21.612a.75.75 0 0 1 .416 1.279l-3.046 2.97.719 4.192a.75.75 0 0 1-1.088.791L8 12.347l-3.766 1.98a.75.75 0 0 1-1.088-.79l.72-4.194L.818 6.374a.75.75 0 0 1 .416-1.28l4.21-.611L7.327.668A.75.75 0 0 1 8 .25z"/>
      {:else}
        <path d="M8 .25a.75.75 0 0 1 .673.418l1.882 3.815 4.21.612a.75.75 0 0 1 .416 1.279l-3.046 2.97.719 4.192a.75.75 0 0 1-1.088.791L8 12.347l-3.766 1.98a.75.75 0 0 1-1.088-.79l.72-4.194L.818 6.374a.75.75 0 0 1 .416-1.28l4.21-.611L7.327.668A.75.75 0 0 1 8 .25zm0 2.445L6.615 5.5a.75.75 0 0 1-.564.41l-3.097.45 2.24 2.184a.75.75 0 0 1 .216.664l-.528 3.084 2.769-1.456a.75.75 0 0 1 .698 0l2.77 1.456-.53-3.084a.75.75 0 0 1 .216-.664l2.24-2.183-3.096-.45a.75.75 0 0 1-.564-.41L8 2.694z"/>
      {/if}
    </svg>
  </button>
</div>

{#if entry.expanded && entry.children}
  {#each children as child (child.path)}
    <svelte:self
      entry={child}
      depth={depth + 1}
      {gitStatuses}
      {copiedPath}
      {favoritePaths}
      on:selectFile
      on:copied
      on:toggleFavorite
    />
  {/each}
{/if}

<style>
  .file-entry {
    display: flex; align-items: center; gap: 6px; width: 100%;
    padding: 3px 10px; background: none; border: none;
    color: var(--fg); font-size: 12px; cursor: pointer; text-align: left;
  }
  .file-entry:hover { background: var(--bg-tertiary); }

  .file-entry.git-modified { color: #e2b93d; }
  .file-entry.git-new { color: #73c991; }
  .file-entry.git-added { color: #73c991; }
  .file-entry.git-deleted { color: #f87171; }
  .file-entry.git-renamed { color: #6bc5d2; }

  .file-icon { font-size: 12px; flex-shrink: 0; }
  .file-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }

  .copy-btn {
    opacity: 0; background: none; border: none; color: var(--fg-muted);
    cursor: pointer; padding: 1px 3px; border-radius: 3px; flex-shrink: 0;
    display: flex; align-items: center; transition: opacity 0.15s;
  }
  .file-entry:hover .copy-btn { opacity: 1; }
  .copy-btn:hover { color: var(--fg); background: var(--bg-secondary); }

  .star-btn {
    opacity: 0; background: none; border: none; color: var(--fg-muted);
    cursor: pointer; padding: 1px 3px; border-radius: 3px; flex-shrink: 0;
    display: flex; align-items: center; transition: opacity 0.15s, color 0.15s;
  }
  .file-entry:hover .star-btn { opacity: 1; }
  .star-btn:hover { color: #eab308; background: var(--bg-secondary); }
  .star-btn.active { opacity: 1; color: #eab308; }

  .git-badge {
    font-size: 10px; font-weight: 700; padding: 0 4px;
    border-radius: 3px; flex-shrink: 0; line-height: 16px;
  }
  .git-badge.git-modified { background: #e2b93d22; color: #e2b93d; }
  .git-badge.git-new { background: #73c99122; color: #73c991; }
  .git-badge.git-added { background: #73c99122; color: #73c991; }
  .git-badge.git-deleted { background: #f8717122; color: #f87171; }
  .git-badge.git-renamed { background: #6bc5d222; color: #6bc5d2; }

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
