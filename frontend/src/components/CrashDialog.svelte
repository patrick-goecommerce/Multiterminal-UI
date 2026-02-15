<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let visible: boolean = false;

  const dispatch = createEventDispatcher();

  function enableLogging() {
    dispatch('enable');
  }

  function dismiss() {
    dispatch('dismiss');
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') dismiss();
    if (e.key === 'Enter') enableLogging();
  }
</script>

<svelte:window on:keydown={visible ? handleKeydown : undefined} />

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="overlay" on:click={dismiss}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation>
      <div class="icon">!</div>
      <h3>Instabilität erkannt</h3>
      <p class="desc">
        Die letzten zwei Sitzungen wurden nicht sauber beendet.
        Möchtest du das Logging aktivieren, um die Ursache zu finden?
      </p>
      <p class="hint">
        Das Log wird automatisch deaktiviert, sobald 3 Sitzungen wieder stabil laufen.
      </p>
      <div class="actions">
        <button class="btn-dismiss" on:click={dismiss}>Nein, danke</button>
        <button class="btn-enable" on:click={enableLogging}>Logging aktivieren</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 200;
  }

  .dialog {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 28px;
    max-width: 420px;
    text-align: center;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
  }

  .icon {
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

  h3 {
    margin: 0 0 12px;
    color: var(--fg);
    font-size: 17px;
  }

  .desc {
    font-size: 13px;
    color: var(--fg);
    margin: 0 0 8px;
    line-height: 1.5;
  }

  .hint {
    font-size: 12px;
    color: var(--fg-muted);
    margin: 0 0 20px;
    font-style: italic;
  }

  .actions {
    display: flex;
    gap: 10px;
    justify-content: center;
  }

  .btn-dismiss {
    padding: 8px 16px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--fg-muted);
    cursor: pointer;
    font-size: 13px;
  }

  .btn-dismiss:hover {
    border-color: var(--fg-muted);
    color: var(--fg);
  }

  .btn-enable {
    padding: 8px 20px;
    background: #ef4444;
    border: none;
    border-radius: 6px;
    color: white;
    cursor: pointer;
    font-size: 13px;
    font-weight: 600;
  }

  .btn-enable:hover {
    background: #dc2626;
  }
</style>
