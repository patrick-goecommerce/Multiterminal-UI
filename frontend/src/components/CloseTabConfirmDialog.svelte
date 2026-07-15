<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { t } from '../stores/i18n';

  export let visible: boolean = false;
  export let tabName: string = '';
  export let paneCount: number = 0;

  const dispatch = createEventDispatcher();

  function confirm() {
    dispatch('confirm');
  }

  function cancel() {
    dispatch('cancel');
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') cancel();
    if (e.key === 'Enter') confirm();
  }
</script>

<svelte:window on:keydown={visible ? handleKeydown : undefined} />

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="overlay" on:click={cancel}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation>
      <h3>{$t('closeTabConfirm.title')}</h3>
      <p class="desc">
        {$t('closeTabConfirm.body', { name: tabName, count: paneCount })}
      </p>
      <div class="actions">
        <button class="btn-cancel" on:click={cancel}>{$t('closeTabConfirm.cancel')}</button>
        <button class="btn-confirm" on:click={confirm}>{$t('closeTabConfirm.confirm')}</button>
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

  h3 {
    margin: 0 0 12px;
    color: var(--fg);
    font-size: 17px;
  }

  .desc {
    font-size: 13px;
    color: var(--fg-muted);
    margin: 0 0 20px;
    line-height: 1.5;
  }

  .actions {
    display: flex;
    gap: 10px;
    justify-content: center;
  }

  .btn-cancel {
    padding: 8px 16px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--fg-muted);
    cursor: pointer;
    font-size: 13px;
  }

  .btn-cancel:hover {
    border-color: var(--fg-muted);
    color: var(--fg);
  }

  .btn-confirm {
    padding: 8px 20px;
    background: #ef4444;
    border: none;
    border-radius: 6px;
    color: white;
    cursor: pointer;
    font-size: 13px;
    font-weight: 600;
  }

  .btn-confirm:hover {
    background: #dc2626;
  }
</style>
