<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let favorites: string[] = [];
  let collapsed = false;

  const dispatch = createEventDispatcher();

  function fileName(path: string): string {
    const parts = path.replace(/\\/g, '/').split('/');
    return parts[parts.length - 1] || path;
  }

  function isDir(path: string): boolean {
    const name = fileName(path);
    return !name.includes('.');
  }

  function handleClick(path: string) {
    dispatch('selectFile', { path });
  }

  function handleRemove(e: MouseEvent, path: string) {
    e.stopPropagation();
    dispatch('removeFavorite', { path });
  }

  function handleDragStart(e: DragEvent, path: string) {
    const formatted = path.includes(' ') ? `"${path}"` : path;
    e.dataTransfer?.setData('text/plain', formatted);
  }
</script>

{#if favorites.length > 0 || !collapsed}
  <div class="favorites-section">
    <button class="favorites-header" on:click={() => (collapsed = !collapsed)}>
      <span class="collapse-icon">{collapsed ? '\u25B6' : '\u25BC'}</span>
      <span class="favorites-title">{'\u2605'} Favorites</span>
      {#if favorites.length > 0}
        <span class="fav-count">{favorites.length}</span>
      {/if}
    </button>

    {#if !collapsed}
      {#if favorites.length === 0}
        <div class="no-favorites">Keine Favoriten</div>
      {:else}
        {#each favorites as fav (fav)}
          <div
            class="fav-entry"
            draggable="true"
            on:dragstart={(e) => handleDragStart(e, fav)}
            on:click={() => handleClick(fav)}
            on:keydown
            role="treeitem"
            tabindex="-1"
            title={fav}
          >
            <span class="fav-icon">{isDir(fav) ? '\u{1F4C1}' : '\u{1F4C4}'}</span>
            <span class="fav-name">{fileName(fav)}</span>
            <button class="remove-btn" on:click={(e) => handleRemove(e, fav)} title="Favorit entfernen">
              <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
                <path d="M8 1a7 7 0 1 0 0 14A7 7 0 0 0 8 1zm3.5 9.5l-1 1L8 9l-2.5 2.5-1-1L7 8 4.5 5.5l1-1L8 7l2.5-2.5 1 1L9 8z"/>
              </svg>
            </button>
          </div>
        {/each}
      {/if}
    {/if}
  </div>
{/if}

<style>
  .favorites-section {
    border-bottom: 1px solid var(--border);
  }

  .favorites-header {
    display: flex; align-items: center; gap: 6px; width: 100%;
    padding: 6px 10px; background: none; border: none;
    color: var(--fg); font-size: 11px; font-weight: 600;
    cursor: pointer; text-align: left;
  }
  .favorites-header:hover { background: var(--bg-tertiary); }

  .collapse-icon { font-size: 8px; width: 10px; flex-shrink: 0; }
  .favorites-title { flex: 1; }

  .fav-count {
    font-size: 10px; font-weight: 700; background: var(--bg-tertiary);
    border-radius: 8px; padding: 0 5px; line-height: 16px; min-width: 16px;
    text-align: center; color: var(--fg-muted);
  }

  .no-favorites {
    padding: 8px 10px; font-size: 11px; color: var(--fg-muted);
    font-style: italic;
  }

  .fav-entry {
    display: flex; align-items: center; gap: 6px;
    padding: 3px 10px 3px 20px; cursor: pointer;
    color: var(--fg); font-size: 12px;
  }
  .fav-entry:hover { background: var(--bg-tertiary); }

  .fav-icon { font-size: 12px; flex-shrink: 0; }
  .fav-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }

  .remove-btn {
    opacity: 0; background: none; border: none; color: var(--fg-muted);
    cursor: pointer; padding: 1px 3px; border-radius: 3px; flex-shrink: 0;
    display: flex; align-items: center; transition: opacity 0.15s;
  }
  .fav-entry:hover .remove-btn { opacity: 1; }
  .remove-btn:hover { color: #f87171; background: var(--bg-secondary); }
</style>
