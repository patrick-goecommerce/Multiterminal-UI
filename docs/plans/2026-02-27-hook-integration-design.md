# Hook Integration & Precise Status Enum Design

**Date:** 2026-02-27
**Branch:** alpha-main
**Status:** Approved

## Overview

Replace PTY-regex-based activity detection for Claude panes with Claude Code's native
hook system. PTY scanning is retained for token/cost data and Shell panes.
Extend the activity state enum from 4 to 6 states for precise UI feedback.

## Goals

1. **Reliable activity detection** — hooks fire synchronously from Claude Code, no regex guessing
2. **Precise states** — distinguish `waitingPermission` (tool approval) from `waitingAnswer` (text input needed)
3. **New `error` state** — explicit signal when a tool fails
4. **Non-breaking fallback** — PTY-scan still works when hooks are not installed

## New Activity State Enum

```
ActivityIdle              → "idle"               (no recent activity)
ActivityActive            → "active"             (tool running / generating)
ActivityDone              → "done"               (prompt returned, session idle)
ActivityWaitingPermission → "waitingPermission"  (NEW: tool needs approval)
ActivityWaitingAnswer     → "waitingAnswer"       (NEW: replaces needsInput)
ActivityError             → "error"              (NEW: PostToolUseFailure)
```

`ActivityNeedsInput` is removed and replaced by `ActivityWaitingAnswer`.

## Architecture

### Data Flow

```
Claude Code runs in PTY
  → Hook event fires (stdin: JSON payload)
  → hook_handler.ps1 reads stdin
  → appends JSONL line to %APPDATA%\Multiterminal\hooks\<session_id>.jsonl

Go Backend (app_hooks.go):
  → fsnotify.Watcher on hooks directory
  → reads new JSONL lines
  → matchHookSession(hookSessionID) → Multiterminal session ID
  → session.SetHookActivity(newState)
  → emits terminal:activity event → Frontend (immediate, no scan delay)
```

### Claude Pane vs Shell Pane

| Source | Activity State | Token/Cost |
|---|---|---|
| Claude Pane (hooks active) | Hooks | PTY scan |
| Claude Pane (hooks missing) | PTY scan (existing) | PTY scan |
| Shell Pane | PTY scan (existing) | — |

The `scanAllSessions()` loop skips `DetectActivity()` for sessions where
`session.HasHookData() == true`. Token scanning always runs.

### Session ID Matching

Claude Code hooks provide their own UUID `session_id`. Multiterminal matches
incoming hook events to running sessions by working directory (`cwd`).
The first hook event for an unknown `session_id` is matched to the running
Claude pane whose `dir` equals the hook's `cwd`. The mapping is cached in
`HookManager` for the session lifetime.

Race condition buffer: events for unmatched sessions are held for 60 seconds
then discarded (handles hook firing before session registers in Multiterminal).

## Hook Events Mapping

| Claude Code Event     | New Activity State        |
|-----------------------|---------------------------|
| `PreToolUse`          | `active`                  |
| `PostToolUse`         | `active`                  |
| `PostToolUseFailure`  | `error`                   |
| `PermissionRequest`   | `waitingPermission`       |
| `Notification`        | `waitingAnswer`           |
| `UserPromptSubmit`    | `active`                  |
| `Stop`                | `done` (or `idle`)        |
| `SessionEnd`          | cleanup session mapping   |

## JSONL Format

Each hook writes one JSON line per event:

```json
{"ts":1740700000,"session_id":"abc-123","event":"PreToolUse","tool":"Bash","cwd":"/project"}
{"ts":1740700001,"session_id":"abc-123","event":"PermissionRequest","tool":"Bash","message":"Run rm -rf?"}
{"ts":1740700002,"session_id":"abc-123","event":"Stop","cwd":"/project"}
```

File path: `%APPDATA%\Multiterminal\hooks\<session_id>.jsonl`
Rotation: file deleted on `SessionEnd` event; truncated if >1 MB.

## Hook Registration

Hooks are auto-registered in `~/.claude/settings.json` at app startup if missing.
A backup (`~/.claude/settings.json.bak.<timestamp>`) is created before modification.
A marker comment in the command string prevents duplicate registration.

