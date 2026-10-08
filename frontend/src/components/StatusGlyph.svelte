<script lang="ts">
  import { GLYPH_LABEL, type GlyphKind } from '../lib/attention';

  export let kind: GlyphKind;
  export let size = 12;
  /** Draw in the muted text colour, where colour would be noise. */
  export let mono = false;

  const COLOR: Record<GlyphKind, string> = {
    permission: 'var(--status-waiting)', question: 'var(--status-waiting)',
    error: 'var(--status-danger)', errorSeen: 'var(--status-idle)',
    done: 'var(--status-done)', doneSeen: 'var(--status-idle)',
    working: 'var(--status-running)', quiet: 'var(--status-idle)',
  };
</script>

<svg class="status-glyph" class:spin={kind === 'working'} width={size} height={size} viewBox="0 0 12 12"
  style="color:{mono ? 'var(--fg-muted)' : COLOR[kind]}" role="img" aria-label={GLYPH_LABEL[kind]}>
  {#if kind === 'permission'}
    <path d="M6 1.2 11.3 10.6H.7Z" fill="currentColor" />
  {:else if kind === 'question'}
    <path d="M6 1.2a4.8 4.8 0 1 0 2.5 8.9l2.6.8-.8-2.5A4.8 4.8 0 0 0 6 1.2Z" fill="currentColor" />
  {:else if kind === 'error' || kind === 'errorSeen'}
    <path d="m2.6 2.6 6.8 6.8m0-6.8-6.8 6.8" stroke="currentColor" stroke-width={kind === 'error' ? 2.4 : 1.4} stroke-linecap="round" fill="none" />
  {:else if kind === 'done' || kind === 'doneSeen'}
    <path d="m1.8 6.4 2.8 2.8 5.6-6" stroke="currentColor" stroke-width={kind === 'done' ? 2.4 : 1.3} stroke-linecap="round" stroke-linejoin="round" fill="none" />
  {:else if kind === 'working'}
    <path d="M6 1.4a4.6 4.6 0 1 1-4.6 4.6" stroke="currentColor" stroke-width="2" stroke-linecap="round" fill="none" />
  {:else}
    <circle cx="6" cy="6" r="2.1" fill="currentColor" />
  {/if}
</svg>

<style>
  .status-glyph { flex: none; display: inline-block; vertical-align: middle; }
  .spin { animation: status-glyph-spin 1.1s linear infinite; }
  @keyframes status-glyph-spin { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) { .spin { animation: none; } }
</style>
