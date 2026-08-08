# Multiterminal UI — Technical Reference

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![Wails](https://img.shields.io/badge/Wails-v3%20alpha-red?logo=wails)
![Svelte](https://img.shields.io/badge/Svelte-5-FF3E00?logo=svelte&logoColor=white)
![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-blue)

For the user-facing feature overview, see [README.md](README.md). This document covers building from source, configuration, and branch layout.

> This branch (`alpha-main`) runs on **Wails v3 (alpha)**, which is still evolving; the stable released app (`main` branch) is one major version behind on the GUI framework. See [Branches](#branches) below.

## Tech Stack

- **Language:** Go (backend) + TypeScript/Svelte 5 (frontend)
- **GUI framework:** [Wails v3](https://v3alpha.wails.io/) alpha (Go ↔ WebView bridge, multi-window support)
- **Frontend:** Svelte 5 + Vite
- **Terminal emulation:** [xterm.js](https://xtermjs.org/) (frontend) + a VT100 screen buffer (backend) for activity/cost scanning
- **PTY:** [go-pty](https://github.com/aymanbagabas/go-pty) (cross-platform: Unix PTY / Windows ConPTY)
- **Agent delegation:** local [MCP](https://modelcontextprotocol.io/) server ([mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)), loopback-only
- **Config:** YAML (`~/.multiterminal.yaml`)

## Prerequisites (to build from source)

- [Go](https://go.dev/dl/) (see `go.mod` for the pinned toolchain version)
- [Node.js 18+](https://nodejs.org/)
- [Wails v3 CLI](https://v3alpha.wails.io/getting-started/installation): `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`

## Build & Run

```bash
wails3 dev              # Development (hot-reload)
wails3 build            # Production build
wails3 build -debug     # Debug build (with devtools)
```

The binary is output as `mtui-portable.exe` (Windows) / `mtui-portable` (Linux/macOS) under `build/bin/` — see `wails.json`.

```bash
go test ./internal/...   # Backend tests
go vet ./...              # Static analysis
npm test                  # Frontend tests (run from frontend/)
```

## Configuration Reference

A config file is auto-created at `~/.multiterminal.yaml` on first run; anything not set falls back to the defaults shown below. Most of it is also editable through the in-app Settings dialog.

```yaml
# Appearance
theme: konzept                           # konzept, dark, light, dracula, nord, solarized
terminal_color: "#39ff14"                # Accent color (hex)
font_family: ""                          # Monospace font name (empty = system default)
font_size: 10                            # 8-20px
language: de                             # de, en, it, es, fr
chat_style: claude-code                  # claude-code, telegram — chat pane display style

# Terminal & workspace
default_shell: ""                        # Default shell (auto-detected)
default_dir: ""                          # Fallback working directory
last_opened_dir: ""                      # Auto-updated: last folder you opened; takes priority over default_dir
max_panes_per_tab: 9                     # 1-9
sidebar_width: 30                        # Sidebar width (characters)
restore_session: true                    # Restore tabs/panes on next launch

# Git
commit_reminder_minutes: 30              # Footer commit-age warning threshold
auto_branch_on_issue: true               # Isolate every issue-launched pane in its own git worktree
finish_prep_prompt: ""                   # Override the built-in worktree-finish prep prompt

# AI tools
claude_enabled: true
claude_command: ""                       # Claude CLI path (empty = auto-detected)
claude_models: [...]                     # Selectable models shown in the launch dialog
codex_enabled: false
codex_command: ""
codex_models: [...]
gemini_enabled: false
gemini_command: ""
gemini_models: [...]

# Session keep-alive — nudges an idle Claude session before its context window expires
keep_alive:
  enabled: true
  interval_minutes: 300
  message: "Hi!"

# Automatic pane naming (via a one-shot model call on the first prompt)
auto_naming:
  enabled: true
  model: claude-haiku-4-5

# Claude Code statusline (written to ~/.claude/settings.json)
status_line:
  enabled: false
  template: standard                     # minimal, standard, extended
  show_model: true
  show_context: true
  show_cost: true
  show_git_branch: false
  show_duration: false

# GitHub issue progress comments
issue_tracking:
  auto_comment_on_start: true
  auto_comment_on_done: true
  auto_comment_on_close: true
  auto_close_issue: false
  include_cost_in_report: true

# Background review/test agents, triggered after each commit
background_agents:
  review_enabled: false
  review_tool: claude                    # claude, codex, gemini
  review_model: claude-haiku-4-5-20251001
  review_prompt: "Review the following commit diff. Flag bugs, security issues, and code quality problems:\n\n{diff}"
  test_enabled: false
  test_command: ""

# Kanban multi-agent orchestrator
orchestrator:
  max_parallel_agents: 3
  default_auto_merge: false
  default_auto_start: false
  max_retries: 2
  review_command: "go test ./... && go vet ./..."
  sync_subtasks_to_github: false

# Local MCP server (lets an agent in a pane open/feed/close other sessions)
mcp_server:
  enabled: true
  port: 51533                            # 127.0.0.1 only, no auth — see README.md "Agent Delegation"

# Speech-to-text (voice dictation)
stt:
  provider: cloud-whisper                # cloud-whisper, whisper-cpp, parakeet
  language: de                           # ISO code, or "auto"
  cloud:
    base_url: ""                         # empty = OpenAI default
    model: whisper-1
    api_key: ""                          # empty = $OPENAI_API_KEY

# Audio notifications
audio:
  enabled: true
  volume: 50
  when_focused: true                     # play sounds even when the window is focused
  done_sound: ""                         # empty = built-in synth sound
  input_sound: ""
  error_sound: ""

# Misc
logging_enabled: false                   # verbose debug logging to file
localhost_auto_open: notify              # auto, notify, off — behavior when a dev server URL is detected
sidebar_pinned: false                    # keep the file browser sidebar open instead of auto-collapsing
commands: []                             # saved command-palette entries
quick_actions: []                        # pane-titlebar quick-action buttons
favorites: {}                            # per-project file bookmarks
setup_done: false                        # set once the first-run setup wizard completes
```

## Project Structure

See [`CLAUDE.md`](CLAUDE.md) for the up-to-date backend/frontend file layout, concurrency notes, and Windows-specific gotchas — it's kept in sync as the primary reference for anyone (human or AI agent) working in this codebase.

## Branches

- **`main`** — stable releases (Wails v2). Hotfixes land here first.
- **`alpha-main`** — active development (Wails v3 alpha, multi-window support, all features listed in README.md). Merges into `main` once stable; hotfixes on `main` get cherry-picked here.
- Feature branches fork off `alpha-main` and PR back into it.
