<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { LANGUAGES, type Language } from '../stores/i18n';
  import { t } from '../stores/i18n';
  import { config } from '../stores/config';

  export let visible = false;
  export let claudeDetected = false;
  export let codexDetected = false;
  export let geminiDetected = false;

  const dispatch = createEventDispatcher<{
    finish: { language: Language; claudeEnabled: boolean; codexEnabled: boolean; geminiEnabled: boolean };
    close: void;
  }>();

  let selectedLang: Language = ($config.language || 'de') as Language;
  let claudeEnabled = $config.claude_enabled !== false;
  let codexEnabled = $config.codex_enabled === true;
  let geminiEnabled = $config.gemini_enabled === true;

  function handleFinish() {
    dispatch('finish', {
      language: selectedLang,
      claudeEnabled,
      codexEnabled,
      geminiEnabled,
    });
  }

  function handleKeydown(e: KeyboardEvent) {
    if (!visible) return;
    if (e.key === 'Escape') dispatch('close');
    if (e.key === 'Enter') handleFinish();
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if visible}
  <div class="backdrop" on:click={() => dispatch('close')}>
    <div class="dialog" on:click|stopPropagation>
      <div class="header">
        <div class="logo">MT</div>
        <h2>{$t('setup.title')}</h2>
        <p class="subtitle">{$t('setup.subtitle')}</p>
      </div>

      <div class="section">
        <label class="section-label">{$t('setup.languageLabel')}</label>
        <div class="lang-grid">
          {#each LANGUAGES as lang}
            <button
              class="lang-btn"
              class:active={selectedLang === lang.code}
              on:click={() => { selectedLang = lang.code; dispatch('langChange', { lang: lang.code }); }}
            >
              <span class="lang-flag">{langFlag(lang.code)}</span>
              <span>{lang.label}</span>
            </button>
          {/each}
        </div>
      </div>

      <div class="section">
        <label class="section-label">{$t('setup.cliToolsLabel')}</label>
        <p class="section-desc">{$t('setup.cliToolsDesc')}</p>
        <div class="tools-list">
          <label class="tool-row">
            <input type="checkbox" bind:checked={claudeEnabled} />
            <span class="tool-name">Claude Code</span>
            <span class="badge anthropic">Anthropic</span>
            {#if claudeDetected}<span class="detected">&#10003;</span>{/if}
          </label>
          <label class="tool-row">
            <input type="checkbox" bind:checked={codexEnabled} />
            <span class="tool-name">Codex CLI</span>
            <span class="badge openai">OpenAI</span>
            {#if codexDetected}<span class="detected">&#10003;</span>{/if}
          </label>
          <label class="tool-row">
            <input type="checkbox" bind:checked={geminiEnabled} />
            <span class="tool-name">Gemini CLI</span>
            <span class="badge google">Google</span>
            {#if geminiDetected}<span class="detected">&#10003;</span>{/if}
          </label>
        </div>
      </div>

      <div class="actions">
        <button class="btn-skip" on:click={() => dispatch('close')}>{$t('setup.skipButton')}</button>
        <button class="btn-finish" on:click={handleFinish}>{$t('setup.finishButton')}</button>
      </div>
    </div>
  </div>
{/if}

<script context="module" lang="ts">
  function langFlag(code: string): string {
    const flags: Record<string, string> = {
      de: '\u{1F1E9}\u{1F1EA}',
      en: '\u{1F1EC}\u{1F1E7}',
      it: '\u{1F1EE}\u{1F1F9}',
      es: '\u{1F1EA}\u{1F1F8}',
      fr: '\u{1F1EB}\u{1F1F7}',
    };
    return flags[code] || '';
  }
</script>

<style>
  .backdrop {
    position: fixed; inset: 0; z-index: 9999;
    background: rgba(0,0,0,0.7);
    display: flex; align-items: center; justify-content: center;
  }
  .dialog {
    background: var(--surface, #1e1e2e);
    border: 1px solid var(--border, #45475a);
    border-radius: 16px;
    padding: 2rem;
    width: 480px; max-width: 90vw;
    box-shadow: 0 20px 60px rgba(0,0,0,0.5);
  }
  .header { text-align: center; margin-bottom: 1.5rem; }
  .logo {
    display: inline-flex; align-items: center; justify-content: center;
    width: 48px; height: 48px;
    background: var(--accent, #39ff14);
    color: #000; font-weight: 800; font-size: 18px;
    border-radius: 12px; margin-bottom: 0.75rem;
  }
  h2 { color: var(--fg, #cdd6f4); font-size: 1.4rem; margin-bottom: 0.25rem; }
  .subtitle { color: var(--fg-muted, #a6adc8); font-size: 0.85rem; }

  .section { margin-bottom: 1.25rem; }
  .section-label {
    display: block; font-size: 0.8rem; font-weight: 600;
    color: var(--fg, #cdd6f4); margin-bottom: 0.5rem;
    text-transform: uppercase; letter-spacing: 0.05em;
  }
  .section-desc { color: var(--fg-muted, #a6adc8); font-size: 0.8rem; margin-bottom: 0.5rem; }

  .lang-grid { display: flex; gap: 0.5rem; flex-wrap: wrap; }
  .lang-btn {
    display: flex; align-items: center; gap: 0.4rem;
    padding: 0.4rem 0.75rem;
    background: var(--bg, #11111b); border: 1px solid var(--border, #45475a);
    border-radius: 8px; color: var(--fg, #cdd6f4);
    cursor: pointer; font-size: 0.85rem;
    transition: border-color 0.15s, background 0.15s;
  }
  .lang-btn:hover { border-color: var(--accent, #39ff14); }
  .lang-btn.active {
    border-color: var(--accent, #39ff14);
    background: rgba(57, 255, 20, 0.1);
  }
  .lang-flag { font-size: 1.1rem; }

  .tools-list { display: flex; flex-direction: column; gap: 0.5rem; }
  .tool-row {
    display: flex; align-items: center; gap: 0.6rem;
    padding: 0.5rem 0.75rem;
    background: var(--bg, #11111b);
    border: 1px solid var(--border, #45475a);
    border-radius: 8px; cursor: pointer;
    transition: border-color 0.15s;
  }
  .tool-row:hover { border-color: var(--fg-muted, #a6adc8); }
  .tool-name { flex: 1; color: var(--fg, #cdd6f4); font-size: 0.9rem; }
  .badge {
    font-size: 0.65rem; padding: 2px 6px; border-radius: 4px;
    font-weight: 600; text-transform: uppercase;
  }
  .badge.anthropic { background: rgba(204,120,51,0.2); color: #cc7833; }
  .badge.openai { background: rgba(16,163,127,0.2); color: #10a37f; }
  .badge.google { background: rgba(66,133,244,0.2); color: #4285f4; }
  .detected { color: var(--accent, #39ff14); font-size: 0.9rem; }

  input[type="checkbox"] {
    accent-color: var(--accent, #39ff14);
    width: 16px; height: 16px; cursor: pointer;
  }

  .actions { display: flex; gap: 0.75rem; justify-content: flex-end; margin-top: 1.5rem; }
  .btn-skip {
    padding: 0.5rem 1.25rem; border-radius: 8px;
    background: transparent; border: 1px solid var(--border, #45475a);
    color: var(--fg-muted, #a6adc8); cursor: pointer; font-size: 0.85rem;
  }
  .btn-skip:hover { border-color: var(--fg-muted, #a6adc8); }
  .btn-finish {
    padding: 0.5rem 1.5rem; border-radius: 8px;
    background: var(--accent, #39ff14); border: none;
    color: #000; font-weight: 600; cursor: pointer; font-size: 0.85rem;
    transition: opacity 0.15s;
  }
  .btn-finish:hover { opacity: 0.85; }
</style>
