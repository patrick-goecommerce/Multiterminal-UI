<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { createTerminal, getTerminalTheme } from '../lib/terminal';
  import type { Pane } from '../stores/tabs';
  import { currentTheme } from '../stores/theme';
  import { config } from '../stores/config';
  import * as App from '../../wailsjs/go/backend/App';
  import { EventsOn, ClipboardGetText, ClipboardSetText } from '../../wailsjs/runtime/runtime';
  import QueuePanel from './QueuePanel.svelte';

  export let pane: Pane;

  const dispatch = createEventDispatcher();

  let containerEl: HTMLDivElement;
  let termInstance: ReturnType<typeof createTerminal> | null = null;
  let resizeObserver: ResizeObserver | null = null;
  let cleanupFn: (() => void) | null = null;

  let editing = false;
  let editName = '';
  let nameInput: HTMLInputElement;
  let zoomTimer: ReturnType<typeof setTimeout> | null = null;
  let resizeTimer: ReturnType<typeof setTimeout> | null = null;
  let isZooming = false;
  let showQueue = false;
  let queueCount = 0;
  let queueCleanup: (() => void) | null = null;

  onMount(() => {
    termInstance = createTerminal($currentTheme);
    termInstance.terminal.open(containerEl);

    // Fit terminal to container
    requestAnimationFrame(() => {
      termInstance?.fitAddon.fit();
      const dims = termInstance?.fitAddon.proposeDimensions();
      if (dims) {
        App.ResizeSession(pane.sessionId, dims.rows, dims.cols);
      }
    });

    // Intercept keys that should NOT go to the PTY but to the app
    termInstance.terminal.attachCustomKeyEventHandler((e: KeyboardEvent) => {
      if (e.type !== 'keydown') return true;
      // Ctrl+V → paste from clipboard into PTY (via Wails native clipboard)
      if (e.ctrlKey && e.key === 'v') {
        ClipboardGetText().then((text) => {
          if (text) {
            const encoder = new TextEncoder();
            const bytes = encoder.encode(text);
            let binary = '';
            for (let i = 0; i < bytes.length; i++) {
              binary += String.fromCharCode(bytes[i]);
            }
            App.WriteToSession(pane.sessionId, btoa(binary));
          }
        }).catch((err) => {
          console.error('[TerminalPane] clipboard read failed:', err);
        });
        return false;
      }
      // Ctrl+C with selection → copy to clipboard
      if (e.ctrlKey && e.key === 'c' && termInstance?.terminal.hasSelection()) {
        ClipboardSetText(termInstance.terminal.getSelection());
        termInstance.terminal.clearSelection();
        return false;
      }
      // Ctrl+Z, Ctrl+N, Ctrl+T, Ctrl+W, Ctrl+B → let app handle (don't send to PTY)
      if (e.ctrlKey && ['z', 'n', 't', 'w', 'b'].includes(e.key)) {
        return false;
      }
      return true;
    });

    // Handle keyboard input → send to PTY
    termInstance.terminal.onData((data: string) => {
      const encoder = new TextEncoder();
      const bytes = encoder.encode(data);
      const b64 = btoa(String.fromCharCode(...bytes));
      App.WriteToSession(pane.sessionId, b64);
    });

    // Listen for PTY output from Go backend
    cleanupFn = EventsOn('terminal:output', (id: number, b64: string) => {
      if (id === pane.sessionId && termInstance) {
        const raw = atob(b64);
        const bytes = new Uint8Array(raw.length);
        for (let i = 0; i < raw.length; i++) {
          bytes[i] = raw.charCodeAt(i);
        }
        termInstance.terminal.write(bytes);
      }
    });

    // Ctrl+Mouse Wheel zoom per terminal pane (debounced)
    containerEl.addEventListener('wheel', (e: WheelEvent) => {
      if (!e.ctrlKey || !termInstance) return;
      e.preventDefault();
      const current = termInstance.terminal.options.fontSize || 14;
      const newSize = e.deltaY < 0 ? current + 1 : current - 1;
      if (newSize >= 8 && newSize <= 32) {
        isZooming = true;
        termInstance.terminal.options.fontSize = newSize;
        if (zoomTimer) clearTimeout(zoomTimer);
        zoomTimer = setTimeout(() => {
          if (termInstance) {
            termInstance.fitAddon.fit();
            const dims = termInstance.fitAddon.proposeDimensions();
            if (dims) {
              App.ResizeSession(pane.sessionId, dims.rows, dims.cols);
            }
          }
          isZooming = false;
        }, 150);
      }
    }, { passive: false });

    // Auto-resize on container size change (debounced, skip during zoom)
    resizeObserver = new ResizeObserver(() => {
      if (!termInstance || isZooming) return;
      if (resizeTimer) clearTimeout(resizeTimer);
      resizeTimer = setTimeout(() => {
        if (termInstance) {
          termInstance.fitAddon.fit();
          const dims = termInstance.fitAddon.proposeDimensions();
          if (dims) {
            App.ResizeSession(pane.sessionId, dims.rows, dims.cols);
          }
        }
      }, 100);
    });
    resizeObserver.observe(containerEl);

    // Listen for queue updates to show badge count
    queueCleanup = EventsOn('queue:update', (sid: number) => {
      if (sid === pane.sessionId) {
        App.GetQueue(pane.sessionId).then(items => {
          queueCount = items.filter((i: any) => i.status !== 'done').length;
        });
      }
    });
  });

  onDestroy(() => {
    if (cleanupFn) cleanupFn();
    if (queueCleanup) queueCleanup();
    resizeObserver?.disconnect();
    termInstance?.dispose();
  });

  // Update theme reactively (including custom accent as cursor color)
  $: if (termInstance && $currentTheme) {
    const theme = getTerminalTheme($currentTheme);
    if ($config.terminal_color) {
      theme.cursor = $config.terminal_color;
      theme.cursorAccent = '#000000';
    }
    termInstance.terminal.options.theme = theme;
  }

  function handleClose() {
    dispatch('close', { paneId: pane.id, sessionId: pane.sessionId });
  }

  function handleMaximize() {
    dispatch('maximize', { paneId: pane.id });
  }

  function handleFocus() {
    dispatch('focus', { paneId: pane.id });
  }

  function startRename() {
    editName = pane.name;
    editing = true;
    requestAnimationFrame(() => {
      nameInput?.focus();
      nameInput?.select();
    });
  }

  function finishRename() {
    editing = false;
    const trimmed = editName.trim();
    if (trimmed && trimmed !== pane.name) {
      dispatch('rename', { paneId: pane.id, name: trimmed });
    }
  }

  function handleRenameKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      finishRename();
    }
    if (e.key === 'Escape') {
      editing = false;
    }
  }

  function getModeLabel(mode: string): string {
    switch (mode) {
      case 'claude': return 'Claude';
      case 'claude-yolo': return 'YOLO';
      default: return 'Shell';
    }
  }

  function getModeBadgeClass(mode: string): string {
    switch (mode) {
      case 'claude': return 'badge-claude';
      case 'claude-yolo': return 'badge-yolo';
      default: return 'badge-shell';
    }
  }

  function getActivityDot(activity: string): string {
    switch (activity) {
      case 'active': return 'dot-active';
      case 'done': return 'dot-done';
      case 'needsInput': return 'dot-needs-input';
      default: return 'dot-idle';
    }
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-static-element-interactions -->
<div
  class="terminal-pane"
  class:focused={pane.focused}
  class:activity-done={pane.activity === 'done'}
  class:activity-needs-input={pane.activity === 'needsInput'}
  on:click={handleFocus}
>
  <div class="pane-titlebar"
    class:titlebar-done={pane.activity === 'done'}
    class:titlebar-needs-input={pane.activity === 'needsInput'}
  >
    <div class="pane-title-left">
      <span class="status-dot {getActivityDot(pane.activity)}"></span>
      {#if editing}
        <input
          class="rename-input"
          type="text"
          bind:value={editName}
          bind:this={nameInput}
          on:blur={finishRename}
          on:keydown={handleRenameKeydown}
          on:click|stopPropagation
        />
      {:else}
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <span class="pane-name" on:dblclick|stopPropagation={startRename} title="Doppelklick zum Umbenennen">{pane.name}</span>
      {/if}
      <span class="mode-badge {getModeBadgeClass(pane.mode)}">{getModeLabel(pane.mode)}</span>
      {#if pane.model}
        <span class="model-label">{pane.model}</span>
      {/if}
    </div>
    <div class="pane-title-right">
      {#if pane.cost}
        <span class="cost-label">{pane.cost}</span>
      {/if}
      <button class="pane-btn queue-toggle" class:queue-active={queueCount > 0} on:click|stopPropagation={() => (showQueue = !showQueue)} title="Pipeline Queue">
        &#9654;{#if queueCount > 0}<span class="queue-badge">{queueCount}</span>{/if}
      </button>
      <button class="pane-btn" on:click|stopPropagation={handleMaximize} title="Maximize">
        &#x26F6;
      </button>
      <button class="pane-btn close" on:click|stopPropagation={handleClose} title="Close">
        &times;
      </button>
    </div>
  </div>
  <QueuePanel sessionId={pane.sessionId} visible={showQueue} />
  <div class="terminal-container" bind:this={containerEl}></div>
</div>

<style>
  .terminal-pane {
    position: relative;
    display: flex;
    flex-direction: column;
    background: var(--pane-bg);
    border: 2px solid var(--pane-border);
    border-radius: 8px;
    overflow: hidden;
    transition: border-color 0.3s, box-shadow 0.3s;
  }

  .terminal-pane.focused {
    border-color: var(--pane-border-focused);
  }

  /* Green glow — Claude finished */
  .terminal-pane.activity-done {
    border-color: #22c55e;
    box-shadow: 0 0 12px rgba(34, 197, 94, 0.5), inset 0 0 4px rgba(34, 197, 94, 0.1);
  }

  /* Red blink — user interaction needed */
  .terminal-pane.activity-needs-input {
    border-color: #ef4444;
    box-shadow: 0 0 14px rgba(239, 68, 68, 0.6), inset 0 0 4px rgba(239, 68, 68, 0.1);
    animation: red-pulse 1.2s ease-in-out infinite;
  }

  @keyframes red-pulse {
    0%, 100% {
      border-color: #ef4444;
      box-shadow: 0 0 14px rgba(239, 68, 68, 0.6), inset 0 0 4px rgba(239, 68, 68, 0.1);
    }
    50% {
      border-color: #dc2626;
      box-shadow: 0 0 24px rgba(239, 68, 68, 0.9), inset 0 0 8px rgba(239, 68, 68, 0.2);
    }
  }

  .pane-titlebar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 8px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    height: 30px;
    min-height: 30px;
    transition: background 0.3s;
  }

  .titlebar-done {
    background: rgba(34, 197, 94, 0.12);
  }

  .titlebar-needs-input {
    background: rgba(239, 68, 68, 0.12);
    animation: titlebar-blink 1.2s ease-in-out infinite;
  }

  @keyframes titlebar-blink {
    0%, 100% { background: rgba(239, 68, 68, 0.12); }
    50% { background: rgba(239, 68, 68, 0.25); }
  }

  .pane-title-left {
    display: flex;
    align-items: center;
    gap: 6px;
    overflow: hidden;
  }

  .pane-title-right {
    display: flex;
    align-items: center;
    gap: 4px;
    flex-shrink: 0;
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
    transition: background 0.3s;
  }

  .dot-idle { background: var(--fg-muted); }
  .dot-active { background: var(--accent); animation: dot-spin 1s linear infinite; }
  .dot-done { background: #22c55e; box-shadow: 0 0 6px rgba(34, 197, 94, 0.8); }
  .dot-needs-input { background: #ef4444; animation: dot-blink 0.8s ease-in-out infinite; }

  @keyframes dot-spin {
    0% { opacity: 0.5; }
    50% { opacity: 1; }
    100% { opacity: 0.5; }
  }

  @keyframes dot-blink {
    0%, 100% { opacity: 1; box-shadow: 0 0 6px rgba(239, 68, 68, 0.8); }
    50% { opacity: 0.3; box-shadow: none; }
  }

  .pane-name {
    font-size: 12px;
    color: var(--fg);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    cursor: default;
  }

  .rename-input {
    font-size: 12px;
    color: var(--fg);
    background: var(--bg-tertiary);
    border: 1px solid var(--accent);
    border-radius: 3px;
    padding: 1px 4px;
    outline: none;
    width: 120px;
  }

  .mode-badge {
    font-size: 10px;
    padding: 1px 6px;
    border-radius: 4px;
    white-space: nowrap;
  }

  .badge-shell { background: var(--bg-tertiary); color: var(--fg-muted); }
  .badge-claude { background: #7c3aed33; color: #a78bfa; }
  .badge-yolo { background: #dc262633; color: #f87171; }

  .model-label { font-size: 10px; color: var(--fg-muted); }

  .cost-label { font-size: 11px; color: var(--warning); font-weight: 500; }

  .pane-btn {
    background: none;
    border: none;
    color: var(--fg-muted);
    cursor: pointer;
    padding: 2px 4px;
    font-size: 14px;
    line-height: 1;
    border-radius: 3px;
  }

  .pane-btn:hover { background: var(--bg-tertiary); color: var(--fg); }
  .pane-btn.close:hover { background: var(--error); color: white; }

  .queue-toggle { position: relative; font-size: 10px; }
  .queue-toggle.queue-active { color: var(--accent); }
  .queue-badge {
    position: absolute;
    top: -4px;
    right: -4px;
    background: var(--accent);
    color: var(--bg);
    font-size: 9px;
    font-weight: 700;
    min-width: 14px;
    height: 14px;
    line-height: 14px;
    text-align: center;
    border-radius: 7px;
    padding: 0 3px;
  }

  .terminal-container {
    flex: 1;
    padding: 4px;
    overflow: hidden;
  }

  .terminal-container :global(.xterm) {
    height: 100%;
  }
</style>
