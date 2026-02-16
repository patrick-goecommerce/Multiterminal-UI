# F-019: File Preview (Quick Peek) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an inline file preview overlay with syntax highlighting when clicking files in the sidebar.

**Architecture:** New Go backend methods `ReadFile` and `OpenFileInEditor` in `app_files.go`. New `FilePreview.svelte` overlay component using highlight.js for syntax highlighting. Integration via `selectFile` event in `App.svelte` opening the preview instead of writing path to terminal.

**Tech Stack:** Go (backend file reading), Svelte 4 (component), highlight.js (syntax highlighting), CSS variables (theming)

---

### Task 1: Backend — ReadFile + OpenFileInEditor

**Files:**
- Modify: `internal/backend/app_files.go` (append after existing code, line 97)

**Step 1: Add ReadFile and OpenFileInEditor to app_files.go**

Append this code after the existing `SearchFiles` function in `internal/backend/app_files.go`:

```go
// maxPreviewSize is the maximum file size (1 MB) for preview.
const maxPreviewSize = 1 << 20

// FileContent holds the result of reading a file for preview.
type FileContent struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
	Error   string `json:"error"`
	Binary  bool   `json:"binary"`
}

// ReadFile reads a file and returns its content for preview.
// Files larger than 1 MB or detected as binary are rejected.
func (a *App) ReadFile(path string) FileContent {
	info, err := os.Stat(path)
	if err != nil {
		return FileContent{Path: path, Name: filepath.Base(path), Error: err.Error()}
	}
	if info.IsDir() {
		return FileContent{Path: path, Name: info.Name(), Error: "Verzeichnis kann nicht angezeigt werden"}
	}
	size := info.Size()
	if size > maxPreviewSize {
		mb := float64(size) / (1 << 20)
		return FileContent{
			Path:  path,
			Name:  info.Name(),
			Size:  size,
			Error: fmt.Sprintf("Datei zu groß (%.1f MB, max 1 MB)", mb),
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return FileContent{Path: path, Name: info.Name(), Size: size, Error: err.Error()}
	}

	// Detect binary: check first 512 bytes for NUL
	probe := data
	if len(probe) > 512 {
		probe = probe[:512]
	}
	for _, b := range probe {
		if b == 0 {
			return FileContent{Path: path, Name: info.Name(), Size: size, Binary: true}
		}
	}

	return FileContent{
		Path:    path,
		Name:    info.Name(),
		Content: string(data),
		Size:    size,
	}
}

// OpenFileInEditor opens the file in the system default editor.
func (a *App) OpenFileInEditor(path string) string {
	cmd := exec.Command("cmd", "/c", "start", "", path)
	if err := cmd.Start(); err != nil {
		return err.Error()
	}
	return ""
}
```

**Step 2: Add missing imports**

The file currently imports `"os"`, `"path/filepath"`, `"sort"`, `"strings"`. Add `"fmt"` and `"os/exec"` to the import block:

```go
import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)
```

**Step 3: Verify Go compiles**

Run: `cd D:/repos/Multiterminal && go build ./internal/backend/`
Expected: No errors

**Step 4: Commit**

```bash
git add internal/backend/app_files.go
git commit -m "feat: add ReadFile and OpenFileInEditor backend APIs for file preview"
```

---

### Task 2: Install highlight.js dependency

**Files:**
- Modify: `frontend/package.json`

**Step 1: Install highlight.js**

Run: `cd D:/repos/Multiterminal/frontend && npm install highlight.js`

**Step 2: Verify install succeeded**

Run: `cd D:/repos/Multiterminal/frontend && node -e "require('highlight.js/lib/core');" && echo OK`
Expected: `OK`

**Step 3: Commit**

```bash
git add frontend/package.json frontend/package-lock.json
git commit -m "feat: add highlight.js dependency for file preview syntax highlighting"
```

---

### Task 3: Create FilePreview.svelte component

**Files:**
- Create: `frontend/src/components/FilePreview.svelte`

**Step 1: Create the FilePreview component**

Create `frontend/src/components/FilePreview.svelte` with this content:

