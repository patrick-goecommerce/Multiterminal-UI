<script lang="ts">
  export let branch: string = '';
  export let totalCost: string = '';
  export let tabInfo: string = '';
  export let commitAgeMinutes: number = -1;

  $: commitLabel = (() => {
    if (commitAgeMinutes < 0) return '';
    if (commitAgeMinutes < 1) return 'Letzter Commit: gerade eben';
    if (commitAgeMinutes < 60) return `Letzter Commit: ${commitAgeMinutes}m`;
    const h = Math.floor(commitAgeMinutes / 60);
    const m = commitAgeMinutes % 60;
    return `Letzter Commit: ${h}h ${m}m`;
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
    <span class="footer-item">{tabInfo}</span>
    {#if totalCost}
      <span class="footer-item cost">
        <span class="label">total:</span> {totalCost}
      </span>
    {/if}
  </div>
  <div class="footer-center">
    {#if commitLabel}
      <span class="commit-age {commitClass}">{commitLabel}</span>
    {/if}
  </div>
  <div class="footer-right">
    <span class="shortcut">Ctrl+N:new</span>
    <span class="shortcut">Ctrl+1-9:pane</span>
    <span class="shortcut">Ctrl+F:search</span>
    <span class="shortcut">Ctrl+Z:zoom</span>
    <span class="shortcut">Ctrl+B:files</span>
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

  .shortcut {
    color: var(--fg-muted);
    font-family: monospace;
    font-size: 12px;
  }
</style>
