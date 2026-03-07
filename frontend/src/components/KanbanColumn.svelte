<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import KanbanCard from './KanbanCard.svelte';
  import { COLUMN_LABELS, COLUMN_COLORS, type ColumnID, type KanbanCard as KanbanCardType } from '../stores/kanban';

  export let columnId: ColumnID;
  export let cards: KanbanCardType[] = [];

  const dispatch = createEventDispatcher<{
    drop: { cardId: string; columnId: string; position: number };
    cardClick: { card: KanbanCardType };
    cardDragStart: { card: KanbanCardType; columnId: string };
  }>();

  let dropTarget = false;
  let dropPosition = -1;

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
    dropTarget = true;

    // Calculate drop position from mouse Y
    const container = e.currentTarget as HTMLElement;
    const cards = container.querySelectorAll('.kanban-card');
    let pos = cards.length;
    for (let i = 0; i < cards.length; i++) {
      const rect = cards[i].getBoundingClientRect();
      if (e.clientY < rect.top + rect.height / 2) {
        pos = i;
        break;
      }
    }
    dropPosition = pos;
  }

  function handleDragLeave() {
    dropTarget = false;
    dropPosition = -1;
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault();
    const cardId = e.dataTransfer?.getData('text/plain');
    if (cardId) {
      dispatch('drop', { cardId, columnId, position: dropPosition >= 0 ? dropPosition : cards.length });
    }
    dropTarget = false;
    dropPosition = -1;
  }

  function handleCardClick(e: CustomEvent<{ card: KanbanCardType }>) {
    dispatch('cardClick', e.detail);
  }

  function handleCardDragStart(e: CustomEvent<{ card: KanbanCardType; columnId: string }>) {
    dispatch('cardDragStart', e.detail);
  }
</script>

<div
  class="kanban-column"
  class:drop-target={dropTarget}
  on:dragover={handleDragOver}
  on:dragleave={handleDragLeave}
  on:drop={handleDrop}
>
  <div class="column-header">
    <span class="column-dot" style="background: {COLUMN_COLORS[columnId]}"></span>
    <span class="column-label">{COLUMN_LABELS[columnId]}</span>
    <span class="column-count">{cards.length}</span>
  </div>
  <div class="column-cards">
    {#each cards as card (card.id)}
      <KanbanCard {card} {columnId} on:click={handleCardClick} on:dragstart={handleCardDragStart} />
    {/each}
    {#if cards.length === 0}
      <div class="column-empty">Keine Karten</div>
    {/if}
    {#if dropTarget}
      <div class="drop-indicator"></div>
    {/if}
  </div>
</div>

<style>
  .kanban-column {
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--bg, #11111b);
    border-radius: 8px;
    border: 1px solid var(--border, #45475a);
    overflow: hidden;
    transition: border-color 0.15s;
  }
  .kanban-column.drop-target {
    border-color: var(--accent, #39ff14);
    background: rgba(57, 255, 20, 0.03);
  }

  .column-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    border-bottom: 1px solid var(--border, #45475a);
    background: var(--bg-secondary, #1e1e2e);
    flex-shrink: 0;
  }
  .column-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .column-label {
    font-size: 0.75rem;
    font-weight: 700;
    color: var(--fg, #cdd6f4);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .column-count {
    margin-left: auto;
    font-size: 0.7rem;
    color: var(--fg-muted, #a6adc8);
    background: var(--bg, #11111b);
    padding: 1px 6px;
    border-radius: 8px;
  }

  .column-cards {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-height: 80px;
  }

  .column-empty {
    text-align: center;
    color: var(--fg-muted, #a6adc8);
    font-size: 0.75rem;
    padding: 20px 0;
    opacity: 0.5;
  }

  .drop-indicator {
    height: 2px;
    background: var(--accent, #39ff14);
    border-radius: 1px;
    margin: 2px 0;
  }
</style>
