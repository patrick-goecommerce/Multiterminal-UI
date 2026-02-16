<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let visible: boolean = false;
  export let currentBranch: string = '';
  export let currentIssueNumber: number = 0;
  export let targetIssueNumber: number = 0;
  export let targetIssueTitle: string = '';
  export let dirtyWorkingTree: boolean = false;

  const dispatch = createEventDispatcher();

  function choose(action: 'switch' | 'stay' | 'worktree') {
    dispatch('choose', { action });
  }

  function close() {
    dispatch('close');
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
    if (e.key === '1' && !dirtyWorkingTree) choose('switch');
    if (e.key === '2') choose('stay');
    if (e.key === '3') choose('worktree');
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
      <h3>Branch-Konflikt</h3>

      <div class="conflict-info">
        <div class="conflict-row">
          <span class="conflict-label">Aktuell:</span>
          <span class="conflict-branch">{currentBranch}</span>
          {#if currentIssueNumber}
            <span class="conflict-issue">#{currentIssueNumber}</span>
          {/if}
        </div>
        <div class="conflict-arrow">&#8595;</div>
        <div class="conflict-row">
          <span class="conflict-label">Ziel:</span>
          <span class="conflict-branch">issue/{targetIssueNumber}-...</span>
          <span class="conflict-issue">#{targetIssueNumber}</span>
        </div>
        {#if targetIssueTitle}
          <div class="conflict-title">{targetIssueTitle}</div>
        {/if}
      </div>

      {#if dirtyWorkingTree}
        <div class="dirty-warning">
          <span class="warning-icon">&#9888;</span>
          <span>Uncommitted Changes vorhanden — Branch-Wechsel nicht moeglich.</span>
        </div>
      {/if}

      <div class="options">
        <button class="option" on:click={() => choose('switch')} disabled={dirtyWorkingTree}>
          <span class="option-key">1</span>
          <span class="option-icon">&#8634;</span>
          <div class="option-text">
            <strong>Branch wechseln</strong>
            <span>Zum Issue-Branch wechseln{dirtyWorkingTree ? ' (dirty tree)' : ''}</span>
          </div>
        </button>

        <button class="option" on:click={() => choose('stay')}>
          <span class="option-key">2</span>
          <span class="option-icon">&#9654;</span>
          <div class="option-text">
            <strong>Im Branch bleiben</strong>
            <span>Session ohne Branch-Wechsel starten</span>
          </div>
        </button>

        <button class="option" on:click={() => choose('worktree')}>
          <span class="option-key">3</span>
          <span class="option-icon">&#128194;</span>
          <div class="option-text">
            <strong>Worktree erstellen</strong>
            <span>Isoliertes Verzeichnis fuer Issue #{targetIssueNumber}</span>
          </div>
        </button>
      </div>

      <div class="dialog-footer">
        <button class="cancel-btn" on:click={close}>Abbrechen (Esc)</button>
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
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 20px;
    min-width: 400px;
    max-width: 480px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
  }

  h3 {
    margin: 0 0 16px;
    color: var(--fg);
    font-size: 16px;
  }

  .conflict-info {
    padding: 12px;
    margin-bottom: 12px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    font-size: 12px;
  }

  .conflict-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .conflict-label {
    color: var(--fg-muted);
    min-width: 50px;
  }

  .conflict-branch {
    font-family: monospace;
    color: var(--accent);
    font-weight: 600;
  }

  .conflict-issue {
    color: var(--fg-muted);
    font-size: 11px;
  }

  .conflict-arrow {
    text-align: center;
    color: var(--fg-muted);
    font-size: 14px;
    margin: 4px 0;
  }

  .conflict-title {
    margin-top: 8px;
    padding-top: 8px;
    border-top: 1px solid var(--border);
    color: var(--fg);
    font-weight: 500;
  }

  .dirty-warning {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    margin-bottom: 12px;
    background: rgba(243, 139, 168, 0.1);
    border: 1px solid rgba(243, 139, 168, 0.4);
    border-radius: 8px;
    font-size: 12px;
    color: #f38ba8;
  }

  .warning-icon {
    font-size: 16px;
  }

  .options {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 16px;
  }

  .option {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--fg);
    cursor: pointer;
    text-align: left;
    transition: all 0.15s;
  }

  .option:hover:not(:disabled) {
    border-color: var(--accent);
    background: var(--bg-tertiary);
  }

  .option:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .option-key {
    font-size: 11px;
    padding: 2px 6px;
    background: var(--bg-tertiary);
    border-radius: 4px;
    color: var(--fg-muted);
    font-family: monospace;
  }

  .option-icon {
    font-size: 20px;
  }

  .option-text {
    display: flex;
    flex-direction: column;
  }

  .option-text strong {
    font-size: 14px;
  }

  .option-text span {
    font-size: 11px;
    color: var(--fg-muted);
  }

  .dialog-footer {
    display: flex;
    justify-content: flex-end;
  }

  .cancel-btn {
    padding: 6px 14px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--fg-muted);
    cursor: pointer;
    font-size: 12px;
  }

  .cancel-btn:hover {
    color: var(--fg);
  }
</style>
