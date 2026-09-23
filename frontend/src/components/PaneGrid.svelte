<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte';
  import { t } from '../stores/i18n';
  import TerminalPane from './TerminalPane.svelte';
  import ChatPane from './ChatPane.svelte';
  import { tabStore, allTabs, paneDisplayName, type Pane } from '../stores/tabs';
  import { config } from '../stores/config';
  import { now } from '../stores/clock';
  import { formatDuration } from '../lib/duration';
  import {
    focusGeometry, floatingRect, promoteInOrder, reconcileOrder, slotFor, stateLabel,
    waitingElsewhere, FLOAT_BAR, type LayoutMode, type Slot, type WaitingEntry,
  } from '../lib/focusLayout';
  import { focusOrders, setFocusOrder, floatingPane, saveFloatPosition, consumePlaced } from '../stores/focusLayout';

  export let panes: Pane[] = [];
  export let active: boolean = true;
  export let tabId: string = '';
  export let tabDir: string = '';
  export let colFractions: number[] | undefined = undefined;
  export let rowFractions: number[] | undefined = undefined;
  export let tabName: string = '';
  export let layoutMode: LayoutMode = 'grid';

  const dispatch = createEventDispatcher();

  // Keep these in sync with the .pane-grid CSS below — used to convert
  // pixel drag deltas into fr-unit fraction deltas.
  const GRID_GAP = 4;
  const GRID_PADDING = 4;
  const MIN_TRACK_PX = 150;

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

  function handleRestart(e: CustomEvent) {
    dispatch('restartPane', e.detail);
  }

  function handleIssueAction(e: CustomEvent) {
    dispatch('issueAction', e.detail);
  }

  function handleCommitPush(e: CustomEvent) {
    dispatch('commitPush', e.detail);
  }

  function handleFinishWorktree(e: CustomEvent) {
    dispatch('finishWorktree', e.detail);
  }

  function handleQuickAction(e: CustomEvent) {
    dispatch('quickAction', e.detail);
  }

  function handleCancelFinish(e: CustomEvent) {
    dispatch('cancelFinish', e.detail);
  }

  function handleNavigateFile(e: CustomEvent) {
    dispatch('navigateFile', e.detail);
  }

  function handleSplitPane() {
    dispatch('splitPane');
  }

  function handleToggleDisplay(e: CustomEvent) {
    dispatch('toggleDisplayPane', e.detail);
  }

  $: maximizedPane = panes.find((p) => p.maximized);
  $: visiblePanes = maximizedPane ? [maximizedPane] : panes;
  $: focus = layoutMode === 'focus';
  // Focus mode renders every pane all the time and only moves them: a pane
  // that left the DOM would lose its terminal. Grid mode keeps its old
  // behaviour of rendering only the maximized pane.
  $: renderPanes = focus ? panes : visiblePanes;
  // A tab with no panes still needs a 1x1 shape: a zero column count would
  // make gridRows NaN (0/0), and Array(NaN) throws.
  $: gridCols = maximizedPane ? 1 : Math.max(1, Math.min(Math.ceil(Math.sqrt(panes.length)), 3));
  $: gridRows = Math.max(1, Math.ceil(visiblePanes.length / gridCols));

  // Fall back to equal fractions whenever the saved array's length doesn't
  // match the current grid shape (pane added/removed since it was saved) —
  // never apply a stale/misaligned sizing.
  $: colFr = (colFractions && colFractions.length === gridCols) ? colFractions : Array(gridCols).fill(1);
  $: rowFr = (rowFractions && rowFractions.length === gridRows) ? rowFractions : Array(gridRows).fill(1);

  // Live-drag state: overrides colFr/rowFr for immediate visual feedback
  // without writing to the store on every mousemove (only committed on
  // mouseup, mirroring the zoomDelta live-then-commit pattern).
  let liveColFr: number[] | null = null;
  let liveRowFr: number[] | null = null;
  $: displayColFr = liveColFr ?? colFr;
  $: displayRowFr = liveRowFr ?? rowFr;

  let gridEl: HTMLDivElement;
  let dragging: 'col' | 'row' | null = null;

  function cumulativePercents(fr: number[]): number[] {
    const total = fr.reduce((a, b) => a + b, 0) || 1;
    const percents: number[] = [];
    let acc = 0;
    for (let i = 0; i < fr.length - 1; i++) {
      acc += fr[i];
      percents.push((acc / total) * 100);
    }
    return percents;
  }

  $: colBoundaryPercents = gridCols > 1 ? cumulativePercents(displayColFr) : [];
  $: rowBoundaryPercents = gridRows > 1 ? cumulativePercents(displayRowFr) : [];

  let dragStartFr: number[] = [];
  let dragIndex = 0;
  let dragStartPos = 0;
  let dragContainerSize = 0;

  function startColDrag(index: number, e: MouseEvent) {
    if (!gridEl) return;
    dragging = 'col';
    dragIndex = index;
    dragStartFr = [...colFr];
    dragStartPos = e.clientX;
    dragContainerSize = gridEl.clientWidth - 2 * GRID_PADDING - (gridCols - 1) * GRID_GAP;
    window.addEventListener('mousemove', onColDragMove);
    window.addEventListener('mouseup', onColDragEnd);
    e.preventDefault();
  }

  function onColDragMove(e: MouseEvent) {
    if (dragContainerSize <= 0) return;
    const total = dragStartFr.reduce((a, b) => a + b, 0);
    const deltaPx = e.clientX - dragStartPos;
    let deltaFraction = (deltaPx / dragContainerSize) * total;

    const minFraction = (MIN_TRACK_PX / dragContainerSize) * total;
    const maxNeg = -(dragStartFr[dragIndex] - minFraction);
    const maxPos = dragStartFr[dragIndex + 1] - minFraction;
    deltaFraction = Math.max(maxNeg, Math.min(maxPos, deltaFraction));

    liveColFr = dragStartFr.map((f, i) =>
      i === dragIndex ? f + deltaFraction : i === dragIndex + 1 ? f - deltaFraction : f
    );
  }

  function onColDragEnd() {
    window.removeEventListener('mousemove', onColDragMove);
    window.removeEventListener('mouseup', onColDragEnd);
    if (liveColFr) tabStore.setGridFractions(tabId, liveColFr, rowFr);
    liveColFr = null;
    dragging = null;
  }

  function startRowDrag(index: number, e: MouseEvent) {
    if (!gridEl) return;
    dragging = 'row';
    dragIndex = index;
    dragStartFr = [...rowFr];
    dragStartPos = e.clientY;
    dragContainerSize = gridEl.clientHeight - 2 * GRID_PADDING - (gridRows - 1) * GRID_GAP;
    window.addEventListener('mousemove', onRowDragMove);
    window.addEventListener('mouseup', onRowDragEnd);
    e.preventDefault();
  }

  function onRowDragMove(e: MouseEvent) {
    if (dragContainerSize <= 0) return;
    const total = dragStartFr.reduce((a, b) => a + b, 0);
    const deltaPx = e.clientY - dragStartPos;
    let deltaFraction = (deltaPx / dragContainerSize) * total;

    const minFraction = (MIN_TRACK_PX / dragContainerSize) * total;
    const maxNeg = -(dragStartFr[dragIndex] - minFraction);
    const maxPos = dragStartFr[dragIndex + 1] - minFraction;
    deltaFraction = Math.max(maxNeg, Math.min(maxPos, deltaFraction));

    liveRowFr = dragStartFr.map((f, i) =>
      i === dragIndex ? f + deltaFraction : i === dragIndex + 1 ? f - deltaFraction : f
    );
  }

  function onRowDragEnd() {
    window.removeEventListener('mousemove', onRowDragMove);
    window.removeEventListener('mouseup', onRowDragEnd);
    if (liveRowFr) tabStore.setGridFractions(tabId, colFr, liveRowFr);
    liveRowFr = null;
    dragging = null;
  }

  // --- Focus mode -----------------------------------------------------------
  let areaW = 0;
  let areaH = 0;

  // Measured only in focus mode, so the grid pays nothing for it.
  let areaObserver: ResizeObserver | null = null;
  $: watchArea(focus, gridEl);
  function watchArea(on: boolean, el: HTMLDivElement | undefined) {
    areaObserver?.disconnect();
    areaObserver = null;
    if (!on || !el) return;
    areaW = el.clientWidth;
    areaH = el.clientHeight;
    if (typeof ResizeObserver === 'undefined') return;
    areaObserver = new ResizeObserver(() => {
      areaW = el.clientWidth;
      areaH = el.clientHeight;
    });
    areaObserver.observe(el);
  }
  onDestroy(() => areaObserver?.disconnect());

  $: order = reconcileOrder($focusOrders[tabId], panes.map((p) => p.id));
  $: waiting = focus ? waitingElsewhere($allTabs, tabId) : [];
  $: geo = focusGeometry(areaW, areaH, panes.length, waiting.length);
  // A pane of this tab shown floating over another tab. Only while this tab
  // is not the active one: here it has its own slot.
  $: floatingId = focus && !active && $floatingPane?.tabId === tabId ? $floatingPane.paneId : '';
  let floatDrag: { x: number; y: number } | null = null;
  $: floatBase = floatingRect(areaW, areaH, $config.layout?.float_x ?? -1, $config.layout?.float_y ?? -1);
  $: floatRectNow = floatDrag ? floatingRect(areaW, areaH, floatDrag.x, floatDrag.y) : floatBase;
  $: floating = floatingId ? { id: floatingId, rect: floatRectNow } : null;

  function slotOf(id: string, ..._deps: unknown[]): Slot {
    return slotFor(id, order, geo, { w: areaW, h: areaH }, floating, maximizedPane?.id ?? '');
  }

  function slotStyle(s: Slot): string {
    const r = s.rect;
    let css = `left:${r.x}px;top:${r.y}px;width:${r.w}px;height:${r.h}px;`;
    if (s.kind === 'float') css += 'visibility:visible;pointer-events:auto;z-index:30;';
    if (s.kind === 'hidden') css += 'visibility:hidden;pointer-events:none;';
    return css;
  }

  function innerStyle(s: Slot): string {
    if (s.kind === 'small' && s.logical) {
      return `left:0;bottom:0;width:${s.logical.w}px;height:${s.logical.h}px;transform:scale(${s.scale});transform-origin:bottom left;`;
    }
    if (s.kind === 'float') return `left:0;right:0;top:${FLOAT_BAR}px;bottom:0;`;
    return 'inset:0;';
  }

  // Promote on focus changes the user makes in this tab (a click, Ctrl+1-9, a
  // new pane). Only while the tab is active, so a session restore, which adds
  // panes to tabs nobody is looking at, leaves the order alone.
  let lastFocus: string | null = null;
  let seen = new Set<string>();
  $: focusedId = panes.find((p) => p.focused)?.id ?? '';
  $: onFocusChange(focusedId, active, focus);

  function onFocusChange(id: string, isActive: boolean, isFocus: boolean) {
    if (!isFocus || !isActive) return;
    if (lastFocus === null || !id) {
      lastFocus = id;
      panes.forEach((p) => seen.add(p.id));
      return;
    }
    if (id === lastFocus) return;
    // The focused pane was closed and the store moved focus to a neighbour.
    // Nobody chose that pane, so it must not jump to the left.
    const closedFocus = !panes.some((p) => p.id === lastFocus);
    lastFocus = id;
    const isNew = !seen.has(id) && !consumePlaced(id);
    panes.forEach((p) => seen.add(p.id));
    if (closedFocus && !isNew) return;
    // Written after this reactive pass, not during it: a store set from inside
    // a `$:` statement does not re-run the statements before it in Svelte 5's
    // legacy mode, so the slots would keep showing the old order.
    queueMicrotask(() => {
      const current = reconcileOrder($focusOrders[tabId], panes.map((p) => p.id));
      const next = promoteInOrder(current, id, isNew);
      if (next !== current) setFocusOrder(tabId, next);
    });
  }

  // The buttons below hand the keyboard to a terminal. TerminalPane leaves
  // focus alone while a button holds it (so it never steals it from a
  // control), so the clicked button has to let go first.
  function releaseButtonFocus() {
    const ae = document.activeElement;
    if (ae instanceof HTMLButtonElement) ae.blur();
  }

  function promoteSmall(id: string) {
    releaseButtonFocus();
    setFocusOrder(tabId, promoteInOrder(order, id, true));
    dispatch('focusPane', { paneId: id });
  }

  function openFloating(w: WaitingEntry) {
    releaseButtonFocus();
    floatingPane.set({ tabId: w.tabId, paneId: w.pane.id });
    tabStore.focusPane(w.tabId, w.pane.id);
  }

  function closeFloating() {
    floatingPane.set(null);
  }

  function gotoFloatingTab(paneId: string) {
    releaseButtonFocus();
    floatingPane.set(null);
    setFocusOrder(tabId, promoteInOrder(order, paneId, true));
    tabStore.setActiveTab(tabId);
    tabStore.focusPane(tabId, paneId);
  }

  let dragStart = { px: 0, py: 0, x: 0, y: 0 };
  function floatPointerDown(e: PointerEvent) {
    if ((e.target as HTMLElement).closest('button')) return;
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    dragStart = { px: e.clientX, py: e.clientY, x: floatRectNow.x, y: floatRectNow.y };
    floatDrag = { x: floatRectNow.x, y: floatRectNow.y };
  }
  function floatPointerMove(e: PointerEvent) {
    if (!floatDrag) return;
    floatDrag = { x: dragStart.x + e.clientX - dragStart.px, y: dragStart.y + e.clientY - dragStart.py };
  }
  function floatPointerUp() {
    if (!floatDrag) return;
    const r = floatingRect(areaW, areaH, floatDrag.x, floatDrag.y);
    floatDrag = null;
    void saveFloatPosition(r.x, r.y);
  }

  // Same colours as the pane titlebar (PaneTitlebar.svelte), so a state reads
  // the same in a big pane, a small card and the waiting list.
  function dotClass(activity: Pane['activity']): string {
    switch (activity) {
      case 'waitingPermission':
      case 'waitingAnswer': return 'dot-waiting';
      case 'active': return 'dot-running';
      case 'done': return 'dot-done';
      case 'error': return 'dot-error';
      case 'sleeping':
      case 'resuming': return 'dot-sleeping';
      default: return 'dot-idle';
    }
  }

  function stateClass(activity: Pane['activity']): string {
    switch (activity) {
      case 'waitingPermission':
      case 'waitingAnswer': return 'state-waiting';
      case 'active': return 'state-running';
      case 'error': return 'state-danger';
      default: return '';
    }
  }
