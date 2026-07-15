<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';

  export let visible = false;
  export let cardId = '';
  export let dir = '';
  export let reason = ''; // "step_stuck" | "qa_fix_exhausted" | "scope_expansion_required" | "max_escalations_reached"
  export let context: any = null; // { failures?: string[], files?: string[], message?: string }

  const dispatch = createEventDispatcher<{
    resolved: { action: string };
  }>();

  let resolving = false;

  const REASON_LABELS: Record<string, string> = {
    step_stuck: 'Schritt blockiert',
    qa_fix_exhausted: 'QA-Korrekturen erschoepft',
    scope_expansion_required: 'Scope-Erweiterung noetig',
    max_escalations_reached: 'Max. Eskalationen erreicht',
    model_escalated: 'Modell eskaliert',
  };

  $: reasonLabel = REASON_LABELS[reason] || reason || 'Unbekannt';

  async function resolveToExecuting() {
    resolving = true;
    try {
      await App.MoveBoardTask(dir, cardId, 'user_resolved_executing');
      dispatch('resolved', { action: 'executing' });
      visible = false;
    } catch (err) {
      console.error('[escalation] resolve to executing failed:', err);
    } finally {
      resolving = false;
    }
  }

  async function resolveToDone() {
    resolving = true;
    try {
      await App.MoveBoardTask(dir, cardId, 'user_resolved_done');
      dispatch('resolved', { action: 'done' });
      visible = false;
    } catch (err) {
      console.error('[escalation] resolve to done failed:', err);
    } finally {
      resolving = false;
    }
  }

  async function resolveToBacklog() {
    resolving = true;
    try {
      await App.MoveBoardTask(dir, cardId, 'user_resolved_backlog');
      dispatch('resolved', { action: 'backlog' });
      visible = false;
    } catch (err) {
      console.error('[escalation] resolve to backlog failed:', err);
    } finally {
      resolving = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') { visible = false; }
  }

  function handleOverlayClick() {
    visible = false;
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="overlay" on:click={handleOverlayClick}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation on:keydown={handleKeydown} tabindex="-1">
      <div class="dialog-header">
        <h3 class="dialog-title">Manuelle Pruefung erforderlich</h3>
        <span class="reason-badge">{reasonLabel}</span>
      </div>

      {#if context?.message}
        <p class="context-message">{context.message}</p>
      {/if}

      {#if context?.failures?.length > 0}
        <div class="context-section">
          <span class="context-label">Fehler:</span>
          <ul class="context-list">
            {#each context.failures as failure}
              <li>{failure}</li>
            {/each}
          </ul>
        </div>
      {/if}

      {#if context?.files?.length > 0}
        <div class="context-section">
          <span class="context-label">Betroffene Dateien:</span>
          <div class="file-tags">
            {#each context.files as file}
              <span class="file-tag">{file}</span>
            {/each}
          </div>
        </div>
      {/if}

      <div class="dialog-actions">
        <button
          class="btn btn-green"
          on:click={resolveToExecuting}
          disabled={resolving}
          title="Karte zurueck in Ausfuehrung setzen"
        >
          Weiter versuchen
        </button>
        <button
          class="btn btn-blue"
          on:click={resolveToDone}
          disabled={resolving}
          title="Karte als erledigt markieren"
        >
          Als erledigt markieren
        </button>
        <button
          class="btn btn-gray"
          on:click={resolveToBacklog}
          disabled={resolving}
          title="Karte zurueck ins Backlog"
        >
          Zurueck zum Backlog
        </button>
      </div>
    </div>
  </div>
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
    min-width: 380px;
    max-width: 520px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
    outline: none;
  }

  .dialog-header {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 12px;
  }

  .dialog-title {
    font-size: 0.95rem;
    font-weight: 700;
    color: var(--fg, #cdd6f4);
    margin: 0;
  }

  .reason-badge {
    font-size: 0.6rem;
    padding: 2px 8px;
    border-radius: 4px;
    background: #f97316;
    color: #fff;
    font-weight: 600;
    white-space: nowrap;
  }

  .context-message {
    font-size: 0.8rem;
    color: var(--fg-muted, #a6adc8);
    margin: 0 0 10px 0;
  }

  .context-section {
    margin-bottom: 10px;
  }

  .context-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--fg, #cdd6f4);
    display: block;
    margin-bottom: 4px;
  }

  .context-list {
    list-style: disc;
    margin: 0;
    padding-left: 16px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .context-list li {
    font-size: 0.7rem;
    color: #ef4444;
  }

  .file-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }
  .file-tag {
    font-size: 0.6rem;
    padding: 1px 6px;
    border-radius: 3px;
    background: rgba(166, 173, 200, 0.15);
    color: var(--fg-muted, #a6adc8);
    font-family: monospace;
  }

  .dialog-actions {
    display: flex;
    gap: 8px;
    margin-top: 16px;
  }

  .btn {
    flex: 1;
    padding: 8px 12px;
    border-radius: 6px;
    border: none;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    transition: opacity 0.15s;
  }
  .btn:hover { opacity: 0.85; }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .btn-green {
    background: #22c55e;
    color: #000;
  }
  .btn-blue {
    background: #3b82f6;
    color: #fff;
  }
  .btn-gray {
    background: var(--bg-secondary, #1e1e2e);
    border: 1px solid var(--border, #45475a);
    color: var(--fg-muted, #a6adc8);
  }
</style>
