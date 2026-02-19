# F-016: Font Selection — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add font family dropdown, font size setting, and zoom persistence to Settings dialog.

**Architecture:** Extend the existing Config struct (Go + TS) with `font_family` and `font_size` fields. Add a curated monospace font list in the frontend with availability checking. TerminalPane reactively watches config changes and applies fonts live. Zoom delta is persisted per pane in the session JSON.

**Tech Stack:** Go (config), Svelte 4 (UI), xterm.js (terminal), TypeScript (stores/lib)

---

### Task 1: Go Config — Add font fields

**Files:**
- Modify: `internal/config/config.go:15-34` (Config struct)
- Modify: `internal/config/config.go:71-107` (DefaultConfig)
- Modify: `internal/config/config.go:142-208` (Load validation)

**Step 1: Add fields to Config struct**

In `internal/config/config.go`, add after `SidebarPinned` (line 33):

```go
FontFamily string `yaml:"font_family" json:"font_family"`
FontSize   int    `yaml:"font_size"   json:"font_size"`
```

**Step 2: Set defaults in DefaultConfig()**

In `DefaultConfig()`, add inside the return struct (after `LocalhostAutoOpen: "notify"`):

```go
FontFamily: "",
FontSize:   14,
```

**Step 3: Add validation in Load()**

After the `LocalhostAutoOpen` validation block (around line 205), add:

```go
if cfg.FontSize < 8 {
    cfg.FontSize = 8
}
if cfg.FontSize > 32 {
    cfg.FontSize = 32
}
```

**Step 4: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add font_family and font_size fields"
```

---

### Task 2: Go Session — Add ZoomDelta to SavedPane

**Files:**
- Modify: `internal/config/session.go:28-34` (SavedPane struct)

**Step 1: Add ZoomDelta field**

In `SavedPane` struct, add after `IssueBranch`:

```go
ZoomDelta int `json:"zoom_delta,omitempty"`
```

**Step 2: Commit**

```bash
git add internal/config/session.go
git commit -m "feat(session): add zoom_delta to SavedPane for font zoom persistence"
```

---

### Task 3: TS Config Store — Mirror new fields

**Files:**
- Modify: `frontend/src/stores/config.ts:22-39` (AppConfig interface)
- Modify: `frontend/src/stores/config.ts:41-69` (store defaults)

**Step 1: Add fields to AppConfig interface**

After `sidebar_pinned: boolean;` (line 38), add:

```typescript
font_family: string;
font_size: number;
```

**Step 2: Add defaults to store**

After `sidebar_pinned: false,` (line 68), add:

```typescript
font_family: '',
font_size: 14,
```

**Step 3: Commit**

```bash
git add frontend/src/stores/config.ts
git commit -m "feat(stores): add font_family and font_size to AppConfig"
```

---

### Task 4: Terminal lib — Curated font list + configurable createTerminal

**Files:**
- Modify: `frontend/src/lib/terminal.ts:14-21` (baseOptions)
- Modify: `frontend/src/lib/terminal.ts:137-166` (createTerminal)

**Step 1: Add curated font list constant**

After the imports (line 4), add:

```typescript
/** Curated list of monospace fonts. Order = priority for fallback chain. */
export const MONOSPACE_FONTS = [
  'Cascadia Code',
  'Cascadia Mono',
  'Fira Code',
  'JetBrains Mono',
  'Source Code Pro',
  'IBM Plex Mono',
  'Ubuntu Mono',
  'Hack',
  'Inconsolata',
  'Consolas',
  'Courier New',
] as const;

/** Default fallback chain used when no font is configured. */
export const DEFAULT_FONT_FAMILY = MONOSPACE_FONTS.map(f => `'${f}'`).join(', ') + ', monospace';

/**
 * Check if a font is available in the browser.
 * Uses document.fonts.check() with a test string at 16px.
 */
export function isFontAvailable(fontName: string): boolean {
  if (fontName === 'monospace') return true;
  try {
    return document.fonts.check(`16px "${fontName}"`);
  } catch {
    return false;
  }
}

