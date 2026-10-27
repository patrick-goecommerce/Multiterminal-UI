<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import type { hub } from '../../wailsjs/go/models';
  import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
  import { cardLines, cardPosition, type Rect } from '../lib/briefing';

  export let briefing: hub.Briefing;
  export let project = '';
  export let pane = '';
  /** Where the entry and its panel sit on screen; the card goes left of the panel. */
  export let entryRect: Rect;
  export let panelRect: Rect;

  const WIDTH = 340;
  const dispatch = createEventDispatcher<{ enter: void; leave: void }>();

  let height = 160;
  $: pos = cardPosition(entryRect, panelRect, { width: WIDTH, height }, { width: window.innerWidth, height: window.innerHeight });
  $: lines = cardLines(briefing, { project, pane });
  $: pr = briefing.pr;

  // The PR number inside the "pr" row becomes a button; the rest stays text.
  function split(text: string, number: number): [string, string, string] {
    const token = `#${number}`;
    const i = text.indexOf(token);
    return i < 0 ? [text, '', ''] : [text.slice(0, i), token, text.slice(i + token.length)];
  }
</script>

<div class="card" role="tooltip" style="left:{pos.left}px;top:{pos.top}px;width:{WIDTH}px" bind:clientHeight={height}
  on:mouseenter={() => dispatch('enter')} on:mouseleave={() => dispatch('leave')}>
  {#each lines as line (line.key)}
    {#if line.key === 'pr' && pr?.number}
      {@const [before, token, after] = split(line.text, pr.number)}
      <div class="row pr">{before}{#if token}<button class="prlink" title="Im Browser öffnen" on:click={() => BrowserOpenURL(pr.url)}>{token}</button>{/if}{after}</div>
    {:else}
      <div class="row {line.key}" class:mono={line.key === 'turn' || line.key === 'branch'}>{line.text}</div>
    {/if}
  {/each}
</div>

<style>
  .card { position: fixed; z-index: 1200; box-sizing: border-box; padding: 9px 11px; display: flex; flex-direction: column; gap: 4px; background: var(--pane-bg); border: 1px solid var(--pane-border); border-radius: 6px; box-shadow: 0 6px 22px rgba(0, 0, 0, .35); font: 12px/1.4 var(--font-sans); color: var(--fg); }
  .row { overflow-wrap: anywhere; }
  .row.head { font-weight: 600; }
  .row.mono { font: 11px/1.4 var(--font-mono); white-space: pre; overflow-wrap: normal; }
  .row.prompt, .row.closing { color: var(--fg-muted); }
  .row.warnings { color: var(--status-waiting); font-weight: 600; }
  .prlink { background: none; border: 0; padding: 0; cursor: pointer; color: var(--accent); font: 11px var(--font-mono); text-decoration: underline; }
  .prlink:focus-visible { outline: 2px solid var(--border-focused); outline-offset: 1px; }
</style>
