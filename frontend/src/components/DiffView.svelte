<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';

  export let dir: string = '';
  export let visible: boolean = false;

  const dispatch = createEventDispatcher();

  interface FileStat {
    path: string;
    status: string;
    insertions: number;
    deletions: number;
  }

  interface DiffLine {
    text: string;
    type: 'add' | 'del' | 'hunk' | 'header' | 'context';
  }

  let files: FileStat[] = [];
  let selectedFile: string = '';
  let diffContent: string = '';
  let checkedFiles: Set<string> = new Set();
  let loading: boolean = false;

  const statusColors: Record<string, string> = {
    M: '#e2b714',
    A: '#50fa7b',
    D: '#ff5555',
    R: '#8be9fd',
    '?': '#50fa7b',
  };

  const statusLabels: Record<string, string> = {
    M: 'M',
    A: 'A',
    D: 'D',
    R: 'R',
    '?': 'U',
  };

  $: if (visible && dir) loadStats();

  async function loadStats() {
    loading = true;
    try {
      files = await App.GetDiffStats(dir);
      checkedFiles = new Set(files.map((f) => f.path));
      if (files.length > 0) {
        await selectFile(files[0].path);
      } else {
        selectedFile = '';
        diffContent = '';
      }
    } finally {
      loading = false;
    }
  }

  async function selectFile(path: string) {
    selectedFile = path;
    try {
      diffContent = await App.GetFileDiff(dir, path);
    } catch {
      diffContent = '';
    }
  }

  function toggleFile(path: string) {
    if (checkedFiles.has(path)) {
      checkedFiles.delete(path);
    } else {
      checkedFiles.add(path);
    }
    checkedFiles = checkedFiles;
  }

  function toggleAll() {
    if (checkedFiles.size === files.length) {
      checkedFiles = new Set();
    } else {
      checkedFiles = new Set(files.map((f) => f.path));
    }
  }

  async function refresh() {
    await loadStats();
    if (selectedFile) {
      await selectFile(selectedFile);
    }
  }

  function parseDiffLines(raw: string): DiffLine[] {
    if (!raw) return [];
    return raw.split('\n').map((text) => {
      let type: DiffLine['type'] = 'context';
      if (text.startsWith('+++') || text.startsWith('---') || text.startsWith('diff ') || text.startsWith('index ')) {
        type = 'header';
      } else if (text.startsWith('@@')) {
        type = 'hunk';
      } else if (text.startsWith('+')) {
        type = 'add';
      } else if (text.startsWith('-')) {
        type = 'del';
      }
      return { text, type };
    });
  }

  function commitRequest() {
    const selectedFiles = Array.from(checkedFiles);
    dispatch('commitRequest', { dir, files: selectedFiles, fileCount: selectedFiles.length });
  }

  function close() {
    dispatch('close');
  }

  function getStatusColor(status: string): string {
    return statusColors[status] || '#text-secondary';
  }

  function getStatusLabel(status: string): string {
    return statusLabels[status] || status;
  }
</script>

