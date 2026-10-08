<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { t } from '../stores/i18n';
  import StatusGlyph from './StatusGlyph.svelte';
  import WaitingPanel from './WaitingPanel.svelte';
  import AttentionMenu from './AttentionMenu.svelte';
  import { glyphKind, panelRows, type AttentionEntry } from '../lib/attention';
  import { jumpTo, preview, walkPaneId } from '../lib/attentionNav';
  import type { LayoutMode } from '../lib/focusLayout';

  const dispatch = createEventDispatcher();

  export let paneCount: number = 0;
  export let maxPanes: number = 10;
  export let tabDir: string = '';
  export let canChangeDir: boolean = true;
  /** Everything that waits, from every tab (attentionList). */
  export let waiting: AttentionEntry[] = [];
  /** Shown where the focus layout's box is not: the grid, or a maximized pane. */
  export let showCounter = false;
  export let layoutMode: LayoutMode = 'grid';

  let open = false;
  let expanded = false;
  let menuFor: { entry: AttentionEntry; x: number; y: number } | null = null;
  $: rows = panelRows(waiting, expanded);

  function newTerminal() {
    dispatch('newTerminal');
  }

  function toggleSidebar() {
    dispatch('toggleSidebar');
  }

  function changeDir() {
    dispatch('changeDir');
  }

  function openCommands() {
    dispatch('openCommands');
  }

  $: dirLabel = tabDir ? tabDir.replace(/\\/g, '/').split('/').pop() || tabDir : $t('toolbar.noDir');
  $: atLimit = paneCount >= maxPanes;
</script>

<svelte:window on:keydown={(e) => e.key === 'Escape' && (open = false)} />

<div class="toolbar">
  <div class="toolbar-left">
    <button
      class="dir-btn"
      class:disabled={!canChangeDir}
      on:click={changeDir}
      disabled={!canChangeDir}
      title={canChangeDir
        ? $t('toolbar.changeDirEnabled', { dir: tabDir })
        : $t('toolbar.changeDirDisabled', { dir: tabDir })}
    >
      <span class="dir-icon">&#128194;</span>
      <span class="dir-path" title={tabDir}>{dirLabel}</span>
      {#if canChangeDir}
        <span class="dir-change">&#9998;</span>
      {:else}
        <span class="dir-lock">&#128274;</span>
      {/if}
    </button>
    <span class="toolbar-info">{$t('toolbar.paneCount', { count: paneCount, max: maxPanes })}</span>
  </div>
  <div class="toolbar-right">
    {#if showCounter && waiting.length > 0}
      <div class="wait-wrap">
        <button class="wait-counter rank-{waiting[0].rank}" aria-expanded={open} on:click={() => (open = !open)}>
          <StatusGlyph kind={glyphKind(waiting[0].pane)} /> {waiting.length} warten
        </button>
        {#if open}
          <div class="wait-pop">
            <WaitingPanel rows={rows.rows} total={waiting.length} hiddenDone={rows.hiddenDone}
              collapsible={rows.collapsible} {expanded}
              selectedPaneId={$walkPaneId}
              on:jump={(e) => { open = false; jumpTo(e.detail.tabId, e.detail.pane.id); }}
              on:preview={(e) => { open = false; preview(e.detail, layoutMode); }}
              on:toggleMore={() => (expanded = !expanded)}
              on:menu={(e) => (menuFor = e.detail)} />
          </div>
        {/if}
      </div>
    {/if}
    <button
      class="toolbar-btn primary"
      class:disabled={atLimit}
      on:click={newTerminal}
      disabled={atLimit}
      title={atLimit ? $t('toolbar.maxReached', { max: maxPanes }) : $t('toolbar.newTerminal')}
    >
      + {$t('toolbar.newTerminal').replace(/ *\(.*\)/, '')}
    </button>
    <button class="toolbar-btn" on:click={openCommands} title={$t('toolbar.commandPalette')}>
      <span class="icon">&#9889;</span> {$t('toolbar.commandPalette')}
    </button>
    <button class="toolbar-btn" on:click={toggleSidebar} title={$t('toolbar.files')}>
      <span class="icon">&#128193;</span> {$t('toolbar.files').replace(/ *\(.*\)/, '')}
    </button>
  </div>
</div>

{#if menuFor}
  <AttentionMenu entry={menuFor.entry} x={menuFor.x} y={menuFor.y} {layoutMode} on:close={() => (menuFor = null)} />
{/if}

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

  .wait-wrap { position: relative; }
  .wait-counter { display: inline-flex; align-items: center; gap: 7px; padding: 5px 10px; border-radius: 6px; font: 500 12.5px var(--font-sans); color: var(--fg); cursor: pointer; background: var(--status-done-tint); border: 1px solid color-mix(in srgb, var(--status-done) 45%, transparent); }
  .wait-counter.rank-0, .wait-counter.rank-1 { background: var(--status-waiting-tint); border-color: color-mix(in srgb, var(--status-waiting) 50%, transparent); }
  .wait-counter.rank-2 { background: var(--status-danger-tint); border-color: color-mix(in srgb, var(--status-danger) 50%, transparent); }
  .wait-pop { position: absolute; top: calc(100% + 6px); right: 0; width: 400px; max-height: 60vh; display: flex; z-index: 50; box-shadow: 0 14px 40px rgba(0,0,0,.4); border-radius: 6px; }

  .mute-btn.muted {
    opacity: 0.5;
  }
</style>
