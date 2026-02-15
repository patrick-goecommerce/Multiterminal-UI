<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import type { SearchAddon } from '@xterm/addon-search';

  export let searchAddon: SearchAddon | null = null;

  const dispatch = createEventDispatcher();

  let searchQuery = '';
  let searchInput: HTMLInputElement;

  export function open() {
    requestAnimationFrame(() => {
      searchInput?.focus();
      searchInput?.select();
    });
  }

  function close() {
    searchQuery = '';
    searchAddon?.clearDecorations();
    dispatch('close');
  }

  function doSearch(direction: 'next' | 'prev' = 'next') {
    if (!searchAddon || !searchQuery) return;
    const opts = {
      regex: false,
      caseSensitive: false,
      wholeWord: false,
      decorations: {
        matchOverviewRuler: '#888',
        activeMatchColorOverviewRuler: '#ffaa00',
        matchBackground: '#44475a',
        activeMatchBackground: '#ffaa0066',
      },
    };
    if (direction === 'prev') {
      searchAddon.findPrevious(searchQuery, opts);
    } else {
      searchAddon.findNext(searchQuery, opts);
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault();
      close();
    } else if (e.key === 'Enter' && e.shiftKey) {
      e.preventDefault();
      doSearch('prev');
    } else if (e.key === 'Enter') {
      e.preventDefault();
      doSearch('next');
    }
  }
</script>

<div class="search-bar">
  <input
    class="search-input"
    type="text"
    placeholder="Suchen... (Enter=weiter, Shift+Enter=zurück)"
    bind:value={searchQuery}
    bind:this={searchInput}
    on:input={() => doSearch('next')}
    on:keydown={handleKeydown}
    on:click|stopPropagation
  />
  <button class="search-btn" on:click|stopPropagation={() => doSearch('prev')} title="Vorheriger (Shift+Enter)">&#x25B2;</button>
  <button class="search-btn" on:click|stopPropagation={() => doSearch('next')} title="Nächster (Enter)">&#x25BC;</button>
  <button class="search-btn close" on:click|stopPropagation={close} title="Schließen (Esc)">&times;</button>
</div>

<style>
  .search-bar {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 8px;
    background: var(--bg-tertiary);
    border-bottom: 1px solid var(--border);
  }

  .search-input {
    flex: 1;
    padding: 4px 8px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--fg);
    font-size: 12px;
    outline: none;
  }

  .search-input:focus { border-color: var(--accent); }
  .search-input::placeholder { color: var(--fg-muted); }

  .search-btn {
    background: none;
    border: none;
    color: var(--fg-muted);
    cursor: pointer;
    padding: 2px 6px;
    font-size: 12px;
    border-radius: 3px;
  }

  .search-btn:hover { background: var(--bg-secondary); color: var(--fg); }
  .search-btn.close:hover { background: var(--error); color: white; }
</style>
