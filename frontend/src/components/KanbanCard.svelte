<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import type { KanbanCard } from '../stores/kanban';

  export let card: KanbanCard;
  export let columnId: string;

  const dispatch = createEventDispatcher<{
    dragstart: { card: KanbanCard; columnId: string };
    click: { card: KanbanCard };
  }>();

  function handleDragStart(e: DragEvent) {
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', card.id);
    }
    dispatch('dragstart', { card, columnId });
  }

  function handleClick() {
    dispatch('click', { card });
  }

  function labelColor(label: string): string {
    const hash = label.split('').reduce((a, c) => a + c.charCodeAt(0), 0);
    const hue = hash % 360;
    return `hsl(${hue}, 60%, 45%)`;
  }

  // Determine left border color based on review result
  $: borderStyle = card.review_result === 'fail'
    ? 'border-left: 3px solid #ef4444;'
    : card.review_result === 'pass'
      ? 'border-left: 3px solid #22c55e;'
      : '';
</script>

<button
  class="kanban-card"
  draggable="true"
  on:dragstart={handleDragStart}
  on:click={handleClick}
  style={borderStyle}
>
  <div class="card-top-row">
    {#if card.issue_number}
      <span class="card-issue">#{card.issue_number}</span>
    {/if}
    {#if card.parent_issue > 0}
      <span class="card-badge card-parent" title="Eltern-Issue #{card.parent_issue}">&#8593;#{card.parent_issue}</span>
    {/if}
    {#if card.auto_merge}
      <span class="card-badge card-auto-merge" title="Auto-Merge aktiv">&#9889;</span>
    {/if}
    {#if card.pr_number > 0}
      <span class="card-badge card-pr" title="Pull Request #{card.pr_number}">PR #{card.pr_number}</span>
    {/if}
  </div>
  <div class="card-title">{card.title}</div>
  {#if card.worktree_branch}
    <div class="card-branch" title="Worktree-Branch">{card.worktree_branch}</div>
  {/if}
  {#if card.labels && card.labels.length > 0}
    <div class="card-labels">
      {#each card.labels.slice(0, 3) as label}
        <span class="card-label" style="background: {labelColor(label)}">{label}</span>
      {/each}
    </div>
  {/if}
  <div class="card-meta">
    {#if card.session_id > 0}
      <span class="card-session" title="Aktive Session">&#9654;</span>
    {/if}
    {#if card.agent_session_id > 0}
      <span class="card-agent" title="Agent-Session aktiv">&#9881;</span>
    {/if}
    {#if card.priority > 0}
      <span class="card-priority" title="Priorität {card.priority}">P{card.priority}</span>
    {/if}
    {#if card.schedule_id}
      <span class="card-scheduled" title="Geplant">&#8635;</span>
    {/if}
    {#if card.retry_count > 0}
      <span class="card-retries" title="Wiederholungen">&#8635; {card.retry_count}/{card.max_retries}</span>
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
    cursor: grab;
    text-align: left;
    width: 100%;
    transition: background 0.1s, border-color 0.1s, transform 0.1s, box-shadow 0.1s;
  }
  .kanban-card:hover {
    background: var(--bg-tertiary, #313244);
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(0,0,0,0.2);
  }
  .kanban-card:active {
    cursor: grabbing;
    opacity: 0.7;
  }

  .card-top-row {
    display: flex;
    align-items: center;
    gap: 5px;
    flex-wrap: wrap;
  }

  .card-issue {
    font-size: 0.7rem;
    color: var(--fg-muted, #a6adc8);
    font-weight: 500;
  }

  .card-badge {
    font-size: 0.6rem;
    padding: 0 4px;
    border-radius: 3px;
    font-weight: 600;
    white-space: nowrap;
  }
  .card-parent {
    background: rgba(139, 92, 246, 0.2);
    color: #a78bfa;
  }
  .card-auto-merge {
    background: rgba(34, 197, 94, 0.2);
    color: #22c55e;
  }
  .card-pr {
    background: rgba(59, 130, 246, 0.2);
    color: #60a5fa;
  }

  .card-branch {
    font-size: 0.6rem;
    color: var(--fg-muted, #a6adc8);
    font-family: monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    opacity: 0.7;
  }

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

  .card-labels {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
    margin-top: 2px;
  }
  .card-label {
    font-size: 0.6rem;
    padding: 1px 5px;
    border-radius: 3px;
    color: #fff;
    font-weight: 600;
    white-space: nowrap;
  }

  .card-meta {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 2px;
  }
  .card-session {
    color: var(--accent, #39ff14);
    font-size: 0.65rem;
  }
  .card-priority {
    font-size: 0.6rem;
    padding: 0 4px;
    border-radius: 3px;
    background: rgba(245, 166, 35, 0.2);
    color: #f5a623;
    font-weight: 600;
  }
  .card-scheduled {
    font-size: 0.75rem;
    color: var(--fg-muted, #a6adc8);
  }
  .card-agent {
    color: #f97316;
    font-size: 0.7rem;
  }
  .card-retries {
    font-size: 0.6rem;
    padding: 0 4px;
    border-radius: 3px;
    background: rgba(239, 68, 68, 0.2);
    color: #f87171;
    font-weight: 600;
  }
</style>
