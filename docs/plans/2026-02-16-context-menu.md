# Context Menu (Rechtsklick) — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rechtsklick-Kontextmenü im Terminal mit Copy, Paste, Select All, Search, Clear und Split Pane.

**Architecture:** Eine neue `ContextMenu.svelte` Komponente wird erstellt und in `TerminalPane.svelte` integriert. Das xterm.js `contextmenu` DOM-Event wird abgefangen, das native Menü unterdrückt und stattdessen das Custom-Menü an der Mausposition geöffnet. Alle Aktionen nutzen bestehende APIs (Clipboard, xterm.js `selectAll()`/`clear()`, `openSearch()`, dispatch `'focus'` + LaunchDialog).

**Tech Stack:** Svelte 4, xterm.js, Wails Runtime (ClipboardGetText/ClipboardSetText)

---

### Task 1: ContextMenu.svelte Komponente erstellen

**Files:**
- Create: `frontend/src/components/ContextMenu.svelte`

**Step 1: Erstelle die ContextMenu Komponente**

```svelte
<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';

  export let x: number = 0;
  export let y: number = 0;
  export let visible: boolean = false;
  export let hasSelection: boolean = false;

  const dispatch = createEventDispatcher();

  let menuEl: HTMLDivElement;

  function handleAction(action: string) {
    dispatch('action', { action });
  }

  function handleClickOutside(e: MouseEvent) {
    if (menuEl && !menuEl.contains(e.target as Node)) {
      dispatch('close');
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('close');
  }

  onMount(() => {
    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleKeydown);
  });

  onDestroy(() => {
    document.removeEventListener('mousedown', handleClickOutside);
    document.removeEventListener('keydown', handleKeydown);
  });

  // Clamp position so menu doesn't overflow viewport
  $: style = (() => {
    const menuW = 180;
    const menuH = 240;
    const clampedX = Math.min(x, window.innerWidth - menuW);
    const clampedY = Math.min(y, window.innerHeight - menuH);
    return `left: ${clampedX}px; top: ${clampedY}px;`;
  })();
</script>

{#if visible}
  <div class="context-menu" bind:this={menuEl} style={style}>
    <button class="ctx-item" class:disabled={!hasSelection} on:click={() => handleAction('copy')} disabled={!hasSelection}>
      <span class="ctx-icon">⎘</span> Kopieren <span class="ctx-shortcut">Ctrl+C</span>
    </button>
    <button class="ctx-item" on:click={() => handleAction('paste')}>
      <span class="ctx-icon">⎗</span> Einfügen <span class="ctx-shortcut">Ctrl+V</span>
    </button>
    <div class="ctx-separator"></div>
    <button class="ctx-item" on:click={() => handleAction('selectAll')}>
      <span class="ctx-icon">☐</span> Alles auswählen
    </button>
    <button class="ctx-item" on:click={() => handleAction('search')}>
      <span class="ctx-icon">⌕</span> Suchen <span class="ctx-shortcut">Ctrl+F</span>
    </button>
    <div class="ctx-separator"></div>
    <button class="ctx-item" on:click={() => handleAction('clear')}>
      <span class="ctx-icon">⌧</span> Terminal leeren
    </button>
    <button class="ctx-item" on:click={() => handleAction('splitPane')}>
      <span class="ctx-icon">⊞</span> Neues Terminal <span class="ctx-shortcut">Ctrl+N</span>
    </button>
  </div>
{/if}

<style>
  .context-menu {
    position: fixed;
    z-index: 1000;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
    padding: 4px 0;
    min-width: 180px;
    animation: ctx-fade-in 0.1s ease-out;
  }

  @keyframes ctx-fade-in {
    from { opacity: 0; transform: scale(0.95); }
    to { opacity: 1; transform: scale(1); }
  }

  .ctx-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 6px 12px;
    background: none;
    border: none;
    color: var(--fg);
    font-size: 12px;
    cursor: pointer;
    text-align: left;
    transition: background 0.1s;
  }

  .ctx-item:hover:not(:disabled) {
    background: var(--bg-tertiary);
  }

  .ctx-item:disabled {
    color: var(--fg-muted);
    cursor: default;
    opacity: 0.5;
  }

  .ctx-icon {
    width: 16px;
    text-align: center;
    font-size: 13px;
    flex-shrink: 0;
  }

  .ctx-shortcut {
    margin-left: auto;
    color: var(--fg-muted);
    font-size: 10px;
  }

  .ctx-separator {
    height: 1px;
    background: var(--border);
    margin: 4px 8px;
  }
</style>
```

