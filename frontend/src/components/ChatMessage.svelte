<script lang="ts">
  import type { ChatMessage } from '../stores/chat';

  export let message: ChatMessage;
  export let isStreaming = false;
  export let streamContent = '';

  $: displayContent = isStreaming ? streamContent : message.content;
  $: isUser = message.role === 'user';
  $: timeStr = (() => {
    try {
      return new Date(message.timestamp).toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' });
    } catch { return ''; }
  })();

  function providerIcon(role: string): string {
    return role === 'user' ? '&#9998;' : '&#9679;';
  }
</script>

<div class="chat-message" class:user={isUser} class:assistant={!isUser}>
  <div class="msg-header">
    <span class="msg-icon">{@html providerIcon(message.role)}</span>
    <span class="msg-role">{isUser ? 'Du' : 'Assistent'}</span>
    {#if timeStr}
      <span class="msg-time">{timeStr}</span>
    {/if}
    {#if message.cost}
      <span class="msg-cost">{message.cost}</span>
    {/if}
  </div>
  <div class="msg-content">
    {#if isStreaming}
      <pre class="msg-text">{streamContent}<span class="cursor-blink">|</span></pre>
    {:else}
      <pre class="msg-text">{displayContent}</pre>
    {/if}
  </div>
</div>

<style>
  .chat-message {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 10px 14px;
    border-radius: 8px;
    max-width: 85%;
  }
  .chat-message.user {
    align-self: flex-end;
    background: rgba(57, 255, 20, 0.08);
    border: 1px solid rgba(57, 255, 20, 0.15);
  }
  .chat-message.assistant {
    align-self: flex-start;
    background: var(--bg-secondary, #1e1e2e);
    border: 1px solid var(--border, #45475a);
  }

  .msg-header {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.7rem;
    color: var(--fg-muted, #a6adc8);
  }
  .msg-icon { font-size: 0.6rem; }
  .msg-role { font-weight: 600; }
  .msg-time { margin-left: auto; opacity: 0.6; }
  .msg-cost {
    font-size: 0.6rem;
    padding: 0 4px;
    border-radius: 3px;
    background: var(--bg-tertiary, #313244);
  }

  .msg-content {
    margin-top: 2px;
  }
  .msg-text {
    font-size: 0.82rem;
    line-height: 1.5;
    color: var(--fg, #cdd6f4);
    white-space: pre-wrap;
    word-wrap: break-word;
    font-family: inherit;
    margin: 0;
  }

  .cursor-blink {
    animation: blink 1s step-end infinite;
    color: var(--accent, #39ff14);
  }
  @keyframes blink {
    50% { opacity: 0; }
  }
</style>
