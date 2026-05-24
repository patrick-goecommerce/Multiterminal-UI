<script lang="ts">
  import type { ChatMessage } from '../stores/chat';
  import { renderMarkdown } from '../lib/markdown';

  export let message: ChatMessage;
  export let isStreaming = false;
  export let streamContent = '';

  $: displayContent = isStreaming ? streamContent : message.content;
  $: isUser = message.role === 'user';
  $: isTool = message.role === 'tool';
  $: renderedHtml = !isUser && !isTool && !isStreaming ? renderMarkdown(displayContent) : '';
  $: timeStr = (() => {
    try {
      return new Date(message.timestamp).toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' });
    } catch { return ''; }
  })();

  function providerIcon(role: string): string {
    if (role === 'user') return '&#9998;';
    if (role === 'tool') return '&#128295;'; // 🔧
    return '&#9679;';
  }

  function roleLabel(role: string): string {
    if (role === 'user') return 'Du';
    if (role === 'tool') return 'Tool';
    return 'Assistent';
  }
</script>

<div class="chat-message" class:user={isUser} class:assistant={!isUser && !isTool} class:tool={isTool}>
  {#if !isTool}
    <div class="msg-header">
      <span class="msg-icon">{@html providerIcon(message.role)}</span>
      <span class="msg-role">{roleLabel(message.role)}</span>
      {#if timeStr}
        <span class="msg-time">{timeStr}</span>
      {/if}
      {#if message.cost}
        <span class="msg-cost">{message.cost}</span>
      {/if}
    </div>
  {/if}
  <div class="msg-content">
    {#if isStreaming}
      <pre class="msg-text">{streamContent}<span class="cursor-blink">|</span></pre>
    {:else if isUser}
      <pre class="msg-text">{displayContent}</pre>
    {:else if isTool}
      {#if message.tool_name}
        <div class="tool-call">
          <span class="tool-icon">🔧</span>
          <span class="tool-name">{message.tool_name}</span>
          {#if timeStr}
            <span class="msg-time tool-time">{timeStr}</span>
          {/if}
        </div>
      {/if}
      {#if message.tool_result}
        <details class="tool-result-wrap">
          <summary class="tool-result-summary">Ergebnis</summary>
          <pre class="tool-result">{message.tool_result}</pre>
        </details>
      {/if}
      {#if !message.tool_name && !message.tool_result}
        <div class="tool-empty">—</div>
      {/if}
    {:else}
      <div class="msg-rendered">{@html renderedHtml}</div>
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
  .chat-message.tool {
    align-self: flex-start;
    background: transparent;
    border: 1px solid var(--border, #45475a);
    border-left: 2px solid var(--accent, #39ff14);
    padding: 4px 10px;
    max-width: 100%;
    opacity: 0.8;
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

  /* Tool call card */
  .tool-call {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.75rem;
    font-family: monospace;
    color: var(--fg-muted, #a6adc8);
  }
  .tool-icon {
    font-size: 0.7rem;
  }
  .tool-name {
    font-weight: 600;
    color: var(--accent, #39ff14);
    background: var(--bg-tertiary, #313244);
    padding: 1px 6px;
    border-radius: 3px;
    letter-spacing: 0.02em;
  }
  .tool-time {
    margin-left: auto;
    font-family: inherit;
    font-size: 0.65rem;
    opacity: 0.5;
  }

  /* Tool result block */
  .tool-result-wrap {
    margin-top: 4px;
  }
  .tool-result-summary {
    font-size: 0.68rem;
    color: var(--fg-muted, #a6adc8);
    cursor: pointer;
    user-select: none;
    padding: 1px 0;
  }
  .tool-result-summary:hover {
    color: var(--fg, #cdd6f4);
  }
  .tool-result {
    font-size: 0.72rem;
    font-family: monospace;
    line-height: 1.4;
    color: var(--fg-muted, #a6adc8);
    background: var(--bg-tertiary, #313244);
    border: 1px solid var(--border, #45475a);
    border-radius: 4px;
    padding: 6px 8px;
    margin: 4px 0 0 0;
    white-space: pre-wrap;
    word-wrap: break-word;
    max-height: 200px;
    overflow: auto;
  }

  .tool-empty {
    font-size: 0.7rem;
    color: var(--fg-muted, #a6adc8);
    opacity: 0.4;
  }

  /* Rendered markdown for assistant messages */
  .msg-rendered {
    font-size: 0.82rem;
    line-height: 1.6;
    color: var(--fg, #cdd6f4);
  }
  .msg-rendered :global(.md-p) {
    margin: 0 0 0.4em 0;
  }
  .msg-rendered :global(.md-p:last-child) {
    margin-bottom: 0;
  }
  .msg-rendered :global(br) {
    display: block;
    content: '';
    margin-top: 0.2em;
  }
  .msg-rendered :global(.md-h1),
  .msg-rendered :global(.md-h2),
  .msg-rendered :global(.md-h3) {
    margin: 0.6em 0 0.3em 0;
    font-weight: 700;
    color: var(--fg, #cdd6f4);
  }
  .msg-rendered :global(.md-h1) { font-size: 1.1rem; }
  .msg-rendered :global(.md-h2) { font-size: 1rem; }
  .msg-rendered :global(.md-h3) { font-size: 0.9rem; }
  .msg-rendered :global(.md-inline-code) {
    background: var(--bg-tertiary, #313244);
    padding: 1px 5px;
    border-radius: 3px;
    font-family: monospace;
    font-size: 0.78rem;
  }
  .msg-rendered :global(.md-code-block) {
    background: var(--bg, #11111b);
    border: 1px solid var(--border, #45475a);
    border-radius: 6px;
    padding: 10px 12px;
    margin: 0.5em 0;
    overflow-x: auto;
  }
  .msg-rendered :global(.md-code-block code) {
    font-family: monospace;
    font-size: 0.78rem;
    line-height: 1.45;
  }
  .msg-rendered :global(.md-list) {
    margin: 0.3em 0;
    padding-left: 1.4em;
  }
  .msg-rendered :global(.md-list li) {
    margin: 0.15em 0;
  }
  .msg-rendered :global(.md-link) {
    color: var(--accent, #39ff14);
    text-decoration: none;
  }
  .msg-rendered :global(.md-link:hover) {
    text-decoration: underline;
  }
  .msg-rendered :global(strong) {
    font-weight: 700;
    color: var(--fg, #cdd6f4);
  }
  .msg-rendered :global(em) {
    font-style: italic;
    opacity: 0.9;
  }

  .cursor-blink {
    animation: blink 1s step-end infinite;
    color: var(--accent, #39ff14);
  }
  @keyframes blink {
    50% { opacity: 0; }
  }

  /* Telegram look: vivid user bubble on the right, rounded, tighter */
  :global(.chat-pane[data-style="telegram"]) .chat-message.user {
    background: var(--accent, #39ff14);
    color: #000;
    border: none;
  }
  :global(.chat-pane[data-style="telegram"]) .chat-message {
    max-width: 75%;
    border-radius: 14px;
  }
  /* Claude-Code look: full-width, left-aligned, calmer */
  :global(.chat-pane[data-style="claude-code"]) .chat-message {
    max-width: 100%;
    align-self: stretch;
  }
</style>
