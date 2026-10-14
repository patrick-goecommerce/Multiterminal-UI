<script lang="ts">
  import { createEventDispatcher, onMount, tick } from 'svelte';
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

  // Opened at the pointer, then measured: where it would run past the window
  // (the list sits bottom right), it opens upward or leftward instead.
  let menuEl: HTMLDivElement;
  let left = x;
  let top = y;
  onMount(place);
  function place() {
    const r = menuEl.getBoundingClientRect();
    const vw = window.innerWidth;
    const vh = window.innerHeight;
    const l = x + r.width > vw ? x - r.width : x;
    const t = y + r.height > vh ? y - r.height : y;
    left = Math.max(0, Math.min(l, vw - r.width));
    top = Math.max(0, Math.min(t, vh - r.height));
  }

  // Close first and let go of the keyboard, then act: a terminal only takes
  // the keyboard when no button holds it, and the menu's button would.
  async function run(fn: () => void) {
    dispatch('close');
    const ae = document.activeElement;
    if (ae instanceof HTMLElement) ae.blur();
    await tick();
    fn();
  }
  function markSeen(ref: string) {
    App.MarkSessionSeen(ref).catch((err) => console.warn('[attention] MarkSessionSeen failed', ref, err));
  }
  function markUnseen(ref: string) {
    App.MarkSessionUnseen(ref).catch((err) => console.warn('[attention] MarkSessionUnseen failed', ref, err));
  }
  function seenAllDone() {
    for (const p of get(allTabs).flatMap((t) => t.panes)) {
      if (paneRank(p) === RANK_DONE) markSeen(p.sessionId);
    }
  }
</script>

<svelte:window on:keydown={(e) => e.key === 'Escape' && dispatch('close')} />
<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div class="am-overlay" on:click={() => dispatch('close')} on:contextmenu|preventDefault={() => dispatch('close')}>
  <div class="am" role="menu" tabindex="-1" bind:this={menuEl} style="left:{left}px; top:{top}px" on:click|stopPropagation>
    {#if entry}
      <button role="menuitem" on:click={() => run(() => jumpTo(entry.tabId, entry.pane.id))}>Hingehen</button>
      {#if !entry.here && layoutMode === 'focus'}
        <button role="menuitem" on:click={() => run(() => preview(entry, layoutMode))}>Vorschau öffnen</button>
      {/if}
    {/if}
    {#if canSee && target}
      <button role="menuitem" on:click={() => run(() => markSeen(target.sessionId))}>Als gesehen markieren</button>
    {/if}
    {#if canUnsee && target}
      <button role="menuitem" on:click={() => run(() => markUnseen(target.sessionId))}>Als ungelesen markieren</button>
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