{#if visible}
  <div class="diff-overlay">
    <!-- Header -->
    <div class="diff-header">
      <div class="diff-title">
        <span>Änderungen</span>
        <span class="file-count-badge">{files.length}</span>
      </div>
      <div class="diff-header-actions">
        <button class="icon-btn" on:click={refresh} title="Aktualisieren">&#8635;</button>
        <button class="icon-btn" on:click={close} title="Schließen">&times;</button>
      </div>
    </div>

    <!-- Body -->
    <div class="diff-body">
      <!-- Left sidebar -->
      <div class="diff-sidebar">
        <label class="select-all">
          <input
            type="checkbox"
            checked={checkedFiles.size === files.length && files.length > 0}
            on:change={toggleAll}
          />
          <span>Alle auswählen</span>
        </label>
        <div class="file-list">
          {#each files as file}
            <div class="file-entry" class:selected={selectedFile === file.path}>
              <input
                type="checkbox"
                checked={checkedFiles.has(file.path)}
                on:click|stopPropagation={() => toggleFile(file.path)}
              />
              <button class="file-btn" on:click={() => selectFile(file.path)}>
                <span class="file-status" style="color: {getStatusColor(file.status)}">{getStatusLabel(file.status)}</span>
                <span class="file-path" title={file.path}>{file.path}</span>
                <span class="file-stats">+{file.insertions} -{file.deletions}</span>
              </button>
            </div>
          {/each}
        </div>
      </div>

      <!-- Right diff area -->
      <div class="diff-content">
        {#if loading}
          <div class="diff-loading">Laden...</div>
        {:else if diffContent}
          <pre class="diff-pre">{#each parseDiffLines(diffContent) as line}<span class="diff-line diff-{line.type}">{line.text}</span>
{/each}</pre>
        {:else if selectedFile}
          <div class="diff-empty">Kein Diff verfügbar</div>
        {:else}
          <div class="diff-empty">Datei auswählen um Diff anzuzeigen</div>
        {/if}
      </div>
    </div>

    <!-- Action bar -->
    <div class="diff-actions">
      <span class="selected-count">
        {checkedFiles.size} von {files.length} Dateien ausgewählt
      </span>
      <button
        class="commit-btn"
        disabled={checkedFiles.size === 0}
        on:click={commitRequest}
      >
        Commit {checkedFiles.size} Dateien
      </button>
    </div>
  </div>
{/if}

<style>
  .diff-overlay {
    position: absolute;
    inset: 0;
    z-index: 50;
    display: flex;
    flex-direction: column;
    background: var(--bg-primary);
    font-family: 'Cascadia Code', 'Fira Code', monospace;
    font-size: 12px;
    color: var(--text-primary);
  }

  .diff-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border-color);
    background: var(--bg-secondary);
  }

  .diff-title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 14px;
    font-weight: 600;
  }

  .file-count-badge {
    background: var(--bg-hover);
    border-radius: 10px;
    padding: 1px 8px;
    font-size: 11px;
    color: var(--text-secondary);
  }

  .diff-header-actions {
    display: flex;
    gap: 4px;
  }

  .icon-btn {
    background: none;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 18px;
    padding: 2px 6px;
    border-radius: 4px;
    line-height: 1;
  }

  .icon-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .diff-body {
    display: flex;
    flex: 1;
    overflow: hidden;
  }

  .diff-sidebar {
    width: 260px;
    min-width: 260px;
    border-right: 1px solid var(--border-color);
    display: flex;
    flex-direction: column;
    background: var(--bg-secondary);
  }

  .select-all {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 10px;
    border-bottom: 1px solid var(--border-color);
    cursor: pointer;
    color: var(--text-secondary);
    font-size: 11px;
  }

  .select-all:hover {
    background: var(--bg-hover);
  }

  .file-list {
    flex: 1;
    overflow-y: auto;
  }

  .file-entry {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 2px 6px;
    border-bottom: 1px solid transparent;
  }

  .file-entry:hover {
    background: var(--bg-hover);
  }

  .file-entry.selected {
    background: var(--bg-selected);
  }

  .file-entry input[type='checkbox'] {
    cursor: pointer;
    flex-shrink: 0;
  }

  .file-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    flex: 1;
    min-width: 0;
    background: none;
    border: none;
    color: var(--text-primary);
    cursor: pointer;
    padding: 4px 2px;
    text-align: left;
    font-family: inherit;
    font-size: inherit;
  }

  .file-status {
    font-weight: 700;
    flex-shrink: 0;
    width: 14px;
    text-align: center;
  }

  .file-path {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .file-stats {
    flex-shrink: 0;
    color: var(--text-secondary);
    font-size: 11px;
  }

  .diff-content {
    flex: 1;
    overflow: auto;
    background: var(--bg-primary);
  }

  .diff-loading,
  .diff-empty {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--text-secondary);
  }

  .diff-pre {
    margin: 0;
    padding: 8px;
    font-family: 'Cascadia Code', 'Fira Code', monospace;
    font-size: 12px;
    line-height: 1.5;
    white-space: pre;
  }

  .diff-line {
    display: block;
  }

  .diff-line.diff-add {
    color: #50fa7b;
    background: #50fa7b11;
  }

  .diff-line.diff-del {
    color: #ff5555;
    background: #ff555511;
  }

  .diff-line.diff-hunk {
    color: #bd93f9;
  }

  .diff-line.diff-header {
    color: #6272a4;
  }

  .diff-line.diff-context {
    color: var(--text-primary);
  }

  .diff-actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    border-top: 1px solid var(--border-color);
    background: var(--bg-secondary);
  }

  .selected-count {
    color: var(--text-secondary);
    font-size: 12px;
  }

  .commit-btn {
    background: #7c3aed;
    color: #fff;
    border: none;
    border-radius: 4px;
    padding: 6px 16px;
    font-family: inherit;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
  }

  .commit-btn:hover:not(:disabled) {
    background: #6d28d9;
  }

  .commit-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
</style>
