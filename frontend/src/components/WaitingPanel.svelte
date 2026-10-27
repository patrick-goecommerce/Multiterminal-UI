<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte';
  import StatusGlyph from './StatusGlyph.svelte';
  import BriefingCard from './BriefingCard.svelte';
  import { briefings } from '../stores/briefings';
  import { verdict } from '../lib/briefing';
  import { createCardHover } from '../lib/cardHover';
  import { paneDisplayName } from '../stores/tabs';
  import { glyphKind, ENTRY_LABEL, type AttentionEntry } from '../lib/attention';
  import { formatDuration } from '../lib/duration';
  import { now } from '../stores/clock';

  export let rows: AttentionEntry[] = [];
  export let total = 0;
  export let hiddenDone = 0;
  export let expanded = false;
  /** More finished entries than the panel keeps; without that, no toggle. */
  export let collapsible = false;
  export let selectedPaneId = '';

  const dispatch = createEventDispatcher<{
    jump: AttentionEntry; preview: AttentionEntry; toggleMore: void;
    menu: { entry: AttentionEntry; x: number; y: number };
  }>();

  // A new permission or question lights up once; nothing pulses for long.
  let known = new Set<string>();
  let flashing = new Set<string>();
  let primed = false;
  $: noteNew(rows);
  function noteNew(list: AttentionEntry[]) {
    const keys = list.map((e) => `${e.pane.id}:${e.since}:${e.rank}`);
    if (primed) {
      const fresh = list.filter((e, i) => e.rank <= 1 && !known.has(keys[i])).map((e) => e.pane.id);
      if (fresh.length) {
        flashing = new Set(fresh);
        setTimeout(() => { flashing = new Set(); }, 1300);
      }
    }
    known = new Set(keys);
    primed = true;
  }

  // The briefing card: only for finished or failed entries that have one.
  const cardOf = (e: AttentionEntry) => (e.rank === 2 || e.rank === 3 ? $briefings[e.pane.sessionId] : undefined);
  const hover = createCardHover(300, 120);
  const shown = hover.shown;
  $: shownEntry = rows.find((e) => e.pane.id === $shown?.paneId);
  // $briefings stays in this statement: $: does not track what cardOf reads.
  $: shownBriefing = shownEntry && (shownEntry.rank === 2 || shownEntry.rank === 3) ? $briefings[shownEntry.pane.sessionId] : undefined;
  onDestroy(hover.destroy);
  // A click focuses the entry too; only keyboard focus opens the card at once.
  let pointerDown = false;
  function onFocus(e: Event, entry: AttentionEntry) {
    if (!pointerDown) over(e, entry, false);
  }
  function over(e: Event, entry: AttentionEntry, delay: boolean) {
    if (!cardOf(entry)) return;
    const el = e.currentTarget as HTMLElement;
    hover.enter(entry.pane.id, () => ({ entry: el.getBoundingClientRect(), panel: (el.closest('.wp') ?? el).getBoundingClientRect() }), delay);
  }

  function activate(e: MouseEvent | KeyboardEvent, entry: AttentionEntry) {
    dispatch(e.altKey ? 'preview' : 'jump', entry);
  }
  function onKey(e: KeyboardEvent, entry: AttentionEntry) {
    if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); activate(e, entry); }
  }
  function onMenu(e: MouseEvent, entry: AttentionEntry) {
    e.preventDefault();
    dispatch('menu', { entry, x: e.clientX, y: e.clientY });
  }
  function tip(entry: AttentionEntry): string {
    const src = entry.pane.attention?.source;
    const at = entry.since ? new Date(entry.since * 1000).toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' }) : '';
    return [src ? `Quelle: ${src}` : '', at ? `Seit: ${at} Uhr` : '', 'Alt+Klick: Vorschau · Rechtsklick: mehr'].filter(Boolean).join('\n');
  }
</script>

<svelte:window on:mouseup={() => (pointerDown = false)} />

