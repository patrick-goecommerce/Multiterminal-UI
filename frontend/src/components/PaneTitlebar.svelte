<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { t } from '../stores/i18n';
  import { paneDisplayName, type Pane } from '../stores/tabs';
  import * as App from '../../wailsjs/go/backend/App';
  import { fetchBranch } from '../lib/git-polling';
  import { CLAUDE_MODES } from '../lib/claude';
  import { renderQuickActionPrompt } from '../lib/quickActions';
  import { config } from '../stores/config';
  import { now } from '../stores/clock';
  import { formatDuration } from '../lib/duration';

  export let pane: Pane;
  export let paneIndex: number = 0;
  export let queueCount: number = 0;
  export let tabDir: string = '';

  const dispatch = createEventDispatcher();

  let editing = false;
  let editName = '';
  let nameInput: HTMLInputElement;

  // Auto names (most-recently-updated source wins) apply until the user manually renames.
  $: displayName = paneDisplayName(pane);

  // Repository root for the worktree tooltip — fetched once per worktree path
  // (assignment happens inside the .then callback, not read back in this
  // block, so it is not itself tracked as a reactive dependency).
  let mainRepoRoot = '';
  $: if (pane.worktreePath) {
    App.GetMainRepoRoot(pane.worktreePath).then((r) => { mainRepoRoot = r; });
  }

  // Fallback branch badge for panes with no worktree (running directly in the
  // main repo's own working directory) — same async-assignment shape as above.
  let fallbackBranch = '';
  $: if (!pane.worktreePath && tabDir) {
    fetchBranch(tabDir).then((b) => { fallbackBranch = b; });
  }

  function startRename() {
    editName = displayName;
    editing = true;
    requestAnimationFrame(() => {
      nameInput?.focus();
      nameInput?.select();
    });
  }

  function finishRename() {
    editing = false;
    const trimmed = editName.trim();
    if (trimmed && trimmed !== displayName) {
      dispatch('rename', { paneId: pane.id, name: trimmed });
    }
  }

  function handleRenameKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') { e.preventDefault(); finishRename(); }
    if (e.key === 'Escape') { editing = false; }
  }

  function handleQuickAction(prompt: string) {
    const rendered = renderQuickActionPrompt(prompt, pane.branch, pane.targetBranch, pane.worktreePath);
    dispatch('quickAction', { sessionId: pane.sessionId, prompt: rendered });
  }

  function getModeLabel(mode: string): string {
    switch (mode) {
      case 'claude': return 'Claude';
      case 'claude-auto': return 'Claude Auto';
      case 'claude-yolo': return 'YOLO';
      case 'codex': return 'Codex';
      case 'codex-auto': return 'Codex Auto';
      case 'gemini': return 'Gemini';
      case 'gemini-yolo': return 'Gemini SB';
      default: return 'Shell';
    }
  }

  function getModeBadgeClass(mode: string): string {
    switch (mode) {
      case 'claude': return 'badge-claude';
      case 'claude-auto': return 'badge-claude-auto';
      case 'claude-yolo': return 'badge-yolo';
      case 'codex': return 'badge-codex';
      case 'codex-auto': return 'badge-codex-auto';
      case 'gemini': return 'badge-gemini';
      case 'gemini-yolo': return 'badge-gemini-yolo';
      default: return 'badge-shell';
    }
  }

  function getActivityDot(activity: string): string {
    switch (activity) {
      case 'active': return 'dot-active';
      case 'done': return 'dot-done';
      case 'waitingPermission': return 'dot-waiting-permission';
      case 'waitingAnswer': return 'dot-waiting-answer';
      case 'error': return 'dot-error';
      // A sleeping pane is a calm, static state — no pulse, no glow.
      case 'sleeping': return 'dot-sleeping';
      case 'resuming': return 'dot-resuming';
      default: return 'dot-idle';
    }
  }

  // Status badge: the state spelled out so the grid is scannable at a glance.
  $: statusLabel = (() => {
    switch (pane.activity) {
      case 'active': return 'läuft';
      case 'done': return 'fertig';
      case 'waitingPermission': return 'wartet auf dich';
      case 'waitingAnswer': return 'wartet auf dich';
      case 'error': return 'Fehler';
      case 'sleeping': return 'schläft';
      case 'resuming': return 'wacht auf';
      default: return '';
    }
  })();
  $: statusTitle = (() => {
    switch (pane.activity) {
      case 'sleeping': return 'Pane schläft – Prozess beendet, Verlauf erhalten. Klicken oder tippen weckt es auf.';
      case 'resuming': return 'Claude lädt den Verlauf neu – das dauert rund 15 Sekunden.';
      default: return '';
    }
  })();
  $: statusClass = (() => {
    switch (pane.activity) {
      case 'active': return 'status-running';
      case 'done': return 'status-done';
      case 'waitingPermission':
      case 'waitingAnswer': return 'status-waiting';
      case 'error': return 'status-danger';
      case 'sleeping':
      case 'resuming': return 'status-sleeping';
      default: return '';
    }
  })();
  // How long the pane has held its current state. No "vor"/"seit" prefix here
  // (see formatDuration) — the label in front already supplies the tense.
  $: durationLabel = formatDuration(pane.activitySince, $now);
  $: badgeText = statusLabel && durationLabel
    ? `${statusLabel} · ${durationLabel}`
    : statusLabel;
  // sleeping/resuming already carry an explanatory tooltip (statusTitle) — keep
  // that one, since it's more useful than a bare timestamp. Everything else
  // falls back to "<state> seit <time>".
  $: badgeTitle = statusTitle || (pane.activitySince
    ? `${statusLabel} seit ${new Date(pane.activitySince * 1000).toLocaleString('de-DE')}`
    : '');

  const chatModes = ['claude', 'claude-auto', 'claude-yolo', 'codex', 'codex-auto', 'gemini', 'gemini-yolo'];
  $: canChat = chatModes.includes(pane.mode);

  let showIssueActions = false;

  function issueAction(action: string) {
    showIssueActions = false;
    dispatch('issueAction', { paneId: pane.id, sessionId: pane.sessionId, issueNumber: pane.issueNumber, action });
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-static-element-interactions -->
<div class="pane-titlebar"
  class:titlebar-done={pane.activity === 'done'}
  class:titlebar-waiting-permission={pane.activity === 'waitingPermission'}
  class:titlebar-waiting-answer={pane.activity === 'waitingAnswer'}
  class:titlebar-error={pane.activity === 'error'}
>
  <div class="pane-title-left">
    {#if paneIndex > 0}
      <span class="pane-index" title="Ctrl+{paneIndex}">{paneIndex}</span>
    {/if}
    <span class="status-dot {getActivityDot(pane.activity)}"></span>
    {#if editing}
      <input
        class="rename-input"
        type="text"
        bind:value={editName}
        bind:this={nameInput}
        on:blur={finishRename}
        on:keydown={handleRenameKeydown}
        on:click|stopPropagation
      />
    {:else}
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <span class="pane-name" on:dblclick|stopPropagation={startRename} title={$t('titlebar.doubleClickRename')}>{displayName}</span>
    {/if}
    <span class="mode-badge {getModeBadgeClass(pane.mode)}">{getModeLabel(pane.mode)}</span>
    {#if statusLabel}
      <span class="status-badge {statusClass}" title={badgeTitle}>{badgeText}</span>
    {/if}
    {#if pane.background}
      <span class="mode-badge badge-bg">BG</span>
    {/if}
    {#if pane.issueNumber}
      <span class="issue-badge" title="Issue #{pane.issueNumber}: {pane.issueTitle}">#{pane.issueNumber}</span>
    {/if}
    {#if pane.model}
      <span class="model-label">{pane.model}</span>
    {/if}
  </div>
  <div class="pane-title-right">
    <button class="pane-btn commit-btn" on:click|stopPropagation={() => dispatch('commitPush', { paneId: pane.id, sessionId: pane.sessionId })} title="Commit & Push">
      ☁
    </button>
    {#if pane.worktreePath}
      <span class="wt-badge" title={`Repository: ${mainRepoRoot || '?'}\nWorktree: ${pane.worktreePath}\nBasis-Branch: ${pane.targetBranch || '?'}`}>⎇ {pane.branch}</span>
      {#if pane.finishPhase === 'preparing' || pane.finishPhase === 'merging' || pane.finishPhase === 'cleanup'}
        <button class="pane-btn finish-btn spinning" title="Fertigstellen läuft – klicken zum Abbrechen"
          on:click|stopPropagation={() => dispatch('cancelFinish', { sessionId: pane.sessionId })}>◌</button>
      {:else}
        <button class="pane-btn finish-btn" title="Worktree fertigstellen: mergen & aufräumen"
          on:click|stopPropagation={() => dispatch('finishWorktree', { paneId: pane.id, sessionId: pane.sessionId })}>✓</button>
      {/if}
    {:else if fallbackBranch}
      <span class="wt-badge wt-badge-main" title={`Repository: ${tabDir}\nBranch: ${fallbackBranch} (Hauptrepo, kein Worktree)`}>⎇ {fallbackBranch}</span>
    {/if}
    {#if CLAUDE_MODES.has(pane.mode)}
      {#each $config.quick_actions as qa, i (i)}
        <button
          class="pane-btn quick-action-btn"
          title={qa.prompt}
          on:click|stopPropagation={() => handleQuickAction(qa.prompt)}
        >{qa.label}</button>
      {/each}
    {/if}
    {#if pane.issueNumber}
      <div class="issue-actions-wrap">
        <button class="pane-btn issue-actions-btn" on:click|stopPropagation={() => (showIssueActions = !showIssueActions)} title={$t('titlebar.issueActions')}>
          &#8943;
        </button>
        {#if showIssueActions}
          <div class="issue-actions-menu">
            <button on:click|stopPropagation={() => issueAction('commit')}>{$t('titlebar.commitPush')}</button>
            <button on:click|stopPropagation={() => issueAction('pr')}>{$t('titlebar.createPR')}</button>
            <button on:click|stopPropagation={() => issueAction('closeIssue')}>{$t('titlebar.closeIssue')}</button>
          </div>
        {/if}
      </div>
    {/if}
    {#if pane.cost}
      <span class="cost-label">{pane.cost}</span>
    {/if}
    {#if canChat}
      <div class="display-toggle" role="group" aria-label="Anzeige umschalten">
        <button
          class="seg seg-active"
          title="Terminal-Ansicht"
          on:click|stopPropagation={() => {}}
        >
          <svg viewBox="0 0 16 16" width="12" height="12" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M3 4l4 4-4 4"/><path d="M8.5 12.5H13"/></svg>
        </button>
        <button
          class="seg"
          title="Als Chat anzeigen"
          on:click|stopPropagation={() => dispatch('toggleDisplay', { paneId: pane.id })}
        >
          <svg viewBox="0 0 16 16" width="12" height="12" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"><rect x="2.2" y="3" width="11.6" height="8" rx="2"/><path d="M5.5 11v2.2l3-2.2"/></svg>
        </button>
      </div>
    {/if}
    <button class="pane-btn queue-toggle" class:queue-active={queueCount > 0} on:click|stopPropagation={() => dispatch('toggleQueue')} title={$t('titlebar.pipelineQueue')}>
      &#9654;{#if queueCount > 0}<span class="queue-badge">{queueCount}</span>{/if}
    </button>
    <button class="pane-btn" on:click|stopPropagation={() => dispatch('maximize', { paneId: pane.id })} title={$t('titlebar.maximize')}>
      &#x26F6;
    </button>
    <button class="pane-btn close" on:click|stopPropagation={() => dispatch('close', { paneId: pane.id, sessionId: pane.sessionId })} title={$t('titlebar.closePane')}>
      &times;
    </button>
  </div>
</div>

<style>
  .pane-titlebar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 8px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    height: 30px;
    min-height: 30px;
    transition: background 0.3s;
  }

  .titlebar-done { background: var(--status-running-tint); }

  .titlebar-waiting-permission,
  .titlebar-waiting-answer {
    background: var(--status-waiting-tint);
    animation: titlebar-blink-waiting 1.6s ease-in-out infinite;
  }

  .titlebar-error {
    background: var(--status-danger-tint);
  }

  @keyframes titlebar-blink-waiting {
    0%, 100% { background: var(--status-waiting-tint); }
    50% { background: rgba(214, 168, 92, 0.28); }
  }

  .pane-title-left { display: flex; align-items: center; gap: 6px; overflow: hidden; }
  .pane-title-right { display: flex; align-items: center; gap: 4px; flex-shrink: 0; }

  .status-dot {
    width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0;
    transition: background 0.3s;
  }

  .dot-idle { background: var(--status-idle, var(--fg-muted)); }
  .dot-active { background: var(--status-running); animation: dot-spin 1s linear infinite; }
  .dot-done { background: var(--status-running); box-shadow: 0 0 6px var(--status-running); }
  /* Sleeping/waking: grey, static. No animation and no glow — a pane that is
   * off must not compete for attention with one that needs it. */
  .dot-sleeping, .dot-resuming { background: var(--fg-muted); opacity: 0.6; }

  :global(.dot-waiting-permission),
  :global(.dot-waiting-answer) {
    background: var(--status-waiting);
    box-shadow: 0 0 7px var(--status-waiting);
    animation: pulse 1.2s ease-in-out infinite;
  }
  :global(.dot-error) {
    background: var(--status-danger);
  }

  @keyframes dot-spin { 0% { opacity: 0.5; } 50% { opacity: 1; } 100% { opacity: 0.5; } }
  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }

  .pane-name {
    font-size: 12px; color: var(--fg); white-space: nowrap;
    overflow: hidden; text-overflow: ellipsis; cursor: default;
  }

  .rename-input {
    font-size: 12px; color: var(--fg); background: var(--bg-tertiary);
    border: 1px solid var(--accent); border-radius: 3px;
    padding: 1px 4px; outline: none; width: 120px;
  }

  .mode-badge { font-size: 10px; padding: 1px 6px; border-radius: 4px; white-space: nowrap; }
  .badge-shell { background: var(--bg-tertiary); color: var(--fg-muted); }
  .badge-claude { background: #7c3aed33; color: #a78bfa; }
  .badge-yolo { background: #dc262633; color: #f87171; }
  .badge-codex { background: #10a37f33; color: #34d399; }
  .badge-codex-auto { background: #e87b3533; color: #fb923c; }
  .badge-claude-auto { background: #c084fc33; color: #c084fc; }
  .badge-gemini { background: #4285f433; color: #60a5fa; }
  .badge-gemini-yolo { background: #ea433533; color: #f87171; }
  .badge-bg { background: #64748b33; color: #94a3b8; font-size: 9px; }

  /* Status text badge — state spelled out, color-coded */
  .status-badge {
    font-size: 9.5px; font-weight: 600; padding: 2px 7px; border-radius: 5px;
    white-space: nowrap; letter-spacing: 0.01em; flex-shrink: 0;
  }
  .status-badge.status-running { color: var(--status-running); background: var(--status-running-tint); }
  .status-badge.status-done { color: var(--fg-muted); background: var(--bg-tertiary); }
  .status-badge.status-waiting { color: var(--status-waiting); background: var(--status-waiting-tint); }
  .status-badge.status-danger { color: var(--status-danger); background: var(--status-danger-tint); }
  .status-badge.status-sleeping { color: var(--fg-muted); background: var(--bg-tertiary); }

  /* Segmented Terminal | Chat toggle */
  .display-toggle {
    display: flex; align-items: center;
    background: var(--bg); border: 1px solid var(--border); border-radius: 6px;
    overflow: hidden; flex-shrink: 0;
  }
  .display-toggle .seg {
    display: flex; align-items: center; justify-content: center;
    padding: 3px 6px; background: none; border: none; cursor: pointer;
    color: var(--fg-muted); transition: background 0.12s, color 0.12s;
  }
  .display-toggle .seg:hover { color: var(--fg); }
  .display-toggle .seg-active {
    color: var(--status-running); background: var(--status-running-tint);
  }

  .issue-badge {
    font-size: 10px; padding: 1px 6px; border-radius: 4px;
    background: #23863633; color: #22c55e; font-weight: 600; white-space: nowrap;
    cursor: default;
  }

  .model-label { font-size: 10px; color: var(--fg-muted); }
  .cost-label { font-size: 11px; color: var(--warning); font-weight: 500; }

  .pane-btn {
    background: none; border: none; color: var(--fg-muted);
    cursor: pointer; padding: 2px 4px; font-size: 14px;
    line-height: 1; border-radius: 3px;
  }

  .pane-btn:hover { background: var(--bg-tertiary); color: var(--fg); }
  .pane-btn.close:hover { background: var(--error); color: white; }

  .queue-toggle { position: relative; font-size: 10px; }
  .queue-toggle.queue-active { color: var(--accent); }
  .queue-badge {
    position: absolute; top: -4px; right: -4px;
    background: var(--accent); color: var(--bg);
    font-size: 9px; font-weight: 700; min-width: 14px;
    height: 14px; line-height: 14px; text-align: center;
    border-radius: 7px; padding: 0 3px;
  }

  .pane-index {
    font-size: 10px; font-weight: 700; color: var(--fg-muted);
    background: var(--bg-tertiary); width: 16px; height: 16px;
    line-height: 16px; text-align: center; border-radius: 3px; flex-shrink: 0;
  }

  .commit-btn { font-size: 12px !important; font-weight: 700; }
  .commit-btn:hover { color: var(--success) !important; }

  .wt-badge { font-size: 10px; color: var(--accent); background: var(--bg-tertiary); border: 1px solid var(--border); border-radius: 4px; padding: 1px 6px; max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .wt-badge-main { color: var(--fg-muted); }
  .finish-btn { color: #4ade80; font-weight: 700; }
  .finish-btn.spinning { animation: wt-spin 1s linear infinite; }
  @keyframes wt-spin { to { transform: rotate(360deg); } }
  .quick-action-btn { font-size: 12px; }
  .issue-actions-wrap { position: relative; }
  .issue-actions-btn { font-size: 16px !important; letter-spacing: 1px; }
  .issue-actions-menu {
    position: absolute; top: 100%; right: 0; z-index: 50;
    background: var(--bg); border: 1px solid var(--border); border-radius: 6px;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.3); min-width: 140px;
    padding: 4px 0; margin-top: 2px;
  }
  .issue-actions-menu button {
    display: block; width: 100%; padding: 6px 12px; text-align: left;
    background: none; border: none; color: var(--fg); font-size: 12px;
    cursor: pointer; transition: background 0.1s;
  }
  .issue-actions-menu button:hover { background: var(--bg-tertiary); }
</style>
