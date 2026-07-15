<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let visible = false;
  export let sessionId = 0;
  export let sessionName = '';
  export let question = '';
  export let options: string[] = [];

  const dispatch = createEventDispatcher<{
    answer: { sessionId: number; answer: string };
    dismiss: { sessionId: number };
  }>();

  let customAnswer = '';

  function handleAnswer(answer: string) {
    dispatch('answer', { sessionId, answer });
  }

  function handleCustom() {
    if (!customAnswer.trim()) return;
    dispatch('answer', { sessionId, answer: customAnswer.trim() });
    customAnswer = '';
  }

  function handleDismiss() {
    dispatch('dismiss', { sessionId });
  }

  function handleKeydown(e: KeyboardEvent) {
    if (!visible) return;
    if (e.key === 'Escape') handleDismiss();
    if (e.key === 'Enter' && options.length === 0) handleCustom();
    // Quick answer shortcuts
    if (e.key === 'y' && options.includes('y')) handleAnswer('y');
    if (e.key === 'Y' && options.includes('Y')) handleAnswer('Y');
    if (e.key === 'n' && options.includes('n')) handleAnswer('n');
    if (e.key === 'N' && options.includes('N')) handleAnswer('N');
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if visible}
  <div class="backdrop" on:click={handleDismiss}>
    <div class="dialog" on:click|stopPropagation>
      <div class="header">
        <span class="header-icon">&#9888;</span>
        <div class="header-text">
          <h3>Agent benötigt Eingabe</h3>
          <span class="session-label">{sessionName || `Session ${sessionId}`}</span>
        </div>
      </div>

      <div class="question-box">
        <pre class="question-text">{question}</pre>
      </div>

      {#if options.length > 0}
        <div class="options">
          {#each options as opt}
            <button
              class="btn-option"
              class:primary={opt === 'Y' || opt === 'y' || opt === 'Yes'}
              on:click={() => handleAnswer(opt)}
            >
              {opt}
            </button>
          {/each}
        </div>
      {/if}

      <div class="custom-input">
        <input
          type="text"
          bind:value={customAnswer}
          placeholder="Eigene Antwort eingeben..."
          on:keydown={(e) => { if (e.key === 'Enter') handleCustom(); }}
        />
        <button class="btn-send" on:click={handleCustom} disabled={!customAnswer.trim()}>Senden</button>
      </div>

      <div class="footer">
        <button class="btn-dismiss" on:click={handleDismiss}>Überspringen</button>
        <span class="hint">Drücke Y/N für Schnellantwort</span>
      </div>
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 9999;
    background: rgba(0,0,0,0.6);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .dialog {
    background: var(--surface, #1e1e2e);
    border: 1px solid var(--border, #45475a);
    border-radius: 12px;
    padding: 1.25rem;
    width: 480px;
    max-width: 90vw;
    box-shadow: 0 16px 48px rgba(0,0,0,0.4);
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .header {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .header-icon {
    font-size: 1.5rem;
    color: #f5a623;
  }
  .header-text {
    flex: 1;
  }
  h3 {
    color: var(--fg, #cdd6f4);
    font-size: 0.95rem;
    margin-bottom: 2px;
  }
  .session-label {
    font-size: 0.7rem;
    color: var(--fg-muted, #a6adc8);
  }

  .question-box {
    background: var(--bg, #11111b);
    border: 1px solid var(--border, #45475a);
    border-radius: 8px;
    padding: 10px 14px;
  }
  .question-text {
    font-size: 0.82rem;
    color: var(--fg, #cdd6f4);
    white-space: pre-wrap;
    word-wrap: break-word;
    font-family: monospace;
    margin: 0;
    line-height: 1.4;
  }

  .options {
    display: flex;
    gap: 8px;
    justify-content: center;
  }
  .btn-option {
    padding: 8px 20px;
    border-radius: 8px;
    background: var(--bg, #11111b);
    border: 1px solid var(--border, #45475a);
    color: var(--fg, #cdd6f4);
    cursor: pointer;
    font-size: 0.85rem;
    font-weight: 600;
    transition: all 0.15s;
    min-width: 60px;
  }
  .btn-option:hover {
    border-color: var(--accent, #39ff14);
    background: rgba(57, 255, 20, 0.08);
  }
  .btn-option.primary {
    background: var(--accent, #39ff14);
    color: #000;
    border-color: transparent;
  }
  .btn-option.primary:hover { opacity: 0.85; }

  .custom-input {
    display: flex;
    gap: 8px;
  }
  .custom-input input {
    flex: 1;
    padding: 8px 12px;
    border-radius: 8px;
    border: 1px solid var(--border, #45475a);
    background: var(--bg, #11111b);
    color: var(--fg, #cdd6f4);
    font-size: 0.8rem;
    outline: none;
  }
  .custom-input input:focus { border-color: var(--accent, #39ff14); }
  .btn-send {
    padding: 8px 14px;
    border-radius: 8px;
    background: var(--accent, #39ff14);
    border: none;
    color: #000;
    font-weight: 600;
    font-size: 0.75rem;
    cursor: pointer;
  }
  .btn-send:disabled { opacity: 0.3; cursor: not-allowed; }

  .footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 4px;
  }
  .btn-dismiss {
    background: none;
    border: none;
    color: var(--fg-muted, #a6adc8);
    cursor: pointer;
    font-size: 0.75rem;
  }
  .btn-dismiss:hover { text-decoration: underline; }
  .hint {
    font-size: 0.65rem;
    color: var(--fg-muted, #a6adc8);
    opacity: 0.5;
  }
</style>
