<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let visible = false;

  const dispatch = createEventDispatcher<{
    create: { title: string; cardType: string; description: string };
    cancel: void;
  }>();

  let title = '';
  let cardType = 'feature';
  let description = '';
  let dialogEl: HTMLDivElement;

  const CARD_TYPES = [
    { value: 'feature', label: 'Feature' },
    { value: 'bugfix', label: 'Bugfix' },
    { value: 'refactor', label: 'Refactor' },
    { value: 'docs', label: 'Dokumentation' },
  ];

  $: if (visible) {
    requestAnimationFrame(() => dialogEl?.focus());
  }

  function resetFields() {
    title = '';
    cardType = 'feature';
    description = '';
  }

  function handleCreate() {
    if (!title.trim()) return;
    dispatch('create', { title: title.trim(), cardType, description: description.trim() });
    resetFields();
  }

  function handleCancel() {
    visible = false;
    dispatch('cancel');
    resetFields();
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') handleCancel();
    if (e.key === 'Enter' && e.target instanceof HTMLInputElement) handleCreate();
  }

  function handleOverlayClick() {
    handleCancel();
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="overlay" on:click={handleOverlayClick}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation bind:this={dialogEl} tabindex="-1" on:keydown={handleKeydown}>
      <div class="dialog-header">
        <h3>Neue Karte erstellen</h3>
        <button class="close-btn" on:click={handleCancel} title="Schliessen">&#10005;</button>
      </div>

      <div class="form-group">
        <label for="create-title">Titel</label>
        <input
          id="create-title"
          class="form-input"
          type="text"
          placeholder="Titel der Karte..."
          bind:value={title}
          autofocus
        />
      </div>

      <div class="form-group">
        <label for="create-type">Typ</label>
        <select id="create-type" class="form-select" bind:value={cardType}>
          {#each CARD_TYPES as ct}
            <option value={ct.value}>{ct.label}</option>
          {/each}
        </select>
      </div>

      <div class="form-group">
        <label for="create-desc">Beschreibung</label>
        <textarea
          id="create-desc"
          class="form-textarea"
          placeholder="Optionale Beschreibung..."
          bind:value={description}
          rows="4"
        ></textarea>
      </div>

      <div class="dialog-footer">
        <button class="btn-cancel" on:click={handleCancel}>Abbrechen</button>
        <button class="btn-create" on:click={handleCreate} disabled={!title.trim()}>Erstellen</button>
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
    background: var(--bg, #11111b);
    border: 1px solid var(--border, #45475a);
    border-radius: 12px;
    padding: 20px;
    min-width: 380px;
    max-width: 480px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
    outline: none;
  }

  .dialog-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
  }

  .dialog-header h3 {
    margin: 0;
    font-size: 1rem;
    font-weight: 700;
    color: var(--fg, #cdd6f4);
  }

  .close-btn {
    padding: 2px 6px;
    background: transparent;
    border: none;
    color: var(--fg-muted, #a6adc8);
    cursor: pointer;
    font-size: 0.9rem;
  }
  .close-btn:hover { color: #ef4444; }

  .form-group {
    margin-bottom: 12px;
  }

  .form-group label {
    display: block;
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--fg, #cdd6f4);
    margin-bottom: 4px;
  }

  .form-input, .form-select, .form-textarea {
    width: 100%;
    padding: 8px 10px;
    border-radius: 6px;
    border: 1px solid var(--border, #45475a);
    background: var(--bg-secondary, #1e1e2e);
    color: var(--fg, #cdd6f4);
    font-size: 0.8rem;
    outline: none;
    box-sizing: border-box;
  }

  .form-input:focus, .form-select:focus, .form-textarea:focus {
    border-color: var(--accent, #39ff14);
  }

  .form-textarea {
    resize: vertical;
    font-family: inherit;
    line-height: 1.4;
  }

  .dialog-footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 16px;
  }

  .btn-cancel {
    padding: 6px 14px;
    background: var(--bg-secondary, #1e1e2e);
    border: 1px solid var(--border, #45475a);
    border-radius: 6px;
    color: var(--fg-muted, #a6adc8);
    cursor: pointer;
    font-size: 0.75rem;
  }
  .btn-cancel:hover { color: var(--fg, #cdd6f4); }

  .btn-create {
    padding: 6px 14px;
    border-radius: 6px;
    background: var(--accent, #39ff14);
    border: none;
    color: #000;
    font-weight: 600;
    cursor: pointer;
    font-size: 0.75rem;
  }
  .btn-create:hover { opacity: 0.85; }
  .btn-create:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
</style>
