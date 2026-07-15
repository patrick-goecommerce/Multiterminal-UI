# Multiterminal UI (mtui)

<img width="1920" height="1032" alt="image" src="https://github.com/user-attachments/assets/4db36c31-1140-4cba-bab7-768a7a1eb65d" />

**[Deutsch](#deutsch) | [English](#english)**

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)
![Wails](https://img.shields.io/badge/Wails-v2-red?logo=wails)
![Svelte](https://img.shields.io/badge/Svelte-4-FF3E00?logo=svelte&logoColor=white)
![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-blue)

---

# Deutsch

## Was ist Multiterminal UI?

Multiterminal UI ist eine native Desktop-Anwendung, die mehrere KI-Coding-Assistenten (Claude Code, Codex, Gemini) in einer professionellen Terminal-Oberfläche vereint. Statt zwischen vielen Terminalfenstern zu wechseln, hast du alle Sessions nebeneinander — mit automatischer Kostenübersicht, Aktivitätserkennung und Git-Integration.

## Funktionen

### Terminals & Tabs
- **Multi-Pane-Layout** — Bis zu 12 Terminal-Sessions pro Tab in einem Kachelraster
- **Projekt-Tabs** — Jeder Tab hat sein eigenes Arbeitsverzeichnis; Projekte über Ordnerauswahl hinzufügen
- **Session-Wiederherstellung** — Alle Tabs, Panes und Layouts werden automatisch gespeichert und beim Neustart wiederhergestellt
- **Pane umbenennen** — Doppelklick auf den Pane-Namen zum Umbenennen
- **Zoom** — Ctrl+Z zum Maximieren/Wiederherstellen eines Panes, Ctrl+Mausrad für Schriftgröße pro Terminal

### KI-Assistenten

Drei KI-CLI-Tools werden unterstützt — beim Öffnen eines neuen Terminals (Ctrl+N) wählst du:

| Tool | Modi | Beschreibung |
|------|------|--------------|
| **Claude Code** (Anthropic) | Normal / YOLO | KI-Coding-Assistent mit optionaler Modellauswahl |
| **Codex** (OpenAI) | Standard / Auto | OpenAI Codex CLI |
| **Gemini** (Google) | Standard / Sandbox | Google Gemini CLI |

### Token- & Kostenübersicht
- **Pro Pane** — Jedes Claude-Pane zeigt seine Kosten in der Titelleiste (z.B. `$0.12`)
- **Gesamtkosten** — Die globale Fußzeile zeigt die Kosten aller Claude-Panes zusammen
- **Automatische Erkennung** — Kosten werden in Echtzeit aus der Claude-Code-Ausgabe geparst

### Aktivitätserkennung

Die Pane-Rahmen zeigen den Status von Claude visuell an:

| Anzeige | Bedeutung |
|---------|-----------|
| **Grünes Leuchten** | Claude ist fertig (Prompt zurückgekehrt) |
| **Rot/Gelbes Blinken** | Claude wartet auf Eingabe (Bestätigung, J/N, etc.) |
| **Pulsierender Punkt** | Claude arbeitet gerade aktiv |

So siehst du auf einen Blick, welches Pane deine Aufmerksamkeit braucht.

### Pipeline-Warteschlange
Reihe mehrere Prompts für ein Claude-Pane aneinander. Sie werden nacheinander ausgeführt:
1. Klicke den **▶**-Button in der Titelleiste eines Panes
2. Gib einen Prompt ein und drücke **Enter**
3. Wenn Claude fertig ist, wird automatisch der nächste Prompt gesendet

Perfekt zum Planen von Aufgabenserien — einrichten und weggehen. Das Badge zeigt die Anzahl wartender Einträge.

### Dateibrowser (Seitenleiste)
- **Ctrl+B** zum Ein-/Ausblenden
- Dateien durchsuchen und navigieren
- Klick auf eine Datei fügt den Pfad ins fokussierte Terminal ein
- **Favoriten** — Häufig genutzte Dateien als Lesezeichen speichern
- Git-Status-Anzeige pro Datei (geändert, neu, unverfolgt, etc.)

### Git-Integration
- **Commit-Erinnerung** — Die Fußzeile zeigt die Zeit seit dem letzten Commit:
  - Grün (unter 15 Min.), Gelb (15–30 Min.), Rot pulsierend (30+ Min.)
- **Quellcodeverwaltung** — Dateiänderungen gruppiert nach Status (Konflikte, Geändert, Hinzugefügt, Gelöscht, Umbenannt)
- **Branch-Anzeige** — Aktueller Branch in der Fußzeile
- **Worktree-Unterstützung** — Optionale isolierte Arbeitsverzeichnisse pro Issue
- **Konflikterkennung** — Visuelle Warnung bei Merge-/Rebase-Konflikten

### GitHub Issues
- **Issue-Seitenleiste** — Alle GitHub Issues des aktuellen Projekts anzeigen
- **Issue-Details** — Titel, Beschreibung, Labels und Kommentare ansehen
- **Issues erstellen/bearbeiten** — Dialog zum Erstellen und Bearbeiten
- **Claude für Issue starten** — Klick auf ▶ bei einem Issue startet eine Claude-Session mit dem Issue-Kontext
- **Issue-Verknüpfung** — Panes können mit GitHub Issues verknüpft werden (z.B. "Claude – #42")
- Benötigt [GitHub CLI](https://cli.github.com/) (`gh`)

### Befehlspalette
- **Ctrl+Shift+P** zum Öffnen
- Eigene Befehle/Skripte speichern und per Name ausführen
- Befehle bearbeiten, löschen und organisieren

### Audio-Benachrichtigungen
- **Fertig-Sound** — Spielt ab, wenn Claude fertig ist
- **Eingabe-Sound** — Spielt ab, wenn Claude auf Eingabe wartet
- **Fehler-Sound** — Spielt bei Fehlern ab
- Lautstärkeregelung und Option "Stumm wenn Fenster fokussiert"
- Eigene Audiodateien möglich

### Themes & Darstellung

| Theme | Beschreibung |
|-------|--------------|
| `dark` | Catppuccin Mocha (Standard) |
| `light` | Helles Theme |
| `dracula` | Dracula-Farbschema |
| `nord` | Nord-Farbschema |
| `solarized` | Solarized Dark |

- **Eigene Akzentfarbe** — Farbwähler, Hex-Eingabe oder 8 Presets
- **Schriftart** — Aus installierten Monospace-Schriften wählen
- **Schriftgröße** — 8–20px in 2px-Schritten

### Sprachen

Die Oberfläche ist verfügbar in: **Deutsch**, Englisch, Italienisch, Spanisch, Französisch.

## Tastenkürzel

| Taste | Aktion |
|-------|--------|
| **Ctrl+T** | Neuer Projekt-Tab (Ordnerauswahl) |
| **Ctrl+W** | Tab schließen |
| **Ctrl+N** | Neues Terminal-Pane (Startdialog) |
| **Ctrl+Z** | Pane maximieren / wiederherstellen |
| **Ctrl+1-9** | Pane nach Index fokussieren |
| **Ctrl+B** | Dateibrowser-Seitenleiste ein-/ausblenden |
| **Ctrl+F** | Terminal-Suche |
| **Ctrl+I** | GitHub Issues anzeigen |
| **Ctrl+Shift+P** | Befehlspalette |
| **Ctrl+V** | Einfügen |
| **Ctrl+C** | Kopieren (bei Auswahl) |
| **Ctrl+Scroll** | Schriftgröße pro Terminal |
| **Esc** | Dialoge schließen |

## Voraussetzungen

**Zum Ausführen:**
- Mindestens eines der KI-CLI-Tools: [Claude Code](https://docs.anthropic.com/en/docs/claude-code), [Codex](https://github.com/openai/codex), oder [Gemini CLI](https://github.com/google-gemini/gemini-cli)
- [GitHub CLI](https://cli.github.com/) (`gh`) — für die GitHub Issues-Integration

**Zum Bauen aus Quellcode:**
- [Go 1.21+](https://go.dev/dl/)
- [Node.js 18+](https://nodejs.org/)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

## Installation & Start

```bash
# Entwicklungsmodus (Hot-Reload)
wails dev

# Produktions-Build
wails build

# Debug-Build (mit DevTools)
wails build -debug
```

Die Binärdatei wird erstellt unter `build/bin/mtui.exe` (Windows) bzw. `build/bin/mtui` (Linux/macOS).

## Konfiguration

Eine Konfigurationsdatei wird beim ersten Start automatisch unter `~/.multiterminal.yaml` erstellt.

```yaml
# Darstellung
theme: dark                              # dark, light, dracula, nord, solarized
terminal_color: "#39ff14"                # Akzentfarbe (Hex)
font_family: ""                          # Monospace-Schriftart
font_size: 10                            # Schriftgröße (px)
language: de                             # Sprache (de/en/it/es/fr)

# Terminal
default_shell: ""                        # Standard-Shell (automatisch erkannt)
default_dir: ""                          # Standard-Arbeitsverzeichnis
max_panes_per_tab: 12                    # Max. Terminals pro Tab
sidebar_width: 30                        # Seitenleisten-Breite (Zeichen)

# Git
commit_reminder_minutes: 30              # Commit-Erinnerung ab (Minuten)
use_worktrees: false                     # Git-Worktrees pro Issue erstellen

# KI-Tools
claude_enabled: true                     # Claude Code aktivieren
claude_command: ""                       # Claude CLI-Pfad (automatisch erkannt)
claude_models:                           # Verfügbare Claude-Modelle
  - label: "Default"
    id: ""
  - label: "Opus 4.6"
    id: "claude-opus-4-6"
  - label: "Sonnet 4.5"
    id: "claude-sonnet-4-5-20250929"
  - label: "Haiku 4.5"
    id: "claude-haiku-4-5-20251001"
codex_enabled: false                     # Codex (OpenAI) aktivieren
codex_command: ""                        # Codex CLI-Pfad
gemini_enabled: false                    # Gemini (Google) aktivieren
gemini_command: ""                       # Gemini CLI-Pfad

# Audio
audio:
  enabled: true                          # Audio-Benachrichtigungen
  volume: 50                             # Lautstärke (0-100)
  when_focused: true                     # Sound auch bei fokussiertem Fenster
  done_sound: ""                         # Eigene Sounddatei (Fertig)
  input_sound: ""                        # Eigene Sounddatei (Eingabe)
  error_sound: ""                        # Eigene Sounddatei (Fehler)

# Sonstiges
logging_enabled: false                   # Debug-Logging aktivieren
commands: []                             # Gespeicherte Befehle (Befehlspalette)
```

---

# English

## What is Multiterminal UI?

Multiterminal UI is a native desktop application that brings multiple AI coding assistants (Claude Code, Codex, Gemini) together in a professional terminal interface. Instead of switching between many terminal windows, you have all sessions side by side — with automatic cost overview, activity detection, and Git integration.

## Features

### Terminals & Tabs
- **Multi-pane layout** — Up to 12 terminal sessions per tab in a tiled grid
- **Project tabs** — Each tab has its own working directory; add projects via folder picker
- **Session restore** — All tabs, panes, and layouts are automatically saved and restored on restart
- **Pane renaming** — Double-click any pane name to rename it
- **Zoom** — Ctrl+Z to maximize/restore a pane, Ctrl+Mouse Wheel for font size per terminal

### AI Assistants

Three AI CLI tools are supported — select one when opening a new terminal (Ctrl+N):

| Tool | Modes | Description |
|------|-------|-------------|
| **Claude Code** (Anthropic) | Normal / YOLO | AI coding assistant with optional model selection |
| **Codex** (OpenAI) | Standard / Auto | OpenAI Codex CLI |
| **Gemini** (Google) | Standard / Sandbox | Google Gemini CLI |

### Token & Cost Tracking
- **Per pane** — Each Claude pane shows its cost in the title bar (e.g. `$0.12`)
- **Total cost** — The global footer shows combined cost across all Claude panes
- **Automatic detection** — Costs are parsed in real-time from Claude Code output

### Activity Detection

Pane borders visually indicate Claude's state:

| Indicator | Meaning |
|-----------|---------|
| **Green glow** | Claude finished generating (prompt returned) |
| **Red/yellow blink** | Claude needs user input (confirmation, Y/N, etc.) |
| **Pulsing dot** | Claude is actively working |

See at a glance which pane needs your attention.

### Pipeline Queue
Queue up multiple prompts for a Claude pane. They execute sequentially:
1. Click the **▶** button in any pane's title bar
2. Type a prompt and press **Enter**
3. When Claude finishes, the next prompt is automatically sent

Perfect for batching tasks — set it up and walk away. The badge shows pending item count.

### File Browser (Sidebar)
- **Ctrl+B** to toggle
- Browse and search files
- Click a file to insert its path into the focused terminal
- **Favorites** — Bookmark frequently used files
- Git status indicator per file (modified, new, untracked, etc.)

### Git Integration
- **Commit reminder** — Footer shows time since last commit:
  - Green (under 15 min), Yellow (15–30 min), Red pulsing (30+ min)
- **Source control view** — File changes grouped by status (conflicts, modified, added, deleted, renamed)
- **Branch display** — Current branch shown in footer
- **Worktree support** — Optional isolated working directories per issue
- **Conflict detection** — Visual warning on merge/rebase conflicts

### GitHub Issues
- **Issue sidebar** — View all GitHub Issues for the current project
- **Issue details** — See title, description, labels, and comments
- **Create/edit issues** — Dialog for creating and editing issues
- **Launch Claude for an issue** — Click ▶ on any issue to start a Claude session with that issue's context
- **Issue linking** — Panes can be linked to GitHub issues (e.g. "Claude – #42")
- Requires [GitHub CLI](https://cli.github.com/) (`gh`)

### Command Palette
- **Ctrl+Shift+P** to open
- Save custom commands/scripts and run them by name
- Edit, delete, and organize your commands

### Audio Notifications
- **Done sound** — Plays when Claude finishes
- **Input sound** — Plays when Claude needs input
- **Error sound** — Plays on errors
- Volume control and "mute when focused" option
- Custom audio file support

### Themes & Appearance

| Theme | Description |
|-------|-------------|
| `dark` | Catppuccin Mocha (default) |
| `light` | Clean light theme |
| `dracula` | Dracula color scheme |
| `nord` | Nord color scheme |
| `solarized` | Solarized Dark |

- **Custom accent color** — Color picker, hex input, or 8 presets
- **Font** — Choose from installed monospace fonts
- **Font size** — 8–20px in 2px steps

### Languages

The UI is available in: German, **English**, Italian, Spanish, French.

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| **Ctrl+T** | New project tab (folder picker) |
| **Ctrl+W** | Close tab |
| **Ctrl+N** | New terminal pane (launch dialog) |
| **Ctrl+Z** | Maximize / restore focused pane |
| **Ctrl+1-9** | Focus pane by index |
| **Ctrl+B** | Toggle file browser sidebar |
| **Ctrl+F** | Terminal search |
| **Ctrl+I** | Show GitHub Issues |
| **Ctrl+Shift+P** | Command palette |
| **Ctrl+V** | Paste |
| **Ctrl+C** | Copy (when text selected) |
| **Ctrl+Scroll** | Font size per terminal |
| **Esc** | Close dialogs |

## Prerequisites

**To run:**
- At least one AI CLI tool: [Claude Code](https://docs.anthropic.com/en/docs/claude-code), [Codex](https://github.com/openai/codex), or [Gemini CLI](https://github.com/google-gemini/gemini-cli)
- [GitHub CLI](https://cli.github.com/) (`gh`) — for GitHub Issues integration

**To build from source:**
- [Go 1.21+](https://go.dev/dl/)
- [Node.js 18+](https://nodejs.org/)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

## Installation & Running

```bash
# Development mode (hot-reload)
wails dev

# Production build
wails build

# Debug build (with DevTools)
wails build -debug
```

The binary is output to `build/bin/mtui.exe` (Windows) or `build/bin/mtui` (Linux/macOS).

## Configuration

A config file is auto-created at `~/.multiterminal.yaml` on first run.

```yaml
# Appearance
theme: dark                              # dark, light, dracula, nord, solarized
terminal_color: "#39ff14"                # Accent color (hex)
font_family: ""                          # Monospace font name
font_size: 10                            # Font size (px)
language: de                             # Language (de/en/it/es/fr)

# Terminal
default_shell: ""                        # Default shell (auto-detected)
default_dir: ""                          # Default working directory
max_panes_per_tab: 12                    # Max terminals per tab
sidebar_width: 30                        # Sidebar width (characters)

# Git
commit_reminder_minutes: 30              # Commit reminder threshold (minutes)
use_worktrees: false                     # Create git worktrees per issue

# AI Tools
claude_enabled: true                     # Enable Claude Code
claude_command: ""                       # Claude CLI path (auto-detected)
claude_models:                           # Available Claude models
  - label: "Default"
    id: ""
  - label: "Opus 4.6"
    id: "claude-opus-4-6"
  - label: "Sonnet 4.5"
    id: "claude-sonnet-4-5-20250929"
  - label: "Haiku 4.5"
    id: "claude-haiku-4-5-20251001"
codex_enabled: false                     # Enable Codex (OpenAI)
codex_command: ""                        # Codex CLI path
gemini_enabled: false                    # Enable Gemini (Google)
gemini_command: ""                       # Gemini CLI path

# Audio
audio:
  enabled: true                          # Audio notifications
  volume: 50                             # Volume (0-100)
  when_focused: true                     # Play sound even when focused
  done_sound: ""                         # Custom done sound file
  input_sound: ""                        # Custom input sound file
  error_sound: ""                        # Custom error sound file

# Misc
logging_enabled: false                   # Enable debug logging
commands: []                             # Saved commands (command palette)
```

---

## License

MIT
