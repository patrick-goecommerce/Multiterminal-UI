<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { get } from 'svelte/store';
  import { config } from '../stores/config';
  import { t, setLanguage, LANGUAGES, type Language } from '../stores/i18n';
  import { applyAccentColor, applyTheme } from '../stores/theme';
  import type { ThemeName } from '../stores/theme';
  import * as App from '../../wailsjs/go/backend/App';
  import ColorPicker from './ColorPicker.svelte';
  import { playBell } from '../lib/audio';
  import { MONOSPACE_FONTS, isFontAvailable } from '../lib/terminal';

  export let visible: boolean = false;

  const dispatch = createEventDispatcher();

  const availableThemes: { value: ThemeName; label: string }[] = [
    { value: 'dark', label: 'Dark (Catppuccin Mocha)' },
    { value: 'light', label: 'Light' },
    { value: 'dracula', label: 'Dracula' },
    { value: 'nord', label: 'Nord' },
    { value: 'solarized', label: 'Solarized Dark' },
  ];

  let colorValue = $config.terminal_color || '#39ff14';
  let selectedTheme: ThemeName = ($config.theme as ThemeName) || 'dark';
  let savedTheme: ThemeName = selectedTheme;
  let loggingEnabled = $config.logging_enabled || false;
  let logPath = '';

  let dialogEl: HTMLDivElement;

  // Claude CLI
  let claudeEnabled = $config.claude_enabled !== false;
  let claudeCommand = $config.claude_command || '';
  let claudeStatus: 'unknown' | 'found' | 'notfound' = 'unknown';
  let claudeStatusPath = '';

  // Codex CLI
  let codexEnabled = $config.codex_enabled || false;
  let codexCommand = $config.codex_command || '';
  let codexStatus: 'unknown' | 'found' | 'notfound' = 'unknown';
  let codexStatusPath = '';

  // Gemini CLI
  let geminiEnabled = $config.gemini_enabled || false;
  let geminiCommand = $config.gemini_command || '';
  let geminiStatus: 'unknown' | 'found' | 'notfound' = 'unknown';
  let geminiStatusPath = '';

  let audioEnabled = $config.audio?.enabled ?? true;
  let audioWhenFocused = $config.audio?.when_focused ?? true;
  let audioVolume = $config.audio?.volume ?? 50;
  let audioDoneSound = $config.audio?.done_sound || '';
  let audioInputSound = $config.audio?.input_sound || '';
  let audioErrorSound = $config.audio?.error_sound || '';
  let keepAliveEnabled = $config.keep_alive?.enabled ?? true;
  let keepAliveInterval = $config.keep_alive?.interval_minutes ?? 300;
  let keepAliveMessage = $config.keep_alive?.message ?? 'Hi!';

  let selectedLang: Language = ($config.language || 'de') as Language;

  let fontFamily = $config.font_family || '';
  let fontSize = $config.font_size || 10;
  let savedFontFamily = fontFamily;
  let savedFontSize = fontSize;
  let availableFonts: { name: string; available: boolean }[] = [];

  let statusLineEnabled = $config.status_line?.enabled ?? false;
  let statusLineTemplate = $config.status_line?.template ?? 'standard';
  let statusLineShowModel = $config.status_line?.show_model ?? true;
  let statusLineShowContext = $config.status_line?.show_context ?? true;
  let statusLineShowCost = $config.status_line?.show_cost ?? true;
  let statusLineShowGitBranch = $config.status_line?.show_git_branch ?? false;
  let statusLineShowDuration = $config.status_line?.show_duration ?? false;
  let statusLineConflictWarning = false;

  $: if (visible) initDialog();

  function initDialog() {
    requestAnimationFrame(() => dialogEl?.focus());
    const c = get(config);
    colorValue = c.terminal_color || '#39ff14';
    selectedTheme = (c.theme as ThemeName) || 'dark';
    savedTheme = selectedTheme;
    loggingEnabled = c.logging_enabled || false;
    useWorktrees = c.use_worktrees || false;
    selectedLang = (c.language || 'de') as Language;
    claudeEnabled = c.claude_enabled !== false;
    claudeCommand = c.claude_command || '';
    codexEnabled = c.codex_enabled || false;
    codexCommand = c.codex_command || '';
    geminiEnabled = c.gemini_enabled || false;
    geminiCommand = c.gemini_command || '';
    audioEnabled = c.audio?.enabled ?? true;
    audioWhenFocused = c.audio?.when_focused ?? true;
    audioVolume = c.audio?.volume ?? 50;
    audioDoneSound = c.audio?.done_sound || '';
    audioInputSound = c.audio?.input_sound || '';
    audioErrorSound = c.audio?.error_sound || '';
    keepAliveEnabled = c.keep_alive?.enabled ?? true;
    keepAliveInterval = c.keep_alive?.interval_minutes ?? 300;
    keepAliveMessage = c.keep_alive?.message ?? 'Hi!';
    fontFamily = c.font_family || '';
    fontSize = c.font_size || 10;
    savedFontFamily = fontFamily;
    savedFontSize = fontSize;
    availableFonts = MONOSPACE_FONTS.map(name => ({
      name,
      available: isFontAvailable(name),
    }));
    App.GetLogPath().then(p => logPath = p).catch(() => {});
    detectClaude();
    detectCodex();
    detectGemini();
    statusLineEnabled = c.status_line?.enabled ?? false;
    statusLineTemplate = c.status_line?.template ?? 'standard';
    statusLineShowModel = c.status_line?.show_model ?? true;
    statusLineShowContext = c.status_line?.show_context ?? true;
    statusLineShowCost = c.status_line?.show_cost ?? true;
    statusLineShowGitBranch = c.status_line?.show_git_branch ?? false;
    statusLineShowDuration = c.status_line?.show_duration ?? false;
    statusLineConflictWarning = false;
    App.GetStatusLineStatus().then(s => {
      statusLineConflictWarning = s.has_existing && !s.is_ours;
    }).catch(() => {});
  }

  function handleColorChange(e: CustomEvent<{ value: string }>) {
    colorValue = e.detail.value;
    applyAccentColor(colorValue);
  }

  function handleThemeChange(e: Event) {
    selectedTheme = (e.target as HTMLSelectElement).value as ThemeName;
    applyTheme(selectedTheme, colorValue);
  }

  function handleLoggingToggle() {
    loggingEnabled = !loggingEnabled;
    if (loggingEnabled) {
      App.EnableLogging(false).then(p => { if (p) logPath = p; });
    } else {
      App.DisableLogging();
    }
  }

  function handleFontFamilyChange(e: Event) {
    fontFamily = (e.target as HTMLSelectElement).value;
    config.update(c => ({ ...c, font_family: fontFamily }));
  }

  async function detectClaude() {
    try {
      const result = await App.DetectClaudePath();
      claudeStatus = result.valid ? 'found' : 'notfound';
      claudeStatusPath = result.valid ? result.path : '';
    } catch {
      claudeStatus = 'unknown';
      claudeStatusPath = '';
    }
  }

  async function browseClaude() {
    try {
      const path = await App.BrowseForClaude();
      if (path) {
        claudeCommand = path;
        const valid = await App.ValidateClaudePath(path);
        claudeStatus = valid ? 'found' : 'notfound';
        claudeStatusPath = valid ? path : '';
      }
    } catch {}
  }

  async function detectCodex() {
    try {
      const result = await App.DetectCodexPath();
      codexStatus = result.valid ? 'found' : 'notfound';
      codexStatusPath = result.valid ? result.path : '';
    } catch {
      codexStatus = 'unknown';
      codexStatusPath = '';
    }
  }

  async function browseCodex() {
    try {
      const path = await App.BrowseForCodex();
      if (path) {
        codexCommand = path;
        const valid = await App.ValidateCodexPath(path);
        codexStatus = valid ? 'found' : 'notfound';
        codexStatusPath = valid ? path : '';
      }
    } catch {}
  }

  async function detectGemini() {
    try {
      const result = await App.DetectGeminiPath();
      geminiStatus = result.valid ? 'found' : 'notfound';
      geminiStatusPath = result.valid ? result.path : '';
    } catch {
      geminiStatus = 'unknown';
      geminiStatusPath = '';
    }
  }

  async function browseGemini() {
    try {
      const path = await App.BrowseForGemini();
      if (path) {
        geminiCommand = path;
        const valid = await App.ValidateGeminiPath(path);
        geminiStatus = valid ? 'found' : 'notfound';
        geminiStatusPath = valid ? path : '';
      }
    } catch {}
  }

  async function browseAudioFile(target: 'done' | 'input' | 'error') {
    try {
      const path = await App.BrowseForAudioFile();
      if (path) {
        if (target === 'done') audioDoneSound = path;
        else if (target === 'input') audioInputSound = path;
        else audioErrorSound = path;
      }
    } catch {}
  }

  function previewAudio() {
    playBell('done', audioVolume, audioDoneSound || undefined);
  }

  async function save() {
    await setLanguage(selectedLang);
    const updated = {
      ...$config,
      terminal_color: colorValue,
      theme: selectedTheme,
      language: selectedLang,
      logging_enabled: loggingEnabled,
      use_worktrees: useWorktrees,
      claude_enabled: claudeEnabled,
      claude_command: claudeCommand,
      codex_enabled: codexEnabled,
      codex_command: codexCommand,
      gemini_enabled: geminiEnabled,
      gemini_command: geminiCommand,
      font_family: fontFamily,
      font_size: fontSize,
      audio: {
        enabled: audioEnabled,
        volume: audioVolume,
        when_focused: audioWhenFocused,
        done_sound: audioDoneSound,
        input_sound: audioInputSound,
        error_sound: audioErrorSound,
      },
      keep_alive: {
        enabled: keepAliveEnabled,
        interval_minutes: keepAliveInterval,
        message: keepAliveMessage,
      },
      status_line: {
        enabled: statusLineEnabled,
        template: statusLineTemplate,
        show_model: statusLineShowModel,
        show_context: statusLineShowContext,
        show_cost: statusLineShowCost,
        show_git_branch: statusLineShowGitBranch,
        show_duration: statusLineShowDuration,
      },
    };
    config.set(updated);
    try { await App.SaveConfig(updated); } catch (err) { console.error('[SettingsDialog] SaveConfig failed:', err); }
    dispatch('saved');
    dispatch('close');
  }

  function close() {
    applyTheme(savedTheme, $config.terminal_color || '#39ff14');
    config.update(c => ({ ...c, font_family: savedFontFamily, font_size: savedFontSize || 10 }));
    dispatch('close');
  }

  function resetDefault() {
    colorValue = '#39ff14';
    selectedTheme = 'dark';
    applyTheme('dark', '#39ff14');
    fontFamily = '';
    fontSize = 10;
    config.update(c => ({ ...c, font_family: '', font_size: 10 }));
    audioEnabled = true;
    audioWhenFocused = true;
    audioVolume = 50;
    audioDoneSound = '';
    audioInputSound = '';
    audioErrorSound = '';
    keepAliveEnabled = true;
    keepAliveInterval = 300;
    keepAliveMessage = 'Hi!';
    statusLineEnabled = false;
    statusLineTemplate = 'standard';
    statusLineShowModel = true;
    statusLineShowContext = true;
    statusLineShowCost = true;
    statusLineShowGitBranch = false;
    statusLineShowDuration = false;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
    // Only save on Enter when not focused on an input/select/textarea
    const tag = (e.target as HTMLElement)?.tagName;
    if (e.key === 'Enter' && tag !== 'INPUT' && tag !== 'SELECT' && tag !== 'TEXTAREA') save();
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="overlay" on:click={(e) => { if (e.target === e.currentTarget) close(); }}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="dialog" bind:this={dialogEl} tabindex="-1" on:keydown={handleKeydown}>
      <h3>{$t('settings.title')}</h3>

      <div class="setting-group">
        <label class="setting-label" for="theme-select">{$t('settings.theme')}</label>
        <p class="setting-desc">{$t('settings.themeDesc')}</p>
        <select id="theme-select" class="theme-select" value={selectedTheme} on:change={handleThemeChange}>
          {#each availableThemes as th}
            <option value={th.value} selected={th.value === selectedTheme}>{th.label}</option>
          {/each}
        </select>
      </div>

      <div class="setting-group">
        <label class="setting-label" for="lang-select">{$t('setup.languageLabel')}</label>
        <select id="lang-select" class="theme-select" bind:value={selectedLang}>
          {#each LANGUAGES as lang}
            <option value={lang.code}>{lang.label}</option>
          {/each}
        </select>
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">{$t('settings.terminalColor')}</label>
        <p class="setting-desc">{$t('settings.terminalColorDesc')}</p>
        <ColorPicker value={colorValue} on:change={handleColorChange} />
      </div>

      <div class="setting-group">
        <label class="setting-label" for="font-select">{$t('settings.fontFamily')}</label>
        <p class="setting-desc">{$t('settings.fontFamilyDesc')}</p>
        <select id="font-select" class="theme-select" value={fontFamily} on:change={handleFontFamilyChange}>
          <option value="">{$t('settings.fontDefault')}</option>
          {#each availableFonts as font}
            <option value={font.name} disabled={!font.available} style={font.available ? `font-family: '${font.name}', monospace` : ''}>
              {font.name}{font.available ? '' : ` ${$t('settings.fontNotInstalled')}`}
            </option>
          {/each}
        </select>
      </div>

      <div class="setting-group">
        <label class="setting-label" for="font-size">{$t('settings.fontSize')}</label>
        <p class="setting-desc">{$t('settings.fontSizeDesc')}</p>
        <select id="font-size" class="theme-select" bind:value={fontSize}>
          {#each [8, 10, 12, 14, 16, 18, 20] as size}
            <option value={size}>{size}px</option>
          {/each}
        </select>
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">{$t('settings.keepAlive')}</label>
        <p class="setting-desc">{$t('settings.keepAliveDesc')}</p>
        <div class="toggle-row" style="margin-bottom: 12px;">
          <button class="toggle-btn" class:toggle-on={keepAliveEnabled} on:click={() => keepAliveEnabled = !keepAliveEnabled}>
            <span class="toggle-knob"></span>
          </button>
          <span class="toggle-label">{keepAliveEnabled ? $t('settings.active') : $t('settings.inactive')}</span>
        </div>
        {#if keepAliveEnabled}
          <div class="keepalive-fields">
            <label for="keepalive-interval">{$t('settings.keepAliveInterval')}</label>
            <input
              id="keepalive-interval"
              type="number"
              min="1"
              max="1440"
              bind:value={keepAliveInterval}
              class="text-input"
            />
            <label for="keepalive-message">{$t('settings.keepAliveMessage')}</label>
            <input
              id="keepalive-message"
              type="text"
              bind:value={keepAliveMessage}
              class="text-input"
              placeholder="Hi!"
            />
          </div>
        {/if}
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">{$t('settings.statusLine')}</label>
        <p class="setting-desc">{$t('settings.statusLineDesc')}</p>
        {#if statusLineConflictWarning}
          <p class="conflict-warning">{$t('settings.statusLineConflict')}</p>
        {/if}
        <div class="toggle-row">
          <button class="toggle-btn" class:toggle-on={statusLineEnabled} on:click={() => statusLineEnabled = !statusLineEnabled}>
            <span class="toggle-knob"></span>
          </button>
          <span class="toggle-label">{statusLineEnabled ? $t('settings.active') : $t('settings.inactive')}</span>
        </div>
        {#if statusLineEnabled}
          <div class="statusline-fields">
            <label for="sl-template" class="sound-label">Template</label>
            <select id="sl-template" class="theme-select" bind:value={statusLineTemplate}>
              <option value="minimal">Minimal — [Model] | XX%</option>
              <option value="standard">Standard — [Model] ████ XX% | $X.XX</option>
              <option value="extended">{$t('settings.statusLineExtended')}</option>
            </select>
            <div class="sl-checkboxes">
              <label><input type="checkbox" bind:checked={statusLineShowModel} /> {$t('settings.statusLineModel')}</label>
              <label><input type="checkbox" bind:checked={statusLineShowContext} /> {$t('settings.statusLineContext')}</label>
              <label><input type="checkbox" bind:checked={statusLineShowCost} /> {$t('settings.statusLineCost')}</label>
              <label><input type="checkbox" bind:checked={statusLineShowGitBranch} /> Git-Branch</label>
              <label><input type="checkbox" bind:checked={statusLineShowDuration} /> {$t('settings.statusLineDuration')}</label>
            </div>
          </div>
        {/if}
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">{$t('settings.logging')}</label>
        <p class="setting-desc">{$t('settings.loggingDesc')}</p>
        <div class="toggle-row">
          <button class="toggle-btn" class:toggle-on={loggingEnabled} on:click={handleLoggingToggle}>
            <span class="toggle-knob"></span>
          </button>
          <span class="toggle-label">{loggingEnabled ? $t('settings.active') : $t('settings.inactive')}</span>
        </div>
        {#if loggingEnabled && logPath}
          <div class="log-path-row">
            <p class="log-path">{logPath}</p>
            <button class="claude-btn" on:click={() => App.OpenLogDir()} title={$t('settings.openLogDir')}>&#128194;</button>
          </div>
        {/if}
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">{$t('settings.worktrees')}</label>
        <p class="setting-desc">{$t('settings.worktreesDesc')}</p>
        <div class="toggle-row">
          <button class="toggle-btn" class:toggle-on={useWorktrees} on:click={() => useWorktrees = !useWorktrees}>
            <span class="toggle-knob"></span>
          </button>
          <span class="toggle-label">{useWorktrees ? $t('settings.active') : $t('settings.inactive')}</span>
        </div>
      </div>

      <!-- CLI Tools Section -->
      <div class="section-divider">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="section-title">{$t('settings.cliTools')}</label>
        <p class="setting-desc">{$t('settings.cliToolsDesc')}</p>
      </div>

      <!-- Claude CLI -->
      <div class="setting-group cli-group">
        <div class="cli-header">
          <button class="toggle-btn" class:toggle-on={claudeEnabled} on:click={() => claudeEnabled = !claudeEnabled}>
            <span class="toggle-knob"></span>
          </button>
          <!-- svelte-ignore a11y-label-has-associated-control -->
          <label class="setting-label" style="margin-bottom:0">Claude Code</label>
          <span class="cli-badge claude-badge">Anthropic</span>
        </div>
        {#if claudeEnabled}
          <p class="setting-desc">{$t('settings.claudePathDesc')}</p>
          <div class="claude-row">
            <input type="text" class="claude-input" bind:value={claudeCommand} placeholder={$t('settings.claudePlaceholder')} />
            <button class="claude-btn" on:click={browseClaude} title={$t('settings.browse')}>&#128194;</button>
            <button class="claude-btn" on:click={detectClaude} title={$t('settings.detect')}>&#128269;</button>
          </div>
          {#if claudeStatus === 'found'}
            <p class="claude-status found">{$t('settings.found', { path: claudeStatusPath })}</p>
          {:else if claudeStatus === 'notfound'}
            <p class="claude-status notfound">{$t('settings.notFound')}</p>
          {/if}
        {/if}
      </div>

      <!-- Codex CLI -->
      <div class="setting-group cli-group">
        <div class="cli-header">
          <button class="toggle-btn" class:toggle-on={codexEnabled} on:click={() => codexEnabled = !codexEnabled}>
            <span class="toggle-knob"></span>
          </button>
          <!-- svelte-ignore a11y-label-has-associated-control -->
          <label class="setting-label" style="margin-bottom:0">Codex</label>
          <span class="cli-badge codex-badge">OpenAI</span>
        </div>
        {#if codexEnabled}
          <p class="setting-desc">{$t('settings.codexPathDesc')}</p>
          <div class="claude-row">
            <input type="text" class="claude-input" bind:value={codexCommand} placeholder={$t('settings.codexPlaceholder')} />
            <button class="claude-btn" on:click={browseCodex} title={$t('settings.browse')}>&#128194;</button>
            <button class="claude-btn" on:click={detectCodex} title={$t('settings.detect')}>&#128269;</button>
          </div>
          {#if codexStatus === 'found'}
            <p class="claude-status found">{$t('settings.found', { path: codexStatusPath })}</p>
          {:else if codexStatus === 'notfound'}
            <p class="claude-status notfound">{$t('settings.codexNotFound')}</p>
          {/if}
        {/if}
      </div>

      <!-- Gemini CLI -->
      <div class="setting-group cli-group">
        <div class="cli-header">
          <button class="toggle-btn" class:toggle-on={geminiEnabled} on:click={() => geminiEnabled = !geminiEnabled}>
            <span class="toggle-knob"></span>
          </button>
          <!-- svelte-ignore a11y-label-has-associated-control -->
          <label class="setting-label" style="margin-bottom:0">Gemini</label>
          <span class="cli-badge gemini-badge">Google</span>
        </div>
        {#if geminiEnabled}
          <p class="setting-desc">{$t('settings.geminiPathDesc')}</p>
          <div class="claude-row">
            <input type="text" class="claude-input" bind:value={geminiCommand} placeholder={$t('settings.geminiPlaceholder')} />
            <button class="claude-btn" on:click={browseGemini} title={$t('settings.browse')}>&#128194;</button>
            <button class="claude-btn" on:click={detectGemini} title={$t('settings.detect')}>&#128269;</button>
          </div>
          {#if geminiStatus === 'found'}
            <p class="claude-status found">{$t('settings.found', { path: geminiStatusPath })}</p>
          {:else if geminiStatus === 'notfound'}
            <p class="claude-status notfound">{$t('settings.geminiNotFound')}</p>
          {/if}
        {/if}
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">{$t('settings.audio')}</label>
        <p class="setting-desc">{$t('settings.audioDesc')}</p>
        <div class="toggle-row" style="margin-bottom: 12px;">
          <button class="toggle-btn" class:toggle-on={audioEnabled} on:click={() => audioEnabled = !audioEnabled}>
            <span class="toggle-knob"></span>
          </button>
          <span class="toggle-label">{audioEnabled ? $t('settings.active') : $t('settings.inactive')}</span>
        </div>
        {#if audioEnabled}
          <div class="toggle-row" style="margin-bottom: 12px;">
            <button class="toggle-btn" class:toggle-on={audioWhenFocused} on:click={() => audioWhenFocused = !audioWhenFocused}>
              <span class="toggle-knob"></span>
            </button>
            <span class="toggle-label">{$t('settings.audioWhenFocused')}</span>
          </div>
          <div class="volume-row">
            <label class="volume-label" for="audio-volume">{$t('settings.volume')}</label>
            <input id="audio-volume" type="range" min="0" max="100" bind:value={audioVolume} class="volume-slider" />
            <span class="volume-value">{audioVolume}%</span>
            <button class="claude-btn" on:click={previewAudio} title={$t('settings.preview')}>&#9654;</button>
          </div>
          <div class="sound-picker">
            <span class="sound-label">{$t('settings.doneSound')}</span>
            <div class="claude-row">
              <input type="text" class="claude-input" bind:value={audioDoneSound} placeholder={$t('settings.doneSoundDefault')} />
              <button class="claude-btn" on:click={() => browseAudioFile('done')} title={$t('settings.browse')}>&#128194;</button>
              {#if audioDoneSound}
                <button class="claude-btn" on:click={() => audioDoneSound = ''} title={$t('settings.reset')}>&times;</button>
              {/if}
            </div>
          </div>
          <div class="sound-picker">
            <span class="sound-label">{$t('settings.inputSound')}</span>
            <div class="claude-row">
              <input type="text" class="claude-input" bind:value={audioInputSound} placeholder={$t('settings.doneSoundDefault')} />
              <button class="claude-btn" on:click={() => browseAudioFile('input')} title={$t('settings.browse')}>&#128194;</button>
              {#if audioInputSound}
                <button class="claude-btn" on:click={() => audioInputSound = ''} title={$t('settings.reset')}>&times;</button>
              {/if}
            </div>
          </div>
          <div class="sound-picker">
            <span class="sound-label">{$t('settings.errorSound')}</span>
            <div class="claude-row">
              <input type="text" class="claude-input" bind:value={audioErrorSound} placeholder={$t('settings.doneSoundDefault')} />
              <button class="claude-btn" on:click={() => browseAudioFile('error')} title={$t('settings.browse')}>&#128194;</button>
              {#if audioErrorSound}
                <button class="claude-btn" on:click={() => audioErrorSound = ''} title={$t('settings.reset')}>&times;</button>
              {/if}
            </div>
          </div>
        {/if}
      </div>

      <div class="dialog-footer">
        <button class="btn-reset" on:click={resetDefault}>{$t('settings.reset')}</button>
        <div class="footer-right-btns">
          <button class="btn-cancel" on:click={close}>{$t('settings.cancel')}</button>
          <button class="btn-save" on:click={save}>{$t('settings.save')}</button>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed; inset: 0; background: rgba(0, 0, 0, 0.5);
    display: flex; align-items: center; justify-content: center; z-index: 100;
  }

  .dialog {
    background: var(--bg); border: 1px solid var(--border);
    border-radius: 12px; padding: 24px; min-width: 400px;
    max-height: 85vh; overflow-y: auto;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
    outline: none;
  }

  h3 { margin: 0 0 20px; color: var(--fg); font-size: 18px; }
  .setting-group { margin-bottom: 24px; }

  .section-divider {
    margin-bottom: 16px;
    padding-top: 8px;
    border-top: 1px solid var(--border);
  }

  .section-title {
    font-size: 15px; font-weight: 700; color: var(--fg);
    display: block; margin-bottom: 4px;
  }

  .cli-group {
    padding: 12px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
  }

  .cli-header {
    display: flex; align-items: center; gap: 10px; margin-bottom: 8px;
  }

  .cli-badge {
    font-size: 10px; padding: 2px 6px; border-radius: 4px;
    font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px;
  }
  .claude-badge { background: rgba(203, 166, 247, 0.2); color: #cba6f7; }
  .codex-badge { background: rgba(16, 163, 127, 0.2); color: #10a37f; }
  .gemini-badge { background: rgba(66, 133, 244, 0.2); color: #4285f4; }

  .theme-select {
    width: 100%; padding: 8px 12px; background: var(--bg-secondary);
    color: var(--fg); border: 1px solid var(--border); border-radius: 6px;
    font-size: 13px; cursor: pointer; outline: none; appearance: auto;
  }

  .theme-select:hover { border-color: var(--accent); }
  .theme-select:focus { border-color: var(--accent); box-shadow: 0 0 0 2px rgba(203, 166, 247, 0.2); }
  .theme-select option { background: var(--bg-secondary); color: var(--fg); }

  .setting-label { font-size: 14px; font-weight: 600; color: var(--fg); display: block; margin-bottom: 4px; }
  .setting-desc { font-size: 12px; color: var(--fg-muted); margin: 0 0 12px; }
  .setting-desc code {
    background: rgba(255, 255, 255, 0.1); padding: 1px 4px;
    border-radius: 3px; font-size: 11px;
  }

  .dialog-footer { display: flex; justify-content: space-between; align-items: center; }
  .footer-right-btns { display: flex; gap: 8px; }

  .btn-reset {
    padding: 8px 14px; background: var(--bg-tertiary); border: 1px solid var(--accent);
    border-radius: 6px; color: var(--accent); cursor: pointer; font-size: 12px;
  }
  .btn-reset:hover { background: var(--accent); color: #000; }

  .btn-cancel {
    padding: 8px 16px; background: var(--bg-tertiary); border: 1px solid var(--accent);
    border-radius: 6px; color: var(--accent); cursor: pointer; font-size: 13px;
  }
  .btn-cancel:hover { background: var(--accent); color: #000; }

  .btn-save {
    padding: 8px 20px; background: var(--accent); border: 1px solid var(--accent);
    border-radius: 6px; color: #000; cursor: pointer; font-size: 13px; font-weight: 600;
  }
  .btn-save:hover { opacity: 0.9; }

  .toggle-btn {
    width: 44px; height: 24px; border-radius: 12px; border: none;
    background: var(--bg-tertiary); cursor: pointer; position: relative;
    transition: background 0.2s; padding: 0;
  }
  .toggle-btn.toggle-on { background: var(--accent); }

  .toggle-knob {
    position: absolute; top: 2px; left: 2px; width: 20px; height: 20px;
    border-radius: 50%; background: var(--fg); transition: transform 0.2s;
  }
  .toggle-btn.toggle-on .toggle-knob { transform: translateX(20px); }

  .toggle-label { font-size: 13px; color: var(--fg-muted); }

  .log-path-row {
    display: flex; align-items: center; gap: 8px; margin-top: 8px;
  }

  .log-path {
    font-size: 11px; color: var(--fg-muted); margin: 0;
    font-family: monospace; word-break: break-all; opacity: 0.7;
    flex: 1;
  }

  .claude-row {
    display: flex; gap: 6px; align-items: center;
  }

  .claude-input {
    flex: 1; padding: 7px 10px; background: var(--bg-secondary);
    color: var(--fg); border: 1px solid var(--border); border-radius: 6px;
    font-size: 12px; font-family: monospace; outline: none;
  }
  .cli-group .claude-input { background: var(--bg); }
  .claude-input:focus { border-color: var(--accent); }
  .claude-input::placeholder { color: var(--fg-muted); opacity: 0.6; }

  .claude-btn {
    padding: 6px 10px; background: var(--bg-tertiary);
    border: 1px solid var(--border); border-radius: 6px;
    color: var(--fg); cursor: pointer; font-size: 14px; line-height: 1;
  }
  .claude-btn:hover { border-color: var(--accent); }

  .claude-status {
    font-size: 11px; margin: 8px 0 0;
    font-family: monospace; word-break: break-all;
  }
  .claude-status.found { color: #a6e3a1; }
  .claude-status.notfound { color: #f38ba8; }

  .volume-row {
    display: flex; align-items: center; gap: 8px; margin-bottom: 12px;
  }

  .volume-label {
    font-size: 12px; color: var(--fg-muted); white-space: nowrap; min-width: 70px;
  }

  .volume-slider {
    flex: 1; height: 4px; accent-color: var(--accent); cursor: pointer;
  }

  .volume-value {
    font-size: 12px; color: var(--fg-muted); min-width: 36px; text-align: right;
  }

  .sound-picker { margin-bottom: 8px; }

  .sound-label {
    font-size: 12px; color: var(--fg-muted); display: block; margin-bottom: 4px;
  }

  .toggle-row {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    margin-top: 6px;
  }
  .keepalive-fields {
    display: grid;
    grid-template-columns: 140px 1fr;
    gap: 6px 12px;
    align-items: center;
    margin-top: 8px;
  }
  .text-input {
    padding: 7px 10px; background: var(--bg-secondary);
    color: var(--fg); border: 1px solid var(--border); border-radius: 6px;
    font-size: 12px; font-family: inherit; outline: none; width: 100%;
  }
  .text-input:focus { border-color: var(--accent); }
  .keepalive-fields label { font-size: 12px; color: var(--fg-muted); }

  .conflict-warning { font-size: 11px; color: #f9e2af; margin: 0 0 8px; }
  .statusline-fields { margin-top: 10px; display: flex; flex-direction: column; gap: 8px; }
  .sl-checkboxes { display: flex; flex-direction: column; gap: 4px; margin-top: 4px; }
  .sl-checkboxes label { font-size: 13px; color: var(--fg-muted); display: flex; align-items: center; gap: 6px; cursor: pointer; }
</style>
