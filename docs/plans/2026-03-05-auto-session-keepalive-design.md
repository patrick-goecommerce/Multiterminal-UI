# Auto-Session-Start & Keep-Alive Design

**Date:** 2026-03-05
**Issue:** #96
**Branch:** `issue/96-auto-session-start-beim-starten-von-mtui`

## Problem

Claude Code sessions have a 5-hour context window. To maximize token usage, the app should:
1. Automatically open a Claude session on startup
2. Periodically send a keep-alive ping so the session stays warm and the next 5h window starts fresh

## Requirements

- **Auto-start on launch**: If after session-restore no Claude pane exists in Tab 1, create one automatically
- **Keep-alive ping**: Send a configurable message to the first Claude pane when no activity has occurred in any pane for the configured interval
- **Configurable**: Toggle, interval (minutes), message — all editable in Settings
- **Default**: enabled=true, interval=300min (5h), message="Hi!"

## Architecture

### Config Extension (`internal/config/config.go`)

```go
type KeepAliveSettings struct {
    Enabled         *bool  `yaml:"enabled" json:"enabled"`
    IntervalMinutes int    `yaml:"interval_minutes" json:"interval_minutes"`
    Message         string `yaml:"message" json:"message"`
}
```

Added to `Config` struct as `KeepAlive KeepAliveSettings`.

Default values:
- `enabled = true`
- `interval_minutes = 300`
- `message = "Hi!"`

### Backend Binding (`internal/backend/app.go`)

```go
func (a *AppService) GetGlobalLastActivityUnix() int64
```

Returns Unix timestamp (seconds) of the most recent `LastOutputAt` across all active sessions. Returns `0` if no sessions exist.

Uses `session.LastOutputAt()` — already tracked per PTY session.

### Frontend Keep-Alive Module (`frontend/src/lib/keepalive.ts`, new file)

```typescript
export function startKeepAliveLoop(cfg: KeepAliveConfig, claudePath: string, tabStore): () => void
```

Logic:
1. Called from `App.svelte` `onMount` after `restoreSession()`
2. **Startup check**: scan `tabStore` for first Claude pane — if none exists, create one via `App.CreateSession()` + `tabStore.addPane()`
3. **Interval loop** (`setInterval`):
   - Call `App.GetGlobalLastActivityUnix()`
   - If `now - lastActivity < intervalSeconds` → skip (activity within window)
   - Find first Claude pane in any tab → `App.WriteToSession(sessionId, encode(message + "\n"))`
4. Returns cleanup function for `onDestroy`

### Settings UI (`frontend/src/components/SettingsDialog.svelte`)

New section "Session Keep-Alive":
- Toggle: **Aktiviert** (default: on)
- Number input: **Intervall (Minuten)** (default: 300, min: 1)
- Text input: **Nachricht** (default: `Hi!`)

## Data Flow

```
App starts
  → restoreSession()
  → startKeepAliveLoop()
      → No Claude pane? → CreateSession(claude argv, tab1.dir)
      → setInterval(intervalMinutes * 60 * 1000)
            → GetGlobalLastActivityUnix()
            → now - last < interval? → skip
            → find first Claude pane → WriteToSession(message)
```

## Activity Tracking

`LastOutputAt` is already tracked inside `terminal.Session` (updated on every PTY byte read). The new `GetGlobalLastActivityUnix()` backend method aggregates across all sessions.

## Edge Cases

- **No Claude pane found at ping time**: skip silently (user may have closed all Claude panes)
- **Feature disabled**: `startKeepAliveLoop` returns immediately if `cfg.enabled === false`
- **Interval = 0**: treated as disabled
- **Secondary windows**: keep-alive only runs in main window (`isMainWindow()` guard)

## Files Changed

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `KeepAliveSettings` struct + field in `Config` + defaults |
| `internal/backend/app.go` | Add `GetGlobalLastActivityUnix()` binding |
| `frontend/src/lib/keepalive.ts` | New: keep-alive loop logic |
| `frontend/src/App.svelte` | Call `startKeepAliveLoop` after `restoreSession` |
| `frontend/src/components/SettingsDialog.svelte` | Add Keep-Alive settings section |
| `wailsjs/go/backend/App.js` + `App.d.ts` | Regenerated bindings (auto via `wails dev`) |
