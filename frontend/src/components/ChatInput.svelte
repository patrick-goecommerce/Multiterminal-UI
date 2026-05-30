<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { startRecording, type VoiceRecorder } from '../lib/voice';
  import * as App from '../../wailsjs/go/backend/App';

  export let disabled = false;
  export let placeholder = 'Nachricht eingeben...';

  const dispatch = createEventDispatcher<{
    send: { content: string };
  }>();

  let text = '';
  let inputEl: HTMLTextAreaElement;

  let voiceState: 'idle' | 'recording' | 'transcribing' = 'idle';
  let recorder: VoiceRecorder | null = null;
  let voiceError = '';

  async function startVoice() {
    if (voiceState !== 'idle' || disabled) return;
    voiceError = '';
    try {
      recorder = await startRecording();
      voiceState = 'recording';
    } catch (e) {
      voiceError = 'Mikrofon nicht verfügbar oder verweigert.';
      voiceState = 'idle';
      recorder = null;
    }
  }

  async function stopVoice() {
    if (voiceState !== 'recording' || !recorder) return;
    voiceState = 'transcribing';
    try {
      const { base64, mime } = await recorder.stop();
      const transcript = (await App.TranscribeAudio(base64, mime)).trim();
      if (transcript) {
        text = text ? text + ' ' + transcript : transcript;
        // resize after value change
        setTimeout(autoResize, 0);
      }
    } catch (e) {
      voiceError = 'Transkription fehlgeschlagen.';
    } finally {
      voiceState = 'idle';
      recorder = null;
    }
  }

  function handleSend() {
    if (!text.trim() || disabled) return;
    dispatch('send', { content: text.trim() });
    text = '';
    if (inputEl) inputEl.style.height = 'auto';
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }

  function autoResize() {
    if (!inputEl) return;
    inputEl.style.height = 'auto';
    inputEl.style.height = Math.min(inputEl.scrollHeight, 120) + 'px';
  }
</script>

<div class="chat-input" class:disabled>
  <textarea
    bind:this={inputEl}
    bind:value={text}
    on:keydown={handleKeydown}
    on:input={autoResize}
    {placeholder}
    {disabled}
    rows="1"
  ></textarea>
  <button
    class="mic-btn"
    class:recording={voiceState === 'recording'}
    disabled={disabled || voiceState === 'transcribing'}
    on:pointerdown|preventDefault={startVoice}
    on:pointerup|preventDefault={stopVoice}
    on:pointerleave={() => voiceState === 'recording' && stopVoice()}
    title="Gedrückt halten zum Diktieren"
  >
    {#if voiceState === 'transcribing'}…{:else}&#127908;{/if}
  </button>
  <button class="send-btn" on:click={handleSend} disabled={!text.trim() || disabled} title="Senden">
    &#10148;
  </button>
</div>
{#if voiceError}
  <div class="voice-error">{voiceError}</div>
{/if}

<style>
  .chat-input {
    display: flex;
    align-items: flex-end;
    gap: 8px;
    padding: 12px 16px;
    border-top: 1px solid var(--border, #45475a);
    background: var(--bg-secondary, #1e1e2e);
  }
  .chat-input.disabled { opacity: 0.6; }

  textarea {
    flex: 1;
    resize: none;
    padding: 8px 12px;
    border-radius: 8px;
    border: 1px solid var(--border, #45475a);
    background: var(--bg, #11111b);
    color: var(--fg, #cdd6f4);
    font-size: 0.85rem;
    font-family: inherit;
    line-height: 1.4;
    outline: none;
    min-height: 36px;
    max-height: 120px;
    overflow-y: auto;
    transition: border-color 0.15s;
  }
  textarea:focus { border-color: var(--accent, #39ff14); }
  textarea::placeholder { color: var(--fg-muted, #a6adc8); opacity: 0.6; }

  .send-btn {
    width: 36px;
    height: 36px;
    border-radius: 8px;
    background: var(--accent, #39ff14);
    border: none;
    color: #000;
    font-size: 1rem;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    transition: opacity 0.15s;
  }
  .send-btn:hover { opacity: 0.85; }
  .send-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  .mic-btn {
    width: 36px; height: 36px; border-radius: 8px; flex-shrink: 0;
    background: var(--bg-tertiary, #313244); border: 1px solid var(--border, #45475a);
    color: var(--fg, #cdd6f4); cursor: pointer; font-size: 1rem;
    display: flex; align-items: center; justify-content: center; transition: background .15s;
    user-select: none; touch-action: none;
  }
  .mic-btn:hover { background: var(--surface, #1e1e2e); }
  .mic-btn.recording { background: #e53935; color: #fff; animation: micpulse 1s infinite; }
  .mic-btn:disabled { opacity: .4; cursor: not-allowed; }
  @keyframes micpulse { 50% { opacity: .6; } }
  .voice-error { padding: 2px 16px 6px; font-size: .7rem; color: #e53935; }
</style>
