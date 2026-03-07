<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';

  export let visible = false;
  export let dir = '';

  const dispatch = createEventDispatcher<{
    done: { skillIds: string[] };
    skip: void;
    close: void;
  }>();

  interface SkillInfo {
    id: string;
    name: string;
    description: string;
    category: string;
  }

  let allSkills: SkillInfo[] = [];
  let detected: Set<string> = new Set();
  let selected: Set<string> = new Set();
  let loading = true;

  const categoryLabels: Record<string, string> = {
    frontend: 'Frontend',
    backend: 'Backend',
    data: 'Datenbank',
    devops: 'DevOps',
    quality: 'Qualität',
  };

  const categoryOrder = ['frontend', 'backend', 'data', 'devops', 'quality'];

  function initDialog() {
    loading = true;
    Promise.all([
      App.GetAllSkills(),
      dir ? App.DetectProjectSkills(dir) : Promise.resolve([]),
    ]).then(([skills, detectedIds]) => {
      allSkills = skills || [];
      detected = new Set(detectedIds || []);
      selected = new Set(detectedIds || []);
      loading = false;
    }).catch(() => { loading = false; });
  }

  $: if (visible) initDialog();

  function toggle(id: string) {
    if (selected.has(id)) {
      selected.delete(id);
    } else {
      selected.add(id);
    }
    selected = new Set(selected);
  }

  function selectAllDetected() {
    selected = new Set(detected);
  }

  function selectNone() {
    selected = new Set();
  }

  function handleConfirm() {
    dispatch('done', { skillIds: Array.from(selected) });
  }

  function handleKeydown(e: KeyboardEvent) {
    if (!visible) return;
    if (e.key === 'Escape') dispatch('close');
    if (e.key === 'Enter') handleConfirm();
  }

  function groupedSkills(skills: SkillInfo[]): [string, SkillInfo[]][] {
    const groups: Record<string, SkillInfo[]> = {};
    for (const s of skills) {
      if (!groups[s.category]) groups[s.category] = [];
      groups[s.category].push(s);
    }
    return categoryOrder
      .filter(c => groups[c])
      .map(c => [c, groups[c]]);
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if visible}
  <div class="backdrop" on:click={() => dispatch('close')}>
    <div class="dialog" on:click|stopPropagation>
      <div class="header">
        <h2>Projekt einrichten</h2>
        <p class="subtitle">Skills für KI-Agenten auswählen</p>
        {#if dir}
          <p class="dir-label">{dir}</p>
        {/if}
      </div>

      {#if loading}
        <div class="loading">Projekttyp wird erkannt...</div>
      {:else}
        {#if detected.size > 0}
          <div class="detected-info">
            Erkannter Projekttyp: <strong>{Array.from(detected).map(id => {
              const s = allSkills.find(s => s.id === id);
              return s?.name || id;
            }).join(', ')}</strong>
          </div>
        {/if}

        <div class="skills-grid">
          {#each groupedSkills(allSkills) as [category, skills]}
            <div class="category">
              <div class="category-label">{categoryLabels[category] || category}</div>
              {#each skills as skill}
                <label class="skill-row" class:recommended={detected.has(skill.id)}>
                  <input
                    type="checkbox"
                    checked={selected.has(skill.id)}
                    on:change={() => toggle(skill.id)}
                  />
                  <div class="skill-info">
                    <span class="skill-name">{skill.name}</span>
                    <span class="skill-desc">{skill.description}</span>
                  </div>
                  {#if detected.has(skill.id)}
                    <span class="rec-badge">erkannt</span>
                  {/if}
                </label>
              {/each}
            </div>
          {/each}
        </div>

        <div class="quick-actions">
          <button class="btn-link" on:click={selectAllDetected}>Empfohlene</button>
          <button class="btn-link" on:click={selectNone}>Keine</button>
          <span class="count">{selected.size} ausgewählt</span>
        </div>
      {/if}

      <div class="actions">
        <button class="btn-skip" on:click={() => dispatch('skip')}>Überspringen</button>
        <button class="btn-confirm" on:click={handleConfirm}>
          Übernehmen ({selected.size})
        </button>
      </div>
    </div>
  </div>
{/if}

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
    padding: 1.5rem;
    width: 600px; max-width: 90vw; max-height: 80vh;
    box-shadow: 0 20px 60px rgba(0,0,0,0.5);
    display: flex; flex-direction: column;
  }
  .header { text-align: center; margin-bottom: 1rem; }
  h2 { color: var(--fg, #cdd6f4); font-size: 1.3rem; margin-bottom: 0.25rem; }
  .subtitle { color: var(--fg-muted, #a6adc8); font-size: 0.8rem; }
  .dir-label {
    color: var(--fg-muted, #a6adc8); font-size: 0.75rem;
    font-family: monospace; margin-top: 0.25rem;
  }
  .loading {
    text-align: center; padding: 2rem;
    color: var(--fg-muted, #a6adc8); font-size: 0.9rem;
  }
  .detected-info {
    background: rgba(57, 255, 20, 0.08);
    border: 1px solid rgba(57, 255, 20, 0.2);
    border-radius: 8px; padding: 0.5rem 0.75rem;
    font-size: 0.8rem; color: var(--fg, #cdd6f4);
    margin-bottom: 0.75rem;
  }
  .skills-grid {
    overflow-y: auto; flex: 1;
    max-height: 400px;
    display: flex; flex-direction: column; gap: 0.75rem;
  }
  .category-label {
    font-size: 0.7rem; font-weight: 600;
    color: var(--fg-muted, #a6adc8);
    text-transform: uppercase; letter-spacing: 0.05em;
    margin-bottom: 0.3rem;
  }
  .skill-row {
    display: flex; align-items: center; gap: 0.5rem;
    padding: 0.4rem 0.6rem;
    background: var(--bg, #11111b);
    border: 1px solid var(--border, #45475a);
    border-radius: 6px; cursor: pointer;
    transition: border-color 0.15s;
  }
  .skill-row:hover { border-color: var(--fg-muted, #a6adc8); }
  .skill-row.recommended { border-color: rgba(57, 255, 20, 0.3); }
  .skill-info { flex: 1; min-width: 0; }
  .skill-name {
    display: block; color: var(--fg, #cdd6f4);
    font-size: 0.8rem; font-weight: 500;
  }
  .skill-desc {
    display: block; color: var(--fg-muted, #a6adc8);
    font-size: 0.7rem; white-space: nowrap;
    overflow: hidden; text-overflow: ellipsis;
  }
  .rec-badge {
    font-size: 0.6rem; padding: 1px 5px; border-radius: 3px;
    background: rgba(57, 255, 20, 0.15); color: var(--accent, #39ff14);
    font-weight: 600; text-transform: uppercase; white-space: nowrap;
  }
  input[type="checkbox"] {
    accent-color: var(--accent, #39ff14);
    width: 14px; height: 14px; cursor: pointer; flex-shrink: 0;
  }
  .quick-actions {
    display: flex; align-items: center; gap: 0.75rem;
    margin-top: 0.5rem; padding-top: 0.5rem;
    border-top: 1px solid var(--border, #45475a);
  }
  .btn-link {
    background: none; border: none; color: var(--accent, #39ff14);
    cursor: pointer; font-size: 0.75rem; text-decoration: underline;
  }
  .btn-link:hover { opacity: 0.8; }
  .count { margin-left: auto; color: var(--fg-muted, #a6adc8); font-size: 0.75rem; }
  .actions { display: flex; gap: 0.75rem; justify-content: flex-end; margin-top: 1rem; }
  .btn-skip {
    padding: 0.45rem 1rem; border-radius: 8px;
    background: transparent; border: 1px solid var(--border, #45475a);
    color: var(--fg-muted, #a6adc8); cursor: pointer; font-size: 0.8rem;
  }
  .btn-skip:hover { border-color: var(--fg-muted, #a6adc8); }
  .btn-confirm {
    padding: 0.45rem 1.25rem; border-radius: 8px;
    background: var(--accent, #39ff14); border: none;
    color: #000; font-weight: 600; cursor: pointer; font-size: 0.8rem;
  }
  .btn-confirm:hover { opacity: 0.85; }
</style>
