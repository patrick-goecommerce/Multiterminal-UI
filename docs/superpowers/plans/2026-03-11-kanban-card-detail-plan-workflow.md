# Kanban Card Detail & Plan Workflow UI

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a card detail dialog for viewing/editing kanban cards and a plan creation/approval workflow so users can move cards from "Definieren" through plan generation to "Bereit".

**Architecture:** Two new Svelte dialog components (KanbanCardDetail, KanbanPlanDialog) integrated into the existing KanbanBoard. One new Go backend method (UpdateKanbanCard). Selection mode in KanbanBoard for multi-card plan generation. Plan badges become clickable to open the review dialog.

**Tech Stack:** Svelte 4, Go, Wails v3 `$Call.ByID` bindings

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `frontend/src/components/KanbanCardDetail.svelte` | CREATE | Modal dialog for viewing/editing a single card |
| `frontend/src/components/KanbanPlanDialog.svelte` | CREATE | Modal dialog for reviewing/approving a draft plan |
| `frontend/src/components/KanbanBoard.svelte` | MODIFY | Wire card click → detail dialog, add selection mode, plan badge click → plan dialog |
| `frontend/src/components/KanbanCard.svelte` | MODIFY | Add optional checkbox for selection mode |
| `internal/backend/app_kanban.go` | MODIFY | Add `UpdateKanbanCard` method |
| `frontend/wailsjs/go/backend/App.js` | MODIFY | Add `UpdateKanbanCard` binding |
| `frontend/wailsjs/go/backend/App.d.ts` | MODIFY | Add `UpdateKanbanCard` type declaration |

---

## Task 1: Backend — UpdateKanbanCard

**Files:**
- Modify: `internal/backend/app_kanban.go` (append after `RemoveKanbanCard`, ~line 157)
- Modify: `frontend/wailsjs/go/backend/App.js` (append)
- Modify: `frontend/wailsjs/go/backend/App.d.ts` (append)

- [ ] **Step 1: Add UpdateKanbanCard to app_kanban.go**

Add after the `RemoveKanbanCard` function (~line 157):

```go
// UpdateKanbanCard updates editable fields of a card in-place.
func (a *AppService) UpdateKanbanCard(dir string, card KanbanCard) error {
	state, err := loadKanbanState(dir)
	if err != nil {
		return fmt.Errorf("load kanban: %w", err)
	}

	for col, cards := range state.Columns {
		for i, c := range cards {
			if c.ID == card.ID {
				// Update editable fields only
				state.Columns[col][i].Title = card.Title
				state.Columns[col][i].Prompt = card.Prompt
				state.Columns[col][i].Priority = card.Priority
				state.Columns[col][i].ParentIssue = card.ParentIssue
				state.Columns[col][i].AutoMerge = card.AutoMerge
				state.Columns[col][i].AutoStart = card.AutoStart
				state.Columns[col][i].MaxRetries = card.MaxRetries
				state.Columns[col][i].Labels = card.Labels
				return saveKanbanState(dir, state)
			}
		}
	}
	return fmt.Errorf("card %s not found", card.ID)
}
```

- [ ] **Step 2: Add Wails v3 binding to App.js**

Append to `frontend/wailsjs/go/backend/App.js`:

```javascript
export function UpdateKanbanCard(arg1, arg2) {
  return $Call.ByID(48718319, arg1, arg2);
}
```

- [ ] **Step 3: Add TypeScript declaration to App.d.ts**

Append to `frontend/wailsjs/go/backend/App.d.ts`:

```typescript
export function UpdateKanbanCard(arg1:string,arg2:any):Promise<void>;
```

- [ ] **Step 4: Verify Go compiles**

