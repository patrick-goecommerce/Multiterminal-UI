<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { createTerminal, getTerminalTheme } from '../lib/terminal';
  import { pasteToSession, copySelection, writeTextToSession } from '../lib/clipboard';
  import { sendNotification } from '../lib/notifications';
  import { playBell, audioMuted } from '../lib/audio';
  import type { Pane } from '../stores/tabs';
  import { currentTheme } from '../stores/theme';
  import { config } from '../stores/config';
  import * as App from '../../wailsjs/go/backend/App';
  import { EventsOn, BrowserOpenURL } from '../../wailsjs/runtime/runtime';
  import { isUrl, LOCALHOST_REGEX } from '../lib/links';
  import QueuePanel from './QueuePanel.svelte';
  import PaneTitlebar from './PaneTitlebar.svelte';
  import TerminalSearch from './TerminalSearch.svelte';
  import ContextMenu from './ContextMenu.svelte';

  export let pane: Pane;
  export let paneIndex: number = 0;

  const dispatch = createEventDispatcher();

  let containerEl: HTMLDivElement;
  let termInstance: ReturnType<typeof createTerminal> | null = null;
  let resizeObserver: ResizeObserver | null = null;
  let cleanupFn: (() => void) | null = null;

  let zoomTimer: ReturnType<typeof setTimeout> | null = null;
  let resizeTimer: ReturnType<typeof setTimeout> | null = null;
  let isZooming = false;
  let showQueue = false;
  let queueCount = 0;
  let queueCleanup: (() => void) | null = null;
  let showSearch = false;
  let searchRef: TerminalSearch;
  let ctxMenuVisible = false;
  let ctxMenuX = 0;
  let ctxMenuY = 0;
  let ctxHasSelection = false;
  let wheelHandler: ((e: WheelEvent) => void) | null = null;
  const seenLocalhostUrls = new Set<string>();

  function handleLink(_event: MouseEvent, uri: string) {
    if (isUrl(uri)) {
      BrowserOpenURL(uri);
    } else {
      // Strip :line:col suffix so the sidebar gets a clean file path
      const path = uri.replace(/:\d+(:\d+)?$/, '');
      dispatch('navigateFile', { path });
    }
  }

  function openSearch() {
    showSearch = true;
    requestAnimationFrame(() => searchRef?.open());
  }

  function closeSearch() {
    showSearch = false;
    termInstance?.terminal.focus();
  }

  function handleContextMenu(e: MouseEvent) {
    e.preventDefault();
    ctxMenuX = e.clientX;
    ctxMenuY = e.clientY;
    ctxHasSelection = termInstance?.terminal.hasSelection() ?? false;
    ctxMenuVisible = true;
  }

  function closeContextMenu() {
    ctxMenuVisible = false;
  }

  function handleContextAction(e: CustomEvent<{ action: string }>) {
    const { action } = e.detail;
    ctxMenuVisible = false;

    if (!termInstance) return;

    switch (action) {
      case 'copy':
        copySelection(termInstance.terminal);
        break;
      case 'paste':
        pasteToSession(pane.sessionId);
        break;
      case 'selectAll':
        termInstance.terminal.selectAll();
        break;
      case 'search':
        openSearch();
        break;
      case 'clear':
        termInstance.terminal.clear();
        break;
      case 'splitPane':
        dispatch('splitPane');
        break;
    }

    termInstance.terminal.focus();
  }

  onMount(() => {
    termInstance = createTerminal($currentTheme, handleLink);
    termInstance.terminal.open(containerEl);

    requestAnimationFrame(() => {
      termInstance?.fitAddon.fit();
      const dims = termInstance?.fitAddon.proposeDimensions();
      if (dims) App.ResizeSession(pane.sessionId, dims.rows, dims.cols);
    });

    termInstance.terminal.attachCustomKeyEventHandler((e: KeyboardEvent) => {
      if (e.type !== 'keydown') return true;
      if (e.ctrlKey && e.key === 'v') {
        e.preventDefault();
        pasteToSession(pane.sessionId);
        return false;
      }
      if (e.ctrlKey && e.key === 'c' && termInstance?.terminal.hasSelection()) {
        copySelection(termInstance.terminal);
        return false;
      }
      if (e.ctrlKey && e.key === 'f') { openSearch(); return false; }
      if (e.ctrlKey && ['z', 'n', 't', 'w', 'b'].includes(e.key)) return false;
      if (e.ctrlKey && e.key >= '1' && e.key <= '9') return false;
      return true;
    });

    termInstance.terminal.onData((data: string) => {
      const encoder = new TextEncoder();
      const bytes = encoder.encode(data);
      App.WriteToSession(pane.sessionId, btoa(String.fromCharCode(...bytes)));
    });

    // Batch PTY output writes per animation frame to avoid cursor flicker.
    // Claude Code (and other TUIs) rewrite status lines in multiple steps;
    // without batching, xterm.js renders each intermediate cursor position.
    let pendingChunks: Uint8Array[] = [];
    let rafPending = false;

    cleanupFn = EventsOn('terminal:output', (id: number, b64: string) => {
      if (id !== pane.sessionId || !termInstance) return;
      const raw = atob(b64);
      const bytes = new Uint8Array(raw.length);
      for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);

      // Scan for localhost URLs
      const mode = $config.localhost_auto_open;
      if (mode !== 'off') {
        const decoded = new TextDecoder().decode(bytes);
        LOCALHOST_REGEX.lastIndex = 0;
        let urlMatch;
        while ((urlMatch = LOCALHOST_REGEX.exec(decoded)) !== null) {
          const url = urlMatch[0];
          if (!seenLocalhostUrls.has(url)) {
            seenLocalhostUrls.add(url);
            if (mode === 'auto') {
              BrowserOpenURL(url);
              sendNotification('Dev Server', url + ' geöffnet');
            } else {
              sendNotification('Dev Server', url + ' erkannt');
            }
          }
        }
      }

      pendingChunks.push(bytes);

      if (!rafPending) {
        rafPending = true;
        requestAnimationFrame(() => {
          if (termInstance && pendingChunks.length > 0) {
            // Merge all pending chunks into a single write
            const total = pendingChunks.reduce((sum, c) => sum + c.length, 0);
            const merged = new Uint8Array(total);
            let offset = 0;
            for (const chunk of pendingChunks) {
              merged.set(chunk, offset);
              offset += chunk.length;
            }
            termInstance.terminal.write(merged);
          }
          pendingChunks = [];
          rafPending = false;
        });
      }
    });

    wheelHandler = (e: WheelEvent) => {
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
            if (dims) App.ResizeSession(pane.sessionId, dims.rows, dims.cols);
          }
          isZooming = false;
        }, 150);
      }
    };
    // Prevent native paste – we handle Ctrl+V manually via attachCustomKeyEventHandler
    // to use the Wails clipboard API. Without this, the browser paste event also fires,
    // causing double-paste through xterm.js onData.
    containerEl.addEventListener('paste', (e) => e.preventDefault(), true);

    containerEl.addEventListener('wheel', wheelHandler, { passive: false });

    resizeObserver = new ResizeObserver(() => {
      if (!termInstance || isZooming) return;
      if (resizeTimer) clearTimeout(resizeTimer);
      resizeTimer = setTimeout(() => {
        if (termInstance) {
          termInstance.fitAddon.fit();
          const dims = termInstance.fitAddon.proposeDimensions();
          if (dims) App.ResizeSession(pane.sessionId, dims.rows, dims.cols);
        }
      }, 100);
    });
    resizeObserver.observe(containerEl);

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
    if (wheelHandler && containerEl) containerEl.removeEventListener('wheel', wheelHandler);
    resizeObserver?.disconnect();
    termInstance?.dispose();
  });

  $: if (termInstance && $currentTheme) {
    const theme = getTerminalTheme($currentTheme);
    if ($config.terminal_color) {
      theme.cursor = $config.terminal_color;
      // Use contrast color so the character inside the block cursor is readable
      const rgb = parseInt($config.terminal_color.slice(1), 16);
      const brightness = ((rgb >> 16) & 0xff) * 0.299 +
                         ((rgb >> 8) & 0xff) * 0.587 +
                         (rgb & 0xff) * 0.114;
      theme.cursorAccent = brightness > 128 ? '#000000' : '#ffffff';
    }
    termInstance.terminal.options.theme = theme;
  }

  // Desktop notifications when Claude state changes and window is not focused
  let dropHighlight = false;

  function handleDragOver(e: DragEvent) {
    if (!e.dataTransfer) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = 'copy';
    dropHighlight = true;
  }

  function handleDragLeave() {
    dropHighlight = false;
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault();
    dropHighlight = false;
    if (!e.dataTransfer) return;
    const text = e.dataTransfer.getData('text/plain');
    if (text) writeTextToSession(pane.sessionId, text);
  }

  let lastNotifiedActivity = '';
  $: if (pane.activity !== lastNotifiedActivity) {
    const prev = lastNotifiedActivity;
    lastNotifiedActivity = pane.activity;
    if (pane.mode === 'claude' || pane.mode === 'claude-yolo') {
      const audio = $config.audio;
      const shouldPlayAudio = audio.enabled && !$audioMuted &&
        (audio.when_focused || !document.hasFocus());

      if (pane.activity === 'done' && prev === 'active') {
        if (!document.hasFocus()) {
          sendNotification(`${pane.name} - Fertig`, 'Claude ist fertig. Prompt bereit.');
        }
        if (shouldPlayAudio) playBell('done', audio.volume, audio.done_sound || undefined);
      } else if (pane.activity === 'needsInput') {
        if (!document.hasFocus()) {
          sendNotification(`${pane.name} - Eingabe nötig`, 'Claude wartet auf Bestätigung.');
        }
        if (shouldPlayAudio) playBell('needsInput', audio.volume, audio.input_sound || undefined);
      }
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
  class:drop-target={dropHighlight}
  on:click={() => dispatch('focus', { paneId: pane.id })}
  on:dragover={handleDragOver}
  on:dragleave={handleDragLeave}
  on:drop={handleDrop}
>
  <PaneTitlebar
    {pane}
    {paneIndex}
    {queueCount}
    on:close
    on:maximize
    on:rename
    on:restart={() => dispatch('restart', { paneId: pane.id, sessionId: pane.sessionId, mode: pane.mode, model: pane.model, name: pane.name })}
    on:toggleQueue={() => (showQueue = !showQueue)}
    on:issueAction
  />
  <QueuePanel sessionId={pane.sessionId} visible={showQueue} />
  {#if showSearch}
    <TerminalSearch
      bind:this={searchRef}
      searchAddon={termInstance?.searchAddon ?? null}
      on:close={closeSearch}
    />
  {/if}
  <div class="terminal-container" bind:this={containerEl} on:contextmenu={handleContextMenu}></div>
  {#if !pane.running}
    <div class="exited-overlay">
      <div class="exited-msg">Prozess beendet</div>
      <button class="restart-btn" on:click|stopPropagation={() => dispatch('restart', { paneId: pane.id, sessionId: pane.sessionId, mode: pane.mode, model: pane.model, name: pane.name })}>Neu starten</button>
      <button class="close-btn-overlay" on:click|stopPropagation={() => dispatch('close', { paneId: pane.id, sessionId: pane.sessionId })}>Schließen</button>
    </div>
  {/if}
  <ContextMenu
    visible={ctxMenuVisible}
    x={ctxMenuX}
    y={ctxMenuY}
    hasSelection={ctxHasSelection}
    on:action={handleContextAction}
    on:close={closeContextMenu}
  />
</div>

<style>
  .terminal-pane {
    position: relative; display: flex; flex-direction: column;
    background: var(--pane-bg); border: 2px solid var(--pane-border);
    border-radius: 8px; overflow: hidden;
    transition: border-color 0.3s, box-shadow 0.3s;
  }

  .terminal-pane.focused { border-color: var(--pane-border-focused); }

  .terminal-pane.drop-target {
    border-color: var(--accent);
    box-shadow: 0 0 12px rgba(203, 166, 247, 0.4), inset 0 0 4px rgba(203, 166, 247, 0.1);
  }

  .terminal-pane.activity-done {
    border-color: #22c55e;
    box-shadow: 0 0 12px rgba(34, 197, 94, 0.5), inset 0 0 4px rgba(34, 197, 94, 0.1);
  }

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

  .terminal-container { flex: 1; padding: 4px; overflow: hidden; }
  .terminal-container :global(.xterm) { height: 100%; }

  .exited-overlay {
    position: absolute; inset: 30px 0 0 0;
    background: rgba(0, 0, 0, 0.75);
    display: flex; flex-direction: column;
    align-items: center; justify-content: center;
    gap: 10px; z-index: 10;
  }

  .exited-msg { color: var(--fg-muted); font-size: 14px; font-weight: 600; }

  .restart-btn {
    background: var(--accent); color: var(--bg); border: none;
    padding: 6px 20px; border-radius: 5px; cursor: pointer;
    font-size: 13px; font-weight: 600;
  }

  .restart-btn:hover { filter: brightness(1.2); }

  .close-btn-overlay {
    background: none; border: 1px solid var(--fg-muted);
    color: var(--fg-muted); padding: 4px 16px;
    border-radius: 5px; cursor: pointer; font-size: 12px;
  }

  .close-btn-overlay:hover { border-color: var(--fg); color: var(--fg); }
</style>