### settings.json hook entries

```json
{
  "hooks": {
    "PreToolUse":        [{"hooks":[{"type":"command","command":"powershell -NonInteractive -File \"%APPDATA%\\Multiterminal\\hook_handler.ps1\" PreToolUse"}]}],
    "PostToolUse":       [{"hooks":[{"type":"command","command":"powershell -NonInteractive -File \"%APPDATA%\\Multiterminal\\hook_handler.ps1\" PostToolUse"}]}],
    "PostToolUseFailure":[{"hooks":[{"type":"command","command":"powershell -NonInteractive -File \"%APPDATA%\\Multiterminal\\hook_handler.ps1\" PostToolUseFailure"}]}],
    "PermissionRequest": [{"hooks":[{"type":"command","command":"powershell -NonInteractive -File \"%APPDATA%\\Multiterminal\\hook_handler.ps1\" PermissionRequest"}]}],
    "Notification":      [{"hooks":[{"type":"command","command":"powershell -NonInteractive -File \"%APPDATA%\\Multiterminal\\hook_handler.ps1\" Notification"}]}],
    "Stop":              [{"hooks":[{"type":"command","command":"powershell -NonInteractive -File \"%APPDATA%\\Multiterminal\\hook_handler.ps1\" Stop"}]}],
    "SessionEnd":        [{"hooks":[{"type":"command","command":"powershell -NonInteractive -File \"%APPDATA%\\Multiterminal\\hook_handler.ps1\" SessionEnd"}]}]
  }
}
```

The `hook_handler.ps1` is deployed to `%APPDATA%\Multiterminal\` at startup
(embedded as Go `embed.FS`). Updated automatically when app version changes.

## New Files

| File | Purpose |
|---|---|
| `internal/backend/app_hooks.go` | HookManager, fsnotify watcher, session matching, event dispatch |
| `internal/backend/app_hooks_installer.go` | settings.json reader/writer, hook registration, backup |
| `frontend/assets/hook_handler.ps1` | PowerShell hook script (embedded via embed.FS) |
| `internal/backend/app_hooks_test.go` | Unit tests for hook matching, installer, event mapping |

## Changed Files

| File | Change |
|---|---|
| `internal/terminal/activity.go` | Add `ActivityWaitingPermission`, `ActivityWaitingAnswer`, `ActivityError`; remove `ActivityNeedsInput` |
| `internal/terminal/session.go` | Add `hookSessionID string`, `hasHookData bool` fields; `SetHookActivity()`, `HasHookData()` methods |
| `internal/backend/app_scan.go` | `activityString()` new values; skip `DetectActivity()` when `HasHookData()` |
| `internal/backend/app.go` | Start `HookManager` in `Startup()` |
| `frontend/src/stores/tabs.ts` | Extend `Pane.activity` type; update `computeTabActivity()` priority |
| `frontend/src/components/PaneTitlebar.svelte` | CSS classes for new states |
| `frontend/src/components/TerminalPane.svelte` | Audio/notification triggers for new states |
| `frontend/src/components/IssuesView.svelte` | Activity dot for new states |
| `frontend/src/lib/audio.ts` | `waitingPermission` maps to existing needs-input sound |

## Error Handling

- **Hooks missing**: `HasHookData() == false` → PTY scan fallback (transparent)
- **settings.json corrupt/missing**: logged, no crash; hooks silently not registered
- **JSONL file too large**: truncated at 1 MB; deleted on `SessionEnd`
- **Unmatched session_id**: buffered 60s then discarded
- **fsnotify error**: logged, `HookManager` disabled, PTY scan takes over

## Tests

`internal/backend/app_hooks_test.go`:
- `TestHookSessionMatching` — event matched to session by cwd
- `TestHookInstaller_NoOp` — re-running install is idempotent
- `TestHookInstaller_Backup` — backup file created before write
- `TestHookEventMapping` — each event type maps to correct ActivityState
- `TestHookBuffer_Expire` — unmatched events expire after 60s

`internal/terminal/activity_test.go` (extend):
- Rename `needsInput` → `waitingAnswer` in existing tests
- `TestHasHookData_Guard` — `DetectActivity()` skipped when hook data present
