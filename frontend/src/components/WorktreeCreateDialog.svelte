<!-- frontend/src/components/WorktreeCreateDialog.svelte -->
<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';

  export let visible: boolean = false;
  export let dir: string = '';

  const dispatch = createEventDispatcher();

  let name = '';
  let baseBranch = '';
  let branches: string[] = [];
  let creating = false;
  let error = '';
  let inputEl: HTMLInputElement;

  $: safeName = name
    .toLowerCase()
    .replace(/[\s/\]+/g, '-')
    .replace(/[^a-z0-9\-_]/g, '')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '');
  $: branchPreview = safeName ? `terminal/${safeName}` : '';

  $: if (visible) {
    name = '';
    baseBranch = '';
    error = '';
    creating = false;
    loadBranches();
    requestAnimationFrame(() => inputEl?.focus());
  }

  async function loadBranches() {
    if (!dir) return;
    try {
      branches = await App.GetLocalBranches(dir);
      if (branches.length > 0) baseBranch = branches[0];
    } catch {}
  }

  async function create() {
    if (!safeName) { error = 'Name erforderlich'; return; }
    creating = true;
    error = '';
    try {
      const wt = await App.CreateNamedWorktree(dir, name, baseBranch);
      dispatch('created', wt);
      dispatch('close');
    } catch (err: any) {
      error = err?.message || String(err);
    } finally {
      creating = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('close');
    if (e.key === 'Enter' && !e.shiftKey) create();
    e.stopPropagation();
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="overlay" on:click={() => dispatch('close')}>
    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation on:keydown={handleKeydown}>
      <div class="dialog-header">
        <span class="dialog-icon">⎇</span>
        <h3>Neuer Terminal Worktree</h3>
      </div>
      <p class="dialog-desc">Erstellt ein isoliertes Arbeitsverzeichnis für diesen Branch.</p>

      <label class="field-label">Worktree-Name</label>
      <input
        class="field-input"
        type="text"
        placeholder="z.B. my-feature"
        bind:value={name}
        bind:this={inputEl}
      />
      {#if branchPreview}
        <div class="branch-preview">Branch: <code>{branchPreview}</code></div>
      {/if}

      <label class="field-label">Base Branch</label>
      {#if branches.length > 0}
        <select class="field-input" bind:value={baseBranch}>
          {#each branches as b}
            <option value={b}>{b}</option>
          {/each}
        </select>
      {:else}
        <input class="field-input" type="text" placeholder="main" bind:value={baseBranch} />
      {/if}
      <div class="field-hint">Branch, von dem der neue Worktree abzweigt</div>

      {#if error}
        <div class="error">{error}</div>
      {/if}

      <div class="dialog-footer">
        <button class="btn-cancel" on:click={() => dispatch('close')}>Abbrechen</button>
        <button class="btn-create" on:click={create} disabled={!safeName || creating}>
          {creating ? 'Erstelle...' : 'Erstellen'}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed; inset: 0;
    background: rgba(0,0,0,0.5);
    display: flex; align-items: center; justify-content: center;
    z-index: 300;
  }
  .dialog {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 20px;
    width: 380px;
    box-shadow: 0 8px 32px rgba(0,0,0,0.4);
  }
  .dialog-header {
    display: flex; align-items: center; gap: 8px; margin-bottom: 6px;
  }
  .dialog-icon { font-size: 18px; }
  h3 { margin: 0; font-size: 15px; color: var(--fg); }
  .dialog-desc { font-size: 12px; color: var(--fg-muted); margin: 0 0 16px; }

  .field-label {
    display: block; font-size: 12px; font-weight: 600;
    color: var(--fg); margin-bottom: 4px;
  }
  .field-input {
    width: 100%; padding: 8px 10px; box-sizing: border-box;
    background: var(--bg-secondary); border: 1px solid var(--border);
    border-radius: 6px; color: var(--fg); font-size: 13px; margin-bottom: 4px;
  }
  .field-input:focus { outline: none; border-color: var(--accent); }
  .branch-preview {
    font-size: 11px; color: var(--fg-muted); margin-bottom: 12px;
  }
  .branch-preview code {
    color: var(--accent); background: var(--bg-tertiary);
    padding: 1px 5px; border-radius: 3px;
  }
  .field-hint { font-size: 11px; color: var(--fg-muted); margin-bottom: 12px; }

  .error {
    background: rgba(239,68,68,0.1); border: 1px solid rgba(239,68,68,0.4);
    border-radius: 6px; padding: 8px 10px; font-size: 12px;
    color: #f87171; margin-bottom: 12px;
  }
  .dialog-footer { display: flex; justify-content: flex-end; gap: 8px; }
  .btn-cancel {
    padding: 7px 14px; background: var(--bg-tertiary); border: 1px solid var(--border);
    border-radius: 6px; color: var(--fg-muted); cursor: pointer; font-size: 12px;
  }
  .btn-create {
    padding: 7px 16px; background: var(--accent); border: none;
    border-radius: 6px; color: var(--bg); cursor: pointer; font-size: 12px; font-weight: 600;
  }
  .btn-create:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn-cancel:hover { color: var(--fg); }
</style>
