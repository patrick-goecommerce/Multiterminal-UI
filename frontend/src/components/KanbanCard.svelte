<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { getBadge } from '../stores/kanban';
  import type { board } from '../../wailsjs/go/models';

  export let card: board.TaskCard;
  export let columnId: string;

  const dispatch = createEventDispatcher<{
    click: { card: board.TaskCard };
    remove: { cardId: string };
    transition: { cardId: string; event: board.Event };
  }>();

  function handleClick() {
    dispatch('click', { card });
  }

  function handleRemove(e: MouseEvent) {
    e.stopPropagation();
    dispatch('remove', { cardId: card.id });
  }

  // Card type display labels (German)
  const TYPE_LABELS: Record<string, { label: string; color: string }> = {
    feature: { label: 'Feature', color: '#8b5cf6' },
    bugfix: { label: 'Bugfix', color: '#ef4444' },
    refactor: { label: 'Refactor', color: '#3b82f6' },
    docs: { label: 'Doku', color: '#06b6d4' },
  };

  // Complexity display
  const COMPLEXITY_LABELS: Record<string, string> = {
    trivial: 'S',
    medium: 'M',
    complex: 'L',
  };

  $: badge = getBadge(card.state as board.TaskState);
  $: typeInfo = TYPE_LABELS[card.card_type] || null;
  $: complexityLabel = card.complexity ? COMPLEXITY_LABELS[card.complexity] || '' : '';
</script>

<button
  class="kanban-card"
  on:click={handleClick}
>
  <div class="card-top-row">
    {#if typeInfo}
      <span class="card-type-tag" style="background: {typeInfo.color}">{typeInfo.label}</span>
    {/if}
    {#if badge}
      <span class="card-state-badge" style="background: {badge.color}">{badge.label}</span>
    {/if}
    {#if complexityLabel}
      <span class="card-complexity" title="Komplexität: {card.complexity}">{complexityLabel}</span>
    {/if}
    <span class="card-spacer"></span>
    <button class="card-remove" on:click={handleRemove} title="Karte entfernen">&#10005;</button>
  </div>
  <div class="card-title">{card.title}</div>
  {#if card.description}
    <div class="card-desc">{card.description}</div>
  {/if}
  <div class="card-meta">
    {#if card.cost_usd > 0}
      <span class="card-cost" title="Kosten">${card.cost_usd.toFixed(2)}</span>
    {/if}
    {#if card.qa_attempts > 0}
      <span class="card-qa" title="QA-Versuche">QA: {card.qa_attempts}</span>
    {/if}
    {#if card.execution_mode}
      <span class="card-mode" title="Ausführungsmodus">{card.execution_mode}</span>
    {/if}
  </div>
</button>

<style>
  .kanban-card {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 10px 12px;
    background: var(--bg-secondary, #1e1e2e);
    border: 1px solid var(--border, #45475a);
    border-radius: 6px;
    cursor: pointer;
    text-align: left;
    width: 100%;
    transition: background 0.1s, border-color 0.1s, transform 0.1s, box-shadow 0.1s;
  }
  .kanban-card:hover {
    background: var(--bg-tertiary, #313244);
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(0,0,0,0.2);
  }

  .card-top-row {
    display: flex;
    align-items: center;
    gap: 5px;
    flex-wrap: wrap;
  }

  .card-type-tag {
    font-size: 0.6rem;
    padding: 1px 5px;
    border-radius: 3px;
    color: #fff;
    font-weight: 600;
    white-space: nowrap;
  }

  .card-state-badge {
    font-size: 0.6rem;
    padding: 1px 5px;
    border-radius: 3px;
    color: #fff;
    font-weight: 600;
    white-space: nowrap;
    animation: pulse-badge 2s ease-in-out infinite;
  }
  @keyframes pulse-badge {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.7; }
  }

  .card-complexity {
    font-size: 0.6rem;
    padding: 1px 5px;
    border-radius: 3px;
    background: rgba(166, 173, 200, 0.2);
    color: var(--fg-muted, #a6adc8);
    font-weight: 700;
    white-space: nowrap;
  }

  .card-spacer { flex: 1; }

  .card-remove {
    font-size: 0.6rem;
    padding: 0 3px;
    background: transparent;
    border: none;
    color: var(--fg-muted, #a6adc8);
    cursor: pointer;
    opacity: 0;
    transition: opacity 0.15s, color 0.15s;
    line-height: 1;
  }
  .kanban-card:hover .card-remove { opacity: 0.6; }
  .card-remove:hover { opacity: 1 !important; color: #ef4444; }

  .card-title {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--fg, #cdd6f4);
    overflow: hidden;
    text-overflow: ellipsis;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    line-height: 1.3;
  }

  .card-desc {
    font-size: 0.7rem;
    color: var(--fg-muted, #a6adc8);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    opacity: 0.7;
  }

  .card-meta {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 2px;
  }
  .card-cost {
    font-size: 0.65rem;
    padding: 0 4px;
    border-radius: 3px;
    background: rgba(34, 197, 94, 0.15);
    color: #22c55e;
    font-weight: 600;
  }
  .card-qa {
    font-size: 0.6rem;
    padding: 0 4px;
    border-radius: 3px;
    background: rgba(6, 182, 212, 0.15);
    color: #06b6d4;
    font-weight: 600;
  }
  .card-mode {
    font-size: 0.6rem;
    color: var(--fg-muted, #a6adc8);
    opacity: 0.6;
  }
</style>