{#if total === 0}
  <div class="wp quiet"><span class="wp-title">Wartet auf dich</span><span class="wp-q">Nichts wartet</span></div>
{:else}
  <div class="wp">
    <div class="wp-head">
      <span class="wp-title">Wartet auf dich</span><span class="wp-n">{total}</span>
      <kbd class="wp-key" title="Nächsten Eintrag öffnen">Strg+Umschalt+J</kbd>
    </div>
    <div class="wp-rows">
      {#each rows as entry (entry.pane.id)}
        <div class="wp-entry r{entry.rank}" class:sel={entry.pane.id === selectedPaneId} class:flash={flashing.has(entry.pane.id)}
          role="button" tabindex="0" title={tip(entry)}
          aria-label="{ENTRY_LABEL[entry.rank]}: {entry.here ? 'dieses Projekt' : entry.tabName}, {paneDisplayName(entry.pane)}"
          on:mouseenter={(e) => over(e, entry, true)} on:mouseleave={hover.leave}
          on:mousedown={() => (pointerDown = true)} on:mouseup={() => (pointerDown = false)}
          on:focus={(e) => onFocus(e, entry)} on:blur={hover.leave}
          on:click={(e) => activate(e, entry)} on:keydown={(e) => onKey(e, entry)} on:contextmenu={(e) => onMenu(e, entry)}>
          <div class="l1">
            <StatusGlyph kind={glyphKind(entry.pane)} />
            <span class="proj" class:here={entry.here}>{entry.here ? 'hier' : entry.tabName}</span>
            <span class="sep">·</span>
            <span class="pane">{paneDisplayName(entry.pane)}</span>
            <span class="age">{entry.since ? formatDuration(entry.since, $now) : ''}</span>
          </div>
          {#if entry.rank === 2 || entry.rank === 3}
            {@const v = verdict($briefings[entry.pane.sessionId], entry.pane.attention)}
            <div class="l2 {v.kind}" class:mono={v.kind === 'detail' && entry.rank === 2}>{v.text || ENTRY_LABEL[entry.rank]}</div>
          {:else}
            <div class="l2" class:mono={entry.rank === 0}>{entry.pane.attention?.detail || ENTRY_LABEL[entry.rank]}</div>
          {/if}
        </div>
      {/each}
      {#if collapsible}
        <button class="more" on:click={() => dispatch('toggleMore')}>{expanded ? 'weniger anzeigen' : `+${hiddenDone} weitere fertig`}</button>
      {/if}
    </div>
  </div>
  {#if $shown && shownEntry && shownBriefing}
    <BriefingCard briefing={shownBriefing} project={shownEntry.here ? 'hier' : shownEntry.tabName} pane={paneDisplayName(shownEntry.pane)}
      entryRect={$shown.entry} panelRect={$shown.panel} on:enter={hover.keep} on:leave={hover.leave} />
  {/if}
{/if}

<style>
  .wp { flex: 1; min-width: 0; min-height: 0; display: flex; flex-direction: column; background: var(--pane-bg); border: 1px solid var(--pane-border); border-radius: 6px; overflow: hidden; font-family: var(--font-sans); }
  .wp.quiet { flex-direction: row; align-items: center; gap: 10px; padding: 0 12px; }
  .wp-q { color: var(--fg-muted); font-size: 12px; }
  .wp-head { flex: none; display: flex; align-items: center; gap: 8px; padding: 7px 10px; border-bottom: 1px solid var(--pane-border); font-size: 12px; }
  .wp-title { font-weight: 600; font-size: 12px; }
  .wp-n { font: 500 11px var(--font-mono); padding: 0 6px; border-radius: 9px; background: var(--bg-tertiary); font-variant-numeric: tabular-nums; }
  .wp-key { margin-left: auto; font: 10.5px var(--font-mono); color: var(--fg-muted); border: 1px solid var(--border); border-radius: 4px; padding: 1px 5px; }
  .wp-rows { overflow-y: auto; min-height: 0; padding: 4px; display: flex; flex-direction: column; gap: 2px; }
  .wp-entry { padding: 5px 7px; border-radius: 5px; cursor: pointer; display: flex; flex-direction: column; gap: 2px; border: 1px solid transparent; }
  .wp-entry:hover { background: var(--bg-tertiary); }
  .wp-entry.r0 { background: var(--status-waiting-tint); }
  .wp-entry.sel { border-color: var(--border-focused); }
  .wp-entry:focus-visible { outline: 2px solid var(--border-focused); outline-offset: -1px; }
  .wp-entry.flash { animation: wp-flash 1.2s ease-out 1; }
  .l1 { display: flex; align-items: center; gap: 6px; font-size: 12px; min-width: 0; }
  .proj { font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; flex: 0 1 auto; }
  .proj.here { color: var(--accent); }
  .sep { color: var(--fg-muted); flex: none; }
  .pane { color: var(--fg-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; flex: 0 1 auto; }
  .age { margin-left: auto; padding-left: 6px; flex: none; font: 10.5px var(--font-mono); color: var(--fg-muted); font-variant-numeric: tabular-nums; }
  .l2 { padding-left: 18px; font-size: 11.5px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .l2.warn { color: var(--status-waiting); }
  .l2.pending, .l2.clean { color: var(--fg-muted); }
  .l2.mono { font: 11px var(--font-mono); }
  .more { align-self: flex-start; margin: 2px 0 4px 18px; background: none; border: 0; padding: 3px 0; color: var(--fg-muted); font: 500 11.5px var(--font-sans); cursor: pointer; }
  .more:hover { color: var(--fg); }
  @keyframes wp-flash {
    0% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--status-waiting) 70%, transparent); background: color-mix(in srgb, var(--status-waiting) 38%, transparent); }
    100% { box-shadow: 0 0 0 10px transparent; }
  }
  @media (prefers-reduced-motion: reduce) { .wp-entry.flash { animation: none; } }
</style>
