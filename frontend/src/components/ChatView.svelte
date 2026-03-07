<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';
  import ChatMessage from './ChatMessage.svelte';
  import ChatInput from './ChatInput.svelte';
  import { chat, activeConversation, type Conversation } from '../stores/chat';
  import { config } from '../stores/config';

  export let dir = '';
  export let claudeDetected = false;
  export let codexDetected = false;
  export let geminiDetected = false;

  let showNewConv = false;
  let newProvider = 'claude';
  let newModel = '';
  let messagesEl: HTMLDivElement;

  function loadConversations() {
    if (!dir) return;
    chat.setDir(dir);
    App.GetConversations(dir)
      .then(convs => chat.setConversations(convs || []))
      .catch(() => chat.setConversations([]));
  }

  $: if (dir) loadConversations();

  // Event listeners for streaming
  let unsubStream: (() => void) | null = null;
  let unsubDone: (() => void) | null = null;
  let unsubError: (() => void) | null = null;

  onMount(() => {
    // These would use wails.Events.On() in production
    // For now, we use a polling fallback or the app's event system
  });

  onDestroy(() => {
    unsubStream?.();
    unsubDone?.();
    unsubError?.();
  });

  async function handleNewConversation() {
    if (!dir) return;
    try {
      const conv = await App.CreateConversation(newProvider, newModel, dir);
      chat.addConversation(conv);
      showNewConv = false;
    } catch (err) {
      console.error('[chat] create error:', err);
    }
  }

  async function handleSend(e: CustomEvent<{ content: string }>) {
    const conv = $activeConversation;
    if (!conv || !dir) return;

    const msg = {
      id: Date.now().toString(),
      role: 'user' as const,
      content: e.detail.content,
      timestamp: new Date().toISOString(),
      cost: '',
      tokens: 0,
    };
    chat.addUserMessage(msg);

    try {
      await App.AddChatMessage(dir, conv.id, e.detail.content);
    } catch (err) {
      console.error('[chat] send error:', err);
      chat.streamError(conv.id);
    }

    scrollToBottom();
  }

  function selectConversation(convId: string) {
    chat.setActive(convId);
    scrollToBottom();
  }

  async function deleteConversation(convId: string) {
    try {
      await App.DeleteConversation(dir, convId);
      chat.removeConversation(convId);
    } catch (err) {
      console.error('[chat] delete error:', err);
    }
  }

  function scrollToBottom() {
    setTimeout(() => {
      if (messagesEl) messagesEl.scrollTop = messagesEl.scrollHeight;
    }, 50);
  }

  function formatTime(ts: string): string {
    try {
      return new Date(ts).toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' });
    } catch { return ''; }
  }

  $: providers = [
    ...(claudeDetected ? [{ id: 'claude', label: 'Claude' }] : []),
    ...(codexDetected ? [{ id: 'codex', label: 'Codex' }] : []),
    ...(geminiDetected ? [{ id: 'gemini', label: 'Gemini' }] : []),
  ];
</script>

