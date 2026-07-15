# Claude Auto Launch Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Claude Auto" launch option that starts `claude --permission-mode auto`, positioned between "Claude Code" and "Claude YOLO" in the launch dialog.

**Architecture:** Introduce a new `'claude-auto'` PaneMode value. The frontend `claude.ts` maps it to the CLI argv `--permission-mode auto`; the launch dialog, titlebar badge, and i18n surface it; the Go backend treats it like other Claude modes for env injection. Mirrors the existing `codex-auto` mode exactly.

**Tech Stack:** TypeScript + Svelte 4 (frontend), Vitest (tests), Go (backend).

---

## File Structure

- `frontend/src/lib/claude.test.ts` — **NEW** unit tests for `buildClaudeArgv` / `getClaudeName` claude-auto behavior.
- `frontend/src/lib/claude.ts` — argv mapping, display name, mode↔index tables.
- `frontend/src/stores/tabs.ts` — `PaneMode` union type.
- `frontend/src/components/LaunchDialog.svelte` — option list, separator grouping, hover CSS.
- `frontend/src/components/PaneTitlebar.svelte` — badge label, badge class, badge CSS.
- `frontend/src/i18n/{de,en,es,fr,it}.ts` — `claudeAuto` / `claudeAutoDesc` keys.
- `internal/backend/app.go` — env-injection mode check.

---

## Task 1: Mode logic in claude.ts (TDD)

**Files:**
- Test: `frontend/src/lib/claude.test.ts` (create)
- Modify: `frontend/src/stores/tabs.ts:3`
- Modify: `frontend/src/lib/claude.ts` (MODE_TO_INDEX, INDEX_TO_MODE, buildClaudeArgv, getClaudeName)

- [ ] **Step 1: Write the failing test**

Create `frontend/src/lib/claude.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { buildClaudeArgv, getClaudeName, MODE_TO_INDEX, INDEX_TO_MODE } from './claude';

describe('claude-auto mode', () => {
  it('builds argv with --permission-mode auto, no model', () => {
    expect(buildClaudeArgv('claude-auto', '', 'claude')).toEqual([
      'claude', '--permission-mode', 'auto',
    ]);
  });

  it('builds argv with --permission-mode auto and model', () => {
    expect(buildClaudeArgv('claude-auto', 'claude-opus-4-7', 'claude')).toEqual([
      'claude', '--permission-mode', 'auto', '--model', 'claude-opus-4-7',
    ]);
  });

  it('produces a display name', () => {
    expect(getClaudeName('claude-auto', '')).toBe('Claude Auto');
    expect(getClaudeName('claude-auto', 'opus')).toBe('Claude Auto (opus)');
  });

  it('appends claude-auto at the end of the index table (persistence safety)', () => {
    // Existing indices must NOT shift — claude-auto is appended, not inserted.
    expect(MODE_TO_INDEX['shell']).toBe(0);
    expect(MODE_TO_INDEX['claude']).toBe(1);
    expect(MODE_TO_INDEX['claude-yolo']).toBe(2);
    expect(MODE_TO_INDEX['codex']).toBe(3);
    expect(MODE_TO_INDEX['codex-auto']).toBe(4);
    expect(MODE_TO_INDEX['gemini']).toBe(5);
    expect(MODE_TO_INDEX['gemini-yolo']).toBe(6);
    expect(MODE_TO_INDEX['claude-auto']).toBe(7);
    expect(INDEX_TO_MODE[7]).toBe('claude-auto');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/lib/claude.test.ts`
Expected: FAIL — `buildClaudeArgv('claude-auto', ...)` returns `[]` (default case) and `MODE_TO_INDEX['claude-auto']` is `undefined`. Also a TypeScript error on `'claude-auto'` not being a valid `PaneMode`.

- [ ] **Step 3: Add 'claude-auto' to the PaneMode type**

In `frontend/src/stores/tabs.ts` line 3, change:

```ts
export type PaneMode = 'shell' | 'claude' | 'claude-yolo' | 'codex' | 'codex-auto' | 'gemini' | 'gemini-yolo';
```
to:
```ts
export type PaneMode = 'shell' | 'claude' | 'claude-auto' | 'claude-yolo' | 'codex' | 'codex-auto' | 'gemini' | 'gemini-yolo';
```

