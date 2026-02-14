<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { config } from '../stores/config';
  import { applyAccentColor, applyTheme } from '../stores/theme';
  import type { ThemeName } from '../stores/theme';
  import * as App from '../../wailsjs/go/backend/App';

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
  let hexInput = colorValue;
  let selectedTheme: ThemeName = ($config.theme as ThemeName) || 'dark';
  let savedTheme: ThemeName = selectedTheme;

  // Sync when dialog opens
  $: if (visible) {
    colorValue = $config.terminal_color || '#39ff14';
    hexInput = colorValue;
    selectedTheme = ($config.theme as ThemeName) || 'dark';
    savedTheme = selectedTheme;
  }

  function handleColorInput(e: Event) {
    const target = e.target as HTMLInputElement;
    colorValue = target.value;
    hexInput = colorValue;
    applyAccentColor(colorValue);
  }

  function handleHexInput(e: Event) {
    const target = e.target as HTMLInputElement;
    let val = target.value.trim();
    if (!val.startsWith('#')) val = '#' + val;
    hexInput = val;
    if (/^#[0-9a-fA-F]{6}$/.test(val)) {
      colorValue = val;
      applyAccentColor(val);
    }
  }

  function handleThemeChange(e: Event) {
    const target = e.target as HTMLSelectElement;
    selectedTheme = target.value as ThemeName;
    applyTheme(selectedTheme, colorValue);
  }

  async function save() {
    const updated = { ...$config, terminal_color: colorValue, theme: selectedTheme };
    config.set(updated);
    try {
      await App.SaveConfig(updated);
    } catch (err) {
      console.error('[SettingsDialog] SaveConfig failed:', err);
    }
    dispatch('close');
  }

  function close() {
    // Revert to saved theme and color
    applyTheme(savedTheme, $config.terminal_color || '#39ff14');
    dispatch('close');
  }

  function resetDefault() {
    colorValue = '#39ff14';
    hexInput = '#39ff14';
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

        <div class="color-row">
          <input
            type="color"
            class="color-picker"
            value={colorValue}
            on:input={handleColorInput}
          />
          <div class="hex-group">
            <span class="hex-hash">#</span>
            <input
              type="text"
              class="hex-input"
              value={hexInput.replace('#', '')}
              on:input={handleHexInput}
              maxlength="6"
              placeholder="39ff14"
            />
          </div>
          <div class="color-preview" style="background: {colorValue}"></div>
        </div>

        <div class="preset-row">
          <button class="preset" style="background: #39ff14" on:click={() => { colorValue = '#39ff14'; hexInput = '#39ff14'; applyAccentColor('#39ff14'); }} title="Toxic Green"></button>
          <button class="preset" style="background: #00ff41" on:click={() => { colorValue = '#00ff41'; hexInput = '#00ff41'; applyAccentColor('#00ff41'); }} title="Matrix Green"></button>
          <button class="preset" style="background: #0ff" on:click={() => { colorValue = '#00ffff'; hexInput = '#00ffff'; applyAccentColor('#00ffff'); }} title="Cyan"></button>
          <button class="preset" style="background: #ff6600" on:click={() => { colorValue = '#ff6600'; hexInput = '#ff6600'; applyAccentColor('#ff6600'); }} title="Orange"></button>
          <button class="preset" style="background: #cba6f7" on:click={() => { colorValue = '#cba6f7'; hexInput = '#cba6f7'; applyAccentColor('#cba6f7'); }} title="Lila"></button>
          <button class="preset" style="background: #f43f5e" on:click={() => { colorValue = '#f43f5e'; hexInput = '#f43f5e'; applyAccentColor('#f43f5e'); }} title="Rose"></button>
          <button class="preset" style="background: #facc15" on:click={() => { colorValue = '#facc15'; hexInput = '#facc15'; applyAccentColor('#facc15'); }} title="Gold"></button>
          <button class="preset" style="background: #38bdf8" on:click={() => { colorValue = '#38bdf8'; hexInput = '#38bdf8'; applyAccentColor('#38bdf8'); }} title="Sky Blue"></button>
        </div>
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
    padding: 24px;
    min-width: 400px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
  }

  h3 {
    margin: 0 0 20px;
    color: var(--fg);
    font-size: 18px;
  }

  .setting-group {
    margin-bottom: 24px;
  }

  .theme-select {
    width: 100%;
    padding: 8px 12px;
    background: var(--bg-secondary);
    color: var(--fg);
    border: 1px solid var(--border);
    border-radius: 6px;
    font-size: 13px;
    cursor: pointer;
    outline: none;
    appearance: auto;
  }

  .theme-select:hover {
    border-color: var(--accent);
  }

  .theme-select:focus {
    border-color: var(--accent);
    box-shadow: 0 0 0 2px rgba(203, 166, 247, 0.2);
  }

  .theme-select option {
    background: var(--bg-secondary);
    color: var(--fg);
  }

  .setting-label {
    font-size: 14px;
    font-weight: 600;
    color: var(--fg);
    display: block;
    margin-bottom: 4px;
  }

  .setting-desc {
    font-size: 12px;
    color: var(--fg-muted);
    margin: 0 0 12px;
  }

  .color-row {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 12px;
  }

  .color-picker {
    width: 48px;
    height: 48px;
    border: 2px solid var(--border);
    border-radius: 8px;
    cursor: pointer;
    padding: 2px;
    background: var(--bg-secondary);
  }

  .color-picker::-webkit-color-swatch-wrapper {
    padding: 0;
  }

  .color-picker::-webkit-color-swatch {
    border: none;
    border-radius: 5px;
  }

  .hex-group {
    display: flex;
    align-items: center;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0 8px;
    height: 40px;
  }

  .hex-hash {
    color: var(--fg-muted);
    font-family: monospace;
    font-size: 14px;
  }

  .hex-input {
    width: 80px;
    background: none;
    border: none;
    color: var(--fg);
    font-family: monospace;
    font-size: 14px;
    outline: none;
    text-transform: uppercase;
    letter-spacing: 1px;
  }

  .color-preview {
    width: 48px;
    height: 48px;
    border-radius: 8px;
    border: 2px solid var(--border);
    flex-shrink: 0;
    box-shadow: 0 0 16px currentColor;
  }

  .preset-row {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .preset {
    width: 28px;
    height: 28px;
    border-radius: 50%;
    border: 2px solid var(--border);
    cursor: pointer;
    transition: transform 0.15s, border-color 0.15s;
  }

  .preset:hover {
    transform: scale(1.2);
    border-color: var(--fg);
  }

  .dialog-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .footer-right-btns {
    display: flex;
    gap: 8px;
  }

  .btn-reset {
    padding: 8px 14px;
    background: var(--bg-tertiary);
    border: 1px solid var(--accent);
    border-radius: 6px;
    color: var(--accent);
    cursor: pointer;
    font-size: 12px;
  }

  .btn-reset:hover {
    background: var(--accent);
    color: #000;
  }

  .btn-cancel {
    padding: 8px 16px;
    background: var(--bg-tertiary);
    border: 1px solid var(--accent);
    border-radius: 6px;
    color: var(--accent);
    cursor: pointer;
    font-size: 13px;
  }

  .btn-cancel:hover {
    background: var(--accent);
    color: #000;
  }

  .btn-save {
    padding: 8px 20px;
    background: var(--accent);
    border: 1px solid var(--accent);
    border-radius: 6px;
    color: #000;
    cursor: pointer;
    font-size: 13px;
    font-weight: 600;
  }

  .btn-save:hover {
    opacity: 0.9;
  }
</style>