Run: `cd D:/repos/Multiterminal && go build -tags desktop ./internal/backend/...`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_kanban.go frontend/wailsjs/go/backend/App.js frontend/wailsjs/go/backend/App.d.ts
git commit -m "feat(kanban): add UpdateKanbanCard backend method + binding"
```

---

## Task 2: KanbanCardDetail Dialog

**Files:**
- Create: `frontend/src/components/KanbanCardDetail.svelte`

- [ ] **Step 1: Create the card detail dialog component**

Create `frontend/src/components/KanbanCardDetail.svelte`:

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';
  import type { KanbanCard, ColumnID } from '../stores/kanban';
  import { COLUMN_LABELS } from '../stores/kanban';

  export let visible = false;
  export let card: KanbanCard | null = null;
  export let columnId: ColumnID = 'define';
  export let dir = '';

  const dispatch = createEventDispatcher<{
    close: void;
    updated: { card: KanbanCard };
    deleted: { cardId: string };
  }>();

  // Editable fields — initialized from card when dialog opens
  let title = '';
  let prompt = '';
  let priority = 0;
  let parentIssue = 0;
  let autoMerge = false;
  let autoStart = false;
  let maxRetries = 0;
  let saving = false;
  let confirmDelete = false;

  function initFields() {
    if (!card) return;
    title = card.title;
    prompt = card.prompt || '';
    priority = card.priority;
    parentIssue = card.parent_issue;
    autoMerge = card.auto_merge;
    autoStart = card.auto_start;
    maxRetries = card.max_retries;
    confirmDelete = false;
  }

  // CLAUDE.md rule: use function call in $: block, not inline assignments
  $: if (visible && card) initFields();

  async function handleSave() {
    if (!card || !dir) return;
    saving = true;
    const updated: KanbanCard = {
      ...card,
      title,
      prompt,
      priority,
      parent_issue: parentIssue,
      auto_merge: autoMerge,
      auto_start: autoStart,
      max_retries: maxRetries,
    };
    try {
      await App.UpdateKanbanCard(dir, updated);
      dispatch('updated', { card: updated });
      dispatch('close');
    } catch (err) {
      console.error('[card-detail] save error:', err);
    } finally {
      saving = false;
    }
  }

  async function handleDelete() {
    if (!card || !dir) return;
    if (!confirmDelete) { confirmDelete = true; return; }
    try {
      await App.RemoveKanbanCard(dir, card.id);
      dispatch('deleted', { cardId: card.id });
      dispatch('close');
    } catch (err) {
      console.error('[card-detail] delete error:', err);
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('close');
  }

  function handleBackdrop() {
    dispatch('close');
  }
</script>

{#if visible && card}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <div class="backdrop" on:click={handleBackdrop} on:keydown={handleKeydown}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation>
      <div class="dialog-header">
        <span class="col-badge" style="color: var(--fg-muted)">{COLUMN_LABELS[columnId] || columnId}</span>
        {#if card.issue_number}
          <span class="issue-num">#{card.issue_number}</span>
        {/if}
        <div class="spacer"></div>
        <button class="btn-close" on:click={() => dispatch('close')}>&#10005;</button>
      </div>

      <div class="dialog-body">
        <div class="field">
          <label class="field-label">Titel</label>
          <input class="field-input" bind:value={title} />
        </div>

        <div class="field">
          <label class="field-label">Prompt / Beschreibung</label>
          <textarea class="field-textarea" bind:value={prompt} rows="5" placeholder="Aufgabenbeschreibung für den Agenten..."></textarea>
        </div>

        <div class="field-row">
          <div class="field field-sm">
            <label class="field-label">Priorität</label>
            <input class="field-input" type="number" bind:value={priority} min="0" />
          </div>
          <div class="field field-sm">
            <label class="field-label">Parent Issue</label>
            <input class="field-input" type="number" bind:value={parentIssue} min="0" placeholder="0" />
          </div>
          <div class="field field-sm">
            <label class="field-label">Max Retries</label>
            <input class="field-input" type="number" bind:value={maxRetries} min="0" />
          </div>
        </div>

        <div class="toggle-row">
          <label class="toggle">
            <input type="checkbox" bind:checked={autoMerge} />
            <span>Auto-Merge</span>
          </label>
          <label class="toggle">
            <input type="checkbox" bind:checked={autoStart} />
            <span>Auto-Start</span>
          </label>
        </div>

        {#if card.labels && card.labels.length > 0}
          <div class="field">
            <label class="field-label">Labels</label>
            <div class="labels-row">
              {#each card.labels as label}
                <span class="label-tag">{label}</span>
              {/each}
            </div>
          </div>
        {/if}

        <!-- Read-only status section -->
        {#if card.worktree_branch || card.agent_session_id || card.review_result || card.pr_number}
          <div class="status-section">
            <div class="status-title">Status</div>
            {#if card.worktree_branch}
              <div class="status-row">
                <span class="status-key">Branch:</span>
                <code class="status-val">{card.worktree_branch}</code>
              </div>
            {/if}
            {#if card.worktree_path}
              <div class="status-row">
                <span class="status-key">Worktree:</span>
                <code class="status-val">{card.worktree_path}</code>
              </div>
            {/if}
            {#if card.agent_session_id > 0}
              <div class="status-row">
                <span class="status-key">Agent-Session:</span>
                <span class="status-val">#{card.agent_session_id}</span>
              </div>
            {/if}
            {#if card.review_result}
              <div class="status-row">
                <span class="status-key">Review:</span>
                <span class="status-val" class:pass={card.review_result === 'pass'} class:fail={card.review_result.includes('fail')}>{card.review_result}</span>
              </div>
            {/if}
            {#if card.pr_number > 0}
              <div class="status-row">
                <span class="status-key">Pull Request:</span>
                <span class="status-val">#{card.pr_number}</span>
              </div>
            {/if}
            {#if card.retry_count > 0}
              <div class="status-row">
                <span class="status-key">Retries:</span>
                <span class="status-val">{card.retry_count}/{card.max_retries}</span>
              </div>
            {/if}
          </div>
        {/if}
      </div>

      <div class="dialog-footer">
        <button class="btn-delete" on:click={handleDelete}>
          {confirmDelete ? 'Wirklich löschen?' : 'Löschen'}
        </button>
        <div class="spacer"></div>
        <button class="btn-cancel" on:click={() => dispatch('close')}>Abbrechen</button>
        <button class="btn-save" on:click={handleSave} disabled={saving}>
          {saving ? 'Speichern...' : 'Speichern'}
        </button>
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
    width: 560px;
    max-height: 85vh;
    display: flex; flex-direction: column;
    box-shadow: 0 20px 60px rgba(0,0,0,0.5);
  }
  .dialog-header {
    display: flex; align-items: center; gap: 8px;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border, #45475a);
  }
  .col-badge {
    font-size: 0.7rem; font-weight: 700;
    text-transform: uppercase; letter-spacing: 0.05em;
  }
  .issue-num { font-size: 0.8rem; color: var(--fg-muted, #a6adc8); }
  .spacer { flex: 1; }
  .btn-close {
    background: none; border: none; color: var(--fg-muted, #a6adc8);
    font-size: 1rem; cursor: pointer; padding: 4px;
  }
  .btn-close:hover { color: var(--fg, #cdd6f4); }

  .dialog-body {
    padding: 20px;
    overflow-y: auto;
    display: flex; flex-direction: column; gap: 14px;
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
    font-size: 0.85rem; outline: none;
  }
  .field-input:focus { border-color: var(--accent, #39ff14); }
  .field-textarea {
    padding: 8px 10px; border-radius: 6px;
    border: 1px solid var(--border, #45475a);
    background: var(--bg, #11111b);
    color: var(--fg, #cdd6f4);
    font-size: 0.85rem; outline: none;
    resize: vertical; font-family: monospace;
    line-height: 1.4;
  }
  .field-textarea:focus { border-color: var(--accent, #39ff14); }

  .field-row { display: flex; gap: 10px; }
  .field-sm { flex: 1; }
  .field-sm .field-input { width: 100%; }

  .toggle-row { display: flex; gap: 16px; }
  .toggle {
    display: flex; align-items: center; gap: 6px;
    font-size: 0.8rem; color: var(--fg, #cdd6f4);
    cursor: pointer;
  }
  .toggle input { accent-color: var(--accent, #39ff14); }

  .labels-row { display: flex; gap: 4px; flex-wrap: wrap; }
  .label-tag {
    font-size: 0.65rem; padding: 2px 6px; border-radius: 4px;
    background: rgba(137, 180, 250, 0.15); color: #89b4fa; font-weight: 600;
  }

  .status-section {
    border-top: 1px solid var(--border, #45475a);
    padding-top: 12px;
    display: flex; flex-direction: column; gap: 6px;
  }
  .status-title {
    font-size: 0.7rem; font-weight: 700;
    color: var(--fg-muted, #a6adc8);
    text-transform: uppercase; letter-spacing: 0.04em;
  }
  .status-row {
    display: flex; align-items: center; gap: 8px;
    font-size: 0.78rem;
  }
  .status-key { color: var(--fg-muted, #a6adc8); min-width: 90px; }
  .status-val { color: var(--fg, #cdd6f4); }
  .status-val.pass { color: #22c55e; font-weight: 600; }
  .status-val.fail { color: #ef4444; font-weight: 600; }
  code.status-val {
    font-size: 0.72rem; background: var(--bg, #11111b);
    padding: 2px 6px; border-radius: 4px;
  }

  .dialog-footer {
    display: flex; align-items: center; gap: 8px;
    padding: 14px 20px;
    border-top: 1px solid var(--border, #45475a);
  }
  .btn-delete {
    padding: 6px 12px; border-radius: 6px;
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #f87171; cursor: pointer; font-size: 0.78rem;
  }
  .btn-delete:hover { background: rgba(239, 68, 68, 0.2); }
  .btn-cancel {
    padding: 6px 14px; border-radius: 6px;
    background: transparent; border: 1px solid var(--border, #45475a);
    color: var(--fg-muted, #a6adc8); cursor: pointer; font-size: 0.78rem;
  }
  .btn-save {
    padding: 6px 14px; border-radius: 6px;
    background: var(--accent, #39ff14); border: none;
    color: #000; font-weight: 600; cursor: pointer; font-size: 0.78rem;
  }
  .btn-save:hover { opacity: 0.85; }
  .btn-save:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/KanbanCardDetail.svelte
git commit -m "feat(kanban): add card detail dialog component"
```