```svelte
<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import hljs from 'highlight.js/lib/core';
  import javascript from 'highlight.js/lib/languages/javascript';
  import typescript from 'highlight.js/lib/languages/typescript';
  import go from 'highlight.js/lib/languages/go';
  import python from 'highlight.js/lib/languages/python';
  import yaml from 'highlight.js/lib/languages/yaml';
  import json from 'highlight.js/lib/languages/json';
  import xml from 'highlight.js/lib/languages/xml';
  import css from 'highlight.js/lib/languages/css';
  import bash from 'highlight.js/lib/languages/bash';
  import markdown from 'highlight.js/lib/languages/markdown';
  import rust from 'highlight.js/lib/languages/rust';
  import sql from 'highlight.js/lib/languages/sql';
  import dockerfile from 'highlight.js/lib/languages/dockerfile';
  import * as App from '../../wailsjs/go/backend/App';

  hljs.registerLanguage('javascript', javascript);
  hljs.registerLanguage('typescript', typescript);
  hljs.registerLanguage('go', go);
  hljs.registerLanguage('python', python);
  hljs.registerLanguage('yaml', yaml);
  hljs.registerLanguage('json', json);
  hljs.registerLanguage('xml', xml);
  hljs.registerLanguage('css', css);
  hljs.registerLanguage('bash', bash);
  hljs.registerLanguage('markdown', markdown);
  hljs.registerLanguage('rust', rust);
  hljs.registerLanguage('sql', sql);
  hljs.registerLanguage('dockerfile', dockerfile);

  export let visible: boolean = false;
  export let filePath: string = '';

  const dispatch = createEventDispatcher();

  let fileName = '';
  let content = '';
  let error = '';
  let binary = false;
  let size = 0;
  let loading = false;
  let highlightedHtml = '';
  let lines: string[] = [];

  $: if (visible && filePath) loadFile(filePath);
  $: if (!visible) reset();

  function reset() {
    content = '';
    error = '';
    binary = false;
    size = 0;
    highlightedHtml = '';
    lines = [];
  }

  async function loadFile(path: string) {
    loading = true;
    reset();
    try {
      const result = await App.ReadFile(path);
      fileName = result.name;
      size = result.size;
      if (result.error) {
        error = result.error;
      } else if (result.binary) {
        binary = true;
      } else {
        content = result.content;
        const highlighted = hljs.highlightAuto(content);
        highlightedHtml = highlighted.value;
        lines = content.split('\n');
      }
    } catch (err) {
      error = String(err);
    }
    loading = false;
  }

  async function openInEditor() {
    const result = await App.OpenFileInEditor(filePath);
    if (result) console.error('[FilePreview] editor open failed:', result);
  }

  function close() {
    dispatch('close');
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
  }

  function handleBackdropClick(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains('preview-backdrop')) close();
  }

  onMount(() => document.addEventListener('keydown', handleKeydown));
  onDestroy(() => document.removeEventListener('keydown', handleKeydown));
</script>

{#if visible}
  <div class="preview-backdrop" on:click={handleBackdropClick} role="presentation">
    <div class="preview-panel">
      <div class="preview-header">
        <div class="preview-title">
          <span class="preview-filename">{fileName}</span>
          <span class="preview-path">{filePath}</span>
        </div>
        <div class="preview-actions">
          <button class="preview-btn" on:click={openInEditor} title="Im Editor öffnen">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
              <path d="M8.636 3.5a.5.5 0 0 0-.5-.5H1.5A1.5 1.5 0 0 0 0 4.5v10A1.5 1.5 0 0 0 1.5 16h10a1.5 1.5 0 0 0 1.5-1.5V7.864a.5.5 0 0 0-1 0V14.5a.5.5 0 0 1-.5.5h-10a.5.5 0 0 1-.5-.5v-10a.5.5 0 0 1 .5-.5h6.636a.5.5 0 0 0 .5-.5z"/>
              <path d="M16 .5a.5.5 0 0 0-.5-.5h-5a.5.5 0 0 0 0 1h3.793L6.146 9.146a.5.5 0 1 0 .708.708L15 1.707V5.5a.5.5 0 0 0 1 0v-5z"/>
            </svg>
            Im Editor
          </button>
          <button class="preview-close" on:click={close} title="Schließen">&times;</button>
        </div>
      </div>

      <div class="preview-content">
        {#if loading}
          <div class="preview-message">Laden...</div>
        {:else if error}
          <div class="preview-message">
            <p>{error}</p>
            <button class="preview-btn" on:click={openInEditor}>Im Editor öffnen</button>
          </div>
        {:else if binary}
          <div class="preview-message">
            <p>Binärdatei kann nicht angezeigt werden</p>
            <button class="preview-btn" on:click={openInEditor}>Im Editor öffnen</button>
          </div>
        {:else}
          <div class="code-container">
            <div class="line-numbers">
              {#each lines as _, i}
                <span>{i + 1}</span>
              {/each}
            </div>
            <pre class="code-block"><code class="hljs">{@html highlightedHtml}</code></pre>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .preview-backdrop {
    position: fixed; inset: 0; z-index: 1000;
    background: rgba(0, 0, 0, 0.6);
    display: flex; align-items: center; justify-content: center;
    animation: fade-in 0.15s ease;
  }

  .preview-panel {
    width: 80%; height: 80%;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    display: flex; flex-direction: column;
    overflow: hidden;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
  }

  .preview-header {
    display: flex; align-items: center; justify-content: space-between;
    padding: 10px 16px;
    border-bottom: 1px solid var(--border);
    gap: 12px; flex-shrink: 0;
  }

  .preview-title {
    display: flex; flex-direction: column; gap: 2px;
    overflow: hidden; min-width: 0;
  }

  .preview-filename {
    font-size: 13px; font-weight: 600; color: var(--fg);
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }

  .preview-path {
    font-size: 11px; color: var(--fg-muted);
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }

  .preview-actions {
    display: flex; align-items: center; gap: 8px; flex-shrink: 0;
  }

  .preview-btn {
    display: flex; align-items: center; gap: 6px;
    padding: 5px 10px; font-size: 12px; font-weight: 500;
    background: var(--bg-tertiary); border: 1px solid var(--border);
    border-radius: 4px; color: var(--fg); cursor: pointer;
    transition: background 0.15s;
  }
  .preview-btn:hover { background: var(--accent); color: #fff; }

  .preview-close {
    background: none; border: none; color: var(--fg-muted);
    cursor: pointer; font-size: 20px; padding: 0 4px; line-height: 1;
  }
  .preview-close:hover { color: var(--fg); }

  .preview-content {
    flex: 1; overflow: auto;
  }

  .preview-message {
    display: flex; flex-direction: column; align-items: center;
    justify-content: center; gap: 12px;
    height: 100%; padding: 24px;
    color: var(--fg-muted); font-size: 14px; text-align: center;
  }

  .code-container {
    display: flex; min-height: 100%;
    font-family: 'Cascadia Code', 'Fira Code', 'Consolas', 'Monaco', monospace;
    font-size: 13px; line-height: 1.5;
  }

  .line-numbers {
    display: flex; flex-direction: column;
    padding: 12px 12px 12px 16px;
    text-align: right; color: var(--fg-muted);
    user-select: none; flex-shrink: 0;
    background: var(--bg-secondary);
    border-right: 1px solid var(--border);
  }
  .line-numbers span { display: block; }

  .code-block {
    flex: 1; margin: 0; padding: 12px 16px;
    overflow-x: auto; tab-size: 4;
  }

  /* highlight.js theme using CSS variables */
  :global(.hljs) { background: transparent !important; color: var(--fg); }
  :global(.hljs-keyword), :global(.hljs-selector-tag) { color: var(--accent); }
  :global(.hljs-string), :global(.hljs-addition) { color: var(--success); }
  :global(.hljs-comment), :global(.hljs-quote) { color: var(--fg-muted); font-style: italic; }
  :global(.hljs-number), :global(.hljs-literal) { color: var(--warning); }
  :global(.hljs-deletion) { color: var(--error); }
  :global(.hljs-title), :global(.hljs-function) { color: var(--accent-hover); }
  :global(.hljs-type), :global(.hljs-built_in) { color: var(--warning); }
  :global(.hljs-attr), :global(.hljs-variable) { color: var(--fg); }
  :global(.hljs-meta) { color: var(--fg-muted); }
  :global(.hljs-symbol), :global(.hljs-bullet) { color: var(--accent); }

  @keyframes fade-in {
    from { opacity: 0; }
    to { opacity: 1; }
  }
</style>
```