- [ ] **Step 4: Implement claude.ts changes**

In `frontend/src/lib/claude.ts`, update the index tables to append `claude-auto` at index 7:

```ts
export const MODE_TO_INDEX: Record<string, number> = {
  shell: 0, claude: 1, 'claude-yolo': 2,
  codex: 3, 'codex-auto': 4,
  gemini: 5, 'gemini-yolo': 6,
  'claude-auto': 7,
};
export const INDEX_TO_MODE: PaneMode[] = [
  'shell', 'claude', 'claude-yolo',
  'codex', 'codex-auto',
  'gemini', 'gemini-yolo',
  'claude-auto',
];
```

In `buildClaudeArgv`, add a case directly after the `claude-yolo` case:

```ts
    case 'claude-auto':
      return model
        ? [claudeCmd, '--permission-mode', 'auto', '--model', model]
        : [claudeCmd, '--permission-mode', 'auto'];
```

In `getClaudeName`, add a case directly after the `claude-yolo` case:

```ts
    case 'claude-auto': return `Claude Auto ${model ? `(${model})` : ''}`.trim();
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd frontend && npx vitest run src/lib/claude.test.ts`
Expected: PASS (all 4 tests green).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/claude.test.ts frontend/src/lib/claude.ts frontend/src/stores/tabs.ts
git commit -m "feat(launch): add claude-auto mode logic and argv mapping"
```

---

## Task 2: Launch dialog option

**Files:**
- Modify: `frontend/src/components/LaunchDialog.svelte` (buildOptions ~line 34-37, needsSeparator ~line 74-82, CSS ~line 258)

- [ ] **Step 1: Add the option in buildOptions**

In `frontend/src/components/LaunchDialog.svelte`, inside the `if (cfg.claude_enabled !== false)` block, insert the auto option *between* the `claude` and `claude-yolo` pushes so the resulting block reads:

```ts
    if (cfg.claude_enabled !== false) {
      opts.push({ mode: 'claude', label: $t('launch.claude'), desc: $t('launch.claudeDesc'), icon: '&#10024;', cssClass: '' });
      opts.push({ mode: 'claude-auto', label: $t('launch.claudeAuto'), desc: $t('launch.claudeAutoDesc'), icon: '&#129302;', cssClass: 'claude-auto' });
      opts.push({ mode: 'claude-yolo', label: $t('launch.claudeYolo'), desc: $t('launch.claudeYoloDesc'), icon: '&#9889;', cssClass: 'yolo' });
    }
```

- [ ] **Step 2: Keep claude-auto in the Claude separator group**

In the `needsSeparator` function's `group` helper, change the Claude line so `claude-auto` shares group `1`:

```ts
      if (m === 'claude' || m === 'claude-auto' || m === 'claude-yolo') return 1;
