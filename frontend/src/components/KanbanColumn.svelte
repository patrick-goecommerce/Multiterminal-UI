<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import KanbanCard from './KanbanCard.svelte';
  import { COLUMN_LABELS, COLUMN_COLORS, type ColumnID } from '../stores/kanban';
  import type { board } from '../../wailsjs/go/models';

  export let columnId: ColumnID;
  export let cards: board.TaskCard[] = [];

  const dispatch = createEventDispatcher<{
    cardClick: { card: board.TaskCard };
    removeCard: { cardId: string };
    transition: { cardId: string; event: board.Event };
  }>();

  function handleCardClick(e: CustomEvent<{ card: board.TaskCard }>) {
    dispatch('cardClick', e.detail);
  }

  function handleRemoveCard(e: CustomEvent<{ cardId: string }>) {
    dispatch('removeCard', e.detail);
  }

  function handleTransition(e: CustomEvent<{ cardId: string; event: board.Event }>) {
    dispatch('transition', e.detail);
  }
</script>

<div class="kanban-column">
  <div class="column-header">
    <span class="column-dot" style="background: {COLUMN_COLORS[columnId]}"></span>
    <span class="column-label">{COLUMN_LABELS[columnId]}</span>
    <span class="column-count">{cards.length}</span>
  </div>
  <div class="column-cards">
    {#each cards as card (card.id)}
      <KanbanCard
        {card}
        {columnId}
        on:click={handleCardClick}
        on:remove={handleRemoveCard}
        on:transition={handleTransition}
      />
    {/each}
    {#if cards.length === 0}
      <div class="column-empty">Keine Karten</div>
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
</style>
