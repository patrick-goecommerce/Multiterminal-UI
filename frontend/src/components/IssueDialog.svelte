<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';

  export let visible: boolean = false;
  export let dir: string = '';
  export let editIssue: { number: number; title: string; body: string; labels: string[]; state: string } | null = null;

  const dispatch = createEventDispatcher();

  let title = '';
  let body = '';
  let selectedLabels: string[] = [];
  let availableLabels: { name: string; color: string }[] = [];
  let newState = '';
  let submitting = false;
  let error = '';

  $: isEdit = editIssue !== null;
  $: dialogTitle = isEdit ? `Issue #${editIssue?.number} bearbeiten` : 'Neues Issue';

  $: if (visible) {
    error = '';
    if (editIssue) {
      title = editIssue.title;
      body = editIssue.body;
      selectedLabels = [...editIssue.labels];
      newState = editIssue.state === 'OPEN' ? 'open' : 'closed';
    } else {
      title = '';
      body = '';
      selectedLabels = [];
      newState = '';
    }
    loadLabels();
  }

  async function loadLabels() {
    if (!dir) return;
    try {
      availableLabels = (await App.GetIssueLabels(dir)) || [];
    } catch {
      availableLabels = [];
    }
  }

  function toggleLabel(name: string) {
    if (selectedLabels.includes(name)) {
      selectedLabels = selectedLabels.filter(l => l !== name);
    } else {
      selectedLabels = [...selectedLabels, name];
    }
  }

  async function submit() {
    if (!title.trim()) {
      error = 'Titel ist erforderlich';
      return;
    }
    submitting = true;
    error = '';

    try {
      if (isEdit && editIssue) {
        await App.UpdateIssue(dir, editIssue.number, title.trim(), body.trim(), newState);
      } else {
        await App.CreateIssue(dir, title.trim(), body.trim(), selectedLabels);
      }
      dispatch('saved');
      close();
    } catch (e: any) {
      error = e?.message || 'Fehler beim Speichern';
    }
    submitting = false;
  }

  function close() {
    dispatch('close');
    title = '';
    body = '';
    selectedLabels = [];
    error = '';
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
    if (e.key === 'Enter' && e.ctrlKey) submit();
  }
</script>

<svelte:window on:keydown={visible ? handleKeydown : undefined} />

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="overlay" on:click={close}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation>
      <h3>{dialogTitle}</h3>

      {#if error}
        <div class="error-msg">{error}</div>
      {/if}

      <div class="field">
        <label for="issue-title">Titel</label>
        <input id="issue-title" type="text" bind:value={title} placeholder="Issue-Titel..." />
      </div>

      <div class="field">
        <label for="issue-body">Beschreibung</label>
        <textarea id="issue-body" bind:value={body} placeholder="Beschreibung (Markdown)..." rows="6"></textarea>
      </div>

      {#if availableLabels.length > 0}
        <div class="field">
          <label>Labels</label>
          <div class="label-grid">
            {#each availableLabels as label}
              <button
                class="label-chip"
                class:selected={selectedLabels.includes(label.name)}
                style="--label-color: #{label.color}"
                on:click={() => toggleLabel(label.name)}
              >
                {label.name}
              </button>
            {/each}
          </div>
        </div>
      {/if}

      {#if isEdit}
        <div class="field">
          <label for="issue-state">Status</label>
          <select id="issue-state" bind:value={newState}>
            <option value="open">Open</option>
            <option value="closed">Closed</option>
          </select>
        </div>
      {/if}

      <div class="dialog-footer">
        <span class="hint">Ctrl+Enter zum Speichern</span>
        <button class="cancel-btn" on:click={close}>Abbrechen</button>
        <button class="submit-btn" on:click={submit} disabled={submitting}>
          {submitting ? 'Speichere...' : isEdit ? 'Speichern' : 'Erstellen'}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed; inset: 0; background: rgba(0, 0, 0, 0.5);
    display: flex; align-items: center; justify-content: center; z-index: 100;
  }
  .dialog {
    background: var(--bg); border: 1px solid var(--border); border-radius: 12px;
    padding: 20px; min-width: 420px; max-width: 520px; width: 90%;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
  }
  h3 { margin: 0 0 16px; color: var(--fg); font-size: 16px; }

  .error-msg {
    padding: 8px 12px; background: rgba(248, 113, 113, 0.15); color: var(--error);
    border-radius: 6px; font-size: 12px; margin-bottom: 12px;
  }

  .field { margin-bottom: 14px; }
  .field label {
    display: block; font-size: 12px; font-weight: 600; color: var(--fg-muted); margin-bottom: 4px;
  }
  .field input, .field textarea, .field select {
    width: 100%; padding: 8px 10px; background: var(--bg-secondary); border: 1px solid var(--border);
    border-radius: 6px; color: var(--fg); font-size: 13px; box-sizing: border-box; font-family: inherit;
  }
  .field input:focus, .field textarea:focus, .field select:focus {
    outline: none; border-color: var(--accent);
  }
  .field textarea { resize: vertical; min-height: 80px; }
  .field input::placeholder, .field textarea::placeholder { color: var(--fg-muted); }

  .label-grid { display: flex; flex-wrap: wrap; gap: 4px; }
  .label-chip {
    font-size: 11px; padding: 3px 10px; border-radius: 12px; cursor: pointer;
    border: 1px solid var(--border); background: var(--bg-tertiary); color: var(--fg);
    transition: all 0.15s; font-weight: 500;
  }
  .label-chip:hover { border-color: var(--label-color, var(--accent)); }
  .label-chip.selected {
    background: var(--label-color, var(--accent)); color: #fff; border-color: transparent;
  }

  .dialog-footer {
    display: flex; align-items: center; gap: 8px; padding-top: 4px;
  }
  .hint { font-size: 10px; color: var(--fg-muted); flex: 1; }
  .cancel-btn {
    padding: 6px 14px; background: var(--bg-tertiary); border: 1px solid var(--border);
    border-radius: 6px; color: var(--fg-muted); cursor: pointer; font-size: 12px;
  }
  .cancel-btn:hover { color: var(--fg); }
  .submit-btn {
    padding: 6px 16px; background: var(--accent); color: #fff; border: none;
    border-radius: 6px; font-size: 12px; font-weight: 600; cursor: pointer; transition: opacity 0.15s;
  }
  .submit-btn:hover { opacity: 0.85; }
  .submit-btn:disabled { opacity: 0.4; cursor: default; }
</style>
