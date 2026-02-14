<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';

  export let visible: boolean = false;

  const dispatch = createEventDispatcher();

  async function openExisting() {
    try {
      const dir = await App.SelectDirectory('');
      if (dir) {
        const name = dir.replace(/\\/g, '/').split('/').pop() || 'Projekt';
        dispatch('create', { name, dir });
        dispatch('close');
      }
    } catch (err) {
      console.error('[ProjectDialog] SelectDirectory failed:', err);
    }
  }

  async function createNew() {
    try {
      const parentDir = await App.SelectDirectory('');
      if (!parentDir) return;
      const name = prompt('Projektname:');
      if (!name) return;
      // Create directory via backend
      const fullPath = parentDir + '\\' + name;
      await App.CreateDirectory(fullPath);
      dispatch('create', { name, dir: fullPath });
      dispatch('close');
    } catch (err) {
      console.error('[ProjectDialog] CreateNew failed:', err);
    }
  }

  function close() {
    dispatch('close');
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
    if (e.key === '1') openExisting();
    if (e.key === '2') createNew();
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <div class="overlay" on:click={close} on:keydown={handleKeydown}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <div class="dialog" on:click|stopPropagation>
      <div class="dialog-header">
        <div>
          <h3>Projekt hinzufügen</h3>
          <p class="subtitle">Wähle wie du ein Projekt öffnen möchtest</p>
        </div>
        <button class="close-btn" on:click={close}>&times;</button>
      </div>

      <div class="options">
        <button class="option" on:click={openExisting}>
          <span class="option-key">1</span>
          <span class="option-icon">&#128194;</span>
          <div class="option-text">
            <strong>Vorhandenen Ordner öffnen</strong>
            <span>Ein bestehendes Projekt auf diesem Computer öffnen</span>
          </div>
          <span class="option-arrow">&#8250;</span>
        </button>

        <button class="option" on:click={createNew}>
          <span class="option-key">2</span>
          <span class="option-icon">&#128230;</span>
          <div class="option-text">
            <strong>Neues Projekt erstellen</strong>
            <span>Einen neuen Projektordner anlegen</span>
          </div>
          <span class="option-arrow">&#8250;</span>
        </button>
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
    padding: 24px;
    min-width: 400px;
    max-width: 480px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
  }

  .dialog-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 20px;
  }

  h3 {
    margin: 0;
    color: var(--fg);
    font-size: 18px;
    font-weight: 600;
  }

  .subtitle {
    margin: 4px 0 0;
    color: var(--fg-muted);
    font-size: 13px;
  }

  .close-btn {
    background: none;
    border: none;
    color: var(--fg-muted);
    font-size: 20px;
    cursor: pointer;
    padding: 0 4px;
    line-height: 1;
  }
  .close-btn:hover {
    color: var(--fg);
  }

  .options {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .option {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 16px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 10px;
    color: var(--fg);
    cursor: pointer;
    text-align: left;
    transition: all 0.15s;
  }

  .option:hover {
    border-color: var(--accent);
    background: var(--bg-tertiary);
  }

  .option-key {
    font-size: 11px;
    padding: 2px 6px;
    background: var(--bg-tertiary);
    border-radius: 4px;
    color: var(--fg-muted);
    font-family: monospace;
    flex-shrink: 0;
  }

  .option-icon {
    font-size: 22px;
    flex-shrink: 0;
  }

  .option-text {
    display: flex;
    flex-direction: column;
    flex: 1;
  }

  .option-text strong {
    font-size: 14px;
  }

  .option-text span {
    font-size: 12px;
    color: var(--fg-muted);
    margin-top: 2px;
  }

  .option-arrow {
    font-size: 18px;
    color: var(--fg-muted);
    flex-shrink: 0;
  }
</style>