---

## Task 3: KanbanPlanDialog

**Files:**
- Create: `frontend/src/components/KanbanPlanDialog.svelte`

- [ ] **Step 1: Create the plan review/approve dialog**

Create `frontend/src/components/KanbanPlanDialog.svelte`:

```svelte
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

  function moveStep(idx: number, dir: -1 | 1) {
    const target = idx + dir;
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
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/KanbanPlanDialog.svelte
git commit -m "feat(kanban): add plan review/approve dialog component"
```

---

## Task 4: Wire KanbanCard selection mode

**Files:**
- Modify: `frontend/src/components/KanbanCard.svelte`

- [ ] **Step 1: Add optional checkbox to KanbanCard**

Add a `selectable` prop and checkbox to the existing card component.

In `<script>` section, after `export let columnId`:
```typescript
  export let selectable = false;
  export let selected = false;
```

Add a new event:
```typescript
  const dispatch = createEventDispatcher<{
    dragstart: { card: KanbanCard; columnId: string };
    click: { card: KanbanCard };
    select: { card: KanbanCard; selected: boolean };
  }>();
```

Add handler:
```typescript
  function handleSelect(e: Event) {
    e.stopPropagation();
    selected = !selected;
    dispatch('select', { card, selected });
  }
```

In the template, add a checkbox before `card-top-row`:
```svelte
  {#if selectable}
    <label class="card-checkbox" on:click|stopPropagation>
      <input type="checkbox" checked={selected} on:change={handleSelect} />
    </label>
  {/if}
```

