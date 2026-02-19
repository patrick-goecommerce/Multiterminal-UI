<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';
  import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
  import IssueDetailComponent from './IssueDetail.svelte';

  export let dir: string = '';

  const dispatch = createEventDispatcher();

  interface Issue {
    number: number;
    title: string;
    state: string;
    author: string;
    labels: string[];
    body: string;
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

  async function handleSubmitComment(e: CustomEvent<{ text: string }>) {
    if (!selectedIssue) return;
    try {
      await App.AddIssueComment(dir, selectedIssue.number, e.detail.text);
      await openIssue(selectedIssue.number);
    } catch {}
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

  function buildDragText(number: number, title: string, body: string, labels: string[]): string {
    let text = `Closes #${number}: ${title}`;
    if (labels.length > 0) {
      text += `\nLabels: ${labels.join(', ')}`;
    }
    if (body) {
      const desc = body.length > 200 ? body.slice(0, 200).trimEnd() + '...' : body;
      text += `\n\n${desc}`;
    }
    text += `\n\nRef: #${number}`;
    return text;
  }

  // Issue-to-pane mapping: which issues have active panes and their activity state
  export let paneIssues: Record<number, { activity: string; cost: string }> = {};

  function handleDragStart(e: DragEvent, issue: Issue) {
    if (!e.dataTransfer) return;
    e.dataTransfer.setData('text/plain', buildDragText(issue.number, issue.title, issue.body, issue.labels));
    e.dataTransfer.setData('application/x-issue-number', String(issue.number));
    e.dataTransfer.effectAllowed = 'copy';
  }

  function launchForIssue(e: MouseEvent, issue: Issue) {
    e.stopPropagation();
    dispatch('launchForIssue', { number: issue.number, title: issue.title, body: issue.body, labels: issue.labels });
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
  <IssueDetailComponent
    issue={selectedIssue}
    {formatDate}
    on:back={goBack}
    on:editIssue
    on:toggleState={toggleState}
    on:submitComment={handleSubmitComment}
  />
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
            {#if paneIssues[issue.number]}
              <span class="activity-dot" class:active={paneIssues[issue.number].activity === 'active'} class:done={paneIssues[issue.number].activity === 'done'} class:needs-input={paneIssues[issue.number].activity === 'needsInput'} title="Agent: {paneIssues[issue.number].activity}">●</span>
            {:else}
              {issue.state === 'OPEN' ? '●' : '✓'}
            {/if}
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
              {#if paneIssues[issue.number]?.cost}
                <span class="issue-cost">{paneIssues[issue.number].cost}</span>
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
          <div class="issue-actions">
            {#if issue.url}
              <button class="action-btn open-btn" on:click|stopPropagation={() => BrowserOpenURL(issue.url)} title="Im Browser öffnen">&#8599;</button>
            {/if}
            {#if issue.state === 'OPEN' && !paneIssues[issue.number]}
              <button class="action-btn launch-btn" on:click={(e) => launchForIssue(e, issue)} title="Claude für dieses Issue starten">▶</button>
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

  .issue-actions { display: flex; gap: 2px; flex-shrink: 0; align-items: center; }
  .action-btn {
    opacity: 0; background: none; border: none; color: var(--fg-muted); cursor: pointer;
    font-size: 14px; padding: 2px 6px; border-radius: 4px;
    transition: opacity 0.15s, background 0.15s, color 0.15s;
  }
  .issue-item:hover .action-btn { opacity: 1; }
  .action-btn:hover { background: var(--bg-tertiary); color: var(--fg); }
  .action-btn.launch-btn { color: var(--accent); }
  .action-btn.open-btn { font-size: 13px; }

  .activity-dot { font-size: 12px; animation: pulse 2s infinite; }
  .activity-dot.active { color: var(--accent); }
  .activity-dot.done { color: var(--success); animation: none; }
  .activity-dot.needs-input { color: var(--warning); }
  .issue-cost { color: var(--warning); font-weight: 600; }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }
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

</style>
