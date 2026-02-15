<script lang="ts">
  import { createEventDispatcher } from 'svelte';
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
  export let sortModifiedFirst: boolean = false;

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
    if (e.shiftKey) {
      e.preventDefault();
      dispatch('shiftclick', { path: entry.path });
      return;
    }
    if (entry.isDir) {
      toggle();
    } else {
      dispatch('selectFile', { path: entry.path });
    }
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
      const aChanged = (gitStatuses[a.path] || '') !== '';
      const bChanged = (gitStatuses[b.path] || '') !== '';
      if (aChanged !== bChanged) return aChanged ? -1 : 1;
      if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
      return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
    });
  }

  $: status = gitStatuses[entry.path] || '';
  $: children = entry.children ? sortEntries(entry.children) : [];
</script>

<button
  class="file-entry {getStatusClass(status)}"
  style="padding-left: {10 + depth * 16}px"
  on:click={handleClick}
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
</button>

{#if entry.expanded && entry.children}
  {#each children as child (child.path)}
    <svelte:self
      entry={child}
      depth={depth + 1}
      {gitStatuses}
      {copiedPath}
      {sortModifiedFirst}
      on:selectFile
      on:shiftclick
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
