<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';

  export let dir: string = '';

  const dispatch = createEventDispatcher();

  interface Issue {
    number: number;
    title: string;
    state: string;
    author: string;
    labels: string[];
    createdAt: string;
    updatedAt: string;
    comments: number;
    url: string;
  }

  interface IssueDetail {
    number: number;
    title: string;
    state: string;
    author: string;
    labels: string[];
    body: string;
    createdAt: string;
    updatedAt: string;
    assignees: string[];
    url: string;
    comments: { author: string; body: string; createdAt: string }[];
  }

  let issues: Issue[] = [];
  let stateFilter: 'open' | 'closed' | 'all' = 'open';
  let searchQuery = '';
  let loading = false;
  let ghStatus = '';
  let selectedIssue: IssueDetail | null = null;
  let commentText = '';
  let submitting = false;

  onMount(async () => {
    ghStatus = await App.CheckGitHubCLI();
    if (ghStatus === 'ok') await loadIssues();
  });

  async function loadIssues() {
    if (!dir) return;
    loading = true;
    try {
      issues = (await App.GetIssues(dir, stateFilter)) || [];
    } catch { issues = []; }
    loading = false;
  }

  async function openIssue(number: number) {
    loading = true;
    try {
      selectedIssue = await App.GetIssueDetail(dir, number);
    } catch { selectedIssue = null; }
    loading = false;
  }

  function goBack() {
    selectedIssue = null;
    commentText = '';
  }

  async function toggleState() {
    if (!selectedIssue) return;
    const newState = selectedIssue.state === 'OPEN' ? 'closed' : 'open';
    try {
      await App.UpdateIssue(dir, selectedIssue.number, '', '', newState);
      await openIssue(selectedIssue.number);
      await loadIssues();
    } catch {}
  }

  async function submitComment() {
    if (!selectedIssue || !commentText.trim()) return;
    submitting = true;
    try {
      await App.AddIssueComment(dir, selectedIssue.number, commentText.trim());
      commentText = '';
      await openIssue(selectedIssue.number);
    } catch {}
    submitting = false;
  }

  function formatDate(iso: string): string {
    if (!iso) return '';
    const d = new Date(iso);
    const now = new Date();
    const diffMs = now.getTime() - d.getTime();
    const diffMin = Math.floor(diffMs / 60000);
    if (diffMin < 1) return 'gerade eben';
    if (diffMin < 60) return `vor ${diffMin}m`;
    const diffH = Math.floor(diffMin / 60);
    if (diffH < 24) return `vor ${diffH}h`;
    const diffD = Math.floor(diffH / 24);
    if (diffD < 30) return `vor ${diffD}d`;
    return d.toLocaleDateString('de-DE');
  }

  $: filteredIssues = searchQuery
    ? issues.filter(i => i.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
        `#${i.number}`.includes(searchQuery))
    : issues;

  $: if (dir && ghStatus === 'ok') loadIssues();
  $: if (stateFilter && ghStatus === 'ok') loadIssues();
  $: openCount = issues.filter(i => i.state === 'OPEN').length;

  function handleDragStart(e: DragEvent, issue: Issue) {
    if (!e.dataTransfer) return;
    const text = `Closes #${issue.number} - ${issue.title}`;
    e.dataTransfer.setData('text/plain', text);
    e.dataTransfer.setData('application/x-issue-number', String(issue.number));
    e.dataTransfer.effectAllowed = 'copy';
  }
</script>