```

- [ ] **Step 3: Add hover CSS**

In the `<style>` block, directly after the `.option.yolo:hover` rule (~line 258), add:

```css
  .option.claude-auto:hover { border-color: #c084fc; }
```

- [ ] **Step 4: Verify build compiles**

Run: `cd frontend && npm run build`
Expected: Build succeeds, no TypeScript errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/LaunchDialog.svelte
git commit -m "feat(launch): show Claude Auto option between Claude and YOLO"
```

---

## Task 3: Titlebar badge

**Files:**
- Modify: `frontend/src/components/PaneTitlebar.svelte` (getModeLabel ~line 64, getModeBadgeClass ~line 76, CSS ~line 299)

- [ ] **Step 1: Add badge label**

In `getModeLabel`, directly after the `case 'claude':` line, add:

```ts
      case 'claude-auto': return 'Claude Auto';
```

- [ ] **Step 2: Add badge class**

In `getModeBadgeClass`, directly after the `case 'claude':` line, add:

```ts
      case 'claude-auto': return 'badge-claude-auto';
```

- [ ] **Step 3: Add badge CSS**

In the `<style>` block, directly after the `.badge-codex-auto` rule (~line 299), add:

```css
  .badge-claude-auto { background: #c084fc33; color: #c084fc; }
```

- [ ] **Step 4: Verify build compiles**

Run: `cd frontend && npm run build`
Expected: Build succeeds, no TypeScript errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/PaneTitlebar.svelte
git commit -m "feat(launch): add Claude Auto titlebar badge"
```

---

## Task 4: i18n keys

**Files:**
- Modify: `frontend/src/i18n/de.ts`, `en.ts`, `es.ts`, `fr.ts`, `it.ts`

In each file, add two keys directly after the existing `claudeYoloDesc:` line (4-space indent, matching surrounding entries).

- [ ] **Step 1: de.ts**

After `claudeYoloDesc: 'Alle Berechtigungen',` add:

```ts
    claudeAuto: 'Claude Auto',
    claudeAutoDesc: 'Auto-Genehmigung',
```

- [ ] **Step 2: en.ts**

After `claudeYoloDesc: 'All permissions',` add:

```ts
    claudeAuto: 'Claude Auto',
    claudeAutoDesc: 'Auto-approve mode',
```

- [ ] **Step 3: es.ts**

After `claudeYoloDesc: 'Todos los permisos',` add:

```ts
    claudeAuto: 'Claude Auto',
    claudeAutoDesc: 'Modo auto-aprobación',
```

- [ ] **Step 4: fr.ts**

After `claudeYoloDesc: 'Toutes les permissions',` add:

```ts
    claudeAuto: 'Claude Auto',
    claudeAutoDesc: 'Mode auto-approbation',
```

- [ ] **Step 5: it.ts**

After `claudeYoloDesc: 'Tutti i permessi',` add:

```ts
    claudeAuto: 'Claude Auto',
    claudeAutoDesc: 'Modalità auto-approvazione',
```

- [ ] **Step 6: Verify build compiles**

Run: `cd frontend && npm run build`
Expected: Build succeeds; the dialog labels resolve (no `launch.claudeAuto` literal shown).

- [ ] **Step 7: Commit**

```bash
git add frontend/src/i18n/de.ts frontend/src/i18n/en.ts frontend/src/i18n/es.ts frontend/src/i18n/fr.ts frontend/src/i18n/it.ts
git commit -m "i18n: add Claude Auto launch labels"
```

---

## Task 5: Backend env injection

**Files:**
- Modify: `internal/backend/app.go:199`

- [ ] **Step 1: Extend the mode check**

Change line 199 from:

```go
	if mode == "claude" || mode == "claude-yolo" {
```
to:
```go
	if mode == "claude" || mode == "claude-auto" || mode == "claude-yolo" {
```

- [ ] **Step 2: Verify it compiles**

Run: `go vet ./internal/backend/...`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/backend/app.go
git commit -m "feat(backend): inject session id for claude-auto panes"
```

---

## Task 6: Full verification

- [ ] **Step 1: Run frontend tests**

Run: `cd frontend && npx vitest run`
Expected: All tests pass, including `claude.test.ts`.

- [ ] **Step 2: Run frontend build**

Run: `cd frontend && npm run build`
Expected: Build succeeds, no type errors.

- [ ] **Step 3: Run go vet**

Run: `go vet ./...`
Expected: No errors.

- [ ] **Step 4: Manual E2E (human)**

1. Launch the app (`wails3 build` per project memory, or the dev build).
2. Press Ctrl+N → confirm three Claude entries appear in order: **Claude Code**, **Claude Auto**, **Claude YOLO** (no separator line between them).
3. Select **Claude Auto** → confirm the spawned process is `claude --permission-mode auto` (check the pane / process args).
4. Confirm the pane titlebar shows the "Claude Auto" badge.
5. Confirm token/cost tracking populates in the titlebar (env injection works).

---

## Self-Review

- **Spec coverage:** All 7 affected areas from the spec map to tasks — tabs.ts + claude.ts (Task 1), LaunchDialog buildOptions/needsSeparator/CSS (Task 2), PaneTitlebar badge+CSS (Task 3), i18n (Task 4), app.go env injection (Task 5), verification (Task 6). ✓
- **Persistence safety:** Task 1 explicitly appends `claude-auto` at index 7 and tests that existing indices are unchanged. ✓
- **No placeholders:** Every code step shows the exact code/diff. ✓
- **Type consistency:** Mode string `'claude-auto'` and i18n keys `claudeAuto` / `claudeAutoDesc` used consistently across all tasks. ✓
