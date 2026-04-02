<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';
  import { EventsOn } from '../../wailsjs/runtime/runtime';
  import KanbanColumn from './KanbanColumn.svelte';
  import KanbanCardDetail from './KanbanCardDetail.svelte';
  import KanbanCreateDialog from './KanbanCreateDialog.svelte';
  import { kanban, tasksByColumn, COLUMN_IDS, type ColumnID } from '../stores/kanban';
  import { board } from '../../wailsjs/go/models';

  export let dir = '';

  let showCreateDialog = false;
  let detailCardId = '';
  let showDetail = false;

  // Event cleanup
  let eventCleanup: (() => void) | null = null;

  function loadBoard() {
    if (!dir) return;
    kanban.setDir(dir);
    kanban.setLoading(true);
    App.GetBoardTasks(dir)
      .then(tasks => kanban.setTasks(tasks || []))
      .catch(err => {
        console.error('[kanban] load error:', err);
        kanban.setLoading(false);
      });
  }

  onMount(() => {
    // Listen for board state transition events
    eventCleanup = EventsOn('board:task-transition', (_payload: any) => {
      // Reload the full task list on any transition
      loadBoard();
    });
  });

  onDestroy(() => {
    if (eventCleanup) eventCleanup();
  });

  // Reload when dir changes
  $: if (dir) { loadBoard(); }

  function handleCardClick(e: CustomEvent<{ card: board.TaskCard }>) {
    detailCardId = e.detail.card.id;
    showDetail = true;
  }

  function handleDetailUpdated() {
    loadBoard();
  }

  async function handleSync() {
    if (!dir) return;
    kanban.setLoading(true);
    try {
      await App.SyncBoard(dir);
      const tasks = await App.GetBoardTasks(dir);
      kanban.setTasks(tasks || []);
    } catch (err) {
      console.error('[kanban] sync error:', err);
      kanban.setLoading(false);
    }
  }

  async function handleAddCard(e: CustomEvent<{ title: string; cardType: string; description: string }>) {
    if (!dir) return;
    const { title, cardType, description } = e.detail;
    try {
      const card = new board.TaskCard({
        title,
        card_type: cardType,
        description,
        state: 'backlog',
      });
      const saved = await App.CreateBoardTask(dir, card);
      kanban.addTask(saved);
      showCreateDialog = false;
    } catch (err) {
      console.error('[kanban] add card error:', err);
    }
  }

  async function handleRemoveCard(e: CustomEvent<{ cardId: string }>) {
    try {
      await App.DeleteBoardTask(dir, e.detail.cardId);
      kanban.removeTask(e.detail.cardId);
    } catch (err) {
      console.error('[kanban] remove card error:', err);
    }
  }

  async function handleTransition(e: CustomEvent<{ cardId: string; event: board.Event }>) {
    try {
      await App.MoveBoardTask(dir, e.detail.cardId, e.detail.event);
      // Reload happens via board:task-transition event listener
    } catch (err) {
      console.error('[kanban] transition error:', err);
      loadBoard(); // Reload on failure to reset UI
    }
  }

</script>

<div class="kanban-board">
  <div class="board-toolbar">
    <h2 class="board-title">Kanban Board</h2>
    {#if dir}
      <span class="board-dir">{dir.split(/[/\\]/).pop()}</span>
    {/if}
    <div class="toolbar-spacer"></div>
    <div class="toolbar-actions">
      <button class="btn-toolbar" on:click={() => { showCreateDialog = true; }} title="Karte hinzufügen">
        + Karte
      </button>
      <button class="btn-toolbar" on:click={handleSync} title="Board synchronisieren">
        &#8635; Sync
      </button>
    </div>
  </div>

  {#if $kanban.loading}
    <div class="loading">Board wird geladen...</div>
  {:else}
    <div class="columns-container">
      {#each COLUMN_IDS as colId (colId)}
        <KanbanColumn
          columnId={colId}
          cards={$tasksByColumn[colId]}
          on:cardClick={handleCardClick}
          on:removeCard={handleRemoveCard}
          on:transition={handleTransition}
        />
      {/each}
    </div>
  {/if}
</div>

<KanbanCardDetail
  bind:visible={showDetail}
  cardId={detailCardId}
  {dir}
  on:updated={handleDetailUpdated}
/>

<KanbanCreateDialog
  bind:visible={showCreateDialog}
  on:create={handleAddCard}
  on:cancel={() => { showCreateDialog = false; }}
/>

<style>
  .kanban-board {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    height: 100%;
    background: var(--bg, #11111b);
    overflow: hidden;
  }

  .board-toolbar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 16px;
    border-bottom: 1px solid var(--border, #45475a);
    background: var(--bg-secondary, #1e1e2e);
    flex-shrink: 0;
  }
  .board-title {
    font-size: 1rem;
    font-weight: 700;
    color: var(--fg, #cdd6f4);
  }
  .board-dir {
    font-size: 0.75rem;
    color: var(--accent, #39ff14);
    font-weight: 500;
  }
  .toolbar-spacer { flex: 1; }
  .toolbar-actions {
    display: flex;
    gap: 6px;
    align-items: center;
  }

  .btn-toolbar {
    padding: 4px 10px;
    border-radius: 6px;
    background: var(--bg, #11111b);
    border: 1px solid var(--border, #45475a);
    color: var(--fg, #cdd6f4);
    cursor: pointer;
    font-size: 0.75rem;
    transition: border-color 0.15s;
  }
  .btn-toolbar:hover { border-color: var(--accent, #39ff14); }

  .loading {
    display: flex;
    align-items: center;
    justify-content: center;
    flex: 1;
    color: var(--fg-muted, #a6adc8);
    font-size: 0.9rem;
  }

  .columns-container {
    display: grid;
    grid-template-columns: repeat(6, 1fr);
    gap: 8px;
    padding: 12px;
    flex: 1;
    overflow-x: auto;
    overflow-y: hidden;
  }
</style>
