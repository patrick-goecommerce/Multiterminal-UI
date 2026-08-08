# Multiterminal UI (mtui)

<img width="1920" height="1032" alt="image" src="https://github.com/user-attachments/assets/4db36c31-1140-4cba-bab7-768a7a1eb65d" />

**[Deutsch](#deutsch) | [English](#english)**

![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-blue)

> Technische Doku (Build, Konfiguration, Architektur) → [README_TECH.md](README_TECH.md)
> Technical docs (build, configuration, architecture) → [README_TECH.md](README_TECH.md)

---

# Deutsch

## Was ist Multiterminal UI?

Multiterminal UI ist eine native Desktop-Anwendung, die mehrere KI-Coding-Assistenten (Claude Code, Codex, Gemini) in einer professionellen Terminal-Oberfläche vereint. Statt zwischen vielen Terminalfenstern zu wechseln, hast du alle Sessions nebeneinander — mit automatischer Kostenübersicht, Aktivitätserkennung und Git-Integration.

## Funktionen

### Terminals & Tabs
- **Multi-Pane-Layout** — Mehrere Terminal-Sessions pro Tab in einem Kachelraster
- **Projekt-Tabs** — Jeder Tab hat sein eigenes Arbeitsverzeichnis; Projekte über Ordnerauswahl hinzufügen
- **Zuletzt geöffneter Ordner** — MTUI merkt sich, wo du zuletzt gearbeitet hast, und startet dort wieder
- **Session-Wiederherstellung** — Alle Tabs, Panes und Layouts werden automatisch gespeichert und beim Neustart wiederhergestellt
- **Mehrere Fenster** — Tabs per Drag & Drop in ein eigenes Fenster abdocken
- **Pane umbenennen** — Doppelklick auf den Pane-Namen zum Umbenennen; optional automatische Benennung anhand des ersten Prompts
- **Zoom** — Ctrl+Z zum Maximieren/Wiederherstellen eines Panes, Ctrl+Mausrad für Schriftgröße pro Terminal

### KI-Assistenten

Drei KI-CLI-Tools werden unterstützt — beim Öffnen eines neuen Terminals (Ctrl+N) wählst du:

| Tool | Modi | Beschreibung |
|------|------|--------------|
| **Claude Code** (Anthropic) | Normal / Auto / YOLO | KI-Coding-Assistent mit optionaler Modellauswahl |
| **Codex** (OpenAI) | Standard / Auto | OpenAI Codex CLI |
| **Gemini** (Google) | Standard / Sandbox | Google Gemini CLI |

Zusätzlich lässt sich per Sprachsteuerung diktieren, und Panes können statt als reines Terminal auch im Chat-Ansicht dargestellt werden.

### Agent-Delegation

Ein Agent, der in einem Pane läuft (z.B. Claude Code), kann selbstständig eine neue Session mit einem der drei Tools öffnen, ihr eine Aufgabe schicken und sie später wieder schließen — praktisch, um eine Teilaufgabe an ein anderes Modell zu delegieren. Die delegierte Session erscheint automatisch sichtbar als neues Pane.

### Multi-Agent-Orchestrierung (Kanban)

Ein Kanban-Board pro Projekt (Spalten von "Entwurf" bis "Fertig") kann Karten automatisch als eigene Agent-Sessions in isolierten Git-Worktrees starten, deren Fortschritt verfolgen und nach erfolgreichem Review/Test automatisch einen Pull Request erstellen.

### Token- & Kostenübersicht
- **Pro Pane** — Jedes Claude-Pane zeigt seine Kosten in der Titelleiste (z.B. `$0.12`)
- **Gesamtkosten** — Die globale Fußzeile zeigt die Kosten aller Claude-Panes zusammen
- **Automatische Erkennung** — Kosten werden in Echtzeit aus der Claude-Code-Ausgabe geparst
- **Claude Statusline** — Optionale Statuszeile in Claude-Code-Panes (Modell, Kontext-%, Kosten, Branch)

### Aktivitätserkennung

Die Pane-Rahmen zeigen den Status visuell an:

| Anzeige | Bedeutung |
|---------|-----------|
| **Grünes Leuchten** | Agent ist fertig (Prompt zurückgekehrt) |
| **Gelbes Pulsieren** | Agent wartet auf Eingabe (Bestätigung, J/N, etc.) |

So siehst du auf einen Blick, welches Pane deine Aufmerksamkeit braucht. Ein optionales Session-Keep-Alive verhindert, dass eine inaktive Claude-Session durch Zeitüberschreitung verloren geht.

### Pipeline-Warteschlange & Quick Actions
- Reihe mehrere Prompts für ein Pane aneinander — sie werden nacheinander gesendet, sobald der Agent bereit ist
- Eigene "Quick Action"-Buttons in der Pane-Titelleiste für wiederkehrende, vorformulierte Prompts

### Dateibrowser (Seitenleiste)
- **Ctrl+B** zum Ein-/Ausblenden
- Dateien durchsuchen und navigieren
- Klick auf eine Datei fügt den Pfad ins fokussierte Terminal ein
- **Favoriten** — Häufig genutzte Dateien als Lesezeichen speichern
- Git-Status-Anzeige pro Datei (geändert, neu, unverfolgt, etc.)

### Git-Integration
- **Commit-Erinnerung** — Die Fußzeile zeigt die Zeit seit dem letzten Commit (grün/gelb/rot)
- **Quellcodeverwaltung** — Dateiänderungen gruppiert nach Status
- **Branch-Anzeige** — Aktueller Branch in der Fußzeile
- **Worktree-Unterstützung** — Isolierte Arbeitsverzeichnisse pro Issue/Karte, inkl. geführtem "Fertigstellen"-Ablauf (Review, Commit, PR, Cleanup)
- **Worktree-Pflicht** (optional) — Claude-Panes dürfen Code dann nur noch in einem Worktree ändern, nie direkt im Projektverzeichnis; Dokumentation und Planung bleiben ausgenommen. Global oder pro Projekt einstellbar
- **Konflikterkennung** — Visuelle Warnung bei Merge-/Rebase-Konflikten

### GitHub Issues & Pull Requests
- **Issue-Seitenleiste** — Alle GitHub Issues des aktuellen Projekts anzeigen, erstellen und bearbeiten
- **Agent für Issue starten** — Klick auf ▶ bei einem Issue startet eine Session mit dem Issue-Kontext
- **Automatischer Fortschrittsbericht** — Optionale Kommentare im Issue bei Start/Fertig/Schließen
- **Schnellzugriff** — Links zu den GitHub-Issues- und Pull-Requests-Seiten des Projekts direkt in der Fußzeile
- Benötigt [GitHub CLI](https://cli.github.com/) (`gh`)

### Dashboard
Ein Übersichtsfenster über alle offenen Panes und Tabs mit Live-Status — praktisch, wenn viele Sessions gleichzeitig laufen.

### Skills-System
Projektspezifische "Skills" (z.B. Backend, DevOps, Technical Writing) lassen sich einem Projekt zuordnen; sie werden automatisch in die `CLAUDE.md` des Projekts eingebunden.

### Hintergrund-Agents
Optionale automatische Review- und Test-Läufe nach jedem Commit, per frei wählbarem KI-Tool und Prompt.

### Befehlspalette
- Eigene Befehle/Skripte speichern und per Toolbar ausführen
- Befehle bearbeiten, löschen und organisieren

### Audio-Benachrichtigungen
- **Fertig-/Eingabe-/Fehler-Sound** — je nach Agent-Status
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

- **Eigene Akzentfarbe** — Farbwähler, Hex-Eingabe oder Presets
- **Schriftart & -größe** — Aus installierten Monospace-Schriften wählen, 8–20px

### Sprachen

Die Oberfläche ist verfügbar in: **Deutsch**, Englisch, Italienisch, Spanisch, Französisch.

## Tastenkürzel

| Taste | Aktion |
|-------|--------|
| **Ctrl+T** | Neuer Projekt-Tab (Ordnerauswahl) |
| **Ctrl+W** | Tab schließen |
| **Ctrl+N** | Neues Terminal-Pane (Startdialog) |
| **Ctrl+Z** | Pane maximieren / wiederherstellen |
| **Ctrl+1–9** | Pane nach Index fokussieren |
| **Ctrl+B** | Dateibrowser-Seitenleiste ein-/ausblenden |
| **Ctrl+F** | Terminal-Suche |
| **Ctrl+I** | GitHub Issues anzeigen |
| **Ctrl+Shift+H** | Dashboard ein-/ausblenden |
| **Ctrl+Shift+S** | Skills-Editor öffnen |
| **Ctrl+V** | Einfügen |
| **Ctrl+C** | Kopieren (bei Auswahl) |
| **Ctrl+Scroll** | Schriftgröße pro Terminal |
| **Esc** | Dialoge schließen |

## Voraussetzungen

- Mindestens eines der KI-CLI-Tools: [Claude Code](https://docs.anthropic.com/en/docs/claude-code), [Codex](https://github.com/openai/codex), oder [Gemini CLI](https://github.com/google-gemini/gemini-cli)
- [GitHub CLI](https://cli.github.com/) (`gh`) — für Issues, Pull Requests und die Agent-Orchestrierung

Alle Einstellungen sind über den Einstellungsdialog in der App erreichbar; für Details zur Konfigurationsdatei siehe [README_TECH.md](README_TECH.md).

---

# English

## What is Multiterminal UI?

Multiterminal UI is a native desktop application that brings multiple AI coding assistants (Claude Code, Codex, Gemini) together in a professional terminal interface. Instead of switching between many terminal windows, you have all sessions side by side — with automatic cost overview, activity detection, and Git integration.

## Features

### Terminals & Tabs
- **Multi-pane layout** — Multiple terminal sessions per tab in a tiled grid
- **Project tabs** — Each tab has its own working directory; add projects via folder picker
- **Remembers your last folder** — MTUI remembers where you last worked and starts there again
- **Session restore** — All tabs, panes, and layouts are automatically saved and restored on restart
- **Multiple windows** — Drag a tab out to detach it into its own window
- **Pane renaming** — Double-click any pane name to rename it; optional automatic naming from the first prompt
- **Zoom** — Ctrl+Z to maximize/restore a pane, Ctrl+Mouse Wheel for font size per terminal

### AI Assistants

Three AI CLI tools are supported — select one when opening a new terminal (Ctrl+N):

| Tool | Modes | Description |
|------|-------|-------------|
| **Claude Code** (Anthropic) | Normal / Auto / YOLO | AI coding assistant with optional model selection |
| **Codex** (OpenAI) | Standard / Auto | OpenAI Codex CLI |
| **Gemini** (Google) | Standard / Sandbox | Google Gemini CLI |

Voice dictation is also supported, and any pane can be switched from a raw terminal to a chat-style view.

### Agent Delegation

An agent running in a pane (e.g. Claude Code) can autonomously open a new session with any of the three tools, hand it a task, and close it again later — handy for delegating a subtask to another model. The delegated session automatically shows up as a visible pane.

### Multi-Agent Orchestration (Kanban)

A per-project Kanban board (columns from "draft" to "done") can automatically launch cards as their own agent sessions in isolated git worktrees, track their progress, and open a pull request once review/tests pass.

### Token & Cost Tracking
- **Per pane** — Each Claude pane shows its cost in the title bar (e.g. `$0.12`)
- **Total cost** — The global footer shows combined cost across all Claude panes
- **Automatic detection** — Costs are parsed in real-time from Claude Code output
- **Claude statusline** — Optional status line inside Claude Code panes (model, context %, cost, branch)

### Activity Detection

Pane borders visually indicate an agent's state:

| Indicator | Meaning |
|-----------|---------|
| **Green glow** | Agent finished generating (prompt returned) |
| **Yellow pulse** | Agent needs user input (confirmation, Y/N, etc.) |

See at a glance which pane needs your attention. An optional session keep-alive prevents an idle Claude session from timing out.

### Pipeline Queue & Quick Actions
- Queue up multiple prompts for a pane — they're sent one after another once the agent is ready
- Custom "quick action" buttons in a pane's title bar for recurring, pre-written prompts

### File Browser (Sidebar)
- **Ctrl+B** to toggle
- Browse and search files
- Click a file to insert its path into the focused terminal
- **Favorites** — Bookmark frequently used files
- Git status indicator per file (modified, new, untracked, etc.)

### Git Integration
- **Commit reminder** — Footer shows time since last commit (green/yellow/red)
- **Source control view** — File changes grouped by status
- **Branch display** — Current branch shown in footer
- **Worktree support** — Isolated working directories per issue/card, with a guided "finish" flow (review, commit, PR, cleanup)
- **Mandatory worktrees** (optional) — Claude panes may then only change code inside a worktree, never directly in the project directory; documentation and planning stay exempt. Configurable globally or per project
- **Conflict detection** — Visual warning on merge/rebase conflicts

### GitHub Issues & Pull Requests
- **Issue sidebar** — View, create, and edit GitHub Issues for the current project
- **Launch an agent for an issue** — Click ▶ on any issue to start a session with that issue's context
- **Automatic progress reporting** — Optional comments on start/done/close
- **Quick access** — Links to the project's GitHub Issues and Pull Requests pages right in the footer
- Requires [GitHub CLI](https://cli.github.com/) (`gh`)

### Dashboard
An overview window across all open panes and tabs with live status — handy when many sessions are running at once.

### Skills System
Project-specific "skills" (e.g. Backend, DevOps, Technical Writing) can be assigned to a project; they're automatically injected into the project's `CLAUDE.md`.

### Background Agents
Optional automatic review and test runs after every commit, using a tool and prompt of your choice.

### Command Palette
- Save custom commands/scripts and run them from the toolbar
- Edit, delete, and organize your commands

### Audio Notifications
- **Done / input / error sounds** depending on agent state
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

- **Custom accent color** — Color picker, hex input, or presets
- **Font & size** — Choose from installed monospace fonts, 8–20px

### Languages

The UI is available in: German, **English**, Italian, Spanish, French.

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| **Ctrl+T** | New project tab (folder picker) |
| **Ctrl+W** | Close tab |
| **Ctrl+N** | New terminal pane (launch dialog) |
| **Ctrl+Z** | Maximize / restore focused pane |
| **Ctrl+1–9** | Focus pane by index |
| **Ctrl+B** | Toggle file browser sidebar |
| **Ctrl+F** | Terminal search |
| **Ctrl+I** | Show GitHub Issues |
| **Ctrl+Shift+H** | Toggle dashboard |
| **Ctrl+Shift+S** | Open skills editor |
| **Ctrl+V** | Paste |
| **Ctrl+C** | Copy (when text selected) |
| **Ctrl+Scroll** | Font size per terminal |
| **Esc** | Close dialogs |

## Prerequisites

- At least one AI CLI tool: [Claude Code](https://docs.anthropic.com/en/docs/claude-code), [Codex](https://github.com/openai/codex), or [Gemini CLI](https://github.com/google-gemini/gemini-cli)
- [GitHub CLI](https://cli.github.com/) (`gh`) — for Issues, Pull Requests, and agent orchestration

All settings are available through the in-app Settings dialog; see [README_TECH.md](README_TECH.md) for the config file reference.

---

## License

MIT
