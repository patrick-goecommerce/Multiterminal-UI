import type { hub } from '../../wailsjs/go/models';
import type { PaneAttention } from '../stores/tabs';

export type Verdict = { kind: 'warn' | 'pending' | 'clean' | 'detail'; text: string };
export type Rect = { left: number; top: number; width: number; height: number };
export type Size = { width: number; height: number };

/** Urgency order of the warning codes; the host sends them in this order already. */
const WARNING_ORDER = ['ci_red', 'pr_blocked', 'check_failed', 'ended_error', 'dirty', 'big_diff'];

export function hasWarnings(b?: hub.Briefing): boolean {
  return !!b && (b.warnings?.length ?? 0) > 0;
}

export function warningShort(code: string, b: hub.Briefing): string {
  switch (code) {
    case 'ci_red': return 'CI rot';
    case 'pr_blocked': return 'PR blockiert';
    case 'check_failed': {
      const failed = (b.checks ?? []).find((c) => c.result === 'fail');
      return `${failed?.label ?? 'Prüfung'} ✗`;
    }
    case 'ended_error': return 'Fehlerende';
    case 'dirty': return 'ungesichert';
    case 'big_diff': return 'großer Diff';
    default: return code;
  }
}

function byPrecedence(codes: string[]): string[] {
  const rank = (c: string) => { const i = WARNING_ORDER.indexOf(c); return i < 0 ? WARNING_ORDER.length : i; };
  return [...codes].sort((a, b) => rank(a) - rank(b));
}

function oneLine(s: string): string {
  return s.replace(/\s+/g, ' ').trim();
}

export function verdict(b: hub.Briefing | undefined, att: PaneAttention | undefined): Verdict {
  const detail = att?.detail ?? '';
  if (!b || att?.source === 'ci') return { kind: 'detail', text: detail };
  if (hasWarnings(b)) {
    const parts = byPrecedence(b.warnings).slice(0, 3).map((c) => warningShort(c, b));
    return { kind: 'warn', text: '⚠ ' + parts.join(' · ') };
  }
  const closing = oneLine(b.closing ?? '');
  const tail = closing ? ` · „${closing}“` : '';
  // A merged or closed PR is no longer watched: its pending checks are frozen.
  if (b.pr?.state === 'OPEN' && b.pr.checks === 'pending') return { kind: 'pending', text: '◷ CI läuft' + tail };
  return { kind: 'clean', text: '✓ sauber' + tail };
}

export function prLine(b: hub.Briefing): string {
  const notes = b.notes ?? [];
  const p = b.pr;
  if (!p) {
    if (notes.includes('ci_paused')) return 'CI-Abfrage pausiert';
    return b.git && !b.git.on_target ? 'kein PR' : '';
  }
  const stale = notes.includes('ci_stale') ? ' · für älteren Stand' : '';
  if (p.state === 'MERGED') return `PR #${p.number} gemergt${stale}`;
  const parts = [`PR #${p.number}`];
  if (p.checks === 'green') parts.push(`CI ${p.checks_passed}/${p.checks_total} ✓`);
  else if (p.checks === 'pending') parts.push(`CI läuft ${p.checks_passed}/${p.checks_total}`);
  else if (p.checks === 'red') parts.push(p.failed_check ? `CI rot: ${p.failed_check}` : 'CI rot');
  if (p.review === 'REVIEW_REQUIRED') parts.push('Review offen');
  else if (p.review === 'CHANGES_REQUESTED') parts.push('Änderungen angefordert');
  else if (p.review === 'APPROVED') parts.push('freigegeben');
  if (p.conflicting) parts.push('Konflikte');
  if (p.draft) parts.push('Entwurf');
  return parts.join(' · ') + stale;
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;
const CHECK_MARK: Record<string, string> = { ok: '✓', fail: '✗', unsure: '?' };

export function cardLines(b: hub.Briefing, head: { project: string; pane: string }): { key: string; text: string }[] {
  const rows: { key: string; text: string }[] = [];
  const g = b.git;

  let h = `${head.project} · ${head.pane}`;
  if (g?.detached) h += ' · abgekoppelt';
  else if (g?.target) h += ` · ${g.on_target ? 'auf' : 'gegen'} ${g.target}`;
  rows.push({ key: 'head', text: h });

  if (b.prompt) rows.push({ key: 'prompt', text: `Auftrag: „${b.prompt}“` });
  if (g?.turn_known) {
    rows.push({ key: 'turn', text: `Turn:   ${plural(g.turn_files, 'Datei', 'Dateien')} +${g.turn_added} −${g.turn_deleted}` });
  }
  if (g) {
    let t = `Branch: ${plural(g.branch_files, 'Datei', 'Dateien')} +${g.branch_added} −${g.branch_deleted} · ${plural(g.commits, 'Commit', 'Commits')}`;
    if (g.no_upstream) t += ' · nie gepusht';
    else if (g.unpushed > 0) t += ` · ${g.unpushed} ungepusht`;
    rows.push({ key: 'branch', text: t });
  }
  if (b.checks?.length) {
    rows.push({ key: 'checks', text: b.checks.map((c) => `${c.label} ${CHECK_MARK[c.result] ?? '?'}`).join('   ') });
  }
  const pr = prLine(b);
  if (pr) rows.push({ key: 'pr', text: pr });
  if (b.closing) rows.push({ key: 'closing', text: b.closing });

  const warn = byPrecedence(b.warnings ?? []).map((c) => warningShort(c, b));
  if ((b.notes ?? []).includes('diff_slow')) warn.push('Diff zu langsam');
  if (warn.length) rows.push({ key: 'warnings', text: warn.join(' · ') });
  return rows;
}

/** Card sits left of the panel at the entry's height, kept inside the window. */
export function cardPosition(entry: Rect, panel: Rect, card: Size, viewport: Size): { left: number; top: number } {
  const left = Math.max(8, panel.left - card.width - 8);
  const maxTop = Math.max(8, viewport.height - card.height - 8);
  const top = Math.min(Math.max(8, entry.top), maxTop);
  return { left, top };
}
