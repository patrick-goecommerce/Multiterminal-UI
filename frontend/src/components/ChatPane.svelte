<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import { get } from 'svelte/store';
  import * as App from '../../wailsjs/go/backend/App';
  import ChatMessage from './ChatMessage.svelte';
  import ChatInput from './ChatInput.svelte';
  import { chat, isStreaming, streamBuffer, streamErrorMsg } from '../stores/chat';
  import { config } from '../stores/config';

  export let conversationId: string;
  export let dir = '';
  export let paneId = '';

  const dispatch = createEventDispatcher();

  let messagesEl: HTMLDivElement;
  $: streaming = isStreaming(conversationId);
  $: buffer = streamBuffer(conversationId);
  $: errorMsg = streamErrorMsg(conversationId);
  $: conv = $chat.conversations.find(c => c.id === conversationId) ?? null;
  $: chatStyle = ($config as any)?.chat_style ?? 'claude-code';

  onMount(() => {
    if (dir && !get(chat).conversations.some(c => c.id === conversationId)) {
      App.GetConversations(dir).then(cs => chat.setConversations(cs || [])).catch(() => {});
    }
    // Pre-warm the claude process so the first reply isn't gated on a cold
    // start. No-op if already running; the process is reaped when the pane closes.
    if (dir && conversationId) {
      App.WarmChatSession(dir, conversationId).catch(() => {});
    }
  });

  async function handleSend(e: CustomEvent<{ content: string }>) {
    if (!conv || get(streaming)) return;
    chat.addUserMessage(conversationId, {
      id: Date.now().toString(), role: 'user', content: e.detail.content,
      timestamp: new Date().toISOString(), cost: '', tokens: 0,
    });
    scrollToBottom();
    try {
      await App.AddChatMessage(dir, conversationId, e.detail.content);
    } catch (err) {
      console.error('[chat] send error:', err);
      chat.streamError(conversationId, err instanceof Error ? err.message : String(err));
    }
  }

  function scrollToBottom() {
    setTimeout(() => { if (messagesEl) messagesEl.scrollTop = messagesEl.scrollHeight; }, 50);
  }
  $: if ($buffer) scrollToBottom();
</script>

