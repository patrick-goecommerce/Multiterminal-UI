<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  const dispatch = createEventDispatcher();

  export let paneCount: number = 0;
  export let maxPanes: number = 10;
  export let tabDir: string = '';
  export let canChangeDir: boolean = true;

  function newTerminal() {
    dispatch('newTerminal');
  }

  function toggleSidebar() {
    dispatch('toggleSidebar');
  }

  function changeDir() {
    dispatch('changeDir');
  }

  function openSettings() {
    dispatch('openSettings');
  }

  $: dirLabel = tabDir ? tabDir.replace(/\\/g, '/').split('/').pop() || tabDir : '(kein Verzeichnis)';
  $: atLimit = paneCount >= maxPanes;
</script>

<div class="toolbar">
  <div class="toolbar-left">
    <button
      class="dir-btn"
      class:disabled={!canChangeDir}
      on:click={changeDir}
      disabled={!canChangeDir}
      title={canChangeDir
        ? `Arbeitsverzeichnis ändern: ${tabDir}`
        : `Alle Terminals schließen um Verzeichnis zu ändern (${tabDir})`}
    >
      <span class="dir-icon">&#128194;</span>
      <span class="dir-path" title={tabDir}>{dirLabel}</span>
      {#if canChangeDir}
        <span class="dir-change">&#9998;</span>
      {:else}
        <span class="dir-lock">&#128274;</span>
      {/if}
    </button>
    <span class="toolbar-info">{paneCount} / {maxPanes} Terminals</span>
  </div>
  <div class="toolbar-right">
    <button
      class="toolbar-btn primary"
      class:disabled={atLimit}
      on:click={newTerminal}
      disabled={atLimit}
      title={atLimit ? `Max. ${maxPanes} Terminals erreicht` : 'Neues Terminal (Ctrl+N)'}
    >
      + New Terminal
    </button>
    <button class="toolbar-btn" on:click={toggleSidebar} title="Dateien (Ctrl+B)">
      <span class="icon">&#128193;</span> Files
    </button>
    <button class="toolbar-btn" on:click={openSettings} title="Einstellungen">
      <span class="icon">&#9881;</span>
    </button>
  </div>
</div>

<style>
  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    background: var(--toolbar-bg);
    border-bottom: 1px solid var(--border);
    height: 46px;
    padding: 0 16px;
  }

  .toolbar-left {
    display: flex;
    align-items: center;
    gap: 12px;
    overflow: hidden;
  }

  .toolbar-info {
    font-size: 12px;
    color: var(--fg-muted);
    white-space: nowrap;
  }

  .toolbar-right {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
  }

  .dir-btn {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 4px 10px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--fg);
    font-size: 12px;
    cursor: pointer;
    transition: all 0.15s;
    overflow: hidden;
    max-width: 280px;
  }

  .dir-btn:hover:not(.disabled) {
    border-color: var(--accent);
    background: var(--bg-tertiary);
  }

  .dir-btn.disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .dir-icon {
    font-size: 13px;
    flex-shrink: 0;
  }

  .dir-path {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .dir-change {
    font-size: 11px;
    color: var(--accent);
    flex-shrink: 0;
  }

  .dir-lock {
    font-size: 10px;
    color: var(--fg-muted);
    flex-shrink: 0;
  }

  .toolbar-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 7px 16px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--fg);
    font-size: 13px;
    cursor: pointer;
    transition: all 0.15s;
    white-space: nowrap;
  }

  .toolbar-btn:hover {
    background: var(--accent);
    color: white;
    border-color: var(--accent);
  }

  .toolbar-btn.primary {
    background: var(--accent);
    color: white;
    border-color: var(--accent);
  }

  .toolbar-btn.primary:hover {
    background: var(--accent-hover);
    border-color: var(--accent-hover);
  }

  .toolbar-btn.disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .toolbar-btn.disabled:hover {
    background: var(--bg-tertiary);
    color: var(--fg);
    border-color: var(--border);
  }

  .icon {
    font-size: 15px;
  }
</style>