{#if ghStatus === 'not_installed'}
  <div class="status-msg">
    <span class="status-icon">!</span>
    <div>
      <strong>GitHub CLI nicht gefunden</strong>
      <p>Bitte <code>gh</code> installieren:</p>
      <code>https://cli.github.com</code>
    </div>
  </div>
{:else if ghStatus === 'not_authenticated'}
  <div class="status-msg">
    <span class="status-icon">!</span>
    <div>
      <strong>Nicht angemeldet</strong>
      <p>Bitte anmelden:</p>
      <code>gh auth login</code>
    </div>
  </div>
{:else if selectedIssue}
  <!-- Detail View -->
  <div class="detail">
    <div class="detail-header">
      <button class="back-btn" on:click={goBack}>&larr;</button>
      <span class="detail-number">#{selectedIssue.number}</span>
      <button
        class="state-badge"
        class:open={selectedIssue.state === 'OPEN'}
        class:closed={selectedIssue.state !== 'OPEN'}
        on:click={toggleState}
        title="Status ändern"
      >
        {selectedIssue.state === 'OPEN' ? 'Open' : 'Closed'}
      </button>
      <button class="edit-btn" on:click={() => dispatch('editIssue', selectedIssue)} title="Bearbeiten">
        <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
          <path d="M11.013 1.427a1.75 1.75 0 012.474 0l1.086 1.086a1.75 1.75 0 010 2.474l-8.61 8.61c-.21.21-.47.364-.756.445l-3.251.93a.75.75 0 01-.927-.928l.929-3.25a1.75 1.75 0 01.445-.758l8.61-8.61zm1.414 1.06a.25.25 0 00-.354 0L3.463 11.1a.25.25 0 00-.064.108l-.631 2.208 2.208-.63a.25.25 0 00.108-.064l8.61-8.61a.25.25 0 000-.354l-1.086-1.086z"/>
        </svg>
      </button>
    </div>

    <h4 class="detail-title">{selectedIssue.title}</h4>

    <div class="detail-meta">
      <span>{selectedIssue.author}</span>
      <span>{formatDate(selectedIssue.createdAt)}</span>
    </div>

    {#if selectedIssue.labels.length > 0}
      <div class="label-row">
        {#each selectedIssue.labels as label}
          <span class="label-badge">{label}</span>
        {/each}
      </div>
    {/if}

    {#if selectedIssue.body}
      <div class="detail-body">{selectedIssue.body}</div>
    {/if}

    {#if selectedIssue.comments && selectedIssue.comments.length > 0}
      <div class="comments-header">Kommentare ({selectedIssue.comments.length})</div>
      {#each selectedIssue.comments as comment}
        <div class="comment">
          <div class="comment-meta">
            <strong>{comment.author}</strong>
            <span>{formatDate(comment.createdAt)}</span>
          </div>
          <div class="comment-body">{comment.body}</div>
        </div>
      {/each}
    {/if}

    <div class="comment-form">
      <textarea
        bind:value={commentText}
        placeholder="Kommentar schreiben..."
        rows="3"
        on:keydown={(e) => { if (e.key === 'Enter' && e.ctrlKey) submitComment(); }}
      ></textarea>
      <button class="send-btn" on:click={submitComment} disabled={!commentText.trim() || submitting}>
        {submitting ? 'Sende...' : 'Senden'}
      </button>
    </div>
  </div>
{:else}
  <!-- List View -->
  <div class="list-controls">
    <div class="filter-row">
      <button class="filter-btn" class:active={stateFilter === 'open'} on:click={() => (stateFilter = 'open')}>Open</button>
      <button class="filter-btn" class:active={stateFilter === 'closed'} on:click={() => (stateFilter = 'closed')}>Closed</button>
      <button class="filter-btn" class:active={stateFilter === 'all'} on:click={() => (stateFilter = 'all')}>Alle</button>
      <button class="icon-btn" on:click={loadIssues} title="Aktualisieren">&#8635;</button>
      <button class="icon-btn create-btn" on:click={() => dispatch('createIssue')} title="Neues Issue">+</button>
    </div>
    <div class="search-box">
      <input type="text" placeholder="Issues filtern..." bind:value={searchQuery} />
    </div>
  </div>

  <div class="issue-list">
    {#if loading}
      <div class="no-results">Laden...</div>
    {:else if filteredIssues.length === 0}
      <div class="no-results">Keine Issues</div>
    {:else}
      {#each filteredIssues as issue (issue.number)}
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="issue-item" draggable="true" on:dragstart={(e) => handleDragStart(e, issue)} on:click={() => openIssue(issue.number)}>
          <div class="issue-icon" class:open={issue.state === 'OPEN'} class:closed={issue.state !== 'OPEN'}>
            {issue.state === 'OPEN' ? '●' : '✓'}
          </div>
          <div class="issue-content">
            <div class="issue-title">
              <span class="issue-num">#{issue.number}</span>
              {issue.title}
            </div>
            <div class="issue-meta">
              <span>{issue.author}</span>
              <span>{formatDate(issue.createdAt)}</span>
              {#if issue.comments > 0}
                <span>💬 {issue.comments}</span>
              {/if}
            </div>
            {#if issue.labels.length > 0}
              <div class="issue-labels">
                {#each issue.labels as label}
                  <span class="label-badge">{label}</span>
                {/each}
              </div>
            {/if}
          </div>
        </div>
      {/each}
    {/if}
  </div>
{/if}

<style>
  .status-msg {
    padding: 16px 12px; display: flex; gap: 10px; align-items: flex-start;
    color: var(--fg-muted); font-size: 12px;
  }
  .status-icon { font-size: 18px; color: var(--warning); flex-shrink: 0; }
  .status-msg strong { color: var(--fg); display: block; margin-bottom: 4px; }
  .status-msg p { margin: 2px 0; }
  .status-msg code { font-size: 11px; background: var(--bg-tertiary); padding: 2px 6px; border-radius: 3px; }

  .list-controls { padding: 6px 8px; border-bottom: 1px solid var(--border); }
  .filter-row { display: flex; gap: 2px; margin-bottom: 6px; align-items: center; }
  .filter-btn {
    padding: 3px 10px; font-size: 11px; font-weight: 600; border: none; border-radius: 4px;
    cursor: pointer; background: transparent; color: var(--fg-muted); transition: all 0.15s;
  }
  .filter-btn:hover { background: var(--bg-tertiary); color: var(--fg); }
  .filter-btn.active { background: var(--accent); color: #fff; }
  .icon-btn {
    padding: 2px 8px; font-size: 14px; background: none; border: none; color: var(--fg-muted);
    cursor: pointer; border-radius: 4px; margin-left: auto;
  }
  .icon-btn:hover { background: var(--bg-tertiary); color: var(--fg); }
  .create-btn { font-size: 18px; font-weight: 700; color: var(--accent); margin-left: 2px; }
  .create-btn:hover { color: var(--fg); }

  .search-box input {
    width: 100%; padding: 5px 8px; background: var(--bg-tertiary); border: 1px solid var(--border);
    border-radius: 4px; color: var(--fg); font-size: 12px; box-sizing: border-box;
  }
  .search-box input::placeholder { color: var(--fg-muted); }

  .issue-list { flex: 1; overflow-y: auto; }
  .no-results { padding: 12px; text-align: center; color: var(--fg-muted); font-size: 12px; }

  .issue-item {
    display: flex; gap: 8px; padding: 8px 10px; cursor: pointer; border-bottom: 1px solid var(--border);
    transition: background 0.1s;
  }
  .issue-item:hover { background: var(--bg-tertiary); }
  .issue-item[draggable="true"] { cursor: grab; }
  .issue-item[draggable="true"]:active { cursor: grabbing; }
  .issue-icon { font-size: 12px; padding-top: 2px; flex-shrink: 0; }
  .issue-icon.open { color: var(--success); }
  .issue-icon.closed { color: #a371f7; }
  .issue-content { flex: 1; min-width: 0; }
  .issue-title { font-size: 12px; font-weight: 600; color: var(--fg); line-height: 1.3; word-break: break-word; }
  .issue-num { color: var(--fg-muted); font-weight: 400; margin-right: 4px; }
  .issue-meta { font-size: 10px; color: var(--fg-muted); margin-top: 3px; display: flex; gap: 8px; }
  .issue-labels { display: flex; flex-wrap: wrap; gap: 3px; margin-top: 4px; }
  .label-badge {
    font-size: 10px; padding: 1px 6px; border-radius: 10px; font-weight: 600;
    background: var(--accent); color: #fff; opacity: 0.85;
  }

  /* Detail View */
  .detail { padding: 0; overflow-y: auto; flex: 1; }
  .detail-header {
    display: flex; align-items: center; gap: 6px; padding: 8px 10px;
    border-bottom: 1px solid var(--border); position: sticky; top: 0; background: var(--bg-secondary);
  }
  .back-btn {
    background: none; border: none; color: var(--fg-muted); cursor: pointer; font-size: 16px;
    padding: 0 4px; border-radius: 4px;
  }
  .back-btn:hover { color: var(--fg); background: var(--bg-tertiary); }
  .detail-number { font-size: 12px; color: var(--fg-muted); font-weight: 600; }
  .state-badge {
    font-size: 10px; font-weight: 700; padding: 2px 8px; border-radius: 10px; border: none;
    cursor: pointer; transition: opacity 0.15s;
  }
  .state-badge:hover { opacity: 0.8; }
  .state-badge.open { background: #238636; color: #fff; }
  .state-badge.closed { background: #8957e5; color: #fff; }
  .edit-btn {
    background: none; border: none; color: var(--fg-muted); cursor: pointer; padding: 4px;
    border-radius: 4px; margin-left: auto; display: flex; align-items: center;
  }
  .edit-btn:hover { color: var(--fg); background: var(--bg-tertiary); }

  .detail-title { font-size: 14px; font-weight: 700; color: var(--fg); padding: 10px 10px 4px; line-height: 1.3; }
  .detail-meta { font-size: 11px; color: var(--fg-muted); padding: 0 10px 8px; display: flex; gap: 8px; }
  .label-row { display: flex; flex-wrap: wrap; gap: 4px; padding: 0 10px 8px; }
  .detail-body {
    font-size: 12px; color: var(--fg); padding: 10px; margin: 0 10px 8px;
    background: var(--bg-tertiary); border-radius: 6px; line-height: 1.5; white-space: pre-wrap;
    word-break: break-word;
  }

  .comments-header {
    font-size: 11px; font-weight: 600; color: var(--fg-muted); padding: 8px 10px 4px;
    border-top: 1px solid var(--border); text-transform: uppercase; letter-spacing: 0.5px;
  }
  .comment { padding: 8px 10px; border-bottom: 1px solid var(--border); }
  .comment-meta { font-size: 10px; color: var(--fg-muted); margin-bottom: 4px; display: flex; gap: 8px; }
  .comment-meta strong { color: var(--fg); }
  .comment-body { font-size: 12px; color: var(--fg); line-height: 1.4; white-space: pre-wrap; word-break: break-word; }

  .comment-form { padding: 10px; border-top: 1px solid var(--border); }
  .comment-form textarea {
    width: 100%; padding: 8px; background: var(--bg-tertiary); border: 1px solid var(--border);
    border-radius: 6px; color: var(--fg); font-size: 12px; resize: vertical; box-sizing: border-box;
    font-family: inherit;
  }
  .comment-form textarea::placeholder { color: var(--fg-muted); }
  .send-btn {
    margin-top: 6px; padding: 5px 14px; background: var(--accent); color: #fff; border: none;
    border-radius: 6px; font-size: 12px; font-weight: 600; cursor: pointer; transition: opacity 0.15s;
  }
  .send-btn:hover { opacity: 0.85; }
  .send-btn:disabled { opacity: 0.4; cursor: default; }
</style>
