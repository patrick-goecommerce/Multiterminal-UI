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
  import { t } from '../stores/i18n';

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

  export let filePath: string = '';
  export let dir: string = '';  // git repo directory for diff

  const dispatch = createEventDispatcher();

  let fileName = '';
  let content = '';
  let error = '';
  let binary = false;
  let size = 0;
  let loading = false;
  let highlightedHtml = '';
  let lines: string[] = [];
  let viewMode: 'view' | 'diff' = 'view';
  let diffContent: string = '';
  let createdAt: string = '';
  let modifiedAt: string = '';

  $: if (filePath) { loadFile(filePath); loadDiff(filePath); }

  function reset() {
    content = '';
    error = '';
    binary = false;
    size = 0;
    highlightedHtml = '';
    lines = [];
    diffContent = '';
    viewMode = 'view';
    createdAt = '';
    modifiedAt = '';
  }

  function formatDate(iso: string): string {
    if (!iso) return '';
    const d = new Date(iso);
    return d.toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: 'numeric' })
      + ' ' + d.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' });
  }

  async function openDir() {
    const result = await App.OpenDirectory(filePath);
    if (result) console.error('[FilePreview] openDir failed:', result);
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
        createdAt = result.created_at || '';
        modifiedAt = result.modified_at || '';
        const highlighted = hljs.highlightAuto(content);
        highlightedHtml = highlighted.value;
        lines = content.split('\n');
      }
    } catch (err) {
      error = String(err);
    }
    loading = false;
  }

  async function loadDiff(path: string) {
    if (!dir || !path) { diffContent = ''; return; }
    try {
      // Extract relative path from absolute path
      const normalizedDir = dir.replace(/\\/g, '/').replace(/\/$/, '');
      const normalizedPath = path.replace(/\\/g, '/');
      const relPath = normalizedPath.startsWith(normalizedDir)
        ? normalizedPath.slice(normalizedDir.length).replace(/^\//, '')
        : normalizedPath;
      diffContent = await App.GetFileDiff(dir, relPath) || '';
    } catch (err) {
      diffContent = '';
      console.error('[FilePreview] loadDiff failed:', err);
    }
  }

  function parseDiffLines(raw: string): Array<{ text: string; type: string }> {
    return raw.split('\n').map(line => {
      let type = 'context';
      if (line.startsWith('+') && !line.startsWith('+++')) type = 'add';
      else if (line.startsWith('-') && !line.startsWith('---')) type = 'del';
      else if (line.startsWith('@@')) type = 'hunk';
      else if (line.startsWith('diff ') || line.startsWith('index ') ||
               line.startsWith('---') || line.startsWith('+++')) type = 'meta';
      return { text: line, type };
    });
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

  onMount(() => document.addEventListener('keydown', handleKeydown));
  onDestroy(() => document.removeEventListener('keydown', handleKeydown));
</script>

<div class="preview-overlay">
      <div class="preview-header">
        <div class="preview-title">
          <span class="preview-filename">{fileName}</span>
          <span class="preview-path">{filePath}</span>
          {#if createdAt || modifiedAt}
            <div class="preview-meta">
              {#if createdAt}<span title="Erstellt">&#128197; {formatDate(createdAt)}</span>{/if}
              {#if modifiedAt}<span title="Geändert">&#9998; {formatDate(modifiedAt)}</span>{/if}
            </div>
          {/if}
        </div>
        <div class="preview-actions">
          <button class="preview-btn" on:click={openDir} title="Verzeichnis öffnen">
            &#128193; Ordner
          </button>
          <div class="view-toggle">
            <button class="toggle-btn" class:active={viewMode === 'view'} on:click={() => viewMode = 'view'}>{$t('filePreview.file')}</button>
            <button class="toggle-btn" class:active={viewMode === 'diff'} on:click={() => viewMode = 'diff'}>Diff</button>
          </div>
          <button class="preview-btn" on:click={openInEditor} title={$t('filePreview.openInEditor')}>
            <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
              <path d="M8.636 3.5a.5.5 0 0 0-.5-.5H1.5A1.5 1.5 0 0 0 0 4.5v10A1.5 1.5 0 0 0 1.5 16h10a1.5 1.5 0 0 0 1.5-1.5V7.864a.5.5 0 0 0-1 0V14.5a.5.5 0 0 1-.5.5h-10a.5.5 0 0 1-.5-.5v-10a.5.5 0 0 1 .5-.5h6.636a.5.5 0 0 0 .5-.5z"/>
              <path d="M16 .5a.5.5 0 0 0-.5-.5h-5a.5.5 0 0 0 0 1h3.793L6.146 9.146a.5.5 0 1 0 .708.708L15 1.707V5.5a.5.5 0 0 0 1 0v-5z"/>
            </svg>
            {$t('filePreview.edit')}
          </button>
          <button class="preview-close" on:click={close} title={$t('filePreview.close')}>&times;</button>
        </div>
      </div>

      <div class="preview-content">
        {#if loading}
          <div class="preview-message">{$t('filePreview.loading')}</div>
        {:else if error}
          <div class="preview-message">
            <p>{error}</p>
            <button class="preview-btn" on:click={openInEditor}>{$t('filePreview.openInEditor')}</button>
          </div>
        {:else if binary}
          <div class="preview-message">
            <p>{$t('filePreview.binaryFile')}</p>
            <button class="preview-btn" on:click={openInEditor}>{$t('filePreview.openInEditor')}</button>
          </div>
        {:else if viewMode === 'diff'}
          {#if diffContent}
            <pre class="diff-pre">{#each parseDiffLines(diffContent) as line}<span class="diff-line diff-{line.type}">{line.text}</span>
{/each}</pre>
          {:else}
            <div class="preview-message">Kein Diff verfügbar (neue oder ungetrackte Datei)</div>
          {/if}
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

<style>
  .preview-overlay {
    position: absolute; inset: 0; z-index: 10;
    background: var(--bg-secondary);
    display: flex; flex-direction: column;
    overflow: hidden;
    animation: fade-in 0.15s ease;
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

  .preview-meta {
    display: flex; gap: 12px; font-size: 10px; color: var(--fg-muted);
    margin-top: 1px;
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

  .view-toggle {
    display: flex;
    border: 1px solid var(--border);
    border-radius: 4px;
    overflow: hidden;
  }
  .toggle-btn {
    padding: 4px 12px;
    font-size: 11px;
    font-weight: 600;
    background: transparent;
    border: none;
    color: var(--fg-muted);
    cursor: pointer;
  }
  .toggle-btn:hover { background: var(--bg-tertiary); }
  .toggle-btn.active {
    background: var(--accent);
    color: #fff;
  }
  .diff-pre {
    margin: 0;
    padding: 12px 16px;
    font-family: 'Cascadia Code', 'Fira Code', 'Consolas', monospace;
    font-size: 13px;
    line-height: 1.5;
    white-space: pre;
    tab-size: 4;
  }
  .diff-line { display: block; }
  .diff-add { color: #50fa7b; background: rgba(80, 250, 123, 0.07); }
  .diff-del { color: #ff5555; background: rgba(255, 85, 85, 0.07); }
  .diff-hunk { color: #bd93f9; }
  .diff-meta { color: var(--fg-muted); }
  .diff-context { color: var(--fg-muted); }

  @keyframes fade-in {
    from { opacity: 0; }
    to { opacity: 1; }
  }
</style>
