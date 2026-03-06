<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { t } from '../stores/i18n';
  import type { Pane } from '../stores/tabs';
  import WorktreeDropdown from './WorktreeDropdown.svelte';
  import WorktreeCreateDialog from './WorktreeCreateDialog.svelte';

  export let pane: Pane;
  export let paneIndex: number = 0;
  export let queueCount: number = 0;
  export let worktrees: any[] = [];
  export let tabDir: string = '';

  const dispatch = createEventDispatcher();

  let editing = false;
  let editName = '';
  let nameInput: HTMLInputElement;

  let showWorktreeDropdown = false;
  let showWorktreeCreate = false;

  function toggleWorktreeDropdown() {
    showWorktreeDropdown = !showWorktreeDropdown;
  }

  function handleWorktreeSelect(e: CustomEvent) {
    showWorktreeDropdown = false;
    dispatch('openWorktreePane', { worktree: e.detail });
  }

  function handleWorktreeCreated(e: CustomEvent) {
    dispatch('openWorktreePane', { worktree: e.detail });
    dispatch('worktreeListChanged');
  }

  $: displayBranch = pane.branch || pane.issueBranch || '';

  function startRename() {
    editName = pane.name;
    editing = true;
    requestAnimationFrame(() => {
      nameInput?.focus();
      nameInput?.select();
    });
  }

  function finishRename() {
    editing = false;
    const trimmed = editName.trim();
    if (trimmed && trimmed !== pane.name) {
      dispatch('rename', { paneId: pane.id, name: trimmed });
    }
  }

  function handleRenameKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') { e.preventDefault(); finishRename(); }
    if (e.key === 'Escape') { editing = false; }
  }

  function getModeLabel(mode: string): string {
    switch (mode) {
      case 'claude': return 'Claude';
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
      default: return 'dot-idle';
    }
  }

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
      <span class="pane-name" on:dblclick|stopPropagation={startRename} title={$t('titlebar.doubleClickRename')}>{pane.name}</span>
    {/if}
    <span class="mode-badge {getModeBadgeClass(pane.mode)}">{getModeLabel(pane.mode)}</span>
    {#if pane.background}
      <span class="mode-badge badge-bg">BG</span>
    {/if}
    {#if pane.issueNumber}
      <span class="issue-badge" title="Issue #{pane.issueNumber}: {pane.issueTitle}">#{pane.issueNumber}</span>
    {/if}
    <div class="worktree-wrap">
      <button
        class="branch-btn"
        class:has-worktree={!!pane.worktreePath}
        on:click|stopPropagation={toggleWorktreeDropdown}
        title={pane.worktreePath ? `Worktree: ${pane.worktreePath}` : 'Worktree auswählen'}
      >
        <span class="branch-icon">⎇</span>
        {displayBranch || '—'}
        <span class="dropdown-arrow">▾</span>
      </button>
      {#if showWorktreeDropdown}
        <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
        <div class="dropdown-backdrop" on:click={() => showWorktreeDropdown = false}></div>
        <WorktreeDropdown
          {worktrees}
          currentBranch={displayBranch}
          on:select={handleWorktreeSelect}
          on:createNew={() => { showWorktreeDropdown = false; showWorktreeCreate = true; }}
          on:close={() => showWorktreeDropdown = false}
        />
      {/if}
    </div>
    {#if pane.model}
      <span class="model-label">{pane.model}</span>
    {/if}
  </div>
  <div class="pane-title-right">
    <button class="pane-btn commit-btn" on:click|stopPropagation={() => dispatch('commitPush', { paneId: pane.id, sessionId: pane.sessionId })} title="Commit & Push">
      ☁
    </button>
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

<WorktreeCreateDialog
  visible={showWorktreeCreate}
  dir={tabDir}
  on:created={handleWorktreeCreated}
  on:close={() => showWorktreeCreate = false}
/>

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

  .titlebar-done { background: rgba(34, 197, 94, 0.12); }

  .titlebar-waiting-permission {
    background: rgba(245, 166, 35, 0.12);
    animation: titlebar-blink-permission 1.2s ease-in-out infinite;
  }

  .titlebar-waiting-answer {
    background: rgba(232, 135, 90, 0.12);
    animation: titlebar-blink-answer 1.2s ease-in-out infinite;
  }

  .titlebar-error {
    background: rgba(224, 82, 82, 0.12);
  }

  @keyframes titlebar-blink-permission {
    0%, 100% { background: rgba(245, 166, 35, 0.12); }
    50% { background: rgba(245, 166, 35, 0.25); }
  }

  @keyframes titlebar-blink-answer {
    0%, 100% { background: rgba(232, 135, 90, 0.12); }
    50% { background: rgba(232, 135, 90, 0.25); }
  }

  .pane-title-left { display: flex; align-items: center; gap: 6px; overflow: hidden; }
  .pane-title-right { display: flex; align-items: center; gap: 4px; flex-shrink: 0; }

  .status-dot {
    width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0;
    transition: background 0.3s;
  }

  .dot-idle { background: var(--fg-muted); }
  .dot-active { background: var(--accent); animation: dot-spin 1s linear infinite; }
  .dot-done { background: #22c55e; box-shadow: 0 0 6px rgba(34, 197, 94, 0.8); }

  :global(.dot-waiting-permission) {
    background: var(--color-warning, #f5a623);
    animation: pulse 1.2s ease-in-out infinite;
  }
  :global(.titlebar-waiting-permission) {
    border-bottom-color: var(--color-warning, #f5a623);
  }
  :global(.dot-waiting-answer) {
    background: var(--color-warning-soft, #e8875a);
    animation: pulse 1.2s ease-in-out infinite;
  }
  :global(.titlebar-waiting-answer) {
    border-bottom-color: var(--color-warning-soft, #e8875a);
  }
  :global(.dot-error) {
    background: var(--color-error, #e05252);
  }
  :global(.titlebar-error) {
    border-bottom-color: var(--color-error, #e05252);
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
  .badge-gemini { background: #4285f433; color: #60a5fa; }
  .badge-gemini-yolo { background: #ea433533; color: #f87171; }
  .badge-bg { background: #64748b33; color: #94a3b8; font-size: 9px; }

  .issue-badge {
    font-size: 10px; padding: 1px 6px; border-radius: 4px;
    background: #23863633; color: #22c55e; font-weight: 600; white-space: nowrap;
    cursor: default;
  }

  .worktree-wrap { position: relative; }

  .branch-btn {
    display: flex; align-items: center; gap: 3px;
    font-size: 10px; padding: 1px 6px; border-radius: 4px;
    background: var(--bg-tertiary); border: 1px solid var(--border);
    color: var(--fg-muted); cursor: pointer; white-space: nowrap;
    transition: border-color 0.15s;
  }
  .branch-btn:hover { border-color: var(--accent); color: var(--fg); }
  .branch-btn.has-worktree { color: #60a5fa; border-color: #2563eb44; }

  .branch-icon { font-size: 9px; }
  .dropdown-arrow { font-size: 8px; opacity: 0.6; }

  .dropdown-backdrop {
    position: fixed; inset: 0; z-index: 199;
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
