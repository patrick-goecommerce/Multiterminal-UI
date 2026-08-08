<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  import { config } from '../stores/config';
  import { applyAccentColor, applyTheme } from '../stores/theme';
  import type { ThemeName } from '../stores/theme';
  import * as App from '../../wailsjs/go/backend/App';
  import ColorPicker from './ColorPicker.svelte';
  import { playBell } from '../lib/audio';
  import { MONOSPACE_FONTS, isFontAvailable } from '../lib/terminal';
  import { EventsOn } from '../../wailsjs/runtime/runtime';

  export let visible: boolean = false;

  const dispatch = createEventDispatcher();

  const availableThemes: { value: ThemeName; label: string }[] = [
    { value: 'konzept', label: 'Konzept (MTUI)' },
    { value: 'dark', label: 'Dark (Catppuccin Mocha)' },
    { value: 'light', label: 'Light' },
    { value: 'dracula', label: 'Dracula' },
    { value: 'nord', label: 'Nord' },
    { value: 'solarized', label: 'Solarized Dark' },
  ];

  let colorValue = $config.terminal_color || '#39ff14';
  let selectedTheme: ThemeName = ($config.theme as ThemeName) || 'konzept';
  let savedTheme: ThemeName = selectedTheme;
  let chatStyle: string = ($config as any).chat_style || 'claude-code';
  let updateChannel: string = ($config as any).update_channel || 'stable';
  let autoUpdateMinutes: number = ($config as any).auto_update_check_minutes ?? 0;
  let manualCheckStatus: 'idle' | 'checking' | 'done' | 'error' = 'idle';
  let manualCheckMessage = '';
  let sttProvider = ($config as any).stt?.provider || 'cloud-whisper';
  let sttLanguage = ($config as any).stt?.language || 'de';
  let sttBaseUrl = ($config as any).stt?.cloud?.base_url || '';
  let sttModel = ($config as any).stt?.cloud?.model || 'whisper-1';
  let sttApiKey = ($config as any).stt?.cloud?.api_key || '';
  let loggingEnabled = $config.logging_enabled || false;
  let useWorktrees = $config.use_worktrees || false;
  let logPath = '';

  let dialogEl: HTMLDivElement;

  let claudeCommand = $config.claude_command || '';
  let claudeStatus: 'unknown' | 'found' | 'notfound' = 'unknown';
  let claudeStatusPath = '';

  let audioEnabled = $config.audio?.enabled ?? true;
  let audioWhenFocused = $config.audio?.when_focused ?? true;
  let audioVolume = $config.audio?.volume ?? 50;
  let audioDoneSound = $config.audio?.done_sound || '';
  let audioInputSound = $config.audio?.input_sound || '';
  let audioErrorSound = $config.audio?.error_sound || '';

  let autoNamingEnabled = $config.auto_naming?.enabled ?? true;
  let autoNamingModel = $config.auto_naming?.model || 'claude-haiku-4-5';

  let mcpEnabled = ($config as any).mcp_server?.enabled ?? true;
  let mcpPort = ($config as any).mcp_server?.port ?? 51533;
  let mcpLivePort = 0;
  let mcpCopied = false;

  let fontFamily = $config.font_family || '';
  let fontSize = $config.font_size || 10;
  let savedFontFamily = fontFamily;
  let savedFontSize = fontSize;
  let availableFonts: { name: string; available: boolean }[] = [];

  let orchMaxParallel = $config.orchestrator?.max_parallel_agents ?? 3;
  let orchAutoMerge = $config.orchestrator?.default_auto_merge ?? false;
  let orchAutoStart = $config.orchestrator?.default_auto_start ?? false;
  let orchMaxRetries = $config.orchestrator?.max_retries ?? 1;
  let orchReviewCommand = $config.orchestrator?.review_command || '';
  let orchSyncSubtasks = $config.orchestrator?.sync_subtasks_to_github ?? false;

  let finishPrepPrompt = $config.finish_prep_prompt || '';
  let quickActions: { label: string; prompt: string }[] = [];

  // STT engine install status
  let sttStatus: { installed: boolean; bin_found: boolean; model_found: boolean; dir: string; bin_path: string; model_path: string } | null = null;
  let sttInstalling = false;
  let sttInstallError = '';
  let sttProgress: { phase: 'binary' | 'model'; pct: number } | null = null;
  let sttDownloadUnsubscribe: (() => void) | null = null;

  onMount(() => {
    sttDownloadUnsubscribe = EventsOn('stt:download', (payload: any) => {
      if (!payload || typeof payload !== 'object') return;
      sttProgress = {
        phase: (payload.phase === 'model' ? 'model' : 'binary'),
        pct: typeof payload.pct === 'number' ? payload.pct : 0,
      };
    });
  });

  onDestroy(() => {
    if (sttDownloadUnsubscribe) sttDownloadUnsubscribe();
  });

  // IMPORTANT: only `visible` may be referenced in this reactive statement.
  // The body is a function so Svelte does NOT track the dozens of variables it
  // assigns/reads — otherwise editing any field (e.g. font size) re-runs the
  // block and resets every control to the config defaults (recurring bug).
  $: if (visible) initDialog();

  function initDialog() {
    requestAnimationFrame(() => dialogEl?.focus());
    colorValue = $config.terminal_color || '#39ff14';
    selectedTheme = ($config.theme as ThemeName) || 'konzept';
    savedTheme = selectedTheme;
    chatStyle = ($config as any).chat_style || 'claude-code';
    updateChannel = ($config as any).update_channel || 'stable';
    autoUpdateMinutes = ($config as any).auto_update_check_minutes ?? 0;
    manualCheckStatus = 'idle';
    manualCheckMessage = '';
    sttProvider = ($config as any).stt?.provider || 'cloud-whisper';
    sttLanguage = ($config as any).stt?.language || 'de';
    sttBaseUrl = ($config as any).stt?.cloud?.base_url || '';
    sttModel = ($config as any).stt?.cloud?.model || 'whisper-1';
    sttApiKey = ($config as any).stt?.cloud?.api_key || '';
    loggingEnabled = $config.logging_enabled || false;
    useWorktrees = $config.use_worktrees || false;
    claudeCommand = $config.claude_command || '';
    audioEnabled = $config.audio?.enabled ?? true;
    audioWhenFocused = $config.audio?.when_focused ?? true;
    audioVolume = $config.audio?.volume ?? 50;
    audioDoneSound = $config.audio?.done_sound || '';
    audioInputSound = $config.audio?.input_sound || '';
    audioErrorSound = $config.audio?.error_sound || '';
    autoNamingEnabled = $config.auto_naming?.enabled ?? true;
    autoNamingModel = $config.auto_naming?.model || 'claude-haiku-4-5';
    mcpEnabled = ($config as any).mcp_server?.enabled ?? true;
    mcpPort = ($config as any).mcp_server?.port ?? 51533;
    App.GetMCPServerPort().then((p: number) => { mcpLivePort = p; }).catch(() => {});
    fontFamily = $config.font_family || '';
    fontSize = $config.font_size || 10;
    savedFontFamily = fontFamily;
    savedFontSize = fontSize;
    availableFonts = MONOSPACE_FONTS.map(name => ({
      name,
      available: isFontAvailable(name),
    }));
    orchMaxParallel = $config.orchestrator?.max_parallel_agents ?? 3;
    orchAutoMerge = $config.orchestrator?.default_auto_merge ?? false;
    orchAutoStart = $config.orchestrator?.default_auto_start ?? false;
    orchMaxRetries = $config.orchestrator?.max_retries ?? 1;
    orchReviewCommand = $config.orchestrator?.review_command || '';
    orchSyncSubtasks = $config.orchestrator?.sync_subtasks_to_github ?? false;
    finishPrepPrompt = $config.finish_prep_prompt || '';
    quickActions = ($config.quick_actions || []).map((qa) => ({ ...qa }));
    App.GetLogPath().then(p => logPath = p).catch(() => {});
    detectClaude();
  }

  function addQuickAction() {
    if (quickActions.length >= 5) return;
    quickActions = [...quickActions, { label: '⭐', prompt: '' }];
  }

  function removeQuickAction(index: number) {
    quickActions = quickActions.filter((_, i) => i !== index);
  }

  // Isolated reactive: re-runs when `visible` or `sttProvider` change. Writes
  // sttStatus/sttInstallError (not read by the main block above), so re-firing
  // does NOT cascade into a reset of the main block's fields.
  $: if (visible && sttProvider !== 'cloud-whisper') refreshSttStatus(sttProvider);

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
      if (result.valid) {
        claudeStatus = 'found';
        claudeStatusPath = result.path;
      } else {
        claudeStatus = 'notfound';
        claudeStatusPath = '';
      }
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

  async function refreshSttStatus(provider: string) {
    sttStatus = null;
    sttInstallError = '';
    if (provider === 'cloud-whisper') {
      sttStatus = { installed: true, bin_found: true, model_found: true, dir: '', bin_path: '', model_path: '' };
      return;
    }
    try {
      sttStatus = await App.CheckSttEngine(provider);
    } catch (e) {
      sttStatus = null;
      sttInstallError = e instanceof Error ? e.message : String(e);
    }
  }

  async function installStt() {
    if (sttInstalling) return;
    sttInstallError = '';
    sttInstalling = true;
    sttProgress = { phase: 'binary', pct: 0 };
    try {
      await App.InstallSttEngine(sttProvider);
      await refreshSttStatus(sttProvider);
    } catch (e) {
      sttInstallError = e instanceof Error ? e.message : String(e);
    } finally {
      sttInstalling = false;
      sttProgress = null;
    }
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

  async function checkForUpdatesNow() {
    manualCheckStatus = 'checking';
    manualCheckMessage = '';
    try {
      const info = await App.CheckForUpdates();
      manualCheckStatus = 'done';
      manualCheckMessage = info.updateAvailable
        ? `Update v${info.latestVersion} verfügbar`
        : 'Du hast bereits die aktuelle Version.';
    } catch (e) {
      manualCheckStatus = 'error';
      manualCheckMessage = e instanceof Error ? e.message : String(e);
    }
  }

  function copyMcpConfig() {
    const port = mcpLivePort || mcpPort;
    const cmd = `claude mcp add --transport http mtui http://127.0.0.1:${port}/mcp`;
    navigator.clipboard.writeText(cmd).then(() => {
      mcpCopied = true;
      setTimeout(() => { mcpCopied = false; }, 1500);
    }).catch(() => {});
  }

  async function save() {
    const updated = {
      ...$config,
      terminal_color: colorValue,
      theme: selectedTheme,
      chat_style: chatStyle,
      update_channel: updateChannel,
      auto_update_check_minutes: autoUpdateMinutes,
      logging_enabled: loggingEnabled,
      use_worktrees: useWorktrees,
      finish_prep_prompt: finishPrepPrompt,
      quick_actions: quickActions,
      claude_command: claudeCommand,
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
      orchestrator: {
        max_parallel_agents: orchMaxParallel,
        default_auto_merge: orchAutoMerge,
        default_auto_start: orchAutoStart,
        max_retries: orchMaxRetries,
        review_command: orchReviewCommand,
        sync_subtasks_to_github: orchSyncSubtasks,
      },
      stt: {
        provider: sttProvider,
        language: sttLanguage,
        cloud: { base_url: sttBaseUrl, model: sttModel, api_key: sttApiKey },
      },
      auto_naming: {
        enabled: autoNamingEnabled,
        model: autoNamingModel,
      },
      mcp_server: {
        enabled: mcpEnabled,
        port: mcpPort,
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
    chatStyle = 'claude-code';
    updateChannel = 'stable';
    autoUpdateMinutes = 0;
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
    autoNamingEnabled = true;
    autoNamingModel = 'claude-haiku-4-5';
    mcpEnabled = true;
    mcpPort = 51533;
    orchMaxParallel = 3;
    orchAutoMerge = false;
    orchAutoStart = false;
    orchMaxRetries = 1;
    orchReviewCommand = '';
    orchSyncSubtasks = false;
    sttProvider = 'cloud-whisper';
    sttLanguage = 'de';
    sttBaseUrl = '';
    sttModel = 'whisper-1';
    sttApiKey = '';
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
    if (e.key === 'Enter') save();
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="overlay" on:click={close}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation bind:this={dialogEl} tabindex="-1" on:keydown={handleKeydown}>
      <h3>Einstellungen</h3>

      <div class="setting-group">
        <label class="setting-label" for="theme-select">Theme</label>
        <p class="setting-desc">Farbschema der gesamten Oberfläche.</p>
        <select id="theme-select" class="theme-select" value={selectedTheme} on:change={handleThemeChange}>
          {#each availableThemes as t}
            <option value={t.value} selected={t.value === selectedTheme}>{t.label}</option>
          {/each}
        </select>
      </div>

      <div class="setting-group">
        <label class="setting-label" for="chat-style-select">Chat-Stil</label>
        <p class="setting-desc">Aussehen der Chat-Blasen im Chat-Modus.</p>
        <select id="chat-style-select" class="theme-select" bind:value={chatStyle}>
          <option value="claude-code">Claude Code</option>
          <option value="telegram">Telegram</option>
        </select>
      </div>

      <div class="setting-group">
        <label class="setting-label" for="update-channel-select">Updates</label>
        <p class="setting-desc">Release-Kanal und automatische Prüfung auf neue Versionen.</p>
        <select id="update-channel-select" class="theme-select" bind:value={updateChannel}>
          <option value="stable">Stable</option>
          <option value="alpha">Alpha</option>
        </select>
        <label class="setting-label" for="update-interval" style="margin-top: 12px;">Automatisch prüfen (Minuten, 0 = deaktiviert)</label>
        <input id="update-interval" type="number" class="orch-number" min="0" bind:value={autoUpdateMinutes} />
        <button class="install-btn" style="margin-top: 8px;" on:click={checkForUpdatesNow} disabled={manualCheckStatus === 'checking'}>
          {#if manualCheckStatus === 'checking'}Suche läuft…{:else}Jetzt nach Updates suchen{/if}
        </button>
        {#if manualCheckMessage}
          <p class="stt-status {manualCheckStatus === 'error' ? 'missing' : 'installed'}">{manualCheckMessage}</p>
        {/if}
      </div>

      <div class="setting-group">
        <label class="setting-label" for="stt-provider">Spracheingabe (STT)</label>
        <p class="setting-desc">Engine für das Mikrofon-Diktat im Chat.</p>
        <select id="stt-provider" class="theme-select" bind:value={sttProvider} on:change={() => refreshSttStatus(sttProvider)}>
          <option value="cloud-whisper">Cloud (Whisper-API)</option>
          <option value="whisper-cpp">Lokal: whisper.cpp</option>
          <option value="parakeet">Lokal: Parakeet (sherpa-onnx)</option>
        </select>
        <label class="setting-label" for="stt-lang" style="margin-top: 12px;">Sprache</label>
        <select id="stt-lang" class="theme-select" bind:value={sttLanguage}>
          <option value="de">Deutsch</option>
          <option value="en">Englisch</option>
          <option value="auto">Automatisch</option>
        </select>
        {#if sttProvider !== 'cloud-whisper'}
          {#if sttStatus}
            {#if sttStatus.installed}
              <p class="stt-status installed">&#x2705; Installiert ({sttStatus.bin_path})</p>
            {:else}
              <p class="stt-status missing">
                &#x26A0;&#xFE0F; Nicht installiert
                {#if !sttStatus.bin_found && !sttStatus.model_found}
                  (Binary + Modell fehlen)
                {:else if !sttStatus.bin_found}
                  (Binary fehlt)
                {:else}
                  (Modell fehlt)
                {/if}
              </p>
              <button class="install-btn" on:click={installStt} disabled={sttInstalling}>
                {#if sttInstalling}Installation läuft…{:else}Jetzt installieren{/if}
              </button>
            {/if}
          {/if}
          {#if sttInstalling && sttProgress}
            <div class="install-progress">
              <div class="install-progress-label">
                {sttProgress.phase === 'binary' ? 'Binary' : 'Modell'} – {sttProgress.pct}%
              </div>
              <div class="install-progress-bar"><div class="install-progress-fill" style="width: {sttProgress.pct}%"></div></div>
            </div>
          {/if}
          {#if sttInstallError}
            <p class="stt-install-error">{sttInstallError}</p>
            {#if !sttStatus || !sttStatus.installed}
              <button class="install-btn" on:click={installStt} disabled={sttInstalling}>
                {#if sttInstalling}Installation läuft…{:else}Erneut versuchen{/if}
              </button>
            {/if}
          {/if}
        {/if}
        {#if sttProvider === 'cloud-whisper'}
          <label class="setting-label" for="stt-key" style="margin-top: 12px;">API-Key (leer = $OPENAI_API_KEY)</label>
          <input id="stt-key" type="password" class="claude-input" bind:value={sttApiKey} placeholder="sk-…" />
          <label class="setting-label" for="stt-url" style="margin-top: 8px;">Base-URL (leer = OpenAI)</label>
          <input id="stt-url" type="text" class="claude-input" bind:value={sttBaseUrl} placeholder="https://api.openai.com/v1" />
          <label class="setting-label" for="stt-model" style="margin-top: 8px;">Modell</label>
          <input id="stt-model" type="text" class="claude-input" bind:value={sttModel} placeholder="whisper-1" />
        {:else}
          <p class="setting-desc" style="margin-top: 12px;">Lokale Engine lädt Binary + Modell beim ersten Gebrauch nach <code>~/.multiterminal/stt/</code>. Benötigt <code>ffmpeg</code> im PATH.</p>
        {/if}
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">Terminal-Farbe</label>
        <p class="setting-desc">Bestimmt Akzentfarbe, Cursor und fokussierte Rahmen.</p>
        <ColorPicker value={colorValue} on:change={handleColorChange} />
      </div>

      <div class="setting-group">
        <label class="setting-label" for="font-select">Schriftart</label>
        <p class="setting-desc">Monospace-Schriftart für alle Terminals.</p>
        <select id="font-select" class="theme-select" value={fontFamily} on:change={handleFontFamilyChange}>
          <option value="">Standard (Cascadia Code, Fira Code, ...)</option>
          {#each availableFonts as font}
            <option value={font.name} disabled={!font.available} style={font.available ? `font-family: '${font.name}', monospace` : ''}>
              {font.name}{font.available ? '' : ' (nicht installiert)'}
            </option>
          {/each}
        </select>
      </div>

      <div class="setting-group">
        <label class="setting-label" for="font-size">Schriftgröße</label>
        <p class="setting-desc">Basis-Schriftgröße in Pixel. Ctrl+Scroll zum Zoomen pro Pane.</p>
        <select id="font-size" class="theme-select" bind:value={fontSize}>
          {#each [8, 10, 12, 14, 16, 18, 20] as size}
            <option value={size}>{size}px</option>
          {/each}
        </select>
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">Logging</label>
        <p class="setting-desc">Schreibt detaillierte Protokolle in eine Datei. Wird automatisch deaktiviert nach 3 stabilen Starts.</p>
        <div class="toggle-row">
          <button class="toggle-btn" class:toggle-on={loggingEnabled} on:click={handleLoggingToggle}>
            <span class="toggle-knob"></span>
          </button>
          <span class="toggle-label">{loggingEnabled ? 'Aktiv' : 'Inaktiv'}</span>
        </div>
        {#if loggingEnabled && logPath}
          <div class="log-path-row">
            <p class="log-path">{logPath}</p>
            <button class="claude-btn" on:click={() => App.OpenLogDir()} title="Log-Ordner öffnen">&#128194;</button>
          </div>
        {/if}
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">Git Worktrees</label>
        <p class="setting-desc">Erstellt pro Issue ein isoliertes Arbeitsverzeichnis statt nur einen Branch zu wechseln.</p>
        <div class="toggle-row">
          <button class="toggle-btn" class:toggle-on={useWorktrees} on:click={() => useWorktrees = !useWorktrees}>
            <span class="toggle-knob"></span>
          </button>
          <span class="toggle-label">{useWorktrees ? 'Aktiv' : 'Inaktiv'}</span>
        </div>
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">Claude CLI</label>
        <p class="setting-desc">Pfad zur Claude Code CLI. Leer lassen für automatische Erkennung.</p>
        <div class="claude-row">
          <input
            type="text"
            class="claude-input"
            bind:value={claudeCommand}
            placeholder="claude (automatisch)"
          />
          <button class="claude-btn" on:click={browseClaude} title="Durchsuchen">&#128194;</button>
          <button class="claude-btn" on:click={detectClaude} title="Erkennen">&#128269;</button>
        </div>
        {#if claudeStatus === 'found'}
          <p class="claude-status found">Gefunden: {claudeStatusPath}</p>
        {:else if claudeStatus === 'notfound'}
          <p class="claude-status notfound">Nicht gefunden</p>
        {/if}
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">Agent-Orchestrierung</label>
        <p class="setting-desc">Einstellungen für die parallele Ausführung von Claude-Agenten.</p>

        <div class="orch-field">
          <label class="orch-label" for="orch-max-parallel">Max. parallele Agenten</label>
          <input id="orch-max-parallel" type="number" class="orch-number" min="1" max="8" bind:value={orchMaxParallel} />
        </div>

        <div class="orch-field">
          <label class="orch-label" for="orch-max-retries">Max. Wiederholungen</label>
          <input id="orch-max-retries" type="number" class="orch-number" min="0" max="5" bind:value={orchMaxRetries} />
        </div>

        <div class="toggle-row" style="margin-bottom: 10px;">
          <button class="toggle-btn" class:toggle-on={orchAutoMerge} on:click={() => orchAutoMerge = !orchAutoMerge}>
            <span class="toggle-knob"></span>
          </button>
          <span class="toggle-label">Standard: Auto-Merge</span>
        </div>

        <div class="toggle-row" style="margin-bottom: 10px;">
          <button class="toggle-btn" class:toggle-on={orchAutoStart} on:click={() => orchAutoStart = !orchAutoStart}>
            <span class="toggle-knob"></span>
          </button>
          <span class="toggle-label">Standard: Auto-Start</span>
        </div>

        <div class="toggle-row" style="margin-bottom: 10px;">
          <button class="toggle-btn" class:toggle-on={orchSyncSubtasks} on:click={() => orchSyncSubtasks = !orchSyncSubtasks}>
            <span class="toggle-knob"></span>
          </button>
          <span class="toggle-label">Sub-Tickets nach GitHub synchronisieren</span>
        </div>

        <div class="orch-field">
          <label class="orch-label" for="orch-review-cmd">Review-Befehl</label>
          <input id="orch-review-cmd" type="text" class="claude-input" bind:value={orchReviewCommand} placeholder="go test ./... && go vet ./..." />
        </div>
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">Automatische Pane-Benennung</label>
        <p class="setting-desc">Benennt Claude-Panes automatisch anhand des ersten Prompts (per {autoNamingModel}). Manuell vergebene Namen bleiben erhalten.</p>
        <div class="toggle-row" style="margin-bottom: 12px;">
          <button class="toggle-btn" class:toggle-on={autoNamingEnabled} on:click={() => autoNamingEnabled = !autoNamingEnabled}>
            <span class="toggle-knob"></span>
          </button>
          <span class="toggle-label">{autoNamingEnabled ? 'Aktiv' : 'Inaktiv'}</span>
        </div>
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">Agent-Steuerung (MCP-Server)</label>
        <p class="setting-desc">Erlaubt einem Agent in einem MTUI-Pane (z.B. Claude Code), selbstständig neue Sessions zu öffnen, ihnen Prompts zu schicken und sie wieder zu schließen &mdash; z.B. um eine Aufgabe an Codex oder Gemini zu delegieren. Nur lokal erreichbar (127.0.0.1).</p>
        <div class="toggle-row" style="margin-bottom: 12px;">
          <button class="toggle-btn" class:toggle-on={mcpEnabled} on:click={() => mcpEnabled = !mcpEnabled}>
            <span class="toggle-knob"></span>
          </button>
          <span class="toggle-label">{mcpEnabled ? 'Aktiv' : 'Inaktiv'}</span>
        </div>
        {#if mcpEnabled}
          <div class="orch-field">
            <label class="orch-label" for="mcp-port">Port</label>
            <input id="mcp-port" type="number" class="claude-input" bind:value={mcpPort} min="1" max="65535" />
          </div>
          <div class="claude-row" style="margin-top: 8px;">
            <input type="text" class="claude-input" readonly value={`http://127.0.0.1:${mcpLivePort || mcpPort}/mcp`} />
            <button class="claude-btn" on:click={copyMcpConfig} title="claude mcp add-Befehl kopieren">{mcpCopied ? 'Kopiert!' : 'Config kopieren'}</button>
          </div>
        {/if}
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">Audio</label>
        <p class="setting-desc">Akustische Benachrichtigungen wenn Claude fertig ist oder Eingabe braucht.</p>
        <div class="toggle-row" style="margin-bottom: 12px;">
          <button class="toggle-btn" class:toggle-on={audioEnabled} on:click={() => audioEnabled = !audioEnabled}>
            <span class="toggle-knob"></span>
          </button>
          <span class="toggle-label">{audioEnabled ? 'Aktiv' : 'Inaktiv'}</span>
        </div>
        {#if audioEnabled}
          <div class="toggle-row" style="margin-bottom: 12px;">
            <button class="toggle-btn" class:toggle-on={audioWhenFocused} on:click={() => audioWhenFocused = !audioWhenFocused}>
              <span class="toggle-knob"></span>
            </button>
            <span class="toggle-label">Auch bei fokussiertem Fenster</span>
          </div>
          <div class="volume-row">
            <label class="volume-label" for="audio-volume">Lautstärke</label>
            <input id="audio-volume" type="range" min="0" max="100" bind:value={audioVolume} class="volume-slider" />
            <span class="volume-value">{audioVolume}%</span>
            <button class="claude-btn" on:click={previewAudio} title="Vorschau">&#9654;</button>
          </div>
          <div class="sound-picker">
            <span class="sound-label">Fertig-Sound</span>
            <div class="claude-row">
              <input type="text" class="claude-input" bind:value={audioDoneSound} placeholder="Standard (Synthesizer)" />
              <button class="claude-btn" on:click={() => browseAudioFile('done')} title="Durchsuchen">&#128194;</button>
              {#if audioDoneSound}
                <button class="claude-btn" on:click={() => audioDoneSound = ''} title="Zurücksetzen">&times;</button>
              {/if}
            </div>
          </div>
          <div class="sound-picker">
            <span class="sound-label">Eingabe-Sound</span>
            <div class="claude-row">
              <input type="text" class="claude-input" bind:value={audioInputSound} placeholder="Standard (Synthesizer)" />
              <button class="claude-btn" on:click={() => browseAudioFile('input')} title="Durchsuchen">&#128194;</button>
              {#if audioInputSound}
                <button class="claude-btn" on:click={() => audioInputSound = ''} title="Zurücksetzen">&times;</button>
              {/if}
            </div>
          </div>
          <div class="sound-picker">
            <span class="sound-label">Fehler-Sound</span>
            <div class="claude-row">
              <input type="text" class="claude-input" bind:value={audioErrorSound} placeholder="Standard (Synthesizer)" />
              <button class="claude-btn" on:click={() => browseAudioFile('error')} title="Durchsuchen">&#128194;</button>
              {#if audioErrorSound}
                <button class="claude-btn" on:click={() => audioErrorSound = ''} title="Zurücksetzen">&times;</button>
              {/if}
            </div>
          </div>
        {/if}
      </div>

      <div class="setting-group">
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">Quick Actions</label>
        <p class="setting-desc">
          Platzhalter: <code>{'{{branch}}'}</code>, <code>{'{{targetBranch}}'}</code>,
          <code>{'{{worktreePath}}'}</code> (leer, wenn kein Worktree aktiv ist).
        </p>

        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label" style="margin-top: 12px;">Fertigstellen-Prompt (✓-Button)</label>
        <textarea
          class="finish-prompt-input"
          rows="4"
          placeholder="Leer lassen für das Standardverhalten (lokal mergen &amp; aufräumen)"
          bind:value={finishPrepPrompt}
        ></textarea>

        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label" style="margin-top: 12px;">Eigene Quick-Actions ({quickActions.length}/5)</label>
        {#each quickActions as qa, i (i)}
          <div class="quick-action-row">
            <input class="quick-action-label" maxlength="2" bind:value={qa.label} placeholder="🔁" />
            <input class="quick-action-prompt" bind:value={qa.prompt} placeholder="Prompt-Text..." />
            <button class="quick-action-remove" on:click={() => removeQuickAction(i)}>✕</button>
          </div>
        {/each}
        <button class="add-quick-action" on:click={addQuickAction} disabled={quickActions.length >= 5}>
          + Quick Action
        </button>
      </div>

      <div class="dialog-footer">
        <button class="btn-reset" on:click={resetDefault}>Standard</button>
        <div class="footer-right-btns">
          <button class="btn-cancel" on:click={close}>Abbrechen</button>
          <button class="btn-save" on:click={save}>Speichern</button>
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

  .toggle-row { display: flex; align-items: center; gap: 10px; }

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

  .orch-field {
    display: flex; align-items: center; gap: 10px; margin-bottom: 10px;
  }

  .orch-label {
    font-size: 12px; color: var(--fg-muted); min-width: 160px;
  }

  .orch-number {
    width: 70px; padding: 6px 8px; background: var(--bg-secondary);
    color: var(--fg); border: 1px solid var(--border); border-radius: 6px;
    font-size: 12px; outline: none; text-align: center;
  }
  .orch-number:focus { border-color: var(--accent); }

  .stt-status {
    font-size: 12px; margin: 8px 0; padding: 6px 8px;
    border-radius: 4px; font-family: monospace; word-break: break-all;
  }
  .stt-status.installed { color: #a6e3a1; background: rgba(166, 227, 161, 0.08); }
  .stt-status.missing { color: #f9e2af; background: rgba(249, 226, 175, 0.08); }
  .install-btn {
    padding: 6px 14px; background: var(--accent); color: #000; border: none;
    border-radius: 6px; cursor: pointer; font-size: 12px; font-weight: 600;
  }
  .install-btn:hover { opacity: 0.9; }
  .install-btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .install-progress { margin-top: 8px; }
  .install-progress-label { font-size: 11px; color: var(--fg-muted); margin-bottom: 4px; }
  .install-progress-bar {
    height: 6px; background: var(--bg-tertiary); border-radius: 3px; overflow: hidden;
  }
  .install-progress-fill {
    height: 100%; background: var(--accent); transition: width 0.2s;
  }
  .stt-install-error {
    font-size: 11px; color: #f38ba8; margin-top: 8px; word-break: break-word;
  }

  .finish-prompt-input {
    width: 100%; box-sizing: border-box; font-family: inherit; font-size: 12px;
    background: var(--bg-tertiary); border: 1px solid var(--border); border-radius: 6px;
    color: var(--fg); padding: 8px; resize: vertical;
  }
  .quick-action-row { display: flex; gap: 6px; margin: 4px 0; align-items: center; }
  .quick-action-label { width: 40px; text-align: center; }
  .quick-action-prompt { flex: 1; }
  .quick-action-row input {
    background: var(--bg-tertiary); border: 1px solid var(--border); border-radius: 4px;
    color: var(--fg); padding: 5px 8px; font-size: 12px;
  }
  .quick-action-remove {
    background: none; border: none; color: var(--fg-muted); cursor: pointer; font-size: 13px;
  }
  .quick-action-remove:hover { color: var(--error); }
  .add-quick-action {
    margin-top: 6px; padding: 6px 12px; background: var(--bg-tertiary); border: 1px solid var(--border);
    border-radius: 6px; color: var(--fg); cursor: pointer; font-size: 12px;
  }
  .add-quick-action:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