**Step 2: Commit**

```bash
git add frontend/src/components/ContextMenu.svelte
git commit -m "feat: add ContextMenu component for terminal right-click menu (#26)"
```

---

### Task 2: ContextMenu in TerminalPane integrieren

**Files:**
- Modify: `frontend/src/components/TerminalPane.svelte`

**Step 1: Import und State hinzufügen**

Am Anfang des `<script>` Blocks:
- Import `ContextMenu` Komponente
- Import `ClipboardGetText`, `ClipboardSetText` sind bereits vorhanden
- Neue State-Variablen: `ctxMenuVisible`, `ctxMenuX`, `ctxMenuY`, `ctxHasSelection`

```ts
import ContextMenu from './ContextMenu.svelte';

let ctxMenuVisible = false;
let ctxMenuX = 0;
let ctxMenuY = 0;
let ctxHasSelection = false;
```

**Step 2: contextmenu Event-Handler hinzufügen**

Neue Funktion im `<script>` Block:

```ts
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
      if (termInstance.terminal.hasSelection()) {
        ClipboardSetText(termInstance.terminal.getSelection());
        termInstance.terminal.clearSelection();
      }
      break;
    case 'paste':
      ClipboardGetText().then((text) => {
        if (text) {
          const encoder = new TextEncoder();
          const bytes = encoder.encode(text);
          let binary = '';
          for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
          App.WriteToSession(pane.sessionId, btoa(binary));
        }
      }).catch((err) => console.error('[ContextMenu] paste failed:', err));
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
```

**Step 3: Event im Template binden**

Am `<div class="terminal-container">` das `contextmenu` Event hinzufügen:

```svelte
<div class="terminal-container" bind:this={containerEl} on:contextmenu={handleContextMenu}></div>
```

**Step 4: ContextMenu Komponente im Template einfügen**

Direkt vor `</div>` (schließendes Tag von `.terminal-pane`):

```svelte
<ContextMenu
  visible={ctxMenuVisible}
  x={ctxMenuX}
  y={ctxMenuY}
  hasSelection={ctxHasSelection}
  on:action={handleContextAction}
  on:close={closeContextMenu}
/>
```

**Step 5: Commit**

```bash
git add frontend/src/components/TerminalPane.svelte
git commit -m "feat: integrate context menu into terminal panes (#26)"
```

---

### Task 3: splitPane Event durch PaneGrid und App.svelte leiten

**Files:**
- Modify: `frontend/src/components/PaneGrid.svelte`
- Modify: `frontend/src/App.svelte`

**Step 1: PaneGrid.svelte — splitPane Event forwarden**

Neue Handler-Funktion hinzufügen:

```ts
function handleSplitPane(e: CustomEvent) {
  dispatch('splitPane', e.detail);
}
```

Am `<TerminalPane>` das Event binden:

```svelte
on:splitPane={handleSplitPane}
```

**Step 2: App.svelte — splitPane Event verarbeiten**

Am `<PaneGrid>` das Event binden:

```svelte
on:splitPane={() => (showLaunchDialog = true)}
```

**Step 3: Commit**

```bash
git add frontend/src/components/PaneGrid.svelte frontend/src/App.svelte
git commit -m "feat: wire splitPane event from context menu to launch dialog (#26)"
```

---

### Task 4: Manueller Test & Feinschliff

**Step 1: Build und Test**

```bash
cd D:\repos\Multiterminal && wails dev
```

Manuell testen:
- [ ] Rechtsklick im Terminal → Kontextmenü erscheint
- [ ] "Kopieren" ist ausgegraut wenn keine Selektion
- [ ] Text selektieren → Rechtsklick → "Kopieren" → funktioniert
- [ ] "Einfügen" fügt Clipboard-Inhalt ins Terminal ein
- [ ] "Alles auswählen" selektiert den gesamten Buffer
- [ ] "Suchen" öffnet die Suchleiste
- [ ] "Terminal leeren" leert den Scrollback
- [ ] "Neues Terminal" öffnet LaunchDialog
- [ ] Klick außerhalb schließt das Menü
- [ ] Escape schließt das Menü
- [ ] Menü bleibt im Viewport (kein Overflow)

**Step 2: Finaler Commit (falls Anpassungen nötig)**

```bash
git add -A
git commit -m "fix: polish context menu positioning and behavior (#26)"
```
