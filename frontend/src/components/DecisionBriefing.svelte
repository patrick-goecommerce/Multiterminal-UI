<script lang="ts">
  export let briefing: any = null;
  export let visible = false;

  function scopeColor(status: string): string {
    if (status === 'within_limits') return '#22c55e';
    if (status === 'warning') return '#f59e0b';
    return '#ef4444';
  }

  function scopeLabel(status: string): string {
    if (status === 'within_limits') return 'OK';
    if (status === 'warning') return 'Warnung';
    return 'Limit erreicht';
  }

  function riskColor(risk: string): string {
    if (risk === 'high') return '#ef4444';
    if (risk === 'medium') return '#f59e0b';
    return '#22c55e';
  }

  function riskLabel(risk: string): string {
    if (risk === 'high') return 'Hoch';
    if (risk === 'medium') return 'Mittel';
    if (risk === 'none') return 'Keine';
    return 'Niedrig';
  }

  function recColor(rec: string): string {
    if (rec === 'proceed_to_qa') return '#22c55e';
    if (rec === 'needs_human_review') return '#f59e0b';
    return '#ef4444';
  }

  function recLabel(rec: string): string {
    if (rec === 'proceed_to_qa') return 'Weiter zur QA';
    if (rec === 'needs_human_review') return 'Manuelle Pruefung';
    return 'Revert empfohlen';
  }

  function kindLabel(kind: string): string {
    if (kind === 'add') return 'Neu';
    if (kind === 'update') return 'Update';
    return 'Entfernt';
  }
</script>

