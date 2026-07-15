<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';

  export let dir = '';

  const dispatch = createEventDispatcher<{
    navigate: { sessionId: number };
  }>();

  interface QueueItem {
    id: number;
    prompt: string;
    status: string;
  }

  interface QueueSession {
    session_id: number;
    session_name: string;
    dir: string;
    activity: string;
    items: QueueItem[];
  }

  let sessions: QueueSession[] = [];
  let loading = true;
  let filter: 'all' | 'pending' | 'sent' | 'done' = 'all';

  function loadQueues() {
    App.GetAllQueues()
      .then(data => {
        sessions = (data || []).filter(s => !dir || s.dir === dir);
        loading = false;
      })
      .catch(() => { loading = false; });
  }

  onMount(loadQueues);

  // Auto-refresh every 3 seconds
  let interval = setInterval(loadQueues, 3000);
  onDestroy(() => clearInterval(interval));

  $: filteredSessions = sessions.map(s => ({
    ...s,
    items: filter === 'all' ? s.items : s.items.filter(i => i.status === filter),
  })).filter(s => s.items.length > 0);

  $: totalPending = sessions.reduce((n, s) => n + s.items.filter(i => i.status === 'pending').length, 0);
  $: totalSent = sessions.reduce((n, s) => n + s.items.filter(i => i.status === 'sent').length, 0);
  $: totalDone = sessions.reduce((n, s) => n + s.items.filter(i => i.status === 'done').length, 0);

  function handleRemove(sessionId: number, itemId: number) {
    App.RemoveFromQueue(sessionId, itemId)
      .then(loadQueues)
      .catch(err => console.error('[queue] remove error:', err));
  }

  function handleClearDone(sessionId: number) {
    App.ClearDoneFromQueue(sessionId)
      .then(loadQueues)
      .catch(err => console.error('[queue] clear error:', err));
  }

  function handleNavigate(sessionId: number) {
    dispatch('navigate', { sessionId });
  }

  function statusColor(status: string): string {
    switch (status) {
      case 'pending': return '#a6adc8';
      case 'sent': return '#39ff14';
      case 'done': return '#22c55e';
      default: return '#a6adc8';
    }
  }

  function statusLabel(status: string): string {
    switch (status) {
      case 'pending': return 'Wartend';
      case 'sent': return 'Aktiv';
      case 'done': return 'Erledigt';
      default: return status;
    }
  }

  function activityDot(activity: string): string {
    switch (activity) {
      case 'active': return '#39ff14';
      case 'done': return '#22c55e';
      case 'waitingPermission':
      case 'waitingAnswer': return '#f5a623';
      default: return '#a6adc8';
    }
  }
</script>