<div class="chat-view">
  <!-- Conversation list sidebar -->
  <div class="conv-sidebar">
    <div class="conv-header">
      <span class="conv-title">Chat</span>
      <button class="btn-new" on:click={() => { showNewConv = !showNewConv; }} title="Neue Konversation">+</button>
    </div>

    {#if showNewConv}
      <div class="new-conv-form">
        <select bind:value={newProvider} class="provider-select">
          {#each providers as p}
            <option value={p.id}>{p.label}</option>
          {/each}
          {#if providers.length === 0}
            <option value="claude">Claude (nicht erkannt)</option>
          {/if}
        </select>
        <button class="btn-create" on:click={handleNewConversation}>Erstellen</button>
      </div>
    {/if}

    <div class="conv-list">
      {#each $chat.conversations as conv (conv.id)}
        <button
          class="conv-item"
          class:active={$chat.activeConvId === conv.id}
          on:click={() => selectConversation(conv.id)}
        >
          <div class="conv-item-title">{conv.title}</div>
          <div class="conv-item-meta">
            <span class="conv-provider">{conv.provider}</span>
            <span class="conv-time">{formatTime(conv.updated_at)}</span>
          </div>
        </button>
      {/each}
      {#if $chat.conversations.length === 0 && !$chat.loading}
        <div class="conv-empty">Noch keine Konversationen</div>
      {/if}
    </div>
  </div>

  <!-- Active chat -->
  <div class="chat-main">
    {#if $activeConversation}
      <div class="chat-header">
        <span class="chat-title">{$activeConversation.title}</span>
        <span class="chat-provider-badge">{$activeConversation.provider}</span>
        {#if $activeConversation.model}
          <span class="chat-model">{$activeConversation.model}</span>
        {/if}
        <div class="chat-spacer"></div>
        <button class="btn-delete" on:click={() => deleteConversation($activeConversation.id)} title="Konversation löschen">&#128465;</button>
      </div>

      <div class="chat-messages" bind:this={messagesEl}>
        {#each $activeConversation.messages as msg (msg.id)}
          <ChatMessage message={msg} />
        {/each}
        {#if $chat.streaming && $chat.streamBuffer}
          <ChatMessage
            message={{ id: 'stream', role: 'assistant', content: '', timestamp: new Date().toISOString(), cost: '', tokens: 0 }}
            isStreaming={true}
            streamContent={$chat.streamBuffer}
          />
        {/if}
        {#if $activeConversation.messages.length === 0 && !$chat.streaming}
          <div class="chat-welcome">
            <p>Starte die Konversation mit einer Nachricht</p>
          </div>
        {/if}
      </div>

      <ChatInput
        disabled={$chat.streaming}
        placeholder={$chat.streaming ? 'Antwort wird generiert...' : 'Nachricht eingeben...'}
        on:send={handleSend}
      />
    {:else}
      <div class="chat-empty">
        <div class="chat-empty-icon">&#128172;</div>
        <p>Wähle eine Konversation oder erstelle eine neue</p>
      </div>
    {/if}
  </div>
</div>

<style>
  .chat-view {
    display: flex;
    flex: 1;
    min-width: 0;
    height: 100%;
    background: var(--bg, #11111b);
    overflow: hidden;
  }

  /* Conversation sidebar */
  .conv-sidebar {
    width: 240px;
    min-width: 240px;
    border-right: 1px solid var(--border, #45475a);
    background: var(--bg-secondary, #1e1e2e);
    display: flex;
    flex-direction: column;
  }
  .conv-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 12px;
    border-bottom: 1px solid var(--border, #45475a);
  }
  .conv-title {
    font-size: 0.85rem;
    font-weight: 700;
    color: var(--fg, #cdd6f4);
  }
  .btn-new {
    width: 24px;
    height: 24px;
    border-radius: 6px;
    background: var(--accent, #39ff14);
    border: none;
    color: #000;
    font-weight: 700;
    font-size: 1rem;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .btn-new:hover { opacity: 0.85; }

  .new-conv-form {
    padding: 8px 12px;
    border-bottom: 1px solid var(--border, #45475a);
    display: flex;
    gap: 6px;
  }
  .provider-select {
    flex: 1;
    padding: 4px 8px;
    border-radius: 6px;
    border: 1px solid var(--border, #45475a);
    background: var(--bg, #11111b);
    color: var(--fg, #cdd6f4);
    font-size: 0.75rem;
  }
  .btn-create {
    padding: 4px 10px;
    border-radius: 6px;
    background: var(--accent, #39ff14);
    border: none;
    color: #000;
    font-weight: 600;
    font-size: 0.7rem;
    cursor: pointer;
  }

  .conv-list {
    flex: 1;
    overflow-y: auto;
    padding: 4px;
  }
  .conv-item {
    display: flex;
    flex-direction: column;
    gap: 2px;
    width: 100%;
    padding: 8px 10px;
    background: none;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    text-align: left;
    color: var(--fg, #cdd6f4);
    transition: background 0.1s;
  }
  .conv-item:hover { background: rgba(255,255,255,0.05); }
  .conv-item.active {
    background: rgba(57, 255, 20, 0.08);
  }
  .conv-item-title {
    font-size: 0.8rem;
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .conv-item-meta {
    display: flex;
    gap: 6px;
    font-size: 0.65rem;
    color: var(--fg-muted, #a6adc8);
  }
  .conv-provider {
    text-transform: capitalize;
  }
  .conv-empty {
    padding: 20px;
    text-align: center;
    color: var(--fg-muted, #a6adc8);
    font-size: 0.75rem;
    opacity: 0.6;
  }

  /* Main chat area */
  .chat-main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .chat-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 16px;
    border-bottom: 1px solid var(--border, #45475a);
    background: var(--bg-secondary, #1e1e2e);
    flex-shrink: 0;
  }
  .chat-title {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--fg, #cdd6f4);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .chat-provider-badge {
    font-size: 0.6rem;
    padding: 1px 5px;
    border-radius: 3px;
    background: rgba(57, 255, 20, 0.15);
    color: var(--accent, #39ff14);
    font-weight: 600;
    text-transform: capitalize;
  }
  .chat-model {
    font-size: 0.65rem;
    color: var(--fg-muted, #a6adc8);
  }
  .chat-spacer { flex: 1; }
  .btn-delete {
    background: none;
    border: none;
    cursor: pointer;
    font-size: 0.8rem;
    opacity: 0.5;
    transition: opacity 0.15s;
  }
  .btn-delete:hover { opacity: 1; }

  .chat-messages {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .chat-welcome {
    display: flex;
    align-items: center;
    justify-content: center;
    flex: 1;
    color: var(--fg-muted, #a6adc8);
    font-size: 0.85rem;
    opacity: 0.5;
  }
  .chat-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    flex: 1;
    color: var(--fg-muted, #a6adc8);
    gap: 8px;
  }
  .chat-empty-icon { font-size: 2rem; opacity: 0.3; }
  .chat-empty p { font-size: 0.85rem; opacity: 0.5; }
</style>