**Step 2: Commit**

```bash
git add frontend/src/components/FilePreview.svelte
git commit -m "feat: add FilePreview overlay component with syntax highlighting"
```

---

### Task 4: Integrate FilePreview into App.svelte

**Files:**
- Modify: `frontend/src/App.svelte`

**Step 1: Add FilePreview import**

In `frontend/src/App.svelte`, add this import after the `BranchConflictDialog` import (line 14):

```typescript
  import FilePreview from './components/FilePreview.svelte';
```

**Step 2: Add state variable**

After `let showIssueDialog = false;` (line 37), add:

```typescript
  let previewFilePath = '';
```

**Step 3: Replace handleSidebarFile function**

Replace the existing `handleSidebarFile` function (lines 347-355) with:

```typescript
  function handleSidebarFile(e: CustomEvent<{ path: string }>) {
    previewFilePath = e.detail.path;
  }
```

**Step 4: Add FilePreview component to template**

After the `BranchConflictDialog` closing tag (line 503), add:

```svelte
  <FilePreview visible={!!previewFilePath} filePath={previewFilePath} on:close={() => (previewFilePath = '')} />
```

**Step 5: Verify frontend compiles**

Run: `cd D:/repos/Multiterminal/frontend && npx vite build`
Expected: Build succeeds without errors

**Step 6: Commit**

```bash
git add frontend/src/App.svelte
git commit -m "feat: integrate FilePreview into App — sidebar file click opens preview overlay

Closes #61"
```

---

### Task 5: Build and verify

**Step 1: Full Wails build**

Run: `cd D:/repos/Multiterminal && wails build`
Expected: Build succeeds, binary at `build/bin/multiterminal.exe`

**Step 2: Manual smoke test**

1. Launch `build/bin/multiterminal.exe`
2. Open sidebar (Ctrl+B)
3. Click on a `.go` file → Preview overlay should appear with syntax highlighting
4. Verify line numbers are visible
5. Click "Im Editor" → file opens in default editor
6. Press Escape → overlay closes
7. Click a large file (>1 MB) → error message with editor button
8. Close app
