<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let value: string = '#39ff14';

  const dispatch = createEventDispatcher();

  let hexInput = value;

  $: hexInput = value;

  const presets = [
    { color: '#39ff14', label: 'Toxic Green' },
    { color: '#00ff41', label: 'Matrix Green' },
    { color: '#00ffff', label: 'Cyan' },
    { color: '#ff6600', label: 'Orange' },
    { color: '#cba6f7', label: 'Lila' },
    { color: '#f43f5e', label: 'Rose' },
    { color: '#facc15', label: 'Gold' },
    { color: '#38bdf8', label: 'Sky Blue' },
  ];

  function emitChange(newValue: string) {
    value = newValue;
    hexInput = newValue;
    dispatch('change', { value: newValue });
  }

  function handleColorInput(e: Event) {
    emitChange((e.target as HTMLInputElement).value);
  }

  function handleHexInput(e: Event) {
    let val = (e.target as HTMLInputElement).value.trim();
    if (!val.startsWith('#')) val = '#' + val;
    hexInput = val;
    if (/^#[0-9a-fA-F]{6}$/.test(val)) emitChange(val);
  }
</script>

<div class="color-row">
  <input type="color" class="color-picker" {value} on:input={handleColorInput} />
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
  <div class="color-preview" style="background: {value}"></div>
</div>

<div class="preset-row">
  {#each presets as p}
    <button class="preset" style="background: {p.color}" on:click={() => emitChange(p.color)} title={p.label}></button>
  {/each}
</div>

<style>
  .color-row { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }

  .color-picker {
    width: 48px; height: 48px; border: 2px solid var(--border);
    border-radius: 8px; cursor: pointer; padding: 2px; background: var(--bg-secondary);
  }

  .color-picker::-webkit-color-swatch-wrapper { padding: 0; }
  .color-picker::-webkit-color-swatch { border: none; border-radius: 5px; }

  .hex-group {
    display: flex; align-items: center; background: var(--bg-secondary);
    border: 1px solid var(--border); border-radius: 6px; padding: 0 8px; height: 40px;
  }

  .hex-hash { color: var(--fg-muted); font-family: monospace; font-size: 14px; }

  .hex-input {
    width: 80px; background: none; border: none; color: var(--fg);
    font-family: monospace; font-size: 14px; outline: none;
    text-transform: uppercase; letter-spacing: 1px;
  }

  .color-preview {
    width: 48px; height: 48px; border-radius: 8px;
    border: 2px solid var(--border); flex-shrink: 0;
    box-shadow: 0 0 16px currentColor;
  }

  .preset-row { display: flex; gap: 8px; flex-wrap: wrap; }

  .preset {
    width: 28px; height: 28px; border-radius: 50%;
    border: 2px solid var(--border); cursor: pointer;
    transition: transform 0.15s, border-color 0.15s;
  }

  .preset:hover { transform: scale(1.2); border-color: var(--fg); }
</style>
