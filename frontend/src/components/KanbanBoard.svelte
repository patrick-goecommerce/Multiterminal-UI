<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';
  import { EventsOn } from '../../wailsjs/runtime/runtime';
  import KanbanColumn from './KanbanColumn.svelte';
  import { kanban, COLUMN_IDS, activePlans, parentIssueProgress, type ColumnID, type KanbanCard } from '../stores/kanban';

  export let dir = '';

  let addCardTitle = '';
  let showAddCard = false;

  // Orchestration status
  let orchStatus = { active: false, running_agents: 0, max_agents: 3, pending_tickets: 0, review_tickets: 0, done_tickets: 0 };
  let orchLoading = false;

  // Event cleanup
  let eventCleanup: (() => void) | null = null;

  function loadBoard() {
    if (!dir) return;
    kanban.setDir(dir);
    App.GetKanbanState(dir)
      .then(state => kanban.setState(state))
      .catch(err => {
        console.error('[kanban] load error:', err);
        kanban.setLoading(false);
      });
  }

  async function refreshOrchStatus() {
    if (!dir) return;
    try {
      orchStatus = await App.GetOrchestrationStatus(dir);
    } catch (e) { console.error('[kanban] orchestration status error:', e); }
  }

  onMount(() => {
    // Listen for orchestrator events
    eventCleanup = EventsOn('orchestrator:update', (payload: any) => {
      if (payload?.dir === dir || !payload?.dir) {
        loadBoard();
        refreshOrchStatus();
      }
    });
    refreshOrchStatus();
  });

  onDestroy(() => {
    if (eventCleanup) eventCleanup();
  });

  // Reload when dir changes
  $: if (dir) { loadBoard(); refreshOrchStatus(); }

  async function handleStartOrchestration() {
    if (!dir) return;
    orchLoading = true;
    try {
      await App.StartOrchestration(dir);
      await refreshOrchStatus();
    } catch (err) {
      console.error('[kanban] start orchestration error:', err);
    } finally {
      orchLoading = false;
    }
  }

  async function handleStopOrchestration() {
    if (!dir) return;
    orchLoading = true;
    try {
      await App.StopOrchestration(dir);
      await refreshOrchStatus();
    } catch (err) {
      console.error('[kanban] stop orchestration error:', err);
    } finally {
      orchLoading = false;
    }
  }

  async function handleSync() {
    if (!dir) return;
    kanban.setLoading(true);
    try {
      const state = await App.SyncKanbanWithIssues(dir);
      kanban.setState(state);
    } catch (err) {
      console.error('[kanban] sync error:', err);
      kanban.setLoading(false);
    }
  }

  async function handleDrop(e: CustomEvent<{ cardId: string; columnId: string; position: number }>) {
    const { cardId, columnId: toCol, position } = e.detail;

    // Find source column
    let fromCol = '';
    for (const col of COLUMN_IDS) {
      const cards = $kanban.state.columns[col] || [];
      if (cards.find(c => c.id === cardId)) {
        fromCol = col;
        break;
      }
    }
    if (!fromCol || fromCol === toCol) return;

    // Optimistic update
    kanban.moveCard(cardId, fromCol, toCol, position);

    // Persist to backend
    try {
      await App.MoveKanbanCard(dir, cardId, toCol, position);
    } catch (err) {
      console.error('[kanban] move error:', err);
      loadBoard(); // Reload on failure
    }
  }

  function handleCardClick(e: CustomEvent<{ card: KanbanCard }>) {
    // Future: open card detail dialog
    console.log('[kanban] card click:', e.detail.card);
  }

  function handleCardDragStart(e: CustomEvent<{ card: KanbanCard; columnId: string }>) {
    kanban.startDrag(e.detail.card, e.detail.columnId);
  }

  async function handleAddCard() {
    if (!addCardTitle.trim() || !dir) return;
    const card: KanbanCard = {
      id: '',
      issue_number: 0,
      title: addCardTitle.trim(),
      labels: [],
      dir,
      session_id: 0,
      priority: 0,
      dependencies: [],
      plan_id: '',
      schedule_id: '',
      created_at: '',
      parent_issue: 0,
      prompt: '',
      auto_merge: false,
      auto_start: false,
      worktree_path: '',
      worktree_branch: '',
      agent_session_id: 0,
      review_result: '',
      pr_number: 0,
      retry_count: 0,
      max_retries: 0,
    };
    try {
      const saved = await App.AddKanbanCard(dir, card);
      kanban.addCard(saved);
      addCardTitle = '';
      showAddCard = false;
    } catch (err) {
      console.error('[kanban] add card error:', err);
    }
  }

  async function handleRemoveCard(cardId: string) {
    try {
      await App.RemoveKanbanCard(dir, cardId);
      kanban.removeCard(cardId);
    } catch (err) {
      console.error('[kanban] remove card error:', err);
    }
  }

  function handleAddCardKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') handleAddCard();
    if (e.key === 'Escape') { showAddCard = false; addCardTitle = ''; }
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
      <button class="btn-toolbar" on:click={() => { showAddCard = !showAddCard; }} title="Karte hinzufügen">
        + Karte
      </button>
      <button class="btn-toolbar" on:click={handleSync} title="Issues synchronisieren">
        &#8635; Sync
      </button>
      {#if orchStatus.active}
        <button class="btn-toolbar btn-stop" on:click={handleStopOrchestration} disabled={orchLoading} title="Orchestrierung stoppen">
          &#9724; Stoppen
        </button>
      {:else}
        <button class="btn-toolbar btn-start" on:click={handleStartOrchestration} disabled={orchLoading} title="Orchestrierung starten">
          &#9654; Starten
        </button>
      {/if}
      <button
        class="btn-tab"
        class:active={$kanban.activeTab === 'board'}
        on:click={() => kanban.setActiveTab('board')}
      >Board</button>
      <button
        class="btn-tab"
        class:active={$kanban.activeTab === 'schedules'}
        on:click={() => kanban.setActiveTab('schedules')}
      >Zeitpläne</button>
    </div>
  </div>

  {#if $activePlans.length > 0}
    <div class="plans-bar">
      {#each $activePlans as plan (plan.id)}
        <div class="plan-badge" class:running={plan.status === 'running'} class:draft={plan.status === 'draft'}>
          <span class="plan-status-dot"></span>
          <span class="plan-label">
            {plan.status === 'running' ? 'Ausführung' : plan.status === 'draft' ? 'Entwurf' : 'Genehmigt'}:
            {plan.steps.length} Schritte
          </span>
          <span class="plan-progress">
            ({plan.steps.filter(s => s.status === 'done').length}/{plan.steps.length})
          </span>
        </div>
      {/each}
    </div>
  {/if}

  {#if orchStatus.active || orchStatus.done_tickets > 0}
    <div class="orch-status-bar">
      <span class="orch-indicator" class:active={orchStatus.active}></span>
      <span class="orch-label">
        Agenten: {orchStatus.running_agents}/{orchStatus.max_agents}
      </span>
      <span class="orch-sep">|</span>
      <span class="orch-label">Bereit: {orchStatus.pending_tickets}</span>
      <span class="orch-sep">|</span>
      <span class="orch-label">Review: {orchStatus.review_tickets}</span>
      <span class="orch-sep">|</span>
      <span class="orch-label">Erledigt: {orchStatus.done_tickets}</span>
    </div>
  {/if}

  {#if Object.keys($parentIssueProgress).length > 0}
    <div class="parent-progress-bar">
      {#each Object.entries($parentIssueProgress) as [issueNum, prog]}
        <div class="parent-progress-badge">
          <span class="parent-issue-num">#{issueNum}</span>
          <span class="parent-progress-text">{prog.done}/{prog.total} Sub-Tickets erledigt</span>
          <div class="parent-progress-track">
            <div class="parent-progress-fill" style="width: {prog.total > 0 ? (prog.done / prog.total * 100) : 0}%"></div>
          </div>
        </div>
      {/each}
    </div>
  {/if}

  {#if showAddCard}
    <div class="add-card-row">
      <input
        class="add-card-input"
        placeholder="Titel der neuen Karte..."
        bind:value={addCardTitle}
        on:keydown={handleAddCardKeydown}
        autofocus
      />
      <button class="btn-add" on:click={handleAddCard}>Hinzufügen</button>
      <button class="btn-cancel" on:click={() => { showAddCard = false; addCardTitle = ''; }}>&#10005;</button>
    </div>
  {/if}

  {#if $kanban.loading}
    <div class="loading">Board wird geladen...</div>
  {:else if $kanban.activeTab === 'board'}
    <div class="columns-container">
      {#each COLUMN_IDS as colId (colId)}
        <KanbanColumn
          columnId={colId}
          cards={$kanban.state.columns[colId] || []}
          on:drop={handleDrop}
          on:cardClick={handleCardClick}
          on:cardDragStart={handleCardDragStart}
        />
      {/each}
    </div>
  {:else}
    <div class="schedules-panel">
      {#if $kanban.state.schedules.length === 0}
        <div class="empty-schedules">
          <p>Keine Zeitpläne konfiguriert</p>
          <p class="empty-hint">Zeitpläne ermöglichen wiederkehrende Automatisierungen</p>
        </div>
      {:else}
        <div class="schedule-list">
          {#each $kanban.state.schedules as task (task.id)}
            <div class="schedule-row" class:disabled={!task.enabled}>
              <div class="schedule-info">
                <span class="schedule-name">{task.name}</span>
                <span class="schedule-detail">{task.schedule} · {task.mode}</span>
              </div>
              <span class="schedule-next">{task.next_run ? new Date(task.next_run).toLocaleString('de-DE') : '—'}</span>
              <span class="schedule-status" class:active={task.enabled}>
                {task.enabled ? 'Aktiv' : 'Inaktiv'}
              </span>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>

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

  .btn-tab {
    padding: 4px 10px;
    border-radius: 6px;
    background: transparent;
    border: 1px solid transparent;
    color: var(--fg-muted, #a6adc8);
    cursor: pointer;
    font-size: 0.75rem;
    transition: all 0.15s;
  }
  .btn-tab:hover { color: var(--fg, #cdd6f4); }
  .btn-tab.active {
    background: rgba(57, 255, 20, 0.08);
    color: var(--accent, #39ff14);
    border-color: rgba(57, 255, 20, 0.2);
  }

  .add-card-row {
    display: flex;
    gap: 8px;
    padding: 8px 16px;
    background: var(--bg-secondary, #1e1e2e);
    border-bottom: 1px solid var(--border, #45475a);
  }
  .add-card-input {
    flex: 1;
    padding: 6px 10px;
    border-radius: 6px;
    border: 1px solid var(--border, #45475a);
    background: var(--bg, #11111b);
    color: var(--fg, #cdd6f4);
    font-size: 0.8rem;
    outline: none;
  }
  .add-card-input:focus { border-color: var(--accent, #39ff14); }
  .btn-add {
    padding: 6px 12px;
    border-radius: 6px;
    background: var(--accent, #39ff14);
    border: none;
    color: #000;
    font-weight: 600;
    cursor: pointer;
    font-size: 0.75rem;
  }
  .btn-add:hover { opacity: 0.85; }
  .btn-cancel {
    padding: 6px 8px;
    border-radius: 6px;
    background: transparent;
    border: 1px solid var(--border, #45475a);
    color: var(--fg-muted, #a6adc8);
    cursor: pointer;
    font-size: 0.8rem;
  }

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
    grid-template-columns: repeat(7, 1fr);
    gap: 8px;
    padding: 12px;
    flex: 1;
    overflow-x: auto;
    overflow-y: hidden;
  }

  /* Schedules panel */
  .schedules-panel {
    flex: 1;
    padding: 16px;
    overflow-y: auto;
  }
  .empty-schedules {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 200px;
    color: var(--fg-muted, #a6adc8);
    font-size: 0.9rem;
  }
  .empty-hint {
    font-size: 0.75rem;
    opacity: 0.6;
    margin-top: 4px;
  }
  .schedule-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .schedule-row {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    background: var(--bg-secondary, #1e1e2e);
    border: 1px solid var(--border, #45475a);
    border-radius: 8px;
    transition: opacity 0.15s;
  }
  .schedule-row.disabled { opacity: 0.5; }
  .schedule-info { flex: 1; min-width: 0; }
  .schedule-name {
    display: block;
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--fg, #cdd6f4);
  }
  .schedule-detail {
    display: block;
    font-size: 0.7rem;
    color: var(--fg-muted, #a6adc8);
  }
  .schedule-next {
    font-size: 0.7rem;
    color: var(--fg-muted, #a6adc8);
    white-space: nowrap;
  }
  .schedule-status {
    font-size: 0.65rem;
    padding: 2px 6px;
    border-radius: 4px;
    font-weight: 600;
    text-transform: uppercase;
    background: rgba(166, 173, 200, 0.15);
    color: var(--fg-muted, #a6adc8);
    white-space: nowrap;
  }
  .schedule-status.active {
    background: rgba(57, 255, 20, 0.15);
    color: var(--accent, #39ff14);
  }

  /* Plans bar */
  .plans-bar {
    display: flex;
    gap: 8px;
    padding: 6px 16px;
    background: var(--bg-secondary, #1e1e2e);
    border-bottom: 1px solid var(--border, #45475a);
    flex-wrap: wrap;
  }
  .plan-badge {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    border-radius: 6px;
    font-size: 0.72rem;
    background: rgba(137, 180, 250, 0.1);
    border: 1px solid rgba(137, 180, 250, 0.2);
    color: #89b4fa;
  }
  .plan-badge.running {
    background: rgba(57, 255, 20, 0.08);
    border-color: rgba(57, 255, 20, 0.25);
    color: var(--accent, #39ff14);
  }
  .plan-badge.draft {
    background: rgba(166, 173, 200, 0.08);
    border-color: rgba(166, 173, 200, 0.2);
    color: var(--fg-muted, #a6adc8);
  }
  .plan-status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
    flex-shrink: 0;
  }
  .plan-badge.running .plan-status-dot {
    animation: pulse-dot 1.5s ease-in-out infinite;
  }
  @keyframes pulse-dot {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.3; }
  }
  .plan-label { font-weight: 500; }
  .plan-progress { opacity: 0.7; }

  /* Orchestration controls */
  .btn-start {
    border-color: rgba(57, 255, 20, 0.4);
    color: var(--accent, #39ff14);
  }
  .btn-start:hover { border-color: var(--accent, #39ff14); background: rgba(57, 255, 20, 0.08); }
  .btn-stop {
    border-color: rgba(239, 68, 68, 0.4);
    color: #f87171;
  }
  .btn-stop:hover { border-color: #ef4444; background: rgba(239, 68, 68, 0.08); }
  .btn-toolbar:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  /* Orchestration status bar */
  .orch-status-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 16px;
    background: var(--bg-secondary, #1e1e2e);
    border-bottom: 1px solid var(--border, #45475a);
    font-size: 0.72rem;
    color: var(--fg-muted, #a6adc8);
    flex-shrink: 0;
  }
  .orch-indicator {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--fg-muted, #a6adc8);
    flex-shrink: 0;
  }
  .orch-indicator.active {
    background: var(--accent, #39ff14);
    animation: pulse-dot 1.5s ease-in-out infinite;
  }
  .orch-label {
    white-space: nowrap;
  }
  .orch-sep {
    opacity: 0.3;
  }

  /* Parent issue progress */
  .parent-progress-bar {
    display: flex;
    gap: 8px;
    padding: 6px 16px;
    background: var(--bg-secondary, #1e1e2e);
    border-bottom: 1px solid var(--border, #45475a);
    flex-wrap: wrap;
    flex-shrink: 0;
  }
  .parent-progress-badge {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    border-radius: 6px;
    font-size: 0.72rem;
    background: rgba(139, 92, 246, 0.08);
    border: 1px solid rgba(139, 92, 246, 0.2);
    color: #a78bfa;
  }
  .parent-issue-num {
    font-weight: 700;
  }
  .parent-progress-text {
    font-weight: 500;
    opacity: 0.9;
  }
  .parent-progress-track {
    width: 40px;
    height: 4px;
    border-radius: 2px;
    background: rgba(139, 92, 246, 0.15);
    overflow: hidden;
  }
  .parent-progress-fill {
    height: 100%;
    background: #a78bfa;
    border-radius: 2px;
    transition: width 0.3s ease;
  }
</style>
