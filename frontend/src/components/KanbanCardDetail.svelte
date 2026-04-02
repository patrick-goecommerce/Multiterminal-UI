<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';
  import { board } from '../../wailsjs/go/models';
  import DecisionBriefing from './DecisionBriefing.svelte';
  import EscalationDialog from './EscalationDialog.svelte';

  export let visible = false;
  export let cardId = '';
  export let dir = '';

  const dispatch = createEventDispatcher<{
    close: void;
    updated: void;
  }>();

  let card: board.TaskCard | null = null;
  let plan: board.Plan | null = null;
  let loading = false;
  let error = '';
  let showEscalation = false;

  // Briefing data — populated from backend context when card is in qa state
  let briefing: any = null;

  // State transition map: valid events per state (German labels)
  const STATE_TRANSITIONS: Record<string, { event: board.Event; label: string }[]> = {
    backlog: [
      { event: 'start_triage', label: 'Triage starten' },
    ],
    triage: [
      { event: 'complexity_trivial', label: 'Trivial (direkt)' },
      { event: 'complexity_non_trivial', label: 'Nicht-trivial (planen)' },
    ],
    planning: [
      { event: 'plan_ready', label: 'Plan fertig' },
    ],
    review: [
      { event: 'approved', label: 'Genehmigen' },
      { event: 'rejected', label: 'Ablehnen' },
    ],
    executing: [
      { event: 'all_steps_done', label: 'Alle Schritte fertig' },
      { event: 'step_stuck', label: 'Blockiert melden' },
    ],
    stuck: [
      { event: 'model_escalated', label: 'Modell eskaliert' },
      { event: 'replan_completed', label: 'Neu geplant' },
      { event: 'max_escalations', label: 'Max. Eskalationen' },
    ],
    qa: [
      { event: 'qa_passed', label: 'QA bestanden' },
      { event: 'qa_failed', label: 'QA fehlgeschlagen' },
    ],
    merging: [
      { event: 'merge_success', label: 'Merge erfolgreich' },
      { event: 'merge_conflict', label: 'Merge-Konflikt' },
    ],
    human_review: [
      { event: 'user_resolved_executing', label: 'Weiter versuchen' },
      { event: 'user_resolved_done', label: 'Als erledigt' },
      { event: 'user_resolved_backlog', label: 'Zurueck ins Backlog' },
    ],
  };

  const STEP_STATUS_STYLES: Record<string, { label: string; color: string }> = {
    pending: { label: 'Ausstehend', color: '#9ca3af' },
    running: { label: 'Laeuft', color: '#f59e0b' },
    done: { label: 'Erledigt', color: '#22c55e' },
    failed: { label: 'Fehlgeschlagen', color: '#ef4444' },
    stuck: { label: 'Blockiert', color: '#ef4444' },
    skipped: { label: 'Uebersprungen', color: '#6b7280' },
  };

  const STATE_LABELS: Record<string, string> = {
    backlog: 'Backlog',
    triage: 'Triage',
    planning: 'Planung',
    review: 'Review',
    executing: 'Ausfuehrung',
    stuck: 'Blockiert',
    qa: 'Qualitaetspruefung',
    merging: 'Merge',
    human_review: 'Manuelle Pruefung',
    done: 'Erledigt',
  };

  function loadCard() {
    if (!dir || !cardId) return;
    loading = true;
    error = '';
    card = null;
    plan = null;
    briefing = null;

    App.GetBoardTask(dir, cardId)
      .then(c => {
        card = c;
        loading = false;
        // Load plan if card is beyond planning
        if (c.state !== 'backlog' && c.state !== 'triage') {
          App.GetBoardPlan(dir, cardId)
            .then(p => { plan = p; })
            .catch(() => { plan = null; });
        }
      })
      .catch(err => {
        error = String(err);
        loading = false;
      });
  }

  $: if (visible && cardId) loadCard();

  $: transitions = card ? (STATE_TRANSITIONS[card.state] || []) : [];

  async function handleTransition(event: board.Event) {
    if (!card) return;
    try {
      await App.MoveBoardTask(dir, card.id, event);
      dispatch('updated');
      loadCard();
    } catch (err) {
      console.error('[detail] transition error:', err);
    }
  }

  function close() {
    visible = false;
    dispatch('close');
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
  }

  function handleOverlayClick() {
    close();
  }

  function handleEscalationResolved() {
    showEscalation = false;
    dispatch('updated');
    loadCard();
  }

  function stepStatus(status: string) {
    return STEP_STATUS_STYLES[status] || STEP_STATUS_STYLES.pending;
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="overlay" on:click={handleOverlayClick}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation on:keydown={handleKeydown} tabindex="-1">
      {#if loading}
        <div class="loading-state">Karte wird geladen...</div>
      {:else if error}
        <div class="error-state">Fehler: {error}</div>
      {:else if card}
        <!-- Header -->
        <div class="detail-header">
          <h3 class="detail-title">{card.title}</h3>
          <button class="close-btn" on:click={close} title="Schliessen">&#10005;</button>
        </div>

        <!-- State & Meta -->
        <div class="detail-meta">
          <span class="state-badge" title="Status">{STATE_LABELS[card.state] || card.state}</span>
          {#if card.card_type}
            <span class="meta-tag">{card.card_type}</span>
          {/if}
          {#if card.complexity}
            <span class="meta-tag">Komplex.: {card.complexity}</span>
          {/if}
          {#if card.cost_usd > 0}
            <span class="cost-tag">${card.cost_usd.toFixed(2)}</span>
          {/if}
          {#if card.qa_attempts > 0}
            <span class="meta-tag">QA: {card.qa_attempts}/3</span>
          {/if}
          {#if card.esc_attempts > 0}
            <span class="meta-tag esc-tag">Esc: {card.esc_attempts}/2</span>
          {/if}
        </div>

        <!-- Description -->
        {#if card.description}
          <div class="detail-section">
            <span class="section-label">Beschreibung</span>
            <p class="detail-desc">{card.description}</p>
          </div>
        {/if}

        <!-- Review reason -->
        {#if card.review_reason}
          <div class="detail-section warning-section">
            <span class="section-label">Review-Grund</span>
            <p class="detail-desc warning-text">{card.review_reason}</p>
          </div>
        {/if}

        <!-- Plan display -->
        {#if plan && plan.steps?.length > 0}
          <div class="detail-section">
            <span class="section-label">Plan ({plan.steps.length} Schritte)</span>
            <div class="step-list">
              {#each plan.steps as step, i}
                <div class="step-item">
                  <span class="step-num">{i + 1}</span>
                  <span class="step-status-dot" style="background: {stepStatus(step.status).color}" title={stepStatus(step.status).label}></span>
                  <span class="step-title">{step.title}</span>
                  {#if step.wave > 0}
                    <span class="step-wave" title="Welle {step.wave}">W{step.wave}</span>
                  {/if}
                  {#if step.model}
                    <span class="step-model">{step.model}</span>
                  {/if}
                </div>
              {/each}
            </div>
          </div>
        {/if}

        <!-- Decision Briefing (for qa state) -->
        {#if card.state === 'qa' && briefing}
          <DecisionBriefing {briefing} visible={true} />
        {/if}

        <!-- Escalation prompt (for human_review state) -->
        {#if card.state === 'human_review'}
          <div class="detail-section warning-section">
            <span class="section-label">Eskalation</span>
            <p class="detail-desc warning-text">
              Diese Karte erfordert manuelle Pruefung.
            </p>
            <button class="btn-escalation" on:click={() => { showEscalation = true; }}>
              Eskalation bearbeiten
            </button>
          </div>
        {/if}

        <!-- Transitions -->
        {#if transitions.length > 0}
          <div class="detail-section">
            <span class="section-label">Aktionen</span>
            <div class="transition-buttons">
              {#each transitions as tr}
                <button class="btn-transition" on:click={() => handleTransition(tr.event)}>
                  {tr.label}
                </button>
              {/each}
            </div>
          </div>
        {/if}
      {/if}
    </div>
  </div>

  <!-- Escalation Dialog -->
  {#if showEscalation && card}
    <EscalationDialog
      bind:visible={showEscalation}
      cardId={card.id}
      {dir}
      reason={card.review_reason}
      on:resolved={handleEscalationResolved}
    />
  {/if}
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .dialog {
    background: var(--bg, #11111b);
    border: 1px solid var(--border, #45475a);
    border-radius: 12px;
    padding: 20px;
    min-width: 420px;
    max-width: 600px;
    max-height: 80vh;
    overflow-y: auto;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
    outline: none;
  }

  .loading-state, .error-state {
    padding: 20px;
    text-align: center;
    color: var(--fg-muted, #a6adc8);
    font-size: 0.85rem;
  }
  .error-state { color: #ef4444; }

  .detail-header {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    margin-bottom: 12px;
  }

  .detail-title {
    flex: 1;
    font-size: 1rem;
    font-weight: 700;
    color: var(--fg, #cdd6f4);
    margin: 0;
    line-height: 1.3;
  }

  .close-btn {
    padding: 2px 6px;
    background: transparent;
    border: none;
    color: var(--fg-muted, #a6adc8);
    cursor: pointer;
    font-size: 0.9rem;
    flex-shrink: 0;
  }
  .close-btn:hover { color: #ef4444; }

  .detail-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 12px;
  }

  .state-badge {
    font-size: 0.65rem;
    padding: 2px 8px;
    border-radius: 4px;
    background: var(--accent, #39ff14);
    color: #000;
    font-weight: 600;
  }

  .meta-tag {
    font-size: 0.65rem;
    padding: 2px 6px;
    border-radius: 3px;
    background: rgba(166, 173, 200, 0.15);
    color: var(--fg-muted, #a6adc8);
    font-weight: 500;
  }

  .esc-tag {
    background: rgba(249, 115, 22, 0.15);
    color: #f97316;
  }

  .cost-tag {
    font-size: 0.65rem;
    padding: 2px 6px;
    border-radius: 3px;
    background: rgba(34, 197, 94, 0.15);
    color: #22c55e;
    font-weight: 600;
  }

  .detail-section {
    margin-bottom: 12px;
    padding: 8px 10px;
    background: var(--bg-secondary, #1e1e2e);
    border-radius: 6px;
  }

  .warning-section {
    border-left: 3px solid #f97316;
  }

  .section-label {
    display: block;
    font-size: 0.7rem;
    font-weight: 600;
    color: var(--fg, #cdd6f4);
    margin-bottom: 4px;
  }

  .detail-desc {
    font-size: 0.8rem;
    color: var(--fg-muted, #a6adc8);
    margin: 0;
    line-height: 1.4;
  }

  .warning-text {
    color: #f97316;
  }

  .step-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .step-item {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 3px 0;
  }

  .step-num {
    font-size: 0.6rem;
    width: 18px;
    text-align: center;
    color: var(--fg-muted, #a6adc8);
    flex-shrink: 0;
  }

  .step-status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .step-title {
    flex: 1;
    font-size: 0.75rem;
    color: var(--fg, #cdd6f4);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .step-wave {
    font-size: 0.55rem;
    padding: 0 4px;
    border-radius: 3px;
    background: rgba(139, 92, 246, 0.2);
    color: #8b5cf6;
    font-weight: 600;
  }

  .step-model {
    font-size: 0.55rem;
    color: var(--fg-muted, #a6adc8);
    opacity: 0.6;
  }

  .transition-buttons {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .btn-transition {
    padding: 6px 12px;
    border-radius: 6px;
    background: var(--bg, #11111b);
    border: 1px solid var(--border, #45475a);
    color: var(--fg, #cdd6f4);
    cursor: pointer;
    font-size: 0.7rem;
    font-weight: 500;
    transition: border-color 0.15s;
  }
  .btn-transition:hover {
    border-color: var(--accent, #39ff14);
  }

  .btn-escalation {
    margin-top: 6px;
    padding: 6px 14px;
    border-radius: 6px;
    background: #f97316;
    border: none;
    color: #fff;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
  }
  .btn-escalation:hover { opacity: 0.85; }
</style>