<div class="queue-overview">
  <div class="queue-toolbar">
    <h2 class="queue-title">Queue-Übersicht</h2>
    <div class="queue-stats">
      <span class="stat">{totalPending} wartend</span>
      <span class="stat">{totalSent} aktiv</span>
      <span class="stat">{totalDone} erledigt</span>
    </div>
    <div class="queue-spacer"></div>
    <div class="filter-tabs">
      <button class="filter-tab" class:active={filter === 'all'} on:click={() => filter = 'all'}>Alle</button>
      <button class="filter-tab" class:active={filter === 'pending'} on:click={() => filter = 'pending'}>Wartend</button>
      <button class="filter-tab" class:active={filter === 'sent'} on:click={() => filter = 'sent'}>Aktiv</button>
      <button class="filter-tab" class:active={filter === 'done'} on:click={() => filter = 'done'}>Erledigt</button>
    </div>
  </div>

  {#if loading}
    <div class="queue-loading">Queue wird geladen...</div>
  {:else if filteredSessions.length === 0}
    <div class="queue-empty">
      <p>Keine Queue-Einträge</p>
      <p class="empty-hint">Prompts können über das Terminal-Panel zur Queue hinzugefügt werden</p>
    </div>
  {:else}
    <div class="queue-sessions">
      {#each filteredSessions as sess (sess.session_id)}
        <div class="session-group">
          <div class="session-header">
            <span class="session-dot" style="background: {activityDot(sess.activity)}"></span>
            <button class="session-name" on:click={() => handleNavigate(sess.session_id)}>
              {sess.session_name || `Session ${sess.session_id}`}
            </button>
            <span class="session-count">{sess.items.length}</span>
            <button class="btn-clear" on:click={() => handleClearDone(sess.session_id)} title="Erledigte löschen">&#128465;</button>
          </div>
          <div class="session-items">
            {#each sess.items as item (item.id)}
              <div class="queue-item">
                <span class="item-status" style="color: {statusColor(item.status)}">{statusLabel(item.status)}</span>
                <span class="item-prompt">{item.prompt}</span>
                {#if item.status === 'pending'}
                  <button class="btn-remove" on:click={() => handleRemove(sess.session_id, item.id)} title="Entfernen">&#10005;</button>
                {/if}
              </div>
            {/each}
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .queue-overview {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    height: 100%;
    background: var(--bg, #11111b);
    overflow: hidden;
  }

  .queue-toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 16px;
    border-bottom: 1px solid var(--border, #45475a);
    background: var(--bg-secondary, #1e1e2e);
    flex-shrink: 0;
  }
  .queue-title {
    font-size: 1rem;
    font-weight: 700;
    color: var(--fg, #cdd6f4);
  }
  .queue-stats {
    display: flex;
    gap: 8px;
    font-size: 0.7rem;
    color: var(--fg-muted, #a6adc8);
  }
  .stat {
    padding: 2px 6px;
    background: var(--bg, #11111b);
    border-radius: 4px;
  }
  .queue-spacer { flex: 1; }

  .filter-tabs {
    display: flex;
    gap: 4px;
  }
  .filter-tab {
    padding: 4px 8px;
    border-radius: 6px;
    background: transparent;
    border: 1px solid transparent;
    color: var(--fg-muted, #a6adc8);
    cursor: pointer;
    font-size: 0.7rem;
  }
  .filter-tab:hover { color: var(--fg, #cdd6f4); }
  .filter-tab.active {
    background: rgba(57, 255, 20, 0.08);
    color: var(--accent, #39ff14);
    border-color: rgba(57, 255, 20, 0.2);
  }

  .queue-loading, .queue-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    flex: 1;
    color: var(--fg-muted, #a6adc8);
    font-size: 0.9rem;
  }
  .empty-hint { font-size: 0.75rem; opacity: 0.5; margin-top: 4px; }

  .queue-sessions {
    flex: 1;
    overflow-y: auto;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .session-group {
    background: var(--bg-secondary, #1e1e2e);
    border: 1px solid var(--border, #45475a);
    border-radius: 8px;
    overflow: hidden;
  }
  .session-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: var(--bg-tertiary, #313244);
    border-bottom: 1px solid var(--border, #45475a);
  }
  .session-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .session-name {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--fg, #cdd6f4);
    background: none;
    border: none;
    cursor: pointer;
    text-align: left;
    flex: 1;
  }
  .session-name:hover { color: var(--accent, #39ff14); }
  .session-count {
    font-size: 0.65rem;
    color: var(--fg-muted, #a6adc8);
    background: var(--bg, #11111b);
    padding: 1px 5px;
    border-radius: 8px;
  }
  .btn-clear {
    background: none;
    border: none;
    cursor: pointer;
    font-size: 0.7rem;
    opacity: 0.4;
    transition: opacity 0.15s;
  }
  .btn-clear:hover { opacity: 1; }

  .session-items {
    display: flex;
    flex-direction: column;
  }
  .queue-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border, #45475a);
  }
  .queue-item:last-child { border-bottom: none; }

  .item-status {
    font-size: 0.65rem;
    font-weight: 600;
    text-transform: uppercase;
    min-width: 50px;
  }
  .item-prompt {
    flex: 1;
    font-size: 0.8rem;
    color: var(--fg, #cdd6f4);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .btn-remove {
    background: none;
    border: none;
    color: var(--fg-muted, #a6adc8);
    cursor: pointer;
    font-size: 0.7rem;
    opacity: 0.5;
    transition: opacity 0.15s;
  }
  .btn-remove:hover { opacity: 1; color: #e05252; }
</style>
