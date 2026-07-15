# CLI Session Management — Design

**Date:** 2026-02-27
**Branch:** wails-v3-alpha

## Problem

After a crash or unexpected behaviour, users cannot clean up the session file without starting the GUI. Tabs that cause problems cannot be removed individually. There is no way to do a clean start for debugging without permanently losing the session.

## Solution

Add CLI flags to `mtui` that allow session management without starting the GUI, plus a `--safe-mode` flag for one-time clean starts that preserves the existing session.

## Flags

```
mtui --help                   List all available flags and exit
mtui --list-tabs              Print tab names from session file, one per line, then exit
mtui --remove-tab "Name"      Remove named tab from session file, then exit
mtui --clean                  Delete the session file entirely, then exit
mtui --safe-mode              Start GUI without loading sessions; restore session on close
```

## Architecture

### `main.go`
- Use the standard `flag` package to parse arguments.
- CLI commands (`--list-tabs`, `--remove-tab`, `--clean`) execute before any GUI init and call `os.Exit(0)` when done.
- `--safe-mode` is passed as a boolean into `backend.NewAppService(app, cfg, safeMode)`.

### `internal/config/session.go`
- Add `RemoveTab(name string) (bool, error)` — loads session, removes the first tab matching the name exactly, saves, returns whether a match was found.

### `internal/backend/app.go`
- Add `safeMode bool` field to `AppService`.
- Add `sessionBackup *config.SessionState` field — populated on startup when safe mode is active.
- `LoadTabs()`: if `safeMode`, return empty list (no tabs loaded into UI).
- `SaveTabs()`: if `safeMode`, skip writing to disk.
- On app shutdown (`OnBeforeClose` / shutdown hook): if `safeMode` and backup is non-nil, call `config.SaveSession(*backup)` to restore the original file.

## Data Flow

```
mtui --list-tabs
  → config.LoadSession() → print tab names → os.Exit(0)

mtui --remove-tab "Foo"
  → config.RemoveTab("Foo") → save updated session → os.Exit(0)

mtui --clean
  → config.ClearSession() → os.Exit(0)

mtui --safe-mode
  → load session into backup (memory only)
  → start GUI with empty tabs
  → auto-save disabled during runtime
  → on shutdown: restore backup to disk
```

## Error Handling

- `--remove-tab` with an unknown name: print error to stderr, `os.Exit(1)`.
- `--list-tabs` / `--remove-tab` when no session file exists: print informative message, `os.Exit(0)`.
- `--safe-mode` when no session file exists: starts with empty tabs, nothing to restore on exit.

## Files Changed

| File | Change |
|------|--------|
| `main.go` | Flag parsing, CLI dispatch |
| `internal/config/session.go` | Add `RemoveTab` |
| `internal/backend/app.go` | `safeMode` field, backup logic, guard in `LoadTabs`/`SaveTabs` |
