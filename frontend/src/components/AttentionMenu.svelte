<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { get } from 'svelte/store';
  import * as App from '../../wailsjs/go/backend/App';
  import { allTabs, type Pane } from '../stores/tabs';
  import { paneRank, RANK_DONE, RANK_ERROR, type AttentionEntry } from '../lib/attention';
  import { jumpTo, preview } from '../lib/attentionNav';
  import type { LayoutMode } from '../lib/focusLayout';

  /** Either an entry of the waiting list, or a pane (its title bar). */
  export let entry: AttentionEntry | null = null;
  export let pane: Pane | null = null;
  export let x = 0;
  export let y = 0;
  export let layoutMode: LayoutMode = 'grid';

  const dispatch = createEventDispatcher<{ close: void }>();
  const target = entry ? entry.pane : pane;
  const rank = target ? paneRank(target) : 99;
  const att = target?.attention;
  const canSee = rank === RANK_ERROR || rank === RANK_DONE;
  const canUnsee = !!att && att.seen && (att.state === 'done' || att.state === 'error');

  function run(fn: () => void) { fn(); dispatch('close'); }
  function seenAllDone() {
    for (const p of get(allTabs).flatMap((t) => t.panes)) {
      if (paneRank(p) === RANK_DONE) App.MarkSessionSeen(p.sessionId).catch(() => {});
    }
  }
</script>

<svelte:window on:keydown={(e) => e.key === 'Escape' && dispatch('close')} />
<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div class="am-overlay" on:click={() => dispatch('close')} on:contextmenu|preventDefault={() => dispatch('close')}>
  <div class="am" role="menu" tabindex="-1" style="left:{x}px; top:{y}px" on:click|stopPropagation>
    {#if entry}
      <button role="menuitem" on:click={() => run(() => jumpTo(entry.tabId, entry.pane.id))}>Hingehen</button>
      {#if !entry.here && layoutMode === 'focus'}
        <button role="menuitem" on:click={() => run(() => preview(entry, layoutMode))}>Vorschau öffnen</button>
      {/if}
    {/if}
    {#if canSee && target}
      <button role="menuitem" on:click={() => run(() => App.MarkSessionSeen(target.sessionId).catch(() => {}))}>Als gesehen markieren</button>
    {/if}
    {#if canUnsee && target}
      <button role="menuitem" on:click={() => run(() => App.MarkSessionUnseen(target.sessionId).catch(() => {}))}>Als ungelesen markieren</button>
    {/if}
    {#if entry}
      <div class="am-sep"></div>
      <button role="menuitem" on:click={() => run(seenAllDone)}>Alle fertigen als gesehen markieren</button>
    {/if}
    {#if !entry && !canSee && !canUnsee}
      <button role="menuitem" on:click={() => dispatch('close')}>Nichts zu markieren</button>
    {/if}
  </div>
</div>

<style>
  .am-overlay { position: fixed; inset: 0; z-index: 1000; }
  .am { position: fixed; min-width: 250px; display: flex; flex-direction: column; padding: 4px; background: var(--bg-secondary); border: 1px solid var(--border); border-radius: 6px; box-shadow: 0 10px 30px rgba(0,0,0,.4); }
  .am button { text-align: left; background: none; border: 0; color: var(--fg); font: 12.5px var(--font-sans); padding: 6px 10px; border-radius: 4px; cursor: pointer; }
  .am button:hover { background: var(--bg-tertiary); }
  .am-sep { border-top: 1px solid var(--border); margin: 4px 0; }
</style>
