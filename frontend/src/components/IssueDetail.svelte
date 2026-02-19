<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';

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

  export let issue: IssueDetail;
  export let formatDate: (iso: string) => string;

  const dispatch = createEventDispatcher();

  let commentText = '';
  let submitting = false;

  function submitComment() {
    if (!commentText.trim()) return;
    submitting = true;
    dispatch('submitComment', { text: commentText.trim() });
    commentText = '';
    submitting = false;
  }
</script>

<div class="detail">
  <div class="detail-header">
    <button class="back-btn" on:click={() => dispatch('back')}>&larr;</button>
    <span class="detail-number">#{issue.number}</span>
    <button
      class="state-badge"
      class:open={issue.state === 'OPEN'}
      class:closed={issue.state !== 'OPEN'}
      on:click={() => dispatch('toggleState')}
      title="Status ändern"
    >
      {issue.state === 'OPEN' ? 'Open' : 'Closed'}
    </button>
    {#if issue.url}
      <button class="edit-btn" on:click={() => BrowserOpenURL(issue.url)} title="Im Browser öffnen">
        <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
          <path d="M3.75 2h3.5a.75.75 0 010 1.5h-3.5a.25.25 0 00-.25.25v8.5c0 .138.112.25.25.25h8.5a.25.25 0 00.25-.25v-3.5a.75.75 0 011.5 0v3.5A1.75 1.75 0 0112.25 14h-8.5A1.75 1.75 0 012 12.25v-8.5C2 2.784 2.784 2 3.75 2zm6.854-.22a.75.75 0 01.22.53v4.25a.75.75 0 01-1.5 0V3.56L6.22 6.72a.75.75 0 01-1.06-1.06l3.1-3.1H5.31a.75.75 0 010-1.5h4.25a.75.75 0 01.53.22z"/>
        </svg>
      </button>
    {/if}
    <button class="edit-btn" on:click={() => dispatch('editIssue', issue)} title="Bearbeiten">
      <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
        <path d="M11.013 1.427a1.75 1.75 0 012.474 0l1.086 1.086a1.75 1.75 0 010 2.474l-8.61 8.61c-.21.21-.47.364-.756.445l-3.251.93a.75.75 0 01-.927-.928l.929-3.25a1.75 1.75 0 01.445-.758l8.61-8.61zm1.414 1.06a.25.25 0 00-.354 0L3.463 11.1a.25.25 0 00-.064.108l-.631 2.208 2.208-.63a.25.25 0 00.108-.064l8.61-8.61a.25.25 0 000-.354l-1.086-1.086z"/>
      </svg>
    </button>
  </div>

  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <h4
    class="detail-title"
    draggable="true"
    on:dragstart={(e) => {
      if (!e.dataTransfer) return;
      let text = `Closes #${issue.number}: ${issue.title}`;
      if (issue.labels.length > 0) text += `\nLabels: ${issue.labels.join(', ')}`;
      if (issue.body) {
        const desc = issue.body.length > 200 ? issue.body.slice(0, 200).trimEnd() + '...' : issue.body;
        text += `\n\n${desc}`;
      }
      text += `\n\nRef: #${issue.number}`;
      e.dataTransfer.setData('text/plain', text);
      e.dataTransfer.effectAllowed = 'copy';
    }}
  >{issue.title}</h4>

  <div class="detail-meta">
    <span>{issue.author}</span>
    <span>{formatDate(issue.createdAt)}</span>
  </div>

  {#if issue.labels.length > 0}
    <div class="label-row">
      {#each issue.labels as label}
        <span class="label-badge">{label}</span>
      {/each}
    </div>
  {/if}

  {#if issue.body}
    <div class="detail-body">{issue.body}</div>
  {/if}

  {#if issue.comments && issue.comments.length > 0}
    <div class="comments-header">Kommentare ({issue.comments.length})</div>
    {#each issue.comments as comment}
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

<style>
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

  .detail-title { font-size: 14px; font-weight: 700; color: var(--fg); padding: 10px 10px 4px; line-height: 1.3; cursor: grab; }
  .detail-title:active { cursor: grabbing; }
  .detail-meta { font-size: 11px; color: var(--fg-muted); padding: 0 10px 8px; display: flex; gap: 8px; }
  .label-row { display: flex; flex-wrap: wrap; gap: 4px; padding: 0 10px 8px; }
  .label-badge {
    font-size: 10px; padding: 1px 6px; border-radius: 10px; font-weight: 600;
    background: var(--accent); color: #fff; opacity: 0.85;
  }
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
