<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import * as App from '../../wailsjs/go/backend/App';
  import ChatMessage from './ChatMessage.svelte';
  import ChatInput from './ChatInput.svelte';
  import { chat, isStreaming, streamBuffer } from '../stores/chat';
  import { config } from '../stores/config';

  export let conversationId: string;
  export let dir = '';

  let messagesEl: HTMLDivElement;
  $: streaming = isStreaming(conversationId);
  $: buffer = streamBuffer(conversationId);
  $: conv = $chat.conversations.find(c => c.id === conversationId) ?? null;
  $: chatStyle = ($config as any)?.chat_style ?? 'claude-code';

  onMount(() => {
    if (dir && !get(chat).conversations.some(c => c.id === conversationId)) {
      App.GetConversations(dir).then(cs => chat.setConversations(cs || [])).catch(() => {});
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
      chat.streamError(conversationId);
    }
  }

  function scrollToBottom() {
    setTimeout(() => { if (messagesEl) messagesEl.scrollTop = messagesEl.scrollHeight; }, 50);
  }
  $: if ($buffer) scrollToBottom();
</script>

<div class="chat-pane" data-style={chatStyle}>
  <div class="chat-messages" bind:this={messagesEl}>
    {#if conv}
      {#each conv.messages as msg (msg.id)}
        <ChatMessage message={msg} />
      {/each}
      {#if $streaming && $buffer}
        <ChatMessage
          message={{ id: 'stream', role: 'assistant', content: '', timestamp: new Date().toISOString(), cost: '', tokens: 0 }}
          isStreaming={true} streamContent={$buffer} />
      {/if}
      {#if conv.messages.length === 0 && !$streaming}
        <div class="chat-welcome">Starte die Konversation mit einer Nachricht</div>
      {/if}
    {/if}
  </div>
  <ChatInput
    disabled={$streaming}
    placeholder={$streaming ? 'Antwort wird generiert...' : 'Nachricht eingeben...'}
    on:send={handleSend} />
</div>

<style>
  .chat-pane { display: flex; flex-direction: column; height: 100%; min-width: 0; background: var(--bg, #11111b); }
  .chat-messages { flex: 1; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 10px; }
  .chat-welcome { margin: auto; color: var(--fg-muted, #a6adc8); opacity: 0.5; font-size: 0.85rem; }

  /* Telegram style: tighter spacing (full bubble styling added in Task 10) */
  .chat-pane[data-style="telegram"] .chat-messages { gap: 6px; }
</style>
