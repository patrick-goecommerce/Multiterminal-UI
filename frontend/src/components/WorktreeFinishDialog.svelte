<!-- frontend/src/components/WorktreeFinishDialog.svelte -->
<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let visible = false;
  export let state: 'ready' | 'blocked' | 'staging' = 'ready';
  export let sessionId = 0; // spread from finishDialog; not used directly here
  export let worktreePath = ''; // spread from finishDialog; consumed by App.svelte handlers
  export let targetBranch = '';
  export let commits: string[] = [];
  export let stat = '';
  export let untracked: string[] = [];
  export let cleanupOnly = false;
  export let reason = '';
  // Staging state (shell panes): changed files + commit message.
  export let files: { path: string; status: string; selected: boolean }[] = [];
  export let commitMessage = '';
  // Distinguishes a rebase-conflict block (abort / resolve-in-terminal) from a
  // regular prep block (cancel / retry).
  export let rebaseConflict = false;
  // A post-merge cleanup failure: the merge is already through, only the
  // worktree removal failed. The block then offers "Cleanup erneut versuchen"
  // (→ retryCleanup / FinishWorktree resume), not "Erneut vorbereiten".
  export let cleanupFailed = false;

  const dispatch = createEventDispatcher();

  // Reference spread-only props so the linter does not flag them as unused.
  $: void sessionId;
  $: void worktreePath;

  // Current selection, computed on demand (no reactive assignment — avoids the
  // known SettingsDialog `$:`-reset trap).
  const selected = () => files.filter((f) => f.selected).map((f) => f.path);

  function toggleFile() {
    // Reassign to retrigger the each-block / button state after bind:checked
    // mutates a nested property in place.
    files = files;
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="overlay" on:click={() => dispatch('close')}>
    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation>
      <div class="dialog-header">
        <span class="dialog-icon">⎇</span>
        <h3>
          {state === 'blocked'
            ? 'Fertigstellen blockiert'
            : state === 'staging'
            ? 'Änderungen prüfen'
            : cleanupOnly
            ? 'Nur aufräumen'
            : `Mergen nach ${targetBranch}`}
        </h3>
      </div>

      {#if state === 'blocked'}
        <p class="reason">{reason}</p>
        {#if rebaseConflict}
          <div class="dialog-footer">
            <button class="btn-cancel" on:click={() => dispatch('abortRebase')}>Rebase abbrechen</button>
            <button class="btn-create" on:click={() => dispatch('resolveInTerminal')}>Im Terminal auflösen</button>
          </div>
        {:else if cleanupFailed}
          <div class="dialog-footer">
            <button class="btn-cancel" on:click={() => dispatch('cancel')}>Abbrechen</button>
            <button class="btn-create" on:click={() => dispatch('retryCleanup')}>Cleanup erneut versuchen</button>
          </div>
        {:else}
          <div class="dialog-footer">
            <button class="btn-cancel" on:click={() => dispatch('cancel')}>Abbrechen</button>
            <button class="btn-create" on:click={() => dispatch('retry')}>Erneut vorbereiten</button>
          </div>
        {/if}
      {:else if state === 'staging'}
        <p class="reason">
          Wähle die Dateien, die nach <code>{targetBranch}</code> committet werden sollen. Artefakte
          und Secrets sind standardmäßig abgewählt.
        </p>
        {#if files.length > 0}
          <div class="file-list">
            {#each files as f}
              <label class="file-row">
                <input type="checkbox" bind:checked={f.selected} on:change={toggleFile} />
                <span class="file-status">{f.status}</span>
                <span class="file-path">{f.path}</span>
              </label>
            {/each}
          </div>
        {:else}
          <p class="reason">Keine geänderten Dateien im Worktree.</p>
        {/if}
        <input
          class="msg-input"
          type="text"
          placeholder="Commit-Nachricht"
          bind:value={commitMessage}
        />
        <div class="dialog-footer">
          <button class="btn-cancel" on:click={() => dispatch('cancel')}>Abbrechen</button>
          {#if selected().length === 0}
            <button class="btn-create" on:click={() => dispatch('rebaseOnly')}>Nur Rebasen</button>
          {:else}
            <button
              class="btn-create"
              disabled={!commitMessage.trim()}
              on:click={() => dispatch('stageCommit', { files: selected(), message: commitMessage })}
            >
              Committen &amp; Rebasen
            </button>
          {/if}
        </div>
      {:else}
        {#if cleanupOnly}
          <p class="reason">
            Der Branch enthält keine neuen Commits gegenüber <code>{targetBranch}</code> — es gibt
            nichts zu mergen. Worktree und Branch werden entfernt.
          </p>
        {:else}
          <div class="commits">
            {#each commits as c}
              <div class="commit-line">{c}</div>
            {/each}
          </div>
          <pre class="stat">{stat}</pre>
        {/if}
        {#if untracked.length > 0}
          <div class="untracked">
            ⚠ Untracked Dateien gehen beim Aufräumen verloren: {untracked.join(', ')}
          </div>
        {/if}
        <div class="dialog-footer">
          <button class="btn-cancel" on:click={() => dispatch('cancel')}>Abbrechen</button>
          <button class="btn-create" on:click={() => dispatch('confirm')}>
            {cleanupOnly ? 'Nur aufräumen' : 'Mergen & Aufräumen'}
          </button>
        </div>
      {/if}
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
    width: 420px;
    box-shadow: 0 8px 32px rgba(0,0,0,0.4);
  }
  .dialog-header {
    display: flex; align-items: center; gap: 8px; margin-bottom: 12px;
  }
  .dialog-icon { font-size: 18px; }
  h3 { margin: 0; font-size: 15px; color: var(--fg); }

  .reason { font-size: 12px; color: var(--fg); margin: 0 0 12px; line-height: 1.5; }
  .reason code {
    color: var(--accent); background: var(--bg-tertiary);
    padding: 1px 5px; border-radius: 3px;
  }

  .commits { max-height: 180px; overflow-y: auto; font-family: monospace; font-size: 11px; margin: 8px 0; }
  .commit-line { padding: 2px 0; color: var(--fg); }
  .stat { font-size: 10px; color: var(--fg-muted); max-height: 120px; overflow: auto; margin: 0 0 8px; }
  .untracked { font-size: 11px; color: #fbbf24; margin: 8px 0; }

  .file-list { max-height: 200px; overflow-y: auto; margin: 8px 0; }
  .file-row {
    display: flex; align-items: center; gap: 8px;
    padding: 3px 0; font-family: monospace; font-size: 11px; cursor: pointer;
  }
  .file-row input { cursor: pointer; }
  .file-status { color: var(--accent); width: 24px; flex: none; text-transform: uppercase; }
  .file-path { color: var(--fg); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .msg-input {
    width: 100%; box-sizing: border-box; margin: 4px 0 0;
    padding: 7px 10px; font-size: 12px;
    background: var(--bg-tertiary); border: 1px solid var(--border);
    border-radius: 6px; color: var(--fg);
  }
  .msg-input:focus { outline: none; border-color: var(--accent); }
  .btn-create:disabled { opacity: 0.5; cursor: not-allowed; }

  .dialog-footer { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
  .btn-cancel {
    padding: 7px 14px; background: var(--bg-tertiary); border: 1px solid var(--border);
    border-radius: 6px; color: var(--fg-muted); cursor: pointer; font-size: 12px;
  }
  .btn-cancel:hover { color: var(--fg); }
  .btn-create {
    padding: 7px 16px; background: var(--accent); border: none;
    border-radius: 6px; color: var(--bg); cursor: pointer; font-size: 12px; font-weight: 600;
  }
</style>
