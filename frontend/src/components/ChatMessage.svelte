<script lang="ts">
  import { onDestroy } from 'svelte';
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

  // Visible "thinking" feedback: a live seconds counter while we wait for the
  // first token, so a slow first response never looks frozen.
  $: thinking = isStreaming && !streamContent;
  let thinkingSecs = 0;
  let thinkTimer: ReturnType<typeof setInterval> | null = null;
  $: if (thinking) startThinkTimer(); else stopThinkTimer();
  function startThinkTimer() {
    if (thinkTimer) return;
    thinkingSecs = 0;
    thinkTimer = setInterval(() => { thinkingSecs += 1; }, 1000);
  }
  function stopThinkTimer() {
    if (thinkTimer) { clearInterval(thinkTimer); thinkTimer = null; }
  }
  onDestroy(stopThinkTimer);

  // Violet "sparkle" avatar marks every AI turn — the design's one AI signifier.
</script>

{#if isUser}
  <!-- Your messages: green bubble, right-aligned -->
  <div class="msg-row user">
    <div class="bubble user-bubble">
      <pre class="msg-text">{displayContent}</pre>
    </div>
    {#if timeStr}<span class="msg-time">{timeStr}</span>{/if}
  </div>
{:else if isTool}
  <!-- Tool call rendered as a card with status -->
  <div class="msg-row assistant">
    <span class="avatar-spacer"></span>
    <div class="tool-card">
      <div class="tool-card-head">
        <span class="tool-card-icon">
          <svg viewBox="0 0 16 16" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M11 3.5l1.5 1.5M3 13l.5-2.5 7-7 2 2-7 7z"/></svg>
        </span>
        <span class="tool-card-name">{message.tool_name || 'Tool'}</span>
        {#if message.tool_result}
          <span class="tool-card-status">✓ fertig</span>
        {/if}
      </div>
      {#if message.tool_result}
        <details class="tool-result-wrap">
          <summary class="tool-result-summary">Ergebnis</summary>
          <pre class="tool-result">{message.tool_result}</pre>
        </details>
      {/if}
    </div>
  </div>
{:else}
  <!-- Claude: violet avatar + readable content -->
  <div class="msg-row assistant">
    <span class="avatar" class:avatar-thinking={thinking}>
      <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor" stroke="none"><path d="M8 2.5l1.1 3.4 3.4 1.1-3.4 1.1L8 11.5 6.9 8.1 3.5 7l3.4-1.1z"/></svg>
    </span>
    <div class="assistant-body">
      {#if isStreaming}
        {#if streamContent}
          <pre class="msg-text">{streamContent}<span class="cursor-blink">▋</span></pre>
          <div class="typing">
            <span>schreibt</span>
            <span class="typing-dots"><i></i><i></i><i></i></span>
          </div>
        {:else}
          <div class="thinking">
            <span class="thinking-label">Claude denkt nach</span>
            <span class="typing-dots"><i></i><i></i><i></i></span>
            {#if thinkingSecs >= 2}<span class="thinking-secs">{thinkingSecs}s</span>{/if}
          </div>
        {/if}
      {:else}
        <div class="msg-rendered">{@html renderedHtml}</div>
        {#if message.cost || timeStr}
          <div class="assistant-meta">
            {#if timeStr}<span>{timeStr}</span>{/if}
            {#if message.cost}<span class="msg-cost">{message.cost}</span>{/if}
          </div>
        {/if}
      {/if}
    </div>
  </div>
{/if}

<style>
  .msg-row { display: flex; gap: 9px; align-items: flex-start; }
  .msg-row.user { flex-direction: column; align-items: flex-end; gap: 4px; }
  .msg-row.assistant { align-items: flex-start; }

  /* Violet AI avatar */
  .avatar {
    width: 26px; height: 26px; border-radius: 8px; flex-shrink: 0; margin-top: 1px;
    display: flex; align-items: center; justify-content: center;
    color: var(--status-ai, #a184f4);
    background: var(--status-ai-tint, rgba(161,132,244,.16));
    border: 1px solid color-mix(in srgb, var(--status-ai, #a184f4) 40%, transparent);
  }
  .avatar-spacer { width: 26px; flex-shrink: 0; }

  /* Your green bubble */
  .bubble.user-bubble {
    max-width: 82%;
    background: var(--status-running-tint, rgba(76,197,106,.14));
    border: 1px solid color-mix(in srgb, var(--status-running, #4cc56a) 25%, transparent);
    border-radius: 13px 13px 4px 13px;
    padding: 9px 12px;
  }

  .assistant-body { min-width: 0; max-width: 84%; display: flex; flex-direction: column; gap: 8px; }

  .msg-text {
    font-size: 0.8rem;
    line-height: 1.5;
    color: var(--fg, #dadfd2);
    white-space: pre-wrap;
    word-wrap: break-word;
    font-family: inherit;
    margin: 0;
  }

  .msg-time { font-size: 0.62rem; color: var(--status-idle, #5a6457); padding-right: 4px; }
  .assistant-meta {
    display: flex; align-items: center; gap: 8px;
    font-size: 0.62rem; color: var(--status-idle, #5a6457);
  }
  .msg-cost {
    padding: 1px 5px;
    border-radius: 3px;
    background: var(--bg-tertiary, #1c241a);
    color: var(--warning, #d6a85c);
  }

  /* Pulsing avatar while waiting for the first token */
  .avatar-thinking {
    animation: avatar-pulse 1.4s ease-in-out infinite;
  }
  @keyframes avatar-pulse {
    0%, 100% { box-shadow: 0 0 0 0 var(--status-ai-tint, rgba(161,132,244,.35)); }
    50% { box-shadow: 0 0 0 5px transparent; }
  }

  /* Prominent "denkt nach" waiting state */
  .thinking {
    display: inline-flex; align-items: center; gap: 9px;
    padding: 7px 12px;
    background: var(--status-ai-tint, rgba(161,132,244,.16));
    border: 1px solid color-mix(in srgb, var(--status-ai, #a184f4) 30%, transparent);
    border-radius: 999px;
    font-size: 0.78rem; font-weight: 500;
    color: var(--status-ai, #a184f4);
  }
  .thinking-label {
    background: linear-gradient(90deg, var(--status-ai, #a184f4) 25%, var(--fg, #dadfd2) 50%, var(--status-ai, #a184f4) 75%);
    background-size: 200% 100%;
    -webkit-background-clip: text; background-clip: text;
    -webkit-text-fill-color: transparent;
    animation: thinking-shimmer 2s linear infinite;
  }
  @keyframes thinking-shimmer {
    0% { background-position: 200% 0; }
    100% { background-position: -200% 0; }
  }
  .thinking-secs {
    font-family: var(--font-mono, monospace); font-size: 0.68rem;
    color: var(--fg-muted, #8b9586);
    padding-left: 2px;
  }

  /* "schreibt…" streaming indicator */
  .typing { display: flex; align-items: center; gap: 6px; font-size: 0.72rem; color: var(--fg-muted, #8b9586); }
  .typing-dots { display: inline-flex; gap: 3px; }
  .typing-dots i {
    width: 5px; height: 5px; border-radius: 50%;
    background: var(--status-ai, #a184f4); display: inline-block;
    animation: typing-bounce 1.3s infinite;
  }
  .typing-dots i:nth-child(2) { animation-delay: 0.2s; }
  .typing-dots i:nth-child(3) { animation-delay: 0.4s; }
  @keyframes typing-bounce {
    0%, 80%, 100% { opacity: 0.25; transform: translateY(0); }
    40% { opacity: 1; transform: translateY(-3px); }
  }

  /* Tool call card */
  .tool-card {
    min-width: 0; flex: 1;
    border: 1px solid var(--border, #222a20);
    border-radius: 9px; overflow: hidden;
    background: var(--bg-secondary, #0b0e0c);
  }
  .tool-card-head {
    display: flex; align-items: center; gap: 8px;
    padding: 7px 11px;
  }
  .tool-card-icon {
    width: 18px; height: 18px; border-radius: 5px; flex-shrink: 0;
    display: flex; align-items: center; justify-content: center;
    color: var(--status-ai, #a184f4);
    background: var(--status-ai-tint, rgba(161,132,244,.16));
  }
  .tool-card-name {
    font-size: 0.72rem; font-weight: 600; color: var(--fg, #dadfd2);
    font-family: var(--font-mono, monospace); flex: 1;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  .tool-card-status {
    font-size: 0.6rem; font-weight: 600;
    color: var(--diff-add, #5bbf8c);
    background: var(--diff-add-bg, rgba(91,191,140,.12));
    padding: 3px 7px; border-radius: 999px; flex-shrink: 0;
  }

  /* Tool result block */
  .tool-result-wrap { border-top: 1px solid var(--border, #222a20); }
  .tool-result-summary {
    font-size: 0.66rem;
    color: var(--fg-muted, #8b9586);
    cursor: pointer;
    user-select: none;
    padding: 6px 11px;
  }
  .tool-result-summary:hover {
    color: var(--fg, #dadfd2);
  }
  .tool-result {
    font-size: 0.7rem;
    font-family: var(--font-mono, monospace);
    line-height: 1.5;
    color: var(--fg-muted, #8b9586);
    background: var(--bg, #0a0d0b);
    border-top: 1px solid var(--border, #222a20);
    padding: 8px 11px;
    margin: 0;
    white-space: pre-wrap;
    word-wrap: break-word;
    max-height: 200px;
    overflow: auto;
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
    color: var(--status-ai, #a184f4);
  }
  @keyframes blink {
    50% { opacity: 0; }
  }

  /* Telegram look: vivid green user bubble, tighter */
  :global(.chat-pane[data-style="telegram"]) .bubble.user-bubble {
    background: var(--status-running, #4cc56a);
    border: none;
  }
  :global(.chat-pane[data-style="telegram"]) .bubble.user-bubble .msg-text {
    color: #07150c;
  }
  /* Claude-Code look: full-width assistant body, calmer */
  :global(.chat-pane[data-style="claude-code"]) .assistant-body {
    max-width: 100%;
  }
</style>
