# tmux-Shim für Claude Code Agent Teams

**Date:** 2026-03-07
**Status:** Approved
**Branch:** claude/analyze-code-studio-integration-R75EK

## Problem

Claude Code Agent Teams nutzt tmux für Split-Pane-Modus (`teammateMode: "tmux"`).
Multiterminal kann bereits Panes verwalten, aber Claude Code erkennt es nicht als
Terminal-Multiplexer. Ziel: Multiterminal als tmux-Ersatz bereitstellen.

## Architecture

```
Claude Code (teammateMode: "tmux")
    │
    ├── tmux split-window -h -- claude ...
    ├── tmux send-keys -t %3 "hello" Enter
    ├── tmux list-panes -F "#{pane_id}:#{pane_pid}"
    ├── tmux kill-pane -t %3
    │
    ▼
tmux.exe  (Go binary, our shim)
    │
    ├── Logs to ~/.multiterminal/tmux-shim.log
    ├── Parses tmux CLI arguments
    ├── HTTP POST → localhost:$MTUI_PORT/api/tmux
    │
    ▼
Multiterminal Backend (HTTP server)
    │
    ├── POST /api/tmux/split-window  → CreateSession()
    ├── POST /api/tmux/send-keys     → WriteToSession()
    ├── GET  /api/tmux/list-panes    → List active sessions
    ├── POST /api/tmux/kill-pane     → CloseSession()
    │
    ▼
GUI: New pane appears automatically
```

## Components

### 1. HTTP API (`internal/backend/app_tmux_api.go`)

- `net/http` server on random port (`localhost:0`)
- Starts on app init, port stored in `MTUI_PORT` env var
- Endpoints map 1:1 to existing session methods
- Phase 1: `/api/tmux/log` — receives raw command, logs it
- Phase 2+: Real pane operations

### 2. tmux.exe Shim (`cmd/tmux-shim/main.go`)

- Standalone Go binary, ~100 lines
- Reads `MTUI_PORT` from environment
- Phase 1: Logs args to file + sends to HTTP API
- Phase 2+: Parses subcommands and calls correct API endpoints
- Returns stdout/stderr that Claude Code expects (e.g. pane IDs)

### 3. PATH Injection (`internal/terminal/session.go`)

- On session start: set `MTUI_PORT` + prepend shim directory to `PATH`
- Shim lives in `build/bin/` next to `multiterminal.exe`

### 4. User Settings

User must set in `~/.claude/settings.json`:
```json
{
  "env": { "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1" },
  "teammateMode": "tmux"
}
```

No Multiterminal-side configuration needed.

## Phases

### Phase 1: Logging Shim (current)
- Start HTTP server + set port in env
- tmux.exe shim: log everything (file + HTTP)
- Goal: discover exact tmux commands Claude Code sends

### Phase 2: Core Pane Management
- `split-window` → new pane in current tab
- `send-keys` → WriteToSession
- `list-panes` → session list in tmux format
- `kill-pane` → CloseSession

### Phase 3: Polish
- Correct tmux output format for all commands
- Handle edge cases (resize, select-pane, etc.)
- Auto-configure Claude Code settings from Multiterminal UI
