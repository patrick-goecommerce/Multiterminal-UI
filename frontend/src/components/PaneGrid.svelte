<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { t } from '../stores/i18n';
  import TerminalPane from './TerminalPane.svelte';
  import ChatPane from './ChatPane.svelte';
  import { tabStore, type Pane } from '../stores/tabs';

  export let panes: Pane[] = [];
  export let active: boolean = true;
  export let tabId: string = '';
  export let tabDir: string = '';
  export let colFractions: number[] | undefined = undefined;
  export let rowFractions: number[] | undefined = undefined;

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
  $: gridCols = maximizedPane ? 1 : Math.min(Math.ceil(Math.sqrt(panes.length)), 3);
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
</script>

<div
  class="pane-grid"
  class:dragging={dragging !== null}
  bind:this={gridEl}
  style="grid-template-columns: {displayColFr.map((f) => f + 'fr').join(' ')}; grid-template-rows: {displayRowFr.map((f) => f + 'fr').join(' ')};"
>
  {#each visiblePanes as pane (pane.id)}
    {#if pane.display === 'chat'}
      <div class="pane-chat-wrapper">
        <ChatPane conversationId={pane.conversationId} dir={tabDir} paneId={pane.id} on:toggleDisplay={handleToggleDisplay} on:close={e => dispatch('closePane', e.detail)} />
      </div>
    {:else}
      <TerminalPane
        {pane}
        {active}
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
  {/each}

  {#if panes.length === 0}
    <div class="empty-state">
      <p>{$t('paneGrid.empty')}</p>
      <p class="hint">{$t('paneGrid.emptyHint', { max: 10 })}</p>
    </div>
  {/if}

  {#if !maximizedPane}
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
