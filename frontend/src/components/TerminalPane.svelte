<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { createTerminal, getTerminalTheme, buildFontFamily } from '../lib/terminal';
  import { pasteToSession, copySelection, writeTextToSession } from '../lib/clipboard';
  import { encodeForPty } from '../lib/claude';
  import { sendNotification } from '../lib/notifications';
  import { playBell, audioMuted } from '../lib/audio';
  import { tabStore, type Pane } from '../stores/tabs';
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
  export let active: boolean = true;
  export let tabId: string = '';

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
    termInstance = createTerminal($currentTheme, handleLink, $config.font_family, ($config.font_size || 10) + (pane.zoomDelta || 0));
    termInstance.terminal.open(containerEl);

    requestAnimationFrame(() => {
      termInstance?.fitAddon.fit();
      const dims = termInstance?.fitAddon.proposeDimensions();
      if (dims) App.ResizeSession(pane.sessionId, dims.rows, dims.cols);
      // Give the shell time to process the resize before showing output.
      // This prevents cursor-hopping from the initial 24x80 → real size transition.
      setTimeout(() => {
        isReady = true;
        if (pendingChunks.length > 0) {
          scheduleFlush();
        }
      }, 50);
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
      App.WriteToSession(pane.sessionId, encodeForPty(data));
    });

    // Batch PTY output writes with a short time window to reduce render overhead.
    // We accumulate chunks then flush them in a single xterm.js write call.
    // xterm.js handles frame-synced rendering internally (via its own rAF loop),
    // so we do NOT wrap in requestAnimationFrame — that would add an extra frame
    // of latency and cause overlapping flush cycles that produce flicker.
    // Output is buffered until xterm.js has been fitted to the actual pane size
    // to avoid cursor-hopping from 24x80 → real size.
    let pendingChunks: Uint8Array[] = [];
    let flushScheduled = false;
    let isReady = false;
    let appCursorVisible = true; // track DECTCEM state across batches
    const FLUSH_DELAY = 16; // ms — one full frame at 60fps
    const HIDE_CURSOR = new Uint8Array([0x1b, 0x5b, 0x3f, 0x32, 0x35, 0x6c]); // \x1b[?25l
    const SHOW_CURSOR = new Uint8Array([0x1b, 0x5b, 0x3f, 0x32, 0x35, 0x68]); // \x1b[?25h

    // Find the last DECTCEM cursor show/hide in the data.
    // Returns true (show), false (hide), or null (no sequence found).
    function lastCursorVisible(data: Uint8Array): boolean | null {
      for (let i = data.length - 6; i >= 0; i--) {
        if (data[i] === 0x1b && data[i+1] === 0x5b && data[i+2] === 0x3f &&
            data[i+3] === 0x32 && data[i+4] === 0x35) {
          if (data[i+5] === 0x68) return true;  // \x1b[?25h — show
          if (data[i+5] === 0x6c) return false; // \x1b[?25l — hide
        }
      }
      return null;
    }

    function flushOutput() {
      if (!termInstance || !isReady || pendingChunks.length === 0) {
        flushScheduled = false;
        return;
      }
      const chunks = pendingChunks;
      pendingChunks = [];

      const total = chunks.reduce((sum, c) => sum + c.length, 0);
      const merged = new Uint8Array(total);
      let offset = 0;
      for (const chunk of chunks) {
        merged.set(chunk, offset);
        offset += chunk.length;
      }
      // Update persistent cursor state if this batch contains a DECTCEM sequence.
      // If not, the previous state carries over — this prevents wrongly showing
      // the cursor when the app hid it in an earlier batch.
      const batchState = lastCursorVisible(merged);
      if (batchState !== null) appCursorVisible = batchState;
      const suffix = appCursorVisible ? SHOW_CURSOR : HIDE_CURSOR;
      // Wrap: hide cursor → data → restore app's intended state
      const buf = new Uint8Array(HIDE_CURSOR.length + total + suffix.length);
      buf.set(HIDE_CURSOR, 0);
      buf.set(merged, HIDE_CURSOR.length);
      buf.set(suffix, HIDE_CURSOR.length + total);
      termInstance.terminal.write(buf);

      // Check if more data arrived while we were processing.
      // Schedule another flush if so, otherwise release the flag.
      if (pendingChunks.length > 0) {
        setTimeout(flushOutput, FLUSH_DELAY);
      } else {
        flushScheduled = false;
      }
    }

    function scheduleFlush() {
      if (!isReady || flushScheduled) return;
      flushScheduled = true;
      setTimeout(flushOutput, FLUSH_DELAY);
    }

    // Pre-built lookup table for fast base64 decoding (avoids intermediate string from atob)
    const B64 = new Uint8Array(128);
    'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'.split('').forEach((c, i) => B64[c.charCodeAt(0)] = i);

    function decodeBase64(b64: string): Uint8Array {
      // Strip trailing padding
      let len = b64.length;
      while (len > 0 && b64[len - 1] === '=') len--;
      const outLen = (len * 3 >>> 2);
      const out = new Uint8Array(outLen);
      let j = 0;
      for (let i = 0; i < len; i += 4) {
        const a = B64[b64.charCodeAt(i)];
        const b = B64[b64.charCodeAt(i + 1)];
        const c = i + 2 < len ? B64[b64.charCodeAt(i + 2)] : 0;
        const d = i + 3 < len ? B64[b64.charCodeAt(i + 3)] : 0;
        out[j++] = (a << 2) | (b >> 4);
        if (j < outLen) out[j++] = ((b & 0xF) << 4) | (c >> 2);
        if (j < outLen) out[j++] = ((c & 0x3) << 6) | d;
      }
      return out;
    }

    cleanupFn = EventsOn('terminal:output', (id: number, b64: string) => {
      if (id !== pane.sessionId || !termInstance) return;
      const bytes = decodeBase64(b64);

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
      scheduleFlush();
    });

    wheelHandler = (e: WheelEvent) => {
      if (!e.ctrlKey || !termInstance) return;
      e.preventDefault();
      const baseSize = $config.font_size || 10;
      const currentDelta = pane.zoomDelta || 0;
      const newDelta = e.deltaY < 0 ? currentDelta + 1 : currentDelta - 1;
      const effectiveSize = baseSize + newDelta;
      if (effectiveSize >= 8 && effectiveSize <= 32) {
        isZooming = true;
        termInstance.terminal.options.fontSize = effectiveSize;
        if (tabId) tabStore.setZoomDelta(tabId, pane.id, newDelta);
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

  $: if (termInstance && $config) {
    const effectiveSize = ($config.font_size || 10) + (pane.zoomDelta || 0);
    const clampedSize = Math.max(8, Math.min(32, effectiveSize));
    const newFamily = buildFontFamily($config.font_family);

    let needsFit = false;
    if (termInstance.terminal.options.fontFamily !== newFamily) {
      termInstance.terminal.options.fontFamily = newFamily;
      needsFit = true;
    }
    if (termInstance.terminal.options.fontSize !== clampedSize) {
      termInstance.terminal.options.fontSize = clampedSize;
      needsFit = true;
    }
    if (needsFit) {
      termInstance.fitAddon.fit();
      const dims = termInstance.fitAddon.proposeDimensions();
      if (dims) App.ResizeSession(pane.sessionId, dims.rows, dims.cols);
    }
  }

  // Re-focus terminal when its tab becomes active or pane gets focused.
  // Guard: skip if an interactive element (input, textarea, select) already
  // has focus — prevents stealing focus from sidebar search, pane rename,
  // terminal search, queue panel, etc.
  $: if (active && pane.focused && termInstance) {
    const ae = document.activeElement;
    const isInteractive = ae instanceof HTMLInputElement ||
                          ae instanceof HTMLTextAreaElement ||
                          ae instanceof HTMLSelectElement ||
                          ae instanceof HTMLButtonElement;
    if (!isInteractive) {
      termInstance.terminal.focus();
    }
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
  let needsInputAlerted = false;
  $: if (pane.activity !== lastNotifiedActivity) {
    const prev = lastNotifiedActivity;
    lastNotifiedActivity = pane.activity;
    // Reset alert flag when Claude finishes real work — allows next needsInput to fire
    if (pane.activity === 'done') needsInputAlerted = false;
    if (pane.mode === 'claude' || pane.mode === 'claude-yolo') {
      const audio = $config.audio;
      const shouldPlayAudio = audio.enabled && !$audioMuted &&
        (audio.when_focused || !document.hasFocus());

      if (pane.activity === 'done' && prev === 'active') {
        if (!document.hasFocus()) {
          sendNotification(`${pane.name} - Fertig`, 'Claude ist fertig. Prompt bereit.');
        }
        if (shouldPlayAudio) playBell('done', audio.volume, audio.done_sound || undefined);
      } else if (pane.activity === 'needsInput' && !needsInputAlerted) {
        needsInputAlerted = true;
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
  on:mousedown={() => dispatch('focus', { paneId: pane.id })}
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
  .terminal-container :global(.xterm-helper-textarea) {
    caret-color: transparent !important;
    color: transparent !important;
    opacity: 0 !important;
  }
  /* Force visible scrollbar in WebView2 — overlay scrollbars auto-hide on Windows 11 */
  .terminal-container :global(.xterm-viewport)::-webkit-scrollbar {
    width: 10px;
    background: transparent;
  }
  .terminal-container :global(.xterm-viewport)::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.2);
    border-radius: 5px;
  }
  .terminal-container :global(.xterm-viewport)::-webkit-scrollbar-thumb:hover {
    background: rgba(255, 255, 255, 0.35);
  }

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
