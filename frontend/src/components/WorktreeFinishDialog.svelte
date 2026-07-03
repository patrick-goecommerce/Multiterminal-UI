<!-- frontend/src/components/WorktreeFinishDialog.svelte -->
<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let visible = false;
  export let state: 'ready' | 'blocked' = 'ready';
  export let sessionId = 0; // spread from finishDialog; not used directly here
  export let targetBranch = '';
  export let commits: string[] = [];
  export let stat = '';
  export let untracked: string[] = [];
  export let cleanupOnly = false;
  export let reason = '';

  const dispatch = createEventDispatcher();

  // Reference sessionId so the linter does not flag the (spread) prop as unused.
  $: void sessionId;
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
            : cleanupOnly
            ? 'Nur aufräumen'
            : `Mergen nach ${targetBranch}`}
        </h3>
      </div>

      {#if state === 'blocked'}
        <p class="reason">{reason}</p>
        <div class="dialog-footer">
          <button class="btn-cancel" on:click={() => dispatch('cancel')}>Abbrechen</button>
          <button class="btn-create" on:click={() => dispatch('retry')}>Erneut vorbereiten</button>
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
