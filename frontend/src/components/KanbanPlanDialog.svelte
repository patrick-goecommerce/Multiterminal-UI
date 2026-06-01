<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';
  import type { Plan, PlanStep } from '../stores/kanban';

  export let visible = false;
  export let plan: Plan | null = null;
  export let dir = '';

  const dispatch = createEventDispatcher<{
    close: void;
    approved: void;
    cancelled: void;
  }>();

  let parentIssue = 0;
  let loading = false;
  let steps: PlanStep[] = [];

  function initDialog() {
    if (!plan) return;
    parentIssue = 0;
    loading = false;
    steps = plan.steps.map(s => ({ ...s }));
  }

  $: if (visible && plan) initDialog();

  function moveStep(idx: number, direction: -1 | 1) {
    const target = idx + direction;
    if (target < 0 || target >= steps.length) return;
    [steps[idx], steps[target]] = [steps[target], steps[idx]];
    steps = steps.map((s, i) => ({ ...s, order: i + 1 }));
  }

  function toggleParallel(idx: number) {
    steps[idx].parallel = !steps[idx].parallel;
    steps = [...steps];
  }

  async function handleApprove() {
    if (!plan || !dir) return;
    loading = true;
    try {
      // Save step edits first
      for (const step of steps) {
        const orig = plan.steps.find(s => s.card_id === step.card_id);
        if (orig && (orig.prompt !== step.prompt || orig.parallel !== step.parallel || orig.order !== step.order)) {
          await App.UpdatePlanStep(dir, plan.id, step);
        }
      }
      await App.ApprovePlan(dir, plan.id, parentIssue);
      dispatch('approved');
      dispatch('close');
    } catch (err) {
      console.error('[plan-dialog] approve error:', err);
    } finally {
      loading = false;
    }
  }

  async function handleCancel() {
    if (!plan || !dir) return;
    loading = true;
    try {
      await App.CancelPlan(dir, plan.id);
      dispatch('cancelled');
      dispatch('close');
    } catch (err) {
      console.error('[plan-dialog] cancel error:', err);
    } finally {
      loading = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('close');
  }
</script>

{#if visible && plan}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <div class="backdrop" on:click={() => dispatch('close')} on:keydown={handleKeydown}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation>
      <div class="dialog-header">
        <span class="header-title">Plan genehmigen</span>
        <span class="header-status">{plan.status}</span>
        <div class="spacer"></div>
        <button class="btn-close" on:click={() => dispatch('close')}>&#10005;</button>
      </div>

      <div class="dialog-body">
        <div class="field">
          <label class="field-label">Parent Issue (optional)</label>
          <input class="field-input" type="number" bind:value={parentIssue} min="0" placeholder="0 = kein Parent" />
          <span class="field-hint">Sub-Tickets werden mit diesem Issue verknüpft</span>
        </div>

        <div class="steps-header">
          <span class="steps-title">Schritte ({steps.length})</span>
        </div>

        <div class="steps-list">
          {#each steps as step, idx (step.card_id || idx)}
            <div class="step-row" class:parallel={step.parallel}>
              <div class="step-order">
                <button class="btn-arrow" on:click={() => moveStep(idx, -1)} disabled={idx === 0}>&#9650;</button>
                <span class="step-num">{idx + 1}</span>
                <button class="btn-arrow" on:click={() => moveStep(idx, 1)} disabled={idx === steps.length - 1}>&#9660;</button>
              </div>
              <div class="step-content">
                <div class="step-title-row">
                  <span class="step-title">{step.title}</span>
                  {#if step.issue_number}
                    <span class="step-issue">#{step.issue_number}</span>
                  {/if}
                  <button
                    class="btn-parallel"
                    class:active={step.parallel}
                    on:click={() => toggleParallel(idx)}
                    title={step.parallel ? 'Parallel (klicken für sequenziell)' : 'Sequenziell (klicken für parallel)'}
                  >
                    {step.parallel ? '&#8644; Parallel' : '&#8595; Sequenziell'}
                  </button>
                </div>
                <textarea
                  class="step-prompt"
                  bind:value={step.prompt}
                  rows="2"
                  placeholder="Agent-Prompt..."
                ></textarea>
              </div>
            </div>
          {/each}
        </div>
      </div>

      <div class="dialog-footer">
        <button class="btn-cancel-plan" on:click={handleCancel} disabled={loading}>
          Plan abbrechen
        </button>
        <div class="spacer"></div>
        <button class="btn-dismiss" on:click={() => dispatch('close')}>Schließen</button>
        {#if plan.status === 'draft'}
          <button class="btn-approve" on:click={handleApprove} disabled={loading || steps.length === 0}>
            {loading ? 'Wird genehmigt...' : 'Genehmigen & Starten'}
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed; inset: 0;
    background: rgba(0,0,0,0.6);
    display: flex; align-items: center; justify-content: center;
    z-index: 1000;
  }
  .dialog {
    background: var(--bg-secondary, #1e1e2e);
    border: 1px solid var(--border, #45475a);
    border-radius: 12px;
    width: 640px; max-height: 85vh;
    display: flex; flex-direction: column;
    box-shadow: 0 20px 60px rgba(0,0,0,0.5);
  }
  .dialog-header {
    display: flex; align-items: center; gap: 10px;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border, #45475a);
  }
  .header-title { font-size: 0.95rem; font-weight: 700; color: var(--fg, #cdd6f4); }
  .header-status {
    font-size: 0.65rem; padding: 2px 8px; border-radius: 4px;
    background: rgba(137, 180, 250, 0.15); color: #89b4fa;
    font-weight: 600; text-transform: uppercase;
  }
  .spacer { flex: 1; }
  .btn-close {
    background: none; border: none; color: var(--fg-muted, #a6adc8);
    font-size: 1rem; cursor: pointer; padding: 4px;
  }

  .dialog-body {
    padding: 20px; overflow-y: auto;
    display: flex; flex-direction: column; gap: 16px;
  }

  .field { display: flex; flex-direction: column; gap: 4px; }
  .field-label {
    font-size: 0.7rem; font-weight: 600;
    color: var(--fg-muted, #a6adc8);
    text-transform: uppercase; letter-spacing: 0.04em;
  }
  .field-input {
    padding: 8px 10px; border-radius: 6px;
    border: 1px solid var(--border, #45475a);
    background: var(--bg, #11111b);
    color: var(--fg, #cdd6f4);
    font-size: 0.85rem; outline: none; max-width: 200px;
  }
  .field-input:focus { border-color: var(--accent, #39ff14); }
  .field-hint { font-size: 0.68rem; color: var(--fg-muted, #a6adc8); opacity: 0.7; }

  .steps-header { display: flex; align-items: center; gap: 8px; }
  .steps-title {
    font-size: 0.75rem; font-weight: 700;
    color: var(--fg, #cdd6f4); text-transform: uppercase; letter-spacing: 0.04em;
  }

  .steps-list { display: flex; flex-direction: column; gap: 8px; }
  .step-row {
    display: flex; gap: 10px;
    padding: 10px 12px;
    background: var(--bg, #11111b);
    border: 1px solid var(--border, #45475a);
    border-radius: 8px;
    transition: border-color 0.15s;
  }
  .step-row.parallel { border-left: 3px solid #3b82f6; }

  .step-order {
    display: flex; flex-direction: column; align-items: center; gap: 2px;
    min-width: 28px;
  }
  .btn-arrow {
    background: none; border: none; color: var(--fg-muted, #a6adc8);
    font-size: 0.6rem; cursor: pointer; padding: 2px;
    opacity: 0.6;
  }
  .btn-arrow:hover:not(:disabled) { opacity: 1; color: var(--fg, #cdd6f4); }
  .btn-arrow:disabled { opacity: 0.2; cursor: default; }
  .step-num {
    font-size: 0.75rem; font-weight: 700; color: var(--fg-muted, #a6adc8);
  }

  .step-content { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 6px; }
  .step-title-row { display: flex; align-items: center; gap: 8px; }
  .step-title {
    font-size: 0.82rem; font-weight: 600; color: var(--fg, #cdd6f4);
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1;
  }
  .step-issue { font-size: 0.7rem; color: var(--fg-muted, #a6adc8); }
  .btn-parallel {
    font-size: 0.62rem; padding: 2px 6px; border-radius: 4px;
    background: rgba(166, 173, 200, 0.1); border: 1px solid rgba(166, 173, 200, 0.2);
    color: var(--fg-muted, #a6adc8); cursor: pointer; white-space: nowrap;
  }
  .btn-parallel.active {
    background: rgba(59, 130, 246, 0.15); border-color: rgba(59, 130, 246, 0.3);
    color: #60a5fa;
  }

  .step-prompt {
    padding: 6px 8px; border-radius: 4px;
    border: 1px solid var(--border, #45475a);
    background: var(--bg-secondary, #1e1e2e);
    color: var(--fg, #cdd6f4);
    font-size: 0.78rem; outline: none; resize: vertical;
    font-family: monospace; line-height: 1.3;
  }
  .step-prompt:focus { border-color: var(--accent, #39ff14); }

  .dialog-footer {
    display: flex; align-items: center; gap: 8px;
    padding: 14px 20px;
    border-top: 1px solid var(--border, #45475a);
  }
  .btn-cancel-plan {
    padding: 6px 12px; border-radius: 6px;
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #f87171; cursor: pointer; font-size: 0.78rem;
  }
  .btn-dismiss {
    padding: 6px 14px; border-radius: 6px;
    background: transparent; border: 1px solid var(--border, #45475a);
    color: var(--fg-muted, #a6adc8); cursor: pointer; font-size: 0.78rem;
  }
  .btn-approve {
    padding: 6px 14px; border-radius: 6px;
    background: var(--accent, #39ff14); border: none;
    color: #000; font-weight: 600; cursor: pointer; font-size: 0.78rem;
  }
  .btn-approve:hover { opacity: 0.85; }
  .btn-approve:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