</script>

<div
  class="pane-grid"
  class:focus
  class:dragging={dragging !== null}
  bind:this={gridEl}
  style={focus ? '' : `grid-template-columns: ${displayColFr.map((f) => f + 'fr').join(' ')}; grid-template-rows: ${displayRowFr.map((f) => f + 'fr').join(' ')};`}
>
  {#each renderPanes as pane (pane.id)}
    <!-- In grid mode both wrappers are display:contents, so the grid sees the
         pane itself exactly as before. In focus mode they position it. -->
    {@const slot = focus ? slotOf(pane.id, order, geo, floating, maximizedPane, areaW, areaH) : null}
    <div class="slot slot-{slot?.kind ?? 'grid'}" style={slot ? slotStyle(slot) : ''}>
    {#if slot?.kind === 'float'}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="float-bar"
        on:pointerdown={floatPointerDown}
        on:pointermove={floatPointerMove}
        on:pointerup={floatPointerUp}
        on:pointercancel={floatPointerUp}
      >
        <span class="dot {dotClass(pane.activity)}"></span>
        <span class="float-tab">{tabName}</span>
        <span class="float-sep">·</span>
        <span class="float-name">{paneDisplayName(pane)}</span>
        <span class="float-state {stateClass(pane.activity)}">{stateLabel(pane.activity)}</span>
        <span class="float-spacer"></span>
        <button class="float-btn" on:click={() => gotoFloatingTab(pane.id)}>Zum Projekt wechseln</button>
        <button class="float-btn float-close" title="Schließen" aria-label="Schließen" on:click={closeFloating}>×</button>
      </div>
    {/if}
    <div class="slot-inner" style={slot ? innerStyle(slot) : ''}>
    {#if pane.display === 'chat'}
      <div class="pane-chat-wrapper">
        <ChatPane conversationId={pane.conversationId} dir={tabDir} paneId={pane.id} on:toggleDisplay={handleToggleDisplay} on:close={e => dispatch('closePane', e.detail)} />
      </div>
    {:else}
      <TerminalPane
        {pane}
        active={active || slot?.kind === 'float'}
        {tabId}
        {tabDir}
        paneIndex={panes.indexOf(pane) + 1}
        on:close={handleClose}
        on:maximize={handleMaximize}
        on:focus={handleFocus}
        on:rename={handleRename}
        on:restart={handleRestart}
        on:toggleDisplay={handleToggleDisplay}
        on:issueAction={handleIssueAction}
        on:commitPush={handleCommitPush}
        on:finishWorktree={handleFinishWorktree}
        on:quickAction={handleQuickAction}
        on:cancelFinish={handleCancelFinish}
        on:navigateFile={handleNavigateFile}
        on:splitPane={handleSplitPane}
      />
    {/if}
    </div>
    {#if slot?.kind === 'small'}
      <button class="small-cover" title="Nach links holen" on:click={() => promoteSmall(pane.id)}>
        <span class="small-head">
          <span class="dot {dotClass(pane.activity)}"></span>
          <span class="small-name">{paneDisplayName(pane)}</span>
          <span class="small-state {stateClass(pane.activity)}">{stateLabel(pane.activity)}</span>
        </span>
      </button>
    {/if}
    </div>
  {/each}

  {#if focus && !maximizedPane}
    <div class="waiting-list" style="left:{geo.list.x}px;top:{geo.list.y}px;width:{geo.list.w}px;height:{geo.list.h}px;">
      <div class="wl-head">
        <span>Wartet auf dich · andere Projekte</span>
        <kbd title="Nächstes wartendes Pane öffnen">Strg+Umschalt+J</kbd>
      </div>
      <div class="wl-rows">
        {#each waiting as w (w.pane.id)}
          <button
            class="wl-row"
            class:open={$floatingPane?.paneId === w.pane.id}
            on:click={() => openFloating(w)}
            title="Schwebend öffnen, ohne den Tab zu wechseln"
          >
            <span class="dot {dotClass(w.pane.activity)}"></span>
            <span class="wl-text">
              <span class="wl-top"><span class="wl-tab">{w.tabName}</span> · {paneDisplayName(w.pane)}</span>
              <span class="wl-state">{stateLabel(w.pane.activity)}{w.pane.activitySince ? ' · ' + formatDuration(w.pane.activitySince, $now) : ''}</span>
            </span>
          </button>
        {:else}
          <div class="wl-empty">Kein anderes Projekt wartet auf dich.</div>
        {/each}
      </div>
    </div>
  {/if}

  {#if panes.length === 0}
    <div class="empty-state" style={focus ? `right:${geo.column.w + 8}px` : ''}>
      <p>{$t('paneGrid.empty')}</p>
      <p class="hint">{$t('paneGrid.emptyHint', { max: 10 })}</p>
    </div>
  {/if}

  {#if !maximizedPane && !focus}
    <div class="resize-overlay">
      {#each colBoundaryPercents as pct, i}
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div
          class="resize-handle col-handle"
          style="left: {pct}%;"
          on:mousedown={(e) => startColDrag(i, e)}
        ></div>
      {/each}
      {#each rowBoundaryPercents as pct, i}
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div
          class="resize-handle row-handle"
          style="top: {pct}%;"
          on:mousedown={(e) => startRowDrag(i, e)}
        ></div>
      {/each}
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
    position: relative;
  }

  .pane-grid.focus {
    display: block;
  }

  /* Grid mode: the wrappers do not exist for layout purposes. */
  .slot, .slot-inner { display: contents; }

  .pane-grid.focus .slot {
    display: block;
    position: absolute;
    overflow: hidden;
  }
  .pane-grid.focus .slot-inner {
    display: flex;
    position: absolute;
  }
  .pane-grid.focus .slot-inner > :global(*) { flex: 1; min-width: 0; min-height: 0; }

  .pane-grid.focus .slot-small {
    border-radius: 8px;
    background: var(--pane-bg, var(--bg));
  }

  .pane-grid.focus .slot-float {
    display: flex;
    flex-direction: column;
    border: 1px solid var(--status-waiting, #d6a85c);
    border-radius: 10px;
    background: var(--bg-secondary, var(--bg));
    box-shadow: 0 18px 48px rgba(0, 0, 0, 0.55);
  }

  .float-bar {
    position: absolute;
    left: 0; right: 0; top: 0;
    height: 32px;
    display: flex; align-items: center; gap: 8px;
    padding: 0 8px 0 12px;
    background: var(--bg-tertiary, #1c241a);
    border-bottom: 1px solid var(--pane-border, #45475a);
    font-size: 12px;
    cursor: grab;
    touch-action: none;
    user-select: none;
  }
  .float-bar:active { cursor: grabbing; }
  .float-tab { font-weight: 600; color: var(--fg); }
  .float-sep { color: var(--fg-muted); }
  .float-name { color: var(--fg); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .float-state { color: var(--fg-muted); white-space: nowrap; }
  .float-spacer { flex: 1; }
  .float-btn {
    background: transparent;
    color: var(--fg);
    border: 1px solid var(--pane-border, #45475a);
    border-radius: 5px;
    padding: 2px 8px;
    font-size: 12px;
    cursor: pointer;
  }
  .float-btn:hover { border-color: var(--accent); }
  .float-close { font-size: 15px; line-height: 1; padding: 1px 7px; }

  .small-cover {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: stretch;
    background: transparent;
    border: none;
    padding: 0;
    cursor: pointer;
    text-align: left;
    color: var(--fg);
    z-index: 2;
  }
  .small-cover:hover { box-shadow: inset 0 0 0 1px var(--accent); border-radius: 8px; }
  .small-head {
    display: flex; align-items: center; gap: 6px;
    height: 24px;
    padding: 0 8px;
    background: var(--bg-tertiary, #1c241a);
    border-bottom: 1px solid var(--pane-border, #45475a);
    font-size: 11px;
  }
  .small-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .small-state { color: var(--fg-muted); white-space: nowrap; }

  .dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; background: var(--status-idle, var(--fg-muted)); }
  .dot-running { background: var(--status-running); animation: fl-spin 1s linear infinite; }
  .dot-done { background: var(--status-running); box-shadow: 0 0 6px var(--status-running); }
  .dot-waiting {
    background: var(--status-waiting);
    box-shadow: 0 0 7px var(--status-waiting);
    animation: fl-pulse 1.2s ease-in-out infinite;
  }
  .dot-error { background: var(--status-danger); }
  .dot-sleeping { background: var(--fg-muted); opacity: 0.6; }
  @keyframes fl-spin { 0% { opacity: 0.5; } 50% { opacity: 1; } 100% { opacity: 0.5; } }
  @keyframes fl-pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.4; } }

  .state-running { color: var(--status-running); }
  .state-waiting { color: var(--status-waiting); }
  .state-danger { color: var(--status-danger); }

  .waiting-list {
    position: absolute;
    display: flex;
    flex-direction: column;
    border: 1px solid var(--pane-border, #45475a);
    border-radius: 8px;
    background: var(--bg-secondary, var(--bg));
    overflow: hidden;
    z-index: 3;
  }
  .wl-head {
    display: flex; align-items: center; justify-content: space-between; gap: 8px;
    height: 30px; flex-shrink: 0;
    padding: 0 10px;
    font-size: 11px; font-weight: 600;
    letter-spacing: 0.04em;
    color: var(--status-waiting, #d6a85c);
    border-bottom: 1px solid var(--pane-border, #45475a);
  }
  .wl-rows { flex: 1; overflow-y: auto; }
  .wl-row {
    width: 100%;
    display: flex; align-items: center; gap: 8px;
    padding: 6px 10px;
    min-height: 46px;
    background: transparent;
    border: none;
    border-bottom: 1px solid var(--pane-border, #45475a);
    color: var(--fg);
    text-align: left;
    cursor: pointer;
  }
  .wl-row:hover, .wl-row.open { background: var(--status-waiting-tint, rgba(214,168,92,.16)); }
  .wl-text { display: flex; flex-direction: column; min-width: 0; gap: 2px; }
  .wl-top { font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .wl-tab { font-weight: 600; }
  .wl-state { font-size: 11px; color: var(--status-waiting, #d6a85c); }
  .wl-empty { padding: 10px; font-size: 12px; color: var(--fg-muted); }

  .pane-grid.focus .empty-state {
    position: absolute;
    left: 0; top: 0; bottom: 0;
  }

  .pane-grid.dragging {
    user-select: none;
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

  /* Chat replaces a pane — same frame, but violet top edge = AI mode */
  .pane-chat-wrapper {
    display: flex;
    min-width: 0;
    overflow: hidden;
    border: 1px solid var(--pane-border, #45475a);
    border-top: 2px solid var(--status-ai, #a184f4);
    border-radius: 9px;
  }

  kbd {
    background: var(--bg-tertiary);
    padding: 2px 6px;
    border-radius: 4px;
    font-family: monospace;
    font-size: 11px;
  }

  .resize-overlay {
    position: absolute;
    inset: 4px; /* matches .pane-grid padding */
    pointer-events: none;
    z-index: 5;
  }

  .resize-handle {
    position: absolute;
    pointer-events: auto;
  }

  .resize-handle.col-handle {
    top: 0;
    bottom: 0;
    width: 8px;
    margin-left: -4px;
    cursor: col-resize;
  }

  .resize-handle.row-handle {
    left: 0;
    right: 0;
    height: 8px;
    margin-top: -4px;
    cursor: row-resize;
  }

  .resize-handle:hover,
  .resize-handle:active {
    background: var(--accent, #39ff14);
    opacity: 0.4;
  }
</style>
