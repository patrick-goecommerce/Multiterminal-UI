<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte';
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
  let cancelRequested = false;

  async function startVoice() {
    if (voiceState !== 'idle' || disabled) return;
    voiceError = '';
    cancelRequested = false;
    voiceState = 'recording'; // claim the slot BEFORE first await
    try {
      recorder = await startRecording();
      if (cancelRequested) {
        recorder.cancel();
        recorder = null;
        voiceState = 'idle';
      }
    } catch (e) {
      voiceError = 'Mikrofon nicht verfügbar oder verweigert.';
      voiceState = 'idle';
      recorder = null;
    }
  }

  async function stopVoice() {
    if (voiceState !== 'recording') return;
    if (!recorder) {
      // getUserMedia still in flight — signal startVoice to abort on resolve
      cancelRequested = true;
      return;
    }
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
      const msg = e instanceof Error ? e.message : String(e);
      voiceError = msg || 'Transkription fehlgeschlagen.';
    } finally {
      voiceState = 'idle';
      recorder = null;
    }
  }

  onDestroy(() => {
    if (recorder) recorder.cancel();
  });

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
  /* Calm pill — no arrow steppers, violet (AI) send affordance */
  .chat-input {
    display: flex;
    align-items: flex-end;
    gap: 6px;
    margin: 8px 10px;
    padding: 5px 6px 5px 13px;
    border: 1px solid var(--border-2, var(--border, #45475a));
    border-radius: 18px;
    background: var(--bg, #0a0d0b);
    transition: border-color 0.15s;
  }
  .chat-input:focus-within { border-color: color-mix(in srgb, var(--status-ai, #a184f4) 55%, transparent); }
  .chat-input.disabled { opacity: 0.6; }

  textarea {
    flex: 1;
    resize: none;
    padding: 6px 0;
    border: none;
    background: transparent;
    color: var(--fg, #dadfd2);
    font-size: 0.82rem;
    font-family: inherit;
    line-height: 1.45;
    outline: none;
    min-height: 24px;
    max-height: 120px;
    overflow-y: auto;
  }
  textarea::placeholder { color: var(--status-idle, #5a6457); }

  .send-btn {
    width: 30px;
    height: 30px;
    border-radius: 50%;
    background: var(--status-ai, #a184f4);
    border: none;
    color: #120a1f;
    font-size: 1rem;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    transition: opacity 0.15s, transform 0.12s;
  }
  .send-btn:hover { transform: translateY(-1px); }
  .send-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
    transform: none;
  }

  .mic-btn {
    width: 30px; height: 30px; border-radius: 50%; flex-shrink: 0;
    background: none; border: none;
    color: var(--fg-muted, #8b9586); cursor: pointer; font-size: 0.95rem;
    display: flex; align-items: center; justify-content: center; transition: background .15s, color .15s;
    user-select: none; touch-action: none;
  }
  .mic-btn:hover { background: var(--bg-tertiary, #1c241a); color: var(--fg); }
  .mic-btn.recording { background: var(--status-danger, #d65f5f); color: #fff; animation: micpulse 1s infinite; }
  .mic-btn:disabled { opacity: .4; cursor: not-allowed; }
  @keyframes micpulse { 50% { opacity: .6; } }
  .voice-error { padding: 2px 16px 6px; font-size: .7rem; color: var(--status-danger, #d65f5f); }
</style>
