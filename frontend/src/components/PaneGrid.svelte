<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import TerminalPane from './TerminalPane.svelte';
  import type { Pane } from '../stores/tabs';

  export let panes: Pane[] = [];

  const dispatch = createEventDispatcher();

  function handleClose(e: CustomEvent) {
    dispatch('closePane', e.detail);
  }

  function handleMaximize(e: CustomEvent) {
    dispatch('maximizePane', e.detail);
  }

  function handleFocus(e: CustomEvent) {
    dispatch('focusPane', e.detail);
  }

  function handleRename(e: CustomEvent) {
    dispatch('renamePane', e.detail);
  }

  $: maximizedPane = panes.find((p) => p.maximized);
  $: visiblePanes = maximizedPane ? [maximizedPane] : panes;
  $: gridCols = maximizedPane ? 1 : Math.min(Math.ceil(Math.sqrt(panes.length)), 3);
</script>

<div
  class="pane-grid"
  style="grid-template-columns: repeat({gridCols}, 1fr);"
>
  {#each visiblePanes as pane (pane.id)}
    <TerminalPane
      {pane}
      on:close={handleClose}
      on:maximize={handleMaximize}
      on:focus={handleFocus}
      on:rename={handleRename}
    />
  {/each}

  {#if panes.length === 0}
    <div class="empty-state">
      <p>Kein Terminal offen.</p>
      <p class="hint">Drücke <kbd>Ctrl+N</kbd> oder klicke <strong>+ New Terminal</strong> (max. 10 pro Tab)</p>
    </div>
  {/if}
</div>

<style>
  .pane-grid {
    display: grid;
    gap: 4px;
    padding: 4px;
    flex: 1;
    overflow: hidden;
    grid-auto-rows: 1fr;
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: var(--fg-muted);
    font-size: 14px;
    grid-column: 1 / -1;
  }

  .empty-state p {
    margin: 4px 0;
  }

  .hint {
    font-size: 12px;
  }

  kbd {
    background: var(--bg-tertiary);
    padding: 2px 6px;
    border-radius: 4px;
    font-family: monospace;
    font-size: 11px;
  }
</style>
