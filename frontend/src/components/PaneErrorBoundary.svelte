<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import type { Pane } from '../stores/tabs';

  export let pane: Pane;

  const dispatch = createEventDispatcher();

  let error: string | null = null;

  function onError(detail: { message: string }) {
    error = detail.message || 'Unbekannter Fehler';
  }

  function handleRestart() {
    error = null;
    dispatch('restart', {
      paneId: pane.id,
      sessionId: pane.sessionId,
      mode: pane.mode,
      model: pane.model,
      name: pane.name
    });
  }

  function handleClose() {
    error = null;
    dispatch('close', { paneId: pane.id, sessionId: pane.sessionId });
  }
</script>

{#if error}
  <div class="error-boundary">
    <div class="error-content">
      <div class="error-icon">!</div>
      <h3 class="error-title">Pane-Fehler</h3>
      <p class="error-name">{pane.name || `Pane ${pane.id}`}</p>
      <p class="error-message">{error}</p>
      <div class="error-actions">
        <button class="restart-btn" on:click={handleRestart}>Neu starten</button>
        <button class="close-btn" on:click={handleClose}>Schließen</button>
      </div>
    </div>
  </div>
{:else}
  <slot {onError} />
{/if}

<style>
  .error-boundary {
    position: relative;
    display: flex;
    flex-direction: column;
    background: var(--pane-bg);
    border: 2px solid #ef4444;
    border-radius: 8px;
    overflow: hidden;
    align-items: center;
    justify-content: center;
  }

  .error-content {
    text-align: center;
    padding: 28px;
    max-width: 360px;
  }

  .error-icon {
    width: 48px;
    height: 48px;
    margin: 0 auto 16px;
    background: rgba(239, 68, 68, 0.15);
    border: 2px solid #ef4444;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 24px;
    font-weight: 700;
    color: #ef4444;
  }

  .error-title {
    margin: 0 0 8px;
    color: var(--fg);
    font-size: 16px;
  }

  .error-name {
    font-size: 12px;
    color: var(--fg-muted);
    margin: 0 0 12px;
  }

  .error-message {
    font-size: 13px;
    color: var(--fg);
    margin: 0 0 20px;
    line-height: 1.5;
    word-break: break-word;
    background: rgba(0, 0, 0, 0.3);
    padding: 8px 12px;
    border-radius: 6px;
    font-family: monospace;
    max-height: 80px;
    overflow-y: auto;
  }

  .error-actions {
    display: flex;
    gap: 10px;
    justify-content: center;
  }

  .restart-btn {
    background: var(--accent);
    color: var(--bg);
    border: none;
    padding: 6px 20px;
    border-radius: 5px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 600;
  }

  .restart-btn:hover {
    filter: brightness(1.2);
  }

  .close-btn {
    background: none;
    border: 1px solid var(--fg-muted);
    color: var(--fg-muted);
    padding: 4px 16px;
    border-radius: 5px;
    cursor: pointer;
    font-size: 12px;
  }

  .close-btn:hover {
    border-color: var(--fg);
    color: var(--fg);
  }
</style>
