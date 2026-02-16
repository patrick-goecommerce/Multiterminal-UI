<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';

  export let x: number = 0;
  export let y: number = 0;
  export let visible: boolean = false;
  export let hasSelection: boolean = false;

  const dispatch = createEventDispatcher();

  let menuEl: HTMLDivElement;

  function handleAction(action: string) {
    dispatch('action', { action });
  }

  function handleClickOutside(e: MouseEvent) {
    if (menuEl && !menuEl.contains(e.target as Node)) {
      dispatch('close');
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('close');
  }

  onMount(() => {
    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleKeydown);
  });

  onDestroy(() => {
    document.removeEventListener('mousedown', handleClickOutside);
    document.removeEventListener('keydown', handleKeydown);
  });

  $: style = (() => {
    const menuW = 180;
    const menuH = 240;
    const clampedX = Math.min(x, window.innerWidth - menuW);
    const clampedY = Math.min(y, window.innerHeight - menuH);
    return `left: ${clampedX}px; top: ${clampedY}px;`;
  })();
</script>

{#if visible}
  <div class="context-menu" bind:this={menuEl} style={style}>
    <button class="ctx-item" class:disabled={!hasSelection} on:click={() => handleAction('copy')} disabled={!hasSelection}>
      <span class="ctx-icon">&#xe16f;</span> Kopieren <span class="ctx-shortcut">Ctrl+C</span>
    </button>
    <button class="ctx-item" on:click={() => handleAction('paste')}>
      <span class="ctx-icon">&#xe172;</span> Einfügen <span class="ctx-shortcut">Ctrl+V</span>
    </button>
    <div class="ctx-separator"></div>
    <button class="ctx-item" on:click={() => handleAction('selectAll')}>
      <span class="ctx-icon">&#x2610;</span> Alles auswählen
    </button>
    <button class="ctx-item" on:click={() => handleAction('search')}>
      <span class="ctx-icon">&#x2315;</span> Suchen <span class="ctx-shortcut">Ctrl+F</span>
    </button>
    <div class="ctx-separator"></div>
    <button class="ctx-item" on:click={() => handleAction('clear')}>
      <span class="ctx-icon">&#x2327;</span> Terminal leeren
    </button>
    <button class="ctx-item" on:click={() => handleAction('splitPane')}>
      <span class="ctx-icon">&#x229e;</span> Neues Terminal <span class="ctx-shortcut">Ctrl+N</span>
    </button>
  </div>
{/if}

<style>
  .context-menu {
    position: fixed;
    z-index: 1000;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
    padding: 4px 0;
    min-width: 180px;
    animation: ctx-fade-in 0.1s ease-out;
  }

  @keyframes ctx-fade-in {
    from { opacity: 0; transform: scale(0.95); }
    to { opacity: 1; transform: scale(1); }
  }

  .ctx-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 6px 12px;
    background: none;
    border: none;
    color: var(--fg);
    font-size: 12px;
    cursor: pointer;
    text-align: left;
    transition: background 0.1s;
  }

  .ctx-item:hover:not(:disabled) {
    background: var(--bg-tertiary);
  }

  .ctx-item:disabled {
    color: var(--fg-muted);
    cursor: default;
    opacity: 0.5;
  }

  .ctx-icon {
    width: 16px;
    text-align: center;
    font-size: 13px;
    flex-shrink: 0;
  }

  .ctx-shortcut {
    margin-left: auto;
    color: var(--fg-muted);
    font-size: 10px;
  }

  .ctx-separator {
    height: 1px;
    background: var(--border);
    margin: 4px 8px;
  }
</style>
