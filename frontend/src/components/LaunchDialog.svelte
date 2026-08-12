<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { config } from '../stores/config';
  import { t } from '../stores/i18n';
  import type { PaneMode } from '../stores/tabs';

  export let visible: boolean = false;
  export let issueContext: { number: number; title: string; body: string; labels: string[] } | null = null;
  export let claudeDetected: boolean = true;
  export let codexDetected: boolean = false;
  export let geminiDetected: boolean = false;

  const dispatch = createEventDispatcher();

  let selectedModel = '';
  let selectedDisplay: 'terminal' | 'chat' = 'terminal';
  let selectedPermissionMode: 'plan' | 'acceptEdits' | 'bypassPermissions' = 'plan';
  let dialogEl: HTMLDivElement;

  // MCP profile (issue #179). Sticky across dialog openings within a session,
  // seeded from the configured default; the pane keeps its choice in the saved
  // session, so a restored pane relaunches with the same profile.
  let selectedMCPProfile: string = $config.default_mcp_profile || '';
  let mcpProfileTouched = false;

  const agentModes: PaneMode[] = ['claude', 'claude-yolo', 'codex', 'codex-auto', 'gemini', 'gemini-yolo'];

  interface LaunchOption {
    mode: PaneMode;
    label: string;
    desc: string;
    icon: string;
    cssClass: string;
  }

  // Build available options based on enabled CLI tools
  $: options = buildOptions(issueContext, $config, $t);

  function buildOptions(issue: typeof issueContext, cfg: typeof $config, _t: any): LaunchOption[] {
    const opts: LaunchOption[] = [];
    if (!issue) {
      opts.push({ mode: 'shell', label: $t('launch.shell'), desc: $t('launch.shellDesc'), icon: '&#9000;', cssClass: '' });
    }
    if (cfg.claude_enabled !== false) {
      opts.push({ mode: 'claude', label: $t('launch.claude'), desc: $t('launch.claudeDesc'), icon: '&#10024;', cssClass: '' });
      opts.push({ mode: 'claude-auto', label: $t('launch.claudeAuto'), desc: $t('launch.claudeAutoDesc'), icon: '&#9881;', cssClass: 'claude-auto' });
      opts.push({ mode: 'claude-yolo', label: $t('launch.claudeYolo'), desc: $t('launch.claudeYoloDesc'), icon: '&#9889;', cssClass: 'yolo' });
    }
    if (cfg.codex_enabled) {
      opts.push({ mode: 'codex', label: $t('launch.codex'), desc: $t('launch.codexDesc'), icon: '&#129302;', cssClass: 'codex' });
      opts.push({ mode: 'codex-auto', label: $t('launch.codexAuto'), desc: $t('launch.codexAutoDesc'), icon: '&#9889;', cssClass: 'codex-auto' });
    }
    if (cfg.gemini_enabled) {
      opts.push({ mode: 'gemini', label: $t('launch.gemini'), desc: $t('launch.geminiDesc'), icon: '&#9733;', cssClass: 'gemini' });
      opts.push({ mode: 'gemini-yolo', label: $t('launch.geminiYolo'), desc: $t('launch.geminiYoloDesc'), icon: '&#9889;', cssClass: 'gemini-yolo' });
    }
    return opts;
  }

  $: if (visible) {
    requestAnimationFrame(() => dialogEl?.focus());
    syncMCPProfile();
  }

  /** Re-seed the MCP profile from config on open (config loads async, after
   *  this component is first created) and drop a selection whose profile was
   *  meanwhile deleted. A function — never inline assignments in a `$:` block,
   *  which would re-run on every referenced variable's change. */
  function syncMCPProfile() {
    const cfg = $config;
    if (!mcpProfileTouched) {
      selectedMCPProfile = cfg.default_mcp_profile || '';
    }
    const known = selectedMCPProfile === '' || selectedMCPProfile === 'none'
      || (cfg.mcp_profiles || []).some((p) => p.name === selectedMCPProfile);
    if (!known) selectedMCPProfile = '';
  }

  function launch(type: PaneMode) {
    const isAgent = agentModes.includes(type);
    const display = isAgent ? selectedDisplay : 'terminal';
    const permissionMode = display === 'chat' ? selectedPermissionMode : 'plan';
    dispatch('launch', {
      type, model: selectedModel, issue: issueContext, display, permissionMode,
      mcpProfile: selectedMCPProfile,
    });
    dispatch('close');
    selectedModel = '';
    selectedDisplay = 'terminal';
    selectedPermissionMode = 'plan';
  }

  function close() {
    dispatch('close');
    selectedModel = '';
    selectedDisplay = 'terminal';
    selectedPermissionMode = 'plan';
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
    const idx = parseInt(e.key) - 1;
    if (idx >= 0 && idx < options.length) {
      launch(options[idx].mode);
    }
  }

  // Show separator between tool groups
  function needsSeparator(i: number): boolean {
    if (i === 0) return false;
    const group = (m: PaneMode) => {
      if (m === 'shell') return 0;
      if (m === 'claude' || m === 'claude-auto' || m === 'claude-yolo') return 1;
      if (m === 'codex' || m === 'codex-auto') return 2;
      return 3;
    };
    return group(options[i - 1].mode) !== group(options[i].mode);
  }

  $: showAgentOptions = options.some(o => agentModes.includes(o.mode));
  $: showClaudeWarning = ($config.claude_enabled !== false) && !claudeDetected;
  $: showCodexWarning = $config.codex_enabled && !codexDetected;
  $: showGeminiWarning = $config.gemini_enabled && !geminiDetected;
  $: allModels = (() => {
    const models: Array<{ label: string; id: string }> = [];
    if ($config.claude_enabled !== false && $config.claude_models?.length) {
      models.push(...$config.claude_models);
    } else if ($config.codex_enabled && $config.codex_models?.length) {
      models.push(...$config.codex_models);
    } else if ($config.gemini_enabled && $config.gemini_models?.length) {
      models.push(...$config.gemini_models);
    }
    return models;
  })();
  $: showModelPicker = allModels.length > 0;
  // MCP flags only exist on the claude CLI, so the picker follows claude panes.
  $: showMCPPicker = $config.claude_enabled !== false
    && options.some(o => o.mode.startsWith('claude'));
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="overlay" on:click={close}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation bind:this={dialogEl} tabindex="-1" on:keydown={handleKeydown}>
      <h3>{issueContext ? $t('launch.titleIssue', { number: issueContext.number }) : $t('launch.titleNew')}</h3>
      {#if issueContext}
        <div class="issue-context">
          <span class="issue-ctx-num">#{issueContext.number}</span>
          <span class="issue-ctx-title">{issueContext.title}</span>
        </div>
      {/if}

      {#if showClaudeWarning}
        <div class="cli-warning">
          <span class="warning-icon">&#9888;</span>
          <span>{$t('launch.claudeNotFound')}</span>
          <button class="warning-link" on:click={() => dispatch('openSettings')}>{$t('launch.settingsLink')}</button>
        </div>
      {/if}

      {#if showCodexWarning}
        <div class="cli-warning">
          <span class="warning-icon">&#9888;</span>
          <span>{$t('launch.codexNotFound')} <code>{$t('launch.codexInstall')}</code></span>
        </div>
      {/if}

      {#if showGeminiWarning}
        <div class="cli-warning">
          <span class="warning-icon">&#9888;</span>
          <span>{$t('launch.geminiNotFound')} <code>{$t('launch.geminiInstall')}</code></span>
        </div>
      {/if}

      <div class="options">
        {#each options as opt, i}
          {#if needsSeparator(i)}
            <div class="separator"></div>
          {/if}
          <button class="option {opt.cssClass}" on:click={() => launch(opt.mode)}>
            <span class="option-key">{i + 1}</span>
            <span class="option-icon">{@html opt.icon}</span>
            <div class="option-text">
              <strong>{opt.label}</strong>
              <span>{opt.desc}</span>
            </div>
          </button>
        {/each}
      </div>

      {#if showModelPicker}
        <div class="model-picker">
          <label>{$t('launch.modelLabel')}</label>
          <select bind:value={selectedModel}>
            {#if $config.claude_enabled !== false}
              <optgroup label="Claude">
                {#each ($config.claude_models || []) as model}
                  <option value={model.id}>{model.label}</option>
                {/each}
              </optgroup>
            {/if}
            {#if $config.codex_enabled && $config.codex_models?.length}
              <optgroup label="Codex">
                {#each $config.codex_models as model}
                  <option value={model.id}>{model.label}</option>
                {/each}
              </optgroup>
            {/if}
            {#if $config.gemini_enabled && $config.gemini_models?.length}
              <optgroup label="Gemini">
                {#each $config.gemini_models as model}
                  <option value={model.id}>{model.label}</option>
                {/each}
              </optgroup>
            {/if}
          </select>
        </div>
      {/if}

      {#if showMCPPicker}
        <div class="mcp-picker">
          <label for="mcp-profile-select">{$t('launch.mcpLabel')}</label>
          <select
            id="mcp-profile-select"
            bind:value={selectedMCPProfile}
            on:change={() => mcpProfileTouched = true}
            title={$t('launch.mcpHint')}
          >
            <option value="">{$t('launch.mcpGlobal')}</option>
            <option value="none">{$t('launch.mcpNone')}</option>
            {#each ($config.mcp_profiles || []) as profile}
              <option value={profile.name}>{profile.name}</option>
            {/each}
          </select>
        </div>
      {/if}

      {#if showAgentOptions}
        <div class="display-picker">
          <label>Anzeige</label>
          <div class="display-toggle">
            <button
              class="toggle-btn {selectedDisplay === 'terminal' ? 'active' : ''}"
              on:click={() => selectedDisplay = 'terminal'}
            >Terminal</button>
            <button
              class="toggle-btn {selectedDisplay === 'chat' ? 'active' : ''}"
              on:click={() => selectedDisplay = 'chat'}
            >Chat</button>
          </div>
        </div>

        {#if selectedDisplay === 'chat'}
          <div class="permission-picker">
            <label>Berechtigungen</label>
            <select bind:value={selectedPermissionMode}>
              <option value="plan">Plan (read-only)</option>
              <option value="acceptEdits">Edits akzeptieren</option>
              <option value="bypassPermissions">Alle Berechtigungen</option>
            </select>
          </div>
        {/if}
      {/if}

      <div class="dialog-footer">
        <button class="cancel-btn" on:click={close}>{$t('launch.cancel')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .dialog {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 20px;
    min-width: 360px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
    outline: none;
  }

  h3 {
    margin: 0 0 16px;
    color: var(--fg);
    font-size: 16px;
  }

  .issue-context {
    display: flex; align-items: center; gap: 8px;
    padding: 8px 12px; margin-bottom: 12px;
    background: var(--bg-secondary); border: 1px solid var(--border);
    border-radius: 8px; font-size: 12px;
  }
  .issue-ctx-num { color: var(--fg-muted); font-weight: 600; }
  .issue-ctx-title { color: var(--fg); font-weight: 500; }

  .options {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 16px;
  }

  .separator {
    height: 1px;
    background: var(--border);
    margin: 4px 0;
  }

  .option {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--fg);
    cursor: pointer;
    text-align: left;
    transition: all 0.15s;
  }

  .option:hover {
    border-color: var(--accent);
    background: var(--bg-tertiary);
  }

  .option.yolo:hover { border-color: var(--error); }
  .option.claude-auto:hover { border-color: #c084fc; }
  .option.codex:hover { border-color: #10a37f; }
  .option.codex-auto:hover { border-color: #e87b35; }
  .option.gemini:hover { border-color: #4285f4; }
  .option.gemini-yolo:hover { border-color: #ea4335; }

  .option-key {
    font-size: 11px;
    padding: 2px 6px;
    background: var(--bg-tertiary);
    border-radius: 4px;
    color: var(--fg-muted);
    font-family: monospace;
  }

  .option-icon { font-size: 20px; }

  .option-text {
    display: flex;
    flex-direction: column;
  }

  .option-text strong { font-size: 14px; }

  .option-text span {
    font-size: 11px;
    color: var(--fg-muted);
  }

  .model-picker {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 16px;
  }

  .model-picker label {
    font-size: 12px;
    color: var(--fg-muted);
  }

  .model-picker select {
    flex: 1;
    padding: 6px 8px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--fg);
    font-size: 12px;
  }

  .mcp-picker {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 12px;
  }

  .mcp-picker label {
    font-size: 12px;
    color: var(--fg-muted);
    white-space: nowrap;
  }

  .mcp-picker select {
    flex: 1;
    padding: 6px 8px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--fg);
    font-size: 12px;
  }

  .dialog-footer {
    display: flex;
    justify-content: flex-end;
  }

  .cancel-btn {
    padding: 6px 14px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--fg-muted);
    cursor: pointer;
    font-size: 12px;
  }

  .cancel-btn:hover { color: var(--fg); }

  .cli-warning {
    display: flex; align-items: center; gap: 8px;
    padding: 8px 12px; margin-bottom: 12px;
    background: rgba(243, 139, 168, 0.1); border: 1px solid rgba(243, 139, 168, 0.4);
    border-radius: 8px; font-size: 12px; color: #f38ba8;
  }

  .cli-warning code {
    background: rgba(255, 255, 255, 0.1);
    padding: 1px 4px;
    border-radius: 3px;
    font-size: 11px;
  }

  .warning-icon { font-size: 16px; }

  .warning-link {
    background: none; border: none; color: var(--accent);
    cursor: pointer; font-size: 12px; text-decoration: underline;
    padding: 0;
  }
  .warning-link:hover { opacity: 0.8; }

  .display-picker {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 12px;
  }

  .display-picker label {
    font-size: 12px;
    color: var(--fg-muted);
    white-space: nowrap;
  }

  .display-toggle {
    display: flex;
    gap: 4px;
  }

  .toggle-btn {
    padding: 4px 12px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--fg-muted);
    cursor: pointer;
    font-size: 12px;
    transition: all 0.15s;
  }

  .toggle-btn.active {
    background: var(--accent);
    border-color: var(--accent);
    color: #fff;
  }

  .toggle-btn:hover:not(.active) {
    border-color: var(--accent);
    color: var(--fg);
  }

  .permission-picker {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 12px;
  }

  .permission-picker label {
    font-size: 12px;
    color: var(--fg-muted);
    white-space: nowrap;
  }

  .permission-picker select {
    flex: 1;
    padding: 6px 8px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--fg);
    font-size: 12px;
  }
</style>
