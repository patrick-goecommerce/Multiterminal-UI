<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { t } from '../stores/i18n';

  export let branch: string = '';
  export let totalCost: string = '';
  export let tabInfo: string = '';
  export let commitAgeMinutes: number = -1;
  export let conflictCount: number = 0;
  export let conflictOperation: string = '';
  export let updateAvailable: boolean = false;
  export let latestVersion: string = '';
  export let downloadURL: string = '';
  export let projectInitialized: boolean = false;

  const dispatch = createEventDispatcher();

  $: commitLabel = (() => {
    if (commitAgeMinutes < 0) return '';
    if (commitAgeMinutes < 1) return $t('footer.commitJustNow');
    if (commitAgeMinutes < 60) return $t('footer.commitMinutes', { min: commitAgeMinutes });
    const h = Math.floor(commitAgeMinutes / 60);
    const m = commitAgeMinutes % 60;
    return $t('footer.commitHours', { h, m });
  })();

  $: conflictLabel = (() => {
    if (conflictCount <= 0) return '';
    const op = conflictOperation
      ? ` (${conflictOperation === 'cherry-pick' ? 'Cherry-Pick' : conflictOperation.charAt(0).toUpperCase() + conflictOperation.slice(1)})`
      : '';
    return $t('footer.conflicts', { count: conflictCount, op });
  })();

  $: commitClass = (() => {
    if (commitAgeMinutes < 0) return '';
    if (commitAgeMinutes < 15) return 'commit-green';
    if (commitAgeMinutes < 30) return 'commit-yellow';
    return 'commit-red';
  })();
</script>

<div class="footer">
  <div class="footer-left">
    {#if branch}
      <span class="footer-item branch">
        <span class="label">branch:</span> {branch}
      </span>
    {/if}
    {#if conflictLabel}
      <span class="footer-item conflict-badge">{conflictLabel}</span>
    {/if}
    <span class="footer-item">{tabInfo}</span>
    {#if totalCost}
      <span class="footer-item cost">
        <span class="label">total:</span> {totalCost}
      </span>
    {/if}
    {#if projectInitialized}
      <button class="footer-btn skills-btn" title={$t('footer.editSkills')} on:click={() => dispatch('editSkills')}>
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
        Skills
      </button>
    {/if}
  </div>
  <div class="footer-center">
    {#if commitLabel}
      <span class="commit-age {commitClass}">{commitLabel}</span>
    {/if}
  </div>
  <div class="footer-update">
    {#if updateAvailable && downloadURL}
      <a class="update-link" href={downloadURL} target="_blank" rel="noopener">
        {$t('footer.updateAvailable', { version: latestVersion })}
      </a>
    {/if}
  </div>
  <div class="footer-right">
    <span class="shortcut">{$t('footerShortcuts.new')}</span>
    <span class="shortcut">{$t('footerShortcuts.pane')}</span>
    <span class="shortcut">{$t('footerShortcuts.search')}</span>
    <span class="shortcut">{$t('footerShortcuts.zoom')}</span>
    <span class="shortcut">{$t('footerShortcuts.files')}</span>
    <span class="shortcut">{$t('footerShortcuts.issues')}</span>
  </div>
</div>

<style>
  .footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    background: var(--footer-bg);
    border-top: 1px solid var(--border);
    height: 28px;
    padding: 0 12px;
    font-size: 13px;
    color: var(--fg-muted);
  }

  .footer-left, .footer-right {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .footer-center {
    flex: 1;
    text-align: center;
  }

  .footer-item {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .label {
    color: var(--accent);
  }

  .branch {
    color: var(--success);
  }

  .cost {
    color: var(--warning);
  }

  .commit-age {
    font-weight: 600;
  }

  .commit-green {
    color: #22c55e;
  }

  .commit-yellow {
    color: #eab308;
  }

  .commit-red {
    color: #ef4444;
    animation: commit-pulse 2s ease-in-out infinite;
  }

  @keyframes commit-pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }

  .footer-update {
    display: flex;
    align-items: center;
  }

  .update-link {
    color: #22c55e;
    text-decoration: none;
    font-weight: 600;
    font-size: 12px;
    animation: update-pulse 2s ease-in-out infinite;
  }

  .update-link:hover {
    text-decoration: underline;
  }

  @keyframes update-pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.6; }
  }

  .conflict-badge {
    color: var(--error, #ef4444);
    font-weight: 600;
    animation: commit-pulse 2s ease-in-out infinite;
  }

  .shortcut {
    color: var(--fg-muted);
    font-family: monospace;
    font-size: 12px;
  }

  .footer-btn {
    background: none;
    border: 1px solid transparent;
    color: var(--fg-muted);
    cursor: pointer;
    padding: 1px 6px;
    font-size: 11px;
    border-radius: 3px;
    display: flex;
    align-items: center;
    gap: 4px;
    transition: all 0.15s;
  }

  .footer-btn:hover {
    color: var(--fg);
    border-color: var(--border);
    background: var(--bg-tertiary);
  }

  .skills-btn {
    color: var(--accent);
  }
</style>
