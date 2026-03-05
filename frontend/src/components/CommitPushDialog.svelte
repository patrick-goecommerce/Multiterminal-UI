<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { App } from '../../wailsjs/go/backend/AppService';

  export let visible: boolean = false;
  export let dir: string = '';
  export let files: string[] = [];
  export let issueNumber: number = 0;
  export let branch: string = '';

  const dispatch = createEventDispatcher();

  const COMMIT_TYPES = [
    { value: 'feat',     label: 'feat',     desc: 'Neues Feature' },
    { value: 'fix',      label: 'fix',      desc: 'Bugfix' },
    { value: 'refactor', label: 'refactor', desc: 'Code-Umbau' },
    { value: 'chore',    label: 'chore',    desc: 'Wartung / Build' },
    { value: 'docs',     label: 'docs',     desc: 'Dokumentation' },
    { value: 'test',     label: 'test',     desc: 'Tests' },
    { value: 'style',    label: 'style',    desc: 'Formatierung' },
    { value: 'perf',     label: 'perf',     desc: 'Performance' },
    { value: 'ci',       label: 'ci',       desc: 'CI/CD' },
    { value: 'build',    label: 'build',    desc: 'Build-System' },
  ];

  let commitType: string = 'feat';
  let scope: string = '';
  let description: string = '';
  let body: string = '';
  let doPush: boolean = true;
  let createPRAfter: boolean = false;
  let committing: boolean = false;
  let error: string = '';

  $: if (visible) initDialog();

  async function initDialog() {
    error = '';
    committing = false;
    doPush = true;
    createPRAfter = issueNumber > 0;
    body = '';
    try {
      const suggestion = await App.GenerateCommitSuggestion(dir, files);
      commitType = suggestion.type || 'feat';
      scope = suggestion.scope || '';
      description = suggestion.description || '';
    } catch {
      commitType = 'feat';
      scope = '';
      description = '';
    }
  }

  function buildMessage(): string {
    let msg = commitType;
    if (scope.trim()) msg += `(${scope.trim()})`;
    msg += ': ' + description.trim();
    if (body.trim()) msg += '\n\n' + body.trim();
    if (issueNumber > 0) msg += `\n\nCloses #${issueNumber}`;
    return msg;
  }

  $: previewLine = (() => {
    let msg = commitType;
    if (scope.trim()) msg += `(${scope.trim()})`;
    msg += ': ' + (description.trim() || '...');
    return msg;
  })();

  $: canCommit = description.trim().length > 0 && !committing;

  async function doCommit() {
    if (!canCommit) return;
    error = '';
    committing = true;
    try {
      await App.StageFiles(dir, files);
      await App.CommitStaged(dir, buildMessage());
      if (doPush) {
        await App.PushBranch(dir);
      }
      if (createPRAfter && doPush && issueNumber > 0) {
        dispatch('createPR', { issueNumber, branch });
      }
      dispatch('committed', { pushed: doPush });
    } catch (e: any) {
      error = e?.message || String(e);
      committing = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      dispatch('close');
    } else if (e.key === 'Enter' && e.ctrlKey) {
      doCommit();
    }
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <div class="backdrop" on:click={() => dispatch('close')} on:keydown={handleKeydown}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <div class="dialog" on:click|stopPropagation on:keydown={handleKeydown} tabindex="-1">
      <h2>{doPush ? 'Commit & Push' : 'Commit'}</h2>

      <div class="summary">
        {files.length} Dateien ausgewählt &middot; <code>{branch}</code>
      </div>

      <div class="section-label">Typ</div>
      <div class="type-grid">
        {#each COMMIT_TYPES as ct}
          <button
            class="type-btn"
            class:active={commitType === ct.value}
            title={ct.desc}
            on:click={() => (commitType = ct.value)}
          >
            {ct.label}
          </button>
        {/each}
      </div>

      <div class="section-label">Scope (optional)</div>
      <input
        class="input"
        type="text"
        bind:value={scope}
        placeholder="z.B. ui, backend, config"
      />

      <div class="section-label">Beschreibung</div>
      <input
        class="input"
        type="text"
        bind:value={description}
        placeholder="Kurze Zusammenfassung der Änderung"
      />

      <div class="section-label">Details (optional)</div>
      <textarea
        class="input textarea"
        rows="2"
        bind:value={body}
        placeholder="Ausführlichere Erklärung..."
      ></textarea>

      <div class="section-label">Vorschau</div>
      <div class="preview">
        <span class="preview-msg">{previewLine}</span>
      </div>

      <div class="options">
        <label class="checkbox-label">
          <input type="checkbox" bind:checked={doPush} />
          Push nach Commit
        </label>
        {#if issueNumber > 0}
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={createPRAfter} disabled={!doPush} />
            Pull Request erstellen (#{issueNumber})
          </label>
        {/if}
      </div>

      {#if error}
        <div class="error-box">{error}</div>
      {/if}

      <div class="buttons">
        <button class="btn btn-cancel" on:click={() => dispatch('close')}>Abbrechen</button>
        <button class="btn btn-primary" disabled={!canCommit} on:click={doCommit}>
          {#if committing}
            Wird committed...
          {:else}
            {doPush ? 'Commit & Push' : 'Commit'}
          {/if}
        </button>
      </div>

      <div class="hint">Ctrl+Enter zum Bestätigen</div>
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 100;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .dialog {
    background: var(--bg-primary);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    padding: 20px;
    width: 440px;
    max-height: 90vh;
    overflow-y: auto;
    outline: none;
  }

  h2 {
    margin: 0 0 8px 0;
    font-size: 18px;
    color: var(--text-primary);
  }

  .summary {
    color: var(--text-secondary);
    font-size: 13px;
    margin-bottom: 16px;
  }

  .summary code {
    background: var(--bg-secondary);
    padding: 1px 5px;
    border-radius: 4px;
    font-size: 12px;
  }

  .section-label {
    color: var(--text-secondary);
    font-size: 12px;
    margin-bottom: 4px;
    margin-top: 10px;
  }

  .type-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .type-btn {
    padding: 3px 10px;
    border: 1px solid var(--border-color);
    border-radius: 6px;
    background: transparent;
    color: var(--text-primary);
    font-size: 13px;
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
  }

  .type-btn:hover {
    background: var(--bg-hover);
  }

  .type-btn.active {
    background: #7c3aed;
    color: #fff;
    border-color: #7c3aed;
  }

  .input {
    width: 100%;
    padding: 7px 10px;
    border: 1px solid var(--border-color);
    border-radius: 6px;
    background: var(--bg-secondary);
    color: var(--text-primary);
    font-size: 13px;
    outline: none;
    box-sizing: border-box;
  }

  .input:focus {
    border-color: #7c3aed;
  }

  .textarea {
    resize: vertical;
    font-family: inherit;
  }

  .preview {
    background: var(--bg-secondary);
    border-radius: 6px;
    padding: 8px 10px;
    margin-top: 2px;
  }

  .preview-msg {
    color: #50fa7b;
    font-family: monospace;
    font-size: 13px;
    word-break: break-all;
  }

  .options {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-top: 12px;
  }

  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--text-primary);
    font-size: 13px;
    cursor: pointer;
  }

  .checkbox-label input[type='checkbox'] {
    accent-color: #7c3aed;
  }

  .error-box {
    background: rgba(255, 50, 50, 0.15);
    border: 1px solid rgba(255, 50, 50, 0.4);
    color: #ff6b6b;
    border-radius: 6px;
    padding: 8px 10px;
    font-size: 13px;
    margin-top: 10px;
  }

  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 16px;
  }

  .btn {
    padding: 7px 16px;
    border-radius: 6px;
    border: none;
    font-size: 13px;
    cursor: pointer;
    transition: background 0.15s;
  }

  .btn-cancel {
    background: var(--bg-secondary);
    color: var(--text-primary);
    border: 1px solid var(--border-color);
  }

  .btn-cancel:hover {
    background: var(--bg-hover);
  }

  .btn-primary {
    background: #7c3aed;
    color: #fff;
  }

  .btn-primary:hover:not(:disabled) {
    background: #6d28d9;
  }

  .btn-primary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .hint {
    text-align: center;
    color: var(--text-secondary);
    font-size: 11px;
    margin-top: 10px;
  }
</style>
