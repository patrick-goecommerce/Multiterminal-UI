# Claude Auto Launch Mode — Design Spec

**Date:** 2026-05-24
**Status:** Approved
**Scope:** Launch dialog only (no scheduler / no Kanban orchestration)

## Problem

Claude Code's CLI now offers an "auto" permission mode (`claude --permission-mode auto`),
which auto-approves actions via classifier rules — more autonomous than the default
interactive mode, but milder and safer than YOLO (`--dangerously-skip-permissions`).

Multiterminal's launch dialog (Ctrl+N) currently exposes only **Claude Code** (normal)
and **Claude YOLO** for the Claude CLI. There is no way to start a pane in auto mode.

## Goal

Add a third Claude launch option, **Claude Auto**, positioned between "Claude Code" and
"Claude YOLO" (logical escalation of autonomy). Selecting it starts:

```
claude --permission-mode auto [--model <id>]
```

The `--model` argument is appended only when a model is selected in the launch dialog
(same convention as the existing modes).

This mirrors the already-existing `codex-auto` mode (`codex --full-auto`) in both structure
and UI treatment.

## Non-Goals

- **Scheduler** (`internal/backend/app_schedule_runner.go`) — not extended.
- **Kanban orchestration** — not extended.
- No new config field, so `models.ts` / `config.Config` are untouched.

## New PaneMode

A new mode value `'claude-auto'` is introduced across the frontend and recognized by the
backend for env injection.

## Affected Files

### 1. `frontend/src/stores/tabs.ts`
Add `'claude-auto'` to the `PaneMode` union type.

### 2. `frontend/src/lib/claude.ts`
- **`buildClaudeArgv`** — new `case 'claude-auto'`:
  ```ts
  case 'claude-auto':
    return model
      ? [claudeCmd, '--permission-mode', 'auto', '--model', model]
      : [claudeCmd, '--permission-mode', 'auto'];
  ```
- **`getClaudeName`** — new case returning `` `Claude Auto ${model ? `(${model})` : ''}`.trim() ``.
- **`MODE_TO_INDEX` / `INDEX_TO_MODE`** — append `'claude-auto'` at **index 7** (the end).
  **Critical:** these indices drive session persistence (mode ↔ integer index). Inserting
  `claude-auto` in the middle would shift every later mode's index, silently remapping
  already-saved sessions to the wrong mode on restore. Therefore it must be appended, not
  inserted — even though the launch dialog displays it in the middle of the Claude group.

### 3. `frontend/src/components/LaunchDialog.svelte`
- **`buildOptions`** — push the `claude-auto` option *between* the `claude` and `claude-yolo`
  pushes (UI order is independent of `MODE_TO_INDEX`). Label/desc from i18n, distinct icon,
  cssClass `claude-auto`.
- **`needsSeparator`** — extend the `group()` helper so `claude-auto` returns group `1`
  (same as `claude` and `claude-yolo`), preventing a separator line between the three Claude
  options.
- **CSS** — add `.option.claude-auto:hover { border-color: <color>; }`.

### 4. `frontend/src/components/PaneTitlebar.svelte`
- Badge label: `case 'claude-auto': return 'Claude Auto';`
- Badge class: `case 'claude-auto': return 'badge-claude-auto';`
- Add `.badge-claude-auto { ... }` CSS with a distinct color (analogous to `badge-codex-auto`).

### 5. i18n
In every locale file that already defines `codexAuto` / `codexAutoDesc`, add:
- `claudeAuto` — e.g. de: `'Claude Auto'`
- `claudeAutoDesc` — e.g. de: `'Auto-Genehmigung'`

Locales without their own translation fall back to English wording, matching how the existing
`codexAuto` keys are handled across locale files.

### 6. `internal/backend/app.go`
Extend the env-injection condition (currently line ~199):
```go
if mode == "claude" || mode == "claude-yolo" || mode == "claude-auto" {
    env = append(env, fmt.Sprintf("MULTITERMINAL_SESSION_ID=%d", id))
}
```
So that `MULTITERMINAL_SESSION_ID` is injected for auto-mode panes too — hooks and
activity/token tracking then work identically to the normal Claude mode.

## Verification

- `cd frontend && npm run build` — TypeScript compiles, all `PaneMode` switch statements
  exhaustive, no type errors.
- `go vet ./...` — backend change compiles cleanly.
- Manual E2E: Ctrl+N → select **Claude Auto** → confirm the spawned process runs
  `claude --permission-mode auto`, the pane titlebar shows the "Claude Auto" badge, and
  token/cost tracking populates.

## Risk Notes

- The only persistence-sensitive change is the `MODE_TO_INDEX` / `INDEX_TO_MODE` ordering;
  appending (never inserting) keeps existing saved sessions valid.
- `--permission-mode auto` is a documented Claude CLI flag; if a user's CLI version predates
  it, the CLI itself surfaces the error in the pane — no special handling needed.
