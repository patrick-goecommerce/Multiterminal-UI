<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { config } from '../stores/config';

  export let visible: boolean = false;
  export let issueContext: { number: number; title: string; body: string; labels: string[] } | null = null;
  export let claudeDetected: boolean = true;

  const dispatch = createEventDispatcher();

  let selectedModel = '';
  let dialogEl: HTMLDivElement;

  $: if (visible) {
    requestAnimationFrame(() => dialogEl?.focus());
  }

  function launch(type: 'shell' | 'claude' | 'claude-yolo') {
    dispatch('launch', { type, model: selectedModel, issue: issueContext });
    dispatch('close');
    selectedModel = '';
  }

  function close() {
    dispatch('close');
    selectedModel = '';
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
    if (issueContext) {
      if (e.key === '1') launch('claude');
      if (e.key === '2') launch('claude-yolo');
    } else {
      if (e.key === '1') launch('shell');
      if (e.key === '2') launch('claude');
      if (e.key === '3') launch('claude-yolo');
    }
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="overlay" on:click={close}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation bind:this={dialogEl} tabindex="-1" on:keydown={handleKeydown}>
      <h3>{issueContext ? `Claude für #${issueContext.number}` : 'Neues Terminal'}</h3>
      {#if issueContext}
        <div class="issue-context">
          <span class="issue-ctx-num">#{issueContext.number}</span>
          <span class="issue-ctx-title">{issueContext.title}</span>
        </div>
      {/if}

      {#if !claudeDetected}
        <div class="claude-warning">
          <span class="warning-icon">&#9888;</span>
          <span>Claude CLI nicht gefunden.</span>
          <button class="warning-link" on:click={() => dispatch('openSettings')}>Einstellungen</button>
        </div>
      {/if}

      <div class="options">
        {#if !issueContext}
          <button class="option" on:click={() => launch('shell')}>
            <span class="option-key">1</span>
            <span class="option-icon">&#9000;</span>
            <div class="option-text">
              <strong>Shell</strong>
              <span>Standard-Terminal</span>
            </div>
          </button>
        {/if}

        <button class="option" on:click={() => launch('claude')}>
          <span class="option-key">{issueContext ? '1' : '2'}</span>
          <span class="option-icon">&#10024;</span>
          <div class="option-text">
            <strong>Claude Code</strong>
            <span>Normal-Modus</span>
          </div>
        </button>

        <button class="option yolo" on:click={() => launch('claude-yolo')}>
          <span class="option-key">{issueContext ? '2' : '3'}</span>
          <span class="option-icon">&#9889;</span>
          <div class="option-text">
            <strong>Claude YOLO</strong>
            <span>Alle Berechtigungen</span>
          </div>
        </button>
      </div>

      {#if $config.claude_models.length > 0}
        <div class="model-picker">
          <label>Modell:</label>
          <select bind:value={selectedModel}>
            {#each $config.claude_models as model}
              <option value={model.id}>{model.label}</option>
            {/each}
          </select>
        </div>
      {/if}

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
    min-width: 360px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
    outline: none;
  }

  h3 {
    margin: 0 0 16px;
    color: var(--fg);
    font-size: 16px;
  }

  .issue-context {
    display: flex; align-items: center; gap: 8px;
    padding: 8px 12px; margin-bottom: 12px;
    background: var(--bg-secondary); border: 1px solid var(--border);
    border-radius: 8px; font-size: 12px;
  }
  .issue-ctx-num { color: var(--fg-muted); font-weight: 600; }
  .issue-ctx-title { color: var(--fg); font-weight: 500; }

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

  .option:hover {
    border-color: var(--accent);
    background: var(--bg-tertiary);
  }

  .option.yolo:hover {
    border-color: var(--error);
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

  .model-picker {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 16px;
  }

  .model-picker label {
    font-size: 12px;
    color: var(--fg-muted);
  }

  .model-picker select {
    flex: 1;
    padding: 6px 8px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--fg);
    font-size: 12px;
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

  .claude-warning {
    display: flex; align-items: center; gap: 8px;
    padding: 8px 12px; margin-bottom: 12px;
    background: rgba(243, 139, 168, 0.1); border: 1px solid rgba(243, 139, 168, 0.4);
    border-radius: 8px; font-size: 12px; color: #f38ba8;
  }

  .warning-icon { font-size: 16px; }

  .warning-link {
    background: none; border: none; color: var(--accent);
    cursor: pointer; font-size: 12px; text-decoration: underline;
    padding: 0;
  }
  .warning-link:hover { opacity: 0.8; }
</style>