Add CSS:
```css
  .card-checkbox {
    position: absolute; top: 8px; right: 8px;
    cursor: pointer;
  }
  .card-checkbox input { accent-color: var(--accent, #39ff14); }
  .kanban-card { position: relative; }
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/KanbanCard.svelte
git commit -m "feat(kanban): add selection checkbox to card component"
```

---

## Task 5: Wire everything into KanbanBoard

**Files:**
- Modify: `frontend/src/components/KanbanBoard.svelte`

- [ ] **Step 1: Add imports and state variables**

At the top of `<script>`, add imports:
```typescript
  import KanbanCardDetail from './KanbanCardDetail.svelte';
  import KanbanPlanDialog from './KanbanPlanDialog.svelte';
```

Add state variables after `orchLoading`:
```typescript
  // Card detail dialog
  let detailCard: KanbanCard | null = null;
  let detailColumnId: ColumnID = 'define';
  let showDetail = false;

  // Plan dialog
  let selectedPlan: Plan | null = null;
  let showPlanDialog = false;

  // Selection mode for plan creation
  let selectMode = false;
  let selectedCardIds: Set<string> = new Set();
```

- [ ] **Step 2: Replace handleCardClick**

Replace the existing `handleCardClick` function:
```typescript
  function handleCardClick(e: CustomEvent<{ card: KanbanCard }>) {
    if (selectMode) return; // handled by select event in selection mode
    const card = e.detail.card;
    // Find which column the card is in
    for (const col of COLUMN_IDS) {
      if (($kanban.state.columns[col] || []).find(c => c.id === card.id)) {
        detailColumnId = col;
        break;
      }
    }
    detailCard = card;
    showDetail = true;
  }
```

