<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime';

  export let dir: string = '';
  export let gitStatuses: Record<string, string> = {};
  export let conflictFiles: string[] = [];
  export let conflictOperation: string = '';

  const dispatch = createEventDispatcher();

  interface ScEntry { path: string; name: string; relPath: string; status: string; }
  interface ScGroup { label: string; code: string; entries: ScEntry[]; }

  const statusGroups = [
    { label: 'Konflikte', code: 'U' },
    { label: 'Modified', code: 'M' },
    { label: 'Added', code: 'A' },
    { label: 'Untracked', code: '?' },
    { label: 'Deleted', code: 'D' },
    { label: 'Renamed', code: 'R' },
  ];

  let copiedPath = '';
  let copiedTimer: ReturnType<typeof setTimeout> | null = null;

  function setCopied(path: string) {
    copiedPath = path;
    if (copiedTimer) clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => { copiedPath = ''; }, 1500);
  }

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
      case 'U': return 'git-conflict';
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
</script>

<div class="file-list">
  {#if conflictFiles.length > 0 && conflictOperation}
    <div class="sc-operation-banner">
      {conflictOperation === 'merge' ? 'Merge' :
       conflictOperation === 'rebase' ? 'Rebase' : 'Cherry-Pick'}
      in Bearbeitung
    </div>
  {/if}
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
            <span class="sc-badge {getStatusClass(entry.status)}">{entry.status === '?' ? 'N' : entry.status === 'U' ? 'C' : entry.status}</span>
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

<style>
  .file-list { flex: 1; overflow-y: auto; padding: 4px 0; }

  .no-results { padding: 12px; text-align: center; color: var(--fg-muted); font-size: 12px; }

  .sc-operation-banner {
    padding: 6px 10px; font-size: 11px; font-weight: 600;
    color: #f97316; background: rgba(249, 115, 22, 0.08);
    border-bottom: 1px solid var(--border);
  }

  .sc-group-header {
    font-size: 11px; font-weight: 600; color: var(--fg-muted);
    padding: 8px 10px 4px; text-transform: uppercase; letter-spacing: 0.5px;
  }

  .sc-entry {
    display: flex; align-items: center; gap: 6px;
    padding: 3px 10px; cursor: pointer; font-size: 12px;
  }
  .sc-entry:hover { background: var(--bg-tertiary); }

  .sc-entry.git-conflict { color: #f97316; }
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
  .sc-badge.git-conflict { background: #f9731622; color: #f97316; }
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
