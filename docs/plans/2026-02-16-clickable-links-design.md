# F-005: Clickable URLs and File Paths — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make URLs and file paths in terminal output clickable via Ctrl+Click — URLs open in browser, file paths navigate in sidebar.

**Architecture:** Single `@xterm/addon-web-links` addon with a combined regex (URLs + file paths) and custom click handler. The handler checks `event.ctrlKey`, then routes URLs to `BrowserOpenURL()` and file paths to a `navigateFile` event that bubbles up to App.svelte which opens the sidebar.

**Tech Stack:** `@xterm/addon-web-links`, Wails `BrowserOpenURL`, Svelte event dispatching

---

### Task 1: Install @xterm/addon-web-links

**Files:**
- Modify: `frontend/package.json`

**Step 1: Install the addon**

Run: `cd frontend && npm install @xterm/addon-web-links`

**Step 2: Verify installation**

Run: `cd frontend && npm ls @xterm/addon-web-links`
Expected: Shows installed version (e.g. `@xterm/addon-web-links@0.11.0`)

**Step 3: Commit**

```bash
git add frontend/package.json frontend/package-lock.json
git commit -m "feat(deps): add @xterm/addon-web-links for clickable terminal links"
```

---

### Task 2: Add WebLinksAddon to terminal.ts

**Files:**
- Modify: `frontend/src/lib/terminal.ts`

**Step 1: Write the failing test**

Create `frontend/src/lib/links.test.ts`:

```typescript
import { describe, it, expect } from 'vitest';
import { LINK_REGEX, isUrl } from './links';

describe('LINK_REGEX', () => {
  const match = (text: string) => {
    LINK_REGEX.lastIndex = 0;
    const m = LINK_REGEX.exec(text);
    return m ? m[0] : null;
  };

  it('matches https URLs', () => {
    expect(match('visit https://example.com for info')).toBe('https://example.com');
  });

  it('matches http URLs with paths', () => {
    expect(match('see http://localhost:3000/api/v1')).toBe('http://localhost:3000/api/v1');
  });

  it('matches relative file paths with extension', () => {
    expect(match('error in ./src/App.svelte')).toBe('./src/App.svelte');
  });

  it('matches file paths with line number', () => {
    expect(match('at src/utils/parse.ts:42')).toBe('src/utils/parse.ts:42');
  });

  it('matches file paths with line:col', () => {
    expect(match('error src/index.ts:10:5')).toBe('src/index.ts:10:5');
  });

  it('matches Windows paths', () => {
    expect(match('file C:\\Users\\foo\\bar.ts')).toBe('C:\\Users\\foo\\bar.ts');
  });

  it('matches parent-relative paths', () => {
    expect(match('see ../lib/helper.go:99')).toBe('../lib/helper.go:99');
  });

  it('does not match plain words', () => {
    expect(match('hello world')).toBeNull();
  });
});

describe('isUrl', () => {
  it('returns true for http', () => {
    expect(isUrl('http://example.com')).toBe(true);
  });

  it('returns true for https', () => {
    expect(isUrl('https://github.com/foo')).toBe(true);
  });

  it('returns false for file paths', () => {
    expect(isUrl('./src/foo.ts')).toBe(false);
  });

  it('returns false for relative paths', () => {
    expect(isUrl('src/foo.ts:42')).toBe(false);
  });
});
```

**Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/lib/links.test.ts`
Expected: FAIL — module `./links` not found

**Step 3: Create the links module**

Create `frontend/src/lib/links.ts`:

```typescript
import { WebLinksAddon } from '@xterm/addon-web-links';
import type { Terminal } from '@xterm/xterm';