- [ ] **Step 3: Add plan and selection handlers**

Add after `handleCardClick`:
```typescript
  function handleCardSelect(e: CustomEvent<{ card: KanbanCard; selected: boolean }>) {
    if (e.detail.selected) {
      selectedCardIds.add(e.detail.card.id);
    } else {
      selectedCardIds.delete(e.detail.card.id);
    }
    selectedCardIds = new Set(selectedCardIds); // trigger reactivity
  }

  function toggleSelectMode() {
    selectMode = !selectMode;
    if (!selectMode) selectedCardIds = new Set();
  }

  async function handleGeneratePlan() {
    if (selectedCardIds.size === 0 || !dir) return;
    try {
      const plan = await App.GeneratePlan(dir, Array.from(selectedCardIds));
      selectedPlan = plan;
      showPlanDialog = true;
      selectMode = false;
      selectedCardIds = new Set();
    } catch (err) {
      console.error('[kanban] generate plan error:', err);
    }
  }

  function handlePlanBadgeClick(plan: Plan) {
    selectedPlan = plan;
    showPlanDialog = true;
  }

  function handleDetailUpdated() {
    loadBoard();
  }

  function handleDetailDeleted(e: CustomEvent<{ cardId: string }>) {
    kanban.removeCard(e.detail.cardId);
  }

  function handlePlanApprovedOrCancelled() {
    loadBoard();
  }
```

- [ ] **Step 4: Update toolbar template**

Add "Plan erstellen" button and selection controls. Replace the `toolbar-actions` div:
```svelte
    <div class="toolbar-actions">
      <button class="btn-toolbar" on:click={() => { showAddCard = !showAddCard; }} title="Karte hinzufügen">
        + Karte
      </button>
      {#if selectMode}
        <button class="btn-toolbar btn-plan-gen" on:click={handleGeneratePlan} disabled={selectedCardIds.size === 0}>
          Plan erstellen ({selectedCardIds.size})
        </button>
        <button class="btn-toolbar" on:click={toggleSelectMode}>Abbrechen</button>
      {:else}
        <button class="btn-toolbar" on:click={toggleSelectMode} title="Karten für Plan auswählen">
          Plan erstellen
        </button>
      {/if}
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
      <button class="btn-tab" class:active={$kanban.activeTab === 'board'} on:click={() => kanban.setActiveTab('board')}>Board</button>
      <button class="btn-tab" class:active={$kanban.activeTab === 'schedules'} on:click={() => kanban.setActiveTab('schedules')}>Zeitpläne</button>
    </div>
```

- [ ] **Step 5: Make plan badges clickable**

Replace the `plans-bar` section:
```svelte
  {#if $activePlans.length > 0}
    <div class="plans-bar">
      {#each $activePlans as plan (plan.id)}
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="plan-badge clickable" class:running={plan.status === 'running'} class:draft={plan.status === 'draft'} on:click={() => handlePlanBadgeClick(plan)}>
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
```

