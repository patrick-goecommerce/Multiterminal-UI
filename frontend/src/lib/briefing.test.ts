import { describe, it, expect } from 'vitest';
import { hasWarnings, warningShort, verdict, prLine, cardLines, cardPosition } from './briefing';

const br = (o: any = {}) => ({ prompt_id: 'p', built_at: 1, prompt: '', closing: '', checks: [], warnings: [], notes: [], ...o }) as any;
const git = (o: any = {}) => ({
  branch: 'hook-refactor', target: 'alpha-main', on_target: false, detached: false, no_upstream: false,
  turn_known: true, turn_files: 3, turn_added: 40, turn_deleted: 5,
  branch_files: 7, branch_added: 212, branch_deleted: 48, commits: 3, unpushed: 2, uncommitted: 0, ...o,
});
const pr = (o: any = {}) => ({
  number: 12, url: '', state: 'OPEN', base: 'alpha-main', head_oid: '', review: '', checks: 'none', failed_check: '',
  draft: false, conflicting: false, checks_total: 0, checks_passed: 0, ...o,
});

describe('hasWarnings / warningShort', () => {
  it('detects warnings', () => {
    expect(hasWarnings(undefined)).toBe(false);
    expect(hasWarnings(br())).toBe(false);
    expect(hasWarnings(br({ warnings: ['dirty'] }))).toBe(true);
  });
  it('short forms', () => {
    const b = br({ checks: [{ label: 'go test', result: 'ok' }, { label: 'go vet', result: 'fail' }] });
    expect(warningShort('ci_red', b)).toBe('CI rot');
    expect(warningShort('pr_blocked', b)).toBe('PR blockiert');
    expect(warningShort('check_failed', b)).toBe('go vet ✗');
    expect(warningShort('ended_error', b)).toBe('Fehlerende');
    expect(warningShort('dirty', b)).toBe('ungesichert');
    expect(warningShort('big_diff', b)).toBe('großer Diff');
  });
});

describe('verdict', () => {
  const failed = [{ label: 'go vet', result: 'fail' }];
  it('lists at most three warnings by precedence', () => {
    const b = br({ warnings: ['check_failed', 'dirty', 'big_diff', 'ci_red'], checks: failed });
    expect(verdict(b, undefined)).toEqual({ kind: 'warn', text: '⚠ CI rot · go vet ✗ · ungesichert' });
  });
  it('pending CI', () => {
    const b = br({ closing: 'Fertig.', git: git(), pr: pr({ checks: 'pending' }) });
    expect(verdict(b, undefined)).toEqual({ kind: 'pending', text: '◷ CI läuft · „Fertig.“' });
  });
  it('a merged or closed PR with checks still pending is clean', () => {
    for (const state of ['MERGED', 'CLOSED']) {
      const b = br({ closing: 'Fertig.', git: git(), pr: pr({ state, checks: 'pending' }) });
      expect(verdict(b, undefined)).toEqual({ kind: 'clean', text: '✓ sauber · „Fertig.“' });
    }
  });
  it('clean', () => {
    expect(verdict(br({ closing: 'Fertig.' }), undefined)).toEqual({ kind: 'clean', text: '✓ sauber · „Fertig.“' });
    expect(verdict(br(), undefined)).toEqual({ kind: 'clean', text: '✓ sauber' });
  });
  it('a retrieved CI error keeps its detail', () => {
    const att = { state: 'error', rank: 2, seen: false, detail: 'CI rot: backend', source: 'ci', since: 0 };
    expect(verdict(br({ closing: 'x' }), att)).toEqual({ kind: 'detail', text: 'CI rot: backend' });
  });
  it('no briefing falls back to the detail', () => {
    const att = { state: 'done', rank: 3, seen: false, detail: 'Fertig', source: '', since: 0 };
    expect(verdict(undefined, att)).toEqual({ kind: 'detail', text: 'Fertig' });
    expect(verdict(undefined, undefined)).toEqual({ kind: 'detail', text: '' });
  });
});