// Combined regex: HTTP(S) URLs | file paths (with optional :line:col)
// File paths must contain at least one directory separator and a file extension.
export const LINK_REGEX = /(https?:\/\/[^\s'")\]>]+|(?:[A-Z]:\\|\.{0,2}[\\/])?[\w.-]+(?:[\\/][\w.-]+)+\.\w+(?::\d+(?::\d+)?)?)/g;

export function isUrl(uri: string): boolean {
  return /^https?:\/\//.test(uri);
}

export type LinkHandler = (event: MouseEvent, uri: string) => void;

export function createWebLinksAddon(handler: LinkHandler): WebLinksAddon {
  return new WebLinksAddon(
    (event: MouseEvent, uri: string) => {
      if (!event.ctrlKey) return;
      handler(event, uri);
    },
    { urlRegex: LINK_REGEX }
  );
}
```

**Step 4: Run test to verify it passes**

Run: `cd frontend && npx vitest run src/lib/links.test.ts`
Expected: All 9 tests PASS

**Step 5: Update terminal.ts to accept a link handler**

Modify `frontend/src/lib/terminal.ts`:

Add import and use in `createTerminal`:

```typescript
import { WebLinksAddon } from '@xterm/addon-web-links';
import { createWebLinksAddon, type LinkHandler } from './links';
```

Update `TerminalInstance` interface to include `webLinksAddon`:

```typescript
export interface TerminalInstance {
  terminal: Terminal;
  fitAddon: FitAddon;
  searchAddon: SearchAddon;
  dispose: () => void;
}
```

Update `createTerminal` signature to accept optional `linkHandler`:

```typescript
export function createTerminal(theme: string = 'dark', linkHandler?: LinkHandler): TerminalInstance {
  const terminal = new Terminal({
    ...baseOptions,
    theme: terminalThemes[theme] || terminalThemes.dark,
  });

  const fitAddon = new FitAddon();
  terminal.loadAddon(fitAddon);

  const searchAddon = new SearchAddon();
  terminal.loadAddon(searchAddon);

  if (linkHandler) {
    const webLinksAddon = createWebLinksAddon(linkHandler);
    terminal.loadAddon(webLinksAddon);
  }

  return {
    terminal,
    fitAddon,
    searchAddon,
    dispose: () => {
      searchAddon.dispose();
      fitAddon.dispose();
      terminal.dispose();
    },
  };
}
```

**Step 6: Run all frontend tests**

Run: `cd frontend && npx vitest run`
Expected: All tests PASS

**Step 7: Commit**

```bash
git add frontend/src/lib/links.ts frontend/src/lib/links.test.ts frontend/src/lib/terminal.ts
git commit -m "feat: add link detection module and wire WebLinksAddon into terminal"
```

---

### Task 3: Wire link handler in TerminalPane.svelte

**Files:**
- Modify: `frontend/src/components/TerminalPane.svelte`

**Step 1: Add BrowserOpenURL import and link handler**

In `TerminalPane.svelte`, add `BrowserOpenURL` to the Wails runtime import:

```typescript
import { EventsOn, ClipboardGetText, ClipboardSetText, BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import { isUrl } from '../lib/links';
```

**Step 2: Create link handler function**

Add a function before `onMount`:

```typescript
function handleLink(_event: MouseEvent, uri: string) {
  if (isUrl(uri)) {
    BrowserOpenURL(uri);
  } else {
    dispatch('navigateFile', { path: uri });
  }
}
```

**Step 3: Pass handler to createTerminal**

Change the `createTerminal` call in `onMount` from:

```typescript
termInstance = createTerminal($currentTheme);
```

to:

```typescript
termInstance = createTerminal($currentTheme, handleLink);
```

**Step 4: Commit**

```bash
git add frontend/src/components/TerminalPane.svelte
git commit -m "feat: wire link click handler in TerminalPane (URLs + file paths)"
```

---

### Task 4: Thread navigateFile event through PaneGrid to App.svelte

**Files:**
- Modify: `frontend/src/components/PaneGrid.svelte`
- Modify: `frontend/src/App.svelte`

**Step 1: Forward navigateFile in PaneGrid**

In `PaneGrid.svelte`, add a handler function:

```typescript
function handleNavigateFile(e: CustomEvent) {
  dispatch('navigateFile', e.detail);
}
```

And add `on:navigateFile={handleNavigateFile}` to the `<TerminalPane>` element.

**Step 2: Handle navigateFile in App.svelte**

In `App.svelte`, add a handler function:

```typescript
function handleNavigateFile(e: CustomEvent<{ path: string }>) {
  showSidebar = true;
  sidebarView = 'explorer';
  // The sidebar will show the file tree — user can locate the file
}
```

And add `on:navigateFile={handleNavigateFile}` to the `<PaneGrid>` element.

**Step 3: Run the app and test manually**

Run: `wails dev`

Test:
1. Open a shell pane
2. Run `echo "https://github.com"` — hover over URL, should underline
3. Ctrl+Click the URL — should open in browser
4. Run `ls src/` or print a file path — hover shows underline
5. Ctrl+Click a file path — sidebar should open

**Step 4: Commit**

```bash
git add frontend/src/components/PaneGrid.svelte frontend/src/App.svelte
git commit -m "feat: route navigateFile events from terminal to sidebar"
```

---

### Task 5: Final verification and cleanup

**Step 1: Run all tests**

Run: `cd frontend && npx vitest run`
Expected: All tests PASS

**Step 2: Build production binary**

Run: `wails build`
Expected: Build succeeds without errors

**Step 3: Final commit if any cleanup needed**

```bash
git add -A
git commit -m "chore: cleanup for F-005 clickable links"
```