<div class="chat-pane" data-style={chatStyle}>
  <div class="chat-header">
    <span class="chat-avatar" aria-hidden="true">
      <svg viewBox="0 0 16 16" width="13" height="13" fill="currentColor" stroke="none"><path d="M8 2.5l1.1 3.4 3.4 1.1-3.4 1.1L8 11.5 6.9 8.1 3.5 7l3.4-1.1z"/></svg>
    </span>
    <div class="chat-header-meta">
      <span class="chat-header-title">{conv?.title || 'Chat'}</span>
      {#if conv?.model || conv?.provider}
        <span class="chat-header-sub">{conv?.model || conv?.provider}</span>
      {/if}
    </div>
    <div class="display-toggle" role="group" aria-label="Anzeige umschalten">
      <button class="seg" title="Als Terminal anzeigen" on:click|stopPropagation={() => dispatch('toggleDisplay', { paneId })}>Terminal</button>
      <button class="seg seg-active" title="Chat-Ansicht">Chat</button>
    </div>
    <button class="chat-header-btn close" title="Schließen" on:click|stopPropagation={() => dispatch('close', { paneId })}>✕</button>
  </div>
  <div class="chat-messages" bind:this={messagesEl}>
    {#if conv}
      {#each conv.messages as msg (msg.id)}
        <ChatMessage message={msg} />
      {/each}
      {#if $streaming}
        <ChatMessage
          message={{ id: 'stream', role: 'assistant', content: '', timestamp: new Date().toISOString(), cost: '', tokens: 0 }}
          isStreaming={true} streamContent={$buffer} />
      {/if}
      {#if conv.messages.length === 0 && !$streaming && !$errorMsg}
        <div class="chat-welcome">Starte die Konversation mit einer Nachricht</div>
      {/if}
    {/if}
    {#if $errorMsg}
      <div class="chat-error" role="alert">
        <span class="chat-error-icon">⚠</span>
        <span class="chat-error-text">{$errorMsg}</span>
      </div>
    {/if}
  </div>
  <ChatInput
    disabled={$streaming}
    placeholder={$streaming ? 'Antwort wird generiert...' : 'Nachricht eingeben...'}
    on:send={handleSend} />
</div>

<style>
  .chat-pane { display: flex; flex: 1; flex-direction: column; height: 100%; min-width: 0; background: var(--pane-bg, #0e1210); }
  .chat-messages { flex: 1; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 14px; }
  .chat-welcome { margin: auto; color: var(--fg-muted, #a6adc8); opacity: 0.5; font-size: 0.85rem; }

  .chat-error {
    display: flex; align-items: flex-start; gap: 8px;
    margin-top: 4px; padding: 9px 11px;
    background: var(--status-danger-tint, rgba(214,95,95,.15));
    border: 1px solid color-mix(in srgb, var(--status-danger, #d65f5f) 40%, transparent);
    border-radius: 9px;
    color: var(--status-danger, #d65f5f);
    font-size: 0.74rem; line-height: 1.45;
  }
  .chat-error-icon { flex-shrink: 0; }
  .chat-error-text { word-break: break-word; font-family: var(--font-mono, monospace); color: var(--fg, #dadfd2); }

  .chat-header {
    display: flex;
    align-items: center;
    gap: 9px;
    padding: 0 9px 0 11px;
    background: var(--bg-secondary, #0b0e0c);
    border-bottom: 1px solid var(--border, #45475a);
    height: 38px;
    min-height: 38px;
    flex-shrink: 0;
  }

  .chat-avatar {
    width: 20px; height: 20px; border-radius: 6px; flex-shrink: 0;
    display: flex; align-items: center; justify-content: center;
    color: var(--status-ai, #a184f4);
    background: var(--status-ai-tint, rgba(161,132,244,.16));
    border: 1px solid color-mix(in srgb, var(--status-ai, #a184f4) 40%, transparent);
  }

  .chat-header-meta {
    display: flex; flex-direction: column; gap: 1px;
    min-width: 0; flex: 1;
  }
  .chat-header-title {
    font-size: 12.5px; font-weight: 600;
    color: var(--fg, #cdd6f4);
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .chat-header-sub {
    font-size: 10px; font-family: var(--font-mono, monospace);
    color: var(--fg-muted, #8b9586);
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }

  /* Segmented Terminal | Chat toggle */
  .display-toggle {
    display: flex; align-items: center; flex-shrink: 0;
    background: var(--bg-tertiary, #1c241a);
    border: 1px solid var(--border-2, var(--border, #45475a));
    border-radius: 7px; padding: 2px;
  }
  .display-toggle .seg {
    padding: 4px 9px; border: none; background: none; cursor: pointer;
    border-radius: 5px; font-size: 11px; font-weight: 500;
    color: var(--fg-muted, #8b9586); transition: background 0.12s, color 0.12s;
  }
  .display-toggle .seg:hover { color: var(--fg); }
  .display-toggle .seg-active {
    color: var(--status-ai, #a184f4);
    background: var(--status-ai-tint, rgba(161,132,244,.16));
  }

  .chat-header-btn {
    background: none;
    border: none;
    color: var(--fg-muted, #8b9586);
    cursor: pointer;
    padding: 4px 6px;
    font-size: 12px;
    line-height: 1;
    border-radius: 5px;
    flex-shrink: 0;
  }

  .chat-header-btn:hover {
    background: var(--bg-tertiary, #313244);
    color: var(--fg, #cdd6f4);
  }
  .chat-header-btn.close:hover { background: var(--status-danger, #d65f5f); color: #fff; }

  /* Telegram style: tighter spacing (full bubble styling added in Task 10) */
  .chat-pane[data-style="telegram"] .chat-messages { gap: 6px; }
</style>