{#if visible && briefing}
  <div class="briefing-panel">
    <h4 class="briefing-title">Entscheidungsbriefing</h4>

    <!-- Scope -->
    <div class="briefing-section">
      <div class="section-header">
        <span class="section-label">Umfang</span>
        <span class="badge" style="background: {scopeColor(briefing.scope_status)}">{scopeLabel(briefing.scope_status)}</span>
      </div>
      <div class="section-stats">
        <span>{briefing.files_changed} Dateien</span>
        <span class="stat-add">+{briefing.lines_added}</span>
        <span class="stat-del">-{briefing.lines_deleted}</span>
      </div>
    </div>

    <!-- Secrets -->
    {#if briefing.secrets_found?.length > 0}
      <div class="briefing-section warning-section">
        <div class="section-header">
          <span class="section-label">Sicherheit</span>
          <span class="badge" style="background: #ef4444">{briefing.secrets_found.length} Treffer</span>
        </div>
        <ul class="finding-list">
          {#each briefing.secrets_found as secret}
            <li class="finding-item">
              <span class="finding-type">{secret.type}</span>
              <span class="finding-file">{secret.file}:{secret.line}</span>
              <code class="finding-preview">{secret.preview}</code>
            </li>
          {/each}
        </ul>
      </div>
    {/if}

    <!-- Mass Deletes -->
    {#if briefing.mass_deletes}
      <div class="briefing-section warning-section">
        <div class="section-header">
          <span class="section-label">Warnung</span>
          <span class="badge" style="background: #ef4444">Grosse Loeschung</span>
        </div>
      </div>
    {/if}

    <!-- Conflicts -->
    <div class="briefing-section">
      <div class="section-header">
        <span class="section-label">Konflikte</span>
        <span class="badge" style="background: {riskColor(briefing.conflict_risk)}">{riskLabel(briefing.conflict_risk)}</span>
      </div>
      {#if briefing.critical_files?.length > 0}
        <div class="file-list">
          <span class="file-list-label">Kritische Dateien:</span>
          {#each briefing.critical_files as f}
            <span class="file-tag">{f}</span>
          {/each}
        </div>
      {/if}
      {#if briefing.shared_surfaces?.length > 0}
        <div class="file-list">
          <span class="file-list-label">Gemeinsame Dateien:</span>
          {#each briefing.shared_surfaces as f}
            <span class="file-tag shared">{f}</span>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Dependencies -->
    {#if briefing.dependency_risk !== 'none'}
      <div class="briefing-section">
        <div class="section-header">
          <span class="section-label">Abhaengigkeiten</span>
          <span class="badge" style="background: {riskColor(briefing.dependency_risk)}">{riskLabel(briefing.dependency_risk)}</span>
        </div>
        {#if briefing.manifest_changes?.length > 0}
          <table class="manifest-table">
            <thead>
              <tr><th>Paket</th><th>Typ</th><th>Von</th><th>Nach</th></tr>
            </thead>
            <tbody>
              {#each briefing.manifest_changes as ch}
                <tr>
                  <td class="pkg-name">{ch.package}</td>
                  <td><span class="kind-tag">{kindLabel(ch.kind)}</span></td>
                  <td>{ch.from || '-'}</td>
                  <td>{ch.to || '-'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>
    {/if}

    <!-- Recommendation -->
    <div class="briefing-section rec-section">
      <div class="section-header">
        <span class="section-label">Empfehlung</span>
        <span class="badge" style="background: {recColor(briefing.recommendation)}">{recLabel(briefing.recommendation)}</span>
      </div>
      {#if briefing.reasons?.length > 0}
        <ul class="reason-list">
          {#each briefing.reasons as reason}
            <li>{reason}</li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>
{/if}

<style>
  .briefing-panel {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 12px;
    background: var(--bg, #11111b);
    border: 1px solid var(--border, #45475a);
    border-radius: 8px;
  }

  .briefing-title {
    font-size: 0.85rem;
    font-weight: 700;
    color: var(--fg, #cdd6f4);
    margin: 0 0 4px 0;
  }

  .briefing-section {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 6px 8px;
    background: var(--bg-secondary, #1e1e2e);
    border-radius: 6px;
  }

  .warning-section {
    border-left: 3px solid #ef4444;
  }

  .rec-section {
    border-left: 3px solid var(--accent, #39ff14);
  }

  .section-header {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .section-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--fg, #cdd6f4);
  }

  .badge {
    font-size: 0.6rem;
    padding: 1px 6px;
    border-radius: 3px;
    color: #fff;
    font-weight: 600;
    white-space: nowrap;
  }

  .section-stats {
    display: flex;
    gap: 10px;
    font-size: 0.7rem;
    color: var(--fg-muted, #a6adc8);
  }
  .stat-add { color: #22c55e; }
  .stat-del { color: #ef4444; }

  .finding-list, .reason-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .reason-list {
    list-style: disc;
    padding-left: 16px;
  }
  .reason-list li {
    font-size: 0.7rem;
    color: var(--fg-muted, #a6adc8);
  }

  .finding-item {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.65rem;
    color: var(--fg-muted, #a6adc8);
  }
  .finding-type {
    padding: 0 4px;
    border-radius: 3px;
    background: rgba(239, 68, 68, 0.2);
    color: #ef4444;
    font-weight: 600;
  }
  .finding-file {
    color: var(--fg-muted, #a6adc8);
  }
  .finding-preview {
    font-size: 0.6rem;
    color: var(--fg-muted, #a6adc8);
    opacity: 0.7;
  }

  .file-list {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    align-items: center;
  }
  .file-list-label {
    font-size: 0.65rem;
    color: var(--fg-muted, #a6adc8);
  }
  .file-tag {
    font-size: 0.6rem;
    padding: 1px 5px;
    border-radius: 3px;
    background: rgba(166, 173, 200, 0.15);
    color: var(--fg-muted, #a6adc8);
  }
  .file-tag.shared {
    background: rgba(249, 115, 22, 0.15);
    color: #f97316;
  }

  .manifest-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.65rem;
  }
  .manifest-table th {
    text-align: left;
    padding: 2px 6px;
    color: var(--fg-muted, #a6adc8);
    border-bottom: 1px solid var(--border, #45475a);
    font-weight: 600;
  }
  .manifest-table td {
    padding: 2px 6px;
    color: var(--fg, #cdd6f4);
  }
  .pkg-name {
    font-family: monospace;
    font-size: 0.6rem;
  }
  .kind-tag {
    font-size: 0.55rem;
    padding: 0 3px;
    border-radius: 2px;
    background: rgba(139, 92, 246, 0.2);
    color: #8b5cf6;
  }
</style>