- [ ] **Step 6: Pass selectMode + events to KanbanColumn/KanbanCard**

In the `columns-container`, update the KanbanColumn usage to pass selection state. Since KanbanColumn dispatches cardClick events, and KanbanCard needs `selectable` and `selected` props, update the KanbanColumn loop:

Note: KanbanColumn dispatches `cardClick` which already works. For selection, we also need a `cardSelect` event. Update the KanbanColumn binding:
```svelte
        <KanbanColumn
          columnId={colId}
          cards={$kanban.state.columns[colId] || []}
          {selectMode}
          {selectedCardIds}
          on:drop={handleDrop}
          on:cardClick={handleCardClick}
          on:cardDragStart={handleCardDragStart}
          on:cardSelect={handleCardSelect}
        />
```

- [ ] **Step 7: Add dialog instances at end of template**

Before the closing `</div>` of `.kanban-board`:
```svelte
  <KanbanCardDetail
    visible={showDetail}
    card={detailCard}
    columnId={detailColumnId}
    {dir}
    on:close={() => { showDetail = false; detailCard = null; }}
    on:updated={handleDetailUpdated}
    on:deleted={handleDetailDeleted}
  />

  <KanbanPlanDialog
    visible={showPlanDialog}
    plan={selectedPlan}
    {dir}
    on:close={() => { showPlanDialog = false; selectedPlan = null; }}
    on:approved={handlePlanApprovedOrCancelled}
    on:cancelled={handlePlanApprovedOrCancelled}
  />
```

- [ ] **Step 8: Add CSS for plan-gen button and clickable badges**

Add to `<style>`:
```css
  .btn-plan-gen {
    border-color: rgba(137, 180, 250, 0.4);
    color: #89b4fa;
  }
  .btn-plan-gen:hover { border-color: #89b4fa; background: rgba(137, 180, 250, 0.08); }
  .btn-plan-gen:disabled { opacity: 0.5; cursor: not-allowed; }
  .plan-badge.clickable { cursor: pointer; }
  .plan-badge.clickable:hover { border-color: currentColor; }
```

- [ ] **Step 9: Commit**

```bash
git add frontend/src/components/KanbanBoard.svelte
git commit -m "feat(kanban): wire card detail, plan dialog, and selection mode into board"
```

---

## Task 6: Update KanbanColumn and KanbanCard for selection

**Files:**
- Modify: `frontend/src/components/KanbanColumn.svelte`

- [ ] **Step 1: Add selection props and events to KanbanColumn**

Add props:
```typescript
  export let selectMode = false;
  export let selectedCardIds: Set<string> = new Set();
```

Add event type:
```typescript
    cardSelect: { card: KanbanCardType; selected: boolean };
```

Update the KanbanCard inside the template:
```svelte
      <KanbanCard
        {card}
        {columnId}
        selectable={selectMode && columnId === 'define'}
        selected={selectedCardIds.has(card.id)}
        on:click={handleCardClick}
        on:dragstart={handleCardDragStart}
        on:select={handleCardSelect}
      />
```

Add handler:
```typescript
  function handleCardSelect(e: CustomEvent<{ card: KanbanCardType; selected: boolean }>) {
    dispatch('cardSelect', e.detail);
  }
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/KanbanColumn.svelte frontend/src/components/KanbanCard.svelte
git commit -m "feat(kanban): pass selection state through column to card"
```

---

## Task 7: Build & verify

- [ ] **Step 1: Build frontend**

Run: `cd D:/repos/Multiterminal/frontend && npm run build`
Expected: Build succeeds, no "not exported" errors

- [ ] **Step 2: Build Go backend**

Run: `cd D:/repos/Multiterminal && go build -o build/bin/multiterminal.exe -tags desktop .`
Expected: Build succeeds

- [ ] **Step 3: Final commit**

```bash
git add -A
git commit -m "feat(kanban): complete card detail + plan workflow UI"
```
