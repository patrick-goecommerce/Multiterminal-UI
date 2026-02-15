<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { config } from '../stores/config';
  import { applyAccentColor, applyTheme } from '../stores/theme';
  import type { ThemeName } from '../stores/theme';
  import * as App from '../../wailsjs/go/backend/App';
  import ColorPicker from './ColorPicker.svelte';

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

  $: if (visible) {
    colorValue = $config.terminal_color || '#39ff14';
    selectedTheme = ($config.theme as ThemeName) || 'dark';
    savedTheme = selectedTheme;
    loggingEnabled = $config.logging_enabled || false;
    App.GetLogPath().then(p => logPath = p).catch(() => {});
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

  async function save() {
    const updated = { ...$config, terminal_color: colorValue, theme: selectedTheme, logging_enabled: loggingEnabled };
    config.set(updated);
    try { await App.SaveConfig(updated); } catch (err) { console.error('[SettingsDialog] SaveConfig failed:', err); }
    dispatch('close');
  }

  function close() {
    applyTheme(savedTheme, $config.terminal_color || '#39ff14');
    dispatch('close');
  }

  function resetDefault() {
    colorValue = '#39ff14';
    selectedTheme = 'dark';
    applyTheme('dark', '#39ff14');
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
    if (e.key === 'Enter') save();
  }
</script>

<svelte:window on:keydown={visible ? handleKeydown : undefined} />

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="overlay" on:click={close}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation>
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
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="setting-label">Terminal-Farbe</label>
        <p class="setting-desc">Bestimmt Akzentfarbe, Cursor und fokussierte Rahmen.</p>
        <ColorPicker value={colorValue} on:change={handleColorChange} />
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
          <p class="log-path">{logPath}</p>
        {/if}
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
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
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

  .log-path {
    font-size: 11px; color: var(--fg-muted); margin: 8px 0 0;
    font-family: monospace; word-break: break-all; opacity: 0.7;
  }
</style>