describe('prLine', () => {
  it('green with open review', () => {
    expect(prLine(br({ pr: pr({ checks: 'green', checks_total: 2, checks_passed: 2, review: 'REVIEW_REQUIRED' }) })))
      .toBe('PR #12 · CI 2/2 ✓ · Review offen');
  });
  it('pending', () => {
    expect(prLine(br({ pr: pr({ checks: 'pending', checks_total: 3, checks_passed: 1 }) }))).toBe('PR #12 · CI läuft 1/3');
  });
  it('merged', () => {
    expect(prLine(br({ pr: pr({ state: 'MERGED' }) }))).toBe('PR #12 gemergt');
  });
  it('red', () => {
    expect(prLine(br({ pr: pr({ checks: 'red', failed_check: 'backend' }) }))).toBe('PR #12 · CI rot: backend');
  });
  it('flags', () => {
    expect(prLine(br({ pr: pr({ conflicting: true }) }))).toBe('PR #12 · Konflikte');
    expect(prLine(br({ pr: pr({ review: 'CHANGES_REQUESTED' }) }))).toBe('PR #12 · Änderungen angefordert');
    expect(prLine(br({ pr: pr({ review: 'APPROVED' }) }))).toBe('PR #12 · freigegeben');
    expect(prLine(br({ pr: pr({ draft: true }) }))).toBe('PR #12 · Entwurf');
  });
  it('stale', () => {
    expect(prLine(br({ pr: pr({ checks: 'pending', checks_total: 3, checks_passed: 1 }), notes: ['ci_stale'] })))
      .toBe('PR #12 · CI läuft 1/3 · für älteren Stand');
  });
  it('no PR', () => {
    expect(prLine(br({ git: git(), notes: ['no_pr'] }))).toBe('kein PR');
    expect(prLine(br({ git: git(), notes: ['ci_paused'] }))).toBe('CI-Abfrage pausiert');
    expect(prLine(br())).toBe('');
  });
  it('says "kein PR" only when the watcher looked and found none', () => {
    // No login, no GitHub remote, ci_watch off, or not asked yet: no PR line.
    expect(prLine(br({ git: git() }))).toBe('');
    expect(prLine(br({ git: git({ on_target: true }), notes: ['no_pr'] }))).toBe('');
  });
});

describe('cardLines', () => {
  const head = { project: 'goki', pane: 'hook-refactor' };
  const full = () => br({
    prompt: 'Baue es um', closing: 'Fertig.', git: git(), pr: pr({ checks: 'green', checks_total: 2, checks_passed: 2 }),
    checks: [{ label: 'go test', result: 'ok' }, { label: 'go vet', result: 'fail' }, { label: 'test', result: 'unsure' }],
    warnings: ['check_failed'], notes: ['diff_slow'],
  });
  const text = (l: { key: string; text: string }[], k: string) => l.find((x) => x.key === k)?.text;
  it('keeps the spec order', () => {
    expect(cardLines(full(), head).map((l) => l.key))
      .toEqual(['head', 'prompt', 'turn', 'branch', 'checks', 'pr', 'closing', 'warnings']);
  });
  it('formats the rows', () => {
    const l = cardLines(full(), head);
    expect(text(l, 'head')).toBe('goki · hook-refactor · gegen alpha-main');
    expect(text(l, 'prompt')).toBe('Auftrag: „Baue es um“');
    expect(text(l, 'turn')).toBe('Turn:   3 Dateien +40 −5');
    expect(text(l, 'branch')).toBe('Branch: 7 Dateien +212 −48 · 3 Commits · 2 ungepusht');
    expect(text(l, 'checks')).toBe('go test ✓   go vet ✗   test ?');
    expect(text(l, 'closing')).toBe('Fertig.');
    expect(text(l, 'warnings')).toBe('go vet ✗ · Diff zu langsam');
  });
  it('variants of head and branch', () => {
    const b = full();
    b.git = git({ on_target: true, no_upstream: true });
    expect(text(cardLines(b, head), 'head')).toBe('goki · hook-refactor · auf alpha-main');
    expect(text(cardLines(b, head), 'branch')).toBe('Branch: 7 Dateien +212 −48 · 3 Commits · nie gepusht');
    b.git = git({ detached: true });
    expect(text(cardLines(b, head), 'head')).toBe('goki · hook-refactor · abgekoppelt');
  });
  it('drops rows without data', () => {
    const l = cardLines(br({ closing: 'x' }), head);
    expect(l.map((r) => r.key)).toEqual(['head', 'closing']);
    expect(text(l, 'head')).toBe('goki · hook-refactor');
  });
});

describe('cardPosition', () => {
  const rect = (left: number, top: number) => ({ left, top, width: 100, height: 30 });
  it('clamps the top to the viewport', () => {
    const p = cardPosition(rect(300, 900), rect(600, 0), { width: 340, height: 300 }, { width: 1600, height: 1000 });
    expect(p.top).toBe(692);
    expect(p.left).toBe(600 - 340 - 8);
  });
  it('clamps the left to the edge', () => {
    const p = cardPosition(rect(300, 100), rect(200, 0), { width: 340, height: 300 }, { width: 1600, height: 1000 });
    expect(p).toEqual({ left: 8, top: 100 });
  });
});