/** Build a CSS font-family string from a configured font name. */
export function buildFontFamily(configuredFont: string): string {
  if (!configuredFont) return DEFAULT_FONT_FAMILY;
  return `'${configuredFont}', monospace`;
}
```

**Step 2: Update baseOptions to remove hardcoded font**

Change `baseOptions` to remove `fontSize` and `fontFamily` (they'll be set per-terminal):

```typescript
const baseOptions: Partial<import('@xterm/xterm').ITerminalOptions> = {
  cursorBlink: true,
  cursorStyle: 'block',
  scrollback: 10000,
  allowProposedApi: true,
};
```

**Step 3: Update createTerminal to accept font options**

Change the `createTerminal` signature to:

```typescript
export function createTerminal(
  theme: string = 'dark',
  linkHandler?: LinkHandler,
  fontFamily?: string,
  fontSize?: number,
): TerminalInstance {
  const terminal = new Terminal({
    ...baseOptions,
    fontFamily: buildFontFamily(fontFamily || ''),
    fontSize: fontSize || 14,
    theme: terminalThemes[theme] || terminalThemes.dark,
  });
```

The rest of the function body stays the same.

**Step 4: Commit**

```bash
git add frontend/src/lib/terminal.ts
git commit -m "feat(terminal): add curated font list and configurable font options"
```

---

### Task 5: Pane store — Add zoomDelta to Pane interface

**Files:**
- Modify: `frontend/src/stores/tabs.ts:5-20` (Pane interface)
- Modify: `frontend/src/stores/tabs.ts:110-137` (addPane method)

**Step 1: Add zoomDelta to Pane interface**

After `worktreePath: string;` (line 19), add:

```typescript
zoomDelta: number;
```

**Step 2: Initialize zoomDelta in addPane**

In the `addPane` method, add to the pushed pane object (after `worktreePath`):

```typescript
zoomDelta: 0,
```

**Step 3: Add setZoomDelta method**

After the `toggleMaximize` method, add:

```typescript
setZoomDelta(tabId: string, paneId: string, delta: number) {
  update((state) => {
    const tab = state.tabs.find((t) => t.id === tabId);
    if (!tab) return state;
    const pane = tab.panes.find((p) => p.id === paneId);
    if (pane) pane.zoomDelta = delta;
    return state;
  });
},
```

**Step 4: Commit**

```bash
git add frontend/src/stores/tabs.ts
git commit -m "feat(tabs): add zoomDelta to Pane for persistent zoom state"
```

---

### Task 6: Session persistence — Save/restore zoomDelta

**Files:**
- Modify: `frontend/src/lib/session.ts:50-67` (saveSession)
- Modify: `frontend/src/lib/session.ts:6-47` (restoreSession)

**Step 1: Include zoomDelta in saveSession**

In `saveSession()`, update the pane mapping (line 58-64) to include `zoom_delta`:

```typescript
panes: tab.panes.map((pane) => ({
  name: pane.name,
  mode: MODE_TO_INDEX[pane.mode] ?? 0,
  model: pane.model || '',
  issue_number: pane.issueNumber || 0,
  issue_branch: pane.issueBranch || '',
  zoom_delta: pane.zoomDelta || 0,
})),
```

**Step 2: Restore zoomDelta in restoreSession**

In `restoreSession()`, after `tabStore.addPane(...)` (around line 21), read zoomDelta from the saved pane. Update the `addPane` call's surrounding code:

After the `tabStore.addPane(...)` call, add:

```typescript
const zd = (savedPane as any).zoom_delta || 0;
if (zd !== 0) {
  tabStore.setZoomDelta(tabId, paneId, zd);
}
```

Note: `addPane` returns a paneId — currently it's not captured. Change line ~21 to:

```typescript
const paneId = tabStore.addPane(tabId, sessionId, savedPane.name, mode, savedPane.model || '', issueNum || null, '', issueBranch);
```

**Step 3: Commit**

```bash
git add frontend/src/lib/session.ts
git commit -m "feat(session): persist and restore per-pane zoom delta"
```

---

### Task 7: TerminalPane — Reactive font + zoom persistence

**Files:**
- Modify: `frontend/src/components/TerminalPane.svelte:106-107` (createTerminal call)
- Modify: `frontend/src/components/TerminalPane.svelte:237-255` (wheel handler)
- Modify: `frontend/src/components/TerminalPane.svelte:293-305` (reactive block)

**Step 1: Import tab store methods**

At line 7, update the import to also get `tabStore`:

```typescript
import { tabStore, type Pane } from '../stores/tabs';
```

And ensure we know the parent tabId. Add a new export prop:

```typescript
export let tabId: string = '';
```

**Step 2: Pass font config to createTerminal**

Change `onMount` terminal creation (line 107) to:

```typescript
termInstance = createTerminal($currentTheme, handleLink, $config.font_family, $config.font_size + pane.zoomDelta);
```

**Step 3: Add reactive font watcher**

After the existing theme reactive block (line 293-305), add:

```typescript
$: if (termInstance && $config) {
  const effectiveSize = ($config.font_size || 14) + (pane.zoomDelta || 0);
  const clampedSize = Math.max(8, Math.min(32, effectiveSize));
  const family = $config.font_family;

  termInstance.terminal.options.fontFamily = buildFontFamily(family);
  if (termInstance.terminal.options.fontSize !== clampedSize) {
    termInstance.terminal.options.fontSize = clampedSize;
    termInstance.fitAddon.fit();
    const dims = termInstance.fitAddon.proposeDimensions();
    if (dims) App.ResizeSession(pane.sessionId, dims.rows, dims.cols);
  }
}
```

Import `buildFontFamily` at the top:

```typescript
import { createTerminal, getTerminalTheme, buildFontFamily } from '../lib/terminal';
```

**Step 4: Update wheel handler for zoomDelta persistence**

In the wheel handler (line 237-255), replace the zoom logic to track zoomDelta:

```typescript
wheelHandler = (e: WheelEvent) => {
  if (!e.ctrlKey || !termInstance) return;
  e.preventDefault();
  const baseSize = $config.font_size || 14;
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
```

**Step 5: Commit**

```bash
git add frontend/src/components/TerminalPane.svelte
git commit -m "feat(pane): reactive font config and persistent zoom delta"
```

---

### Task 8: PaneGrid — Pass tabId to TerminalPane

**Files:**
- Modify: `frontend/src/components/PaneGrid.svelte` (where TerminalPane is rendered)

**Step 1: Find the TerminalPane usage and add tabId prop**

Search for `<TerminalPane` in PaneGrid.svelte and add `tabId={$activeTab.id}` (or equivalent) as a prop.

**Step 2: Commit**

```bash
git add frontend/src/components/PaneGrid.svelte
git commit -m "feat(grid): pass tabId to TerminalPane for zoom persistence"
```

---

### Task 9: Settings Dialog — Font section UI

**Files:**
- Modify: `frontend/src/components/SettingsDialog.svelte`

**Step 1: Add font-related local state variables**

After the audio variables (around line 40), add:

```typescript
let fontFamily = $config.font_family || '';
let fontSize = $config.font_size || 14;
let savedFontFamily = fontFamily;
let savedFontSize = fontSize;
let availableFonts: { name: string; available: boolean }[] = [];
```

**Step 2: Import font helpers and populate availability**

Add import at top of script:

```typescript
import { MONOSPACE_FONTS, isFontAvailable } from '../lib/terminal';
```

In the `$: if (visible)` reactive block, add:

```typescript
fontFamily = $config.font_family || '';
fontSize = $config.font_size || 14;
savedFontFamily = fontFamily;
savedFontSize = fontSize;
availableFonts = MONOSPACE_FONTS.map(name => ({
  name,
  available: isFontAvailable(name),
}));
```

**Step 3: Add live-apply handlers**

```typescript
function handleFontFamilyChange(e: Event) {
  fontFamily = (e.target as HTMLSelectElement).value;
  config.update(c => ({ ...c, font_family: fontFamily }));
}

function handleFontSizeChange(e: Event) {
  fontSize = parseInt((e.target as HTMLInputElement).value) || 14;
  config.update(c => ({ ...c, font_size: fontSize }));
}
```

**Step 4: Add font section HTML**

After the Terminal-Farbe setting-group (after line 192), add:

```html
<div class="setting-group">
  <label class="setting-label" for="font-select">Schriftart</label>
  <p class="setting-desc">Monospace-Schriftart für alle Terminals.</p>
  <select id="font-select" class="theme-select" value={fontFamily} on:change={handleFontFamilyChange}>
    <option value="">Standard (Cascadia Code, Fira Code, ...)</option>
    {#each availableFonts as font}
      <option value={font.name} disabled={!font.available} style={font.available ? `font-family: '${font.name}', monospace` : ''}>
        {font.name}{font.available ? '' : ' (nicht installiert)'}
      </option>
    {/each}
  </select>
</div>

<div class="setting-group">
  <label class="setting-label" for="font-size">Schriftgröße</label>
  <p class="setting-desc">Basis-Schriftgröße in Pixel (8–32). Ctrl+Scroll zum Zoomen pro Pane.</p>
  <div class="volume-row">
    <input id="font-size" type="range" min="8" max="32" step="1" bind:value={fontSize} on:input={handleFontSizeChange} class="volume-slider" />
    <span class="volume-value">{fontSize}px</span>
  </div>
</div>
```

**Step 5: Update save() to include font fields**

In `save()` (line 122-143), add to the `updated` object:

```typescript
font_family: fontFamily,
font_size: fontSize,
```

**Step 6: Update close() to revert font changes**

In `close()` (line 145-148), revert font:

```typescript
function close() {
  applyTheme(savedTheme, $config.terminal_color || '#39ff14');
  config.update(c => ({ ...c, font_family: savedFontFamily, font_size: savedFontSize }));
  dispatch('close');
}
```

**Step 7: Update resetDefault() to include font**

In `resetDefault()` (line 150-160), add:

```typescript
fontFamily = '';
fontSize = 14;
config.update(c => ({ ...c, font_family: '', font_size: 14 }));
```

**Step 8: Commit**

```bash
git add frontend/src/components/SettingsDialog.svelte
git commit -m "feat(settings): add font family dropdown and font size slider"
```

---

### Task 10: Build and manual test

**Step 1: Build the project**

```bash
export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"
cd D:/repos/Multiterminal && wails build -debug
```

**Step 2: Manual test checklist**

- [ ] Open Settings dialog — font section visible
- [ ] Font dropdown shows curated list with availability markers
- [ ] Selecting a font immediately applies to all terminals
- [ ] Font size slider works and applies live
- [ ] Cancel reverts font changes
- [ ] Save persists font to `~/.multiterminal.yaml`
- [ ] Ctrl+Wheel zoom still works
- [ ] Close and reopen app — font setting restored
- [ ] Close and reopen app — zoom delta restored per pane

**Step 3: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix: address issues found during font selection testing"
```
