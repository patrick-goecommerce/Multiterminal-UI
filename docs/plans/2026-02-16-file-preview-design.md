# F-019: Datei-Vorschau (Quick Peek) im Sidebar

Closes #61

## Overview

Inline-Dateivorschau als schwebendes Overlay-Modal. Klick auf eine Datei im Sidebar-Explorer öffnet eine read-only Preview mit Syntax-Highlighting, Zeilennummern und einem Button zum Öffnen im System-Editor.

## Backend API

Zwei neue Methoden in `internal/backend/app_files.go`:

### ReadFile(path string) FileContent

```go
type FileContent struct {
    Path    string `json:"path"`
    Name    string `json:"name"`
    Content string `json:"content"`
    Size    int64  `json:"size"`
    Error   string `json:"error"`
    Binary  bool   `json:"binary"`
}
```

- Dateigröße prüfen: >1 MB → `Error: "Datei zu groß"`
- Binärerkennung: NUL-Bytes in den ersten 512 Bytes → `Binary: true`, kein Content
- UTF-8-Content als String zurückgeben

### OpenFileInEditor(path string) string

- Windows: `cmd /c start "" "path"` (ShellExecute-Semantik)
- Gibt leeren String bei Erfolg, Fehlermeldung bei Fehler zurück

## Frontend-Komponente: FilePreview.svelte

### Layout

- **Backdrop:** Halbtransparent schwarz, Klick schließt Modal
- **Panel:** 80% Breite, 80% Höhe, zentriert, abgerundete Ecken
- **Hintergrund:** `var(--bg-secondary)`, Border: `var(--border)`

### Header

- Links: Dateiname (groß) + voller Pfad (klein, `--fg-muted`)
- Rechts: "Im Editor öffnen"-Button + Schließen-Button (×)

### Content

- Zeilennummern links (feste Spalte, `--fg-muted`, rechtsbündig)
- Code rechts mit highlight.js Syntax-Highlighting
- Horizontales + vertikales Scrolling
- Monospace-Font

### Fehlerzustände

- Datei zu groß → Hinweis + Editor-Button
- Binärdatei → Hinweis + Editor-Button
- Lesefehler → Fehlermeldung

### Tastatur

- Escape → schließt Modal

## highlight.js Integration

- Paket: `highlight.js` via npm
- Import: `highlight.js/lib/core` + einzelne Sprachen registrieren
- Sprachen: JavaScript, TypeScript, Go, Python, YAML, JSON, HTML, CSS, Bash, Markdown, Rust, SQL, Dockerfile
- Spracherkennung: `hljs.highlightAuto()`
- Fallback: Plain-Text wenn keine Sprache erkannt

### Theme (CSS-Variablen-basiert)

```css
.hljs { background: var(--bg-tertiary); color: var(--fg); }
.hljs-keyword, .hljs-selector-tag { color: var(--accent); }
.hljs-string, .hljs-addition { color: var(--success); }
.hljs-comment, .hljs-quote { color: var(--fg-muted); font-style: italic; }
.hljs-number, .hljs-literal { color: var(--warning); }
.hljs-deletion { color: var(--error); }
.hljs-title, .hljs-function { color: var(--accent-hover); }
.hljs-type, .hljs-built_in { color: var(--warning); }
.hljs-attr, .hljs-variable { color: var(--fg); }
```

Funktioniert automatisch mit allen 5 Themes.

## Integration in App.svelte

### Datenfluss

```
Klick auf Datei → selectFile-Event → App.svelte
  → previewFilePath = path
  → FilePreview wird sichtbar
  → FilePreview ruft App.ReadFile(path) auf
  → Zeigt Inhalt mit highlight.js an
```

### Änderungen

- `handleSidebarFile` umschreiben: Preview öffnen statt Pfad ins Terminal schreiben
- Neue State-Variable: `previewFilePath: string = ''`
- FilePreview-Komponente einbinden mit `visible={!!previewFilePath}`

## Scope

### Enthalten

- ReadFile + OpenFileInEditor Backend-APIs
- FilePreview.svelte Overlay-Modal
- Syntax-Highlighting mit highlight.js (13 Sprachen)
- CSS-Variablen-basiertes Theme
- 1 MB Dateigrößen-Limit
- Binärdateierkennung

### Nicht enthalten (YAGNI)

- Kein Editieren in der Preview
- Keine Datei-Tabs / History
- Keine Bildervorschau
- Kein konfigurierbarer Editor-Command
- Kein Caching
- Keine Dateigröße im Sidebar

## Dateien

| Datei | Aktion |
|---|---|
| `internal/backend/app_files.go` | Erweitern |
| `frontend/src/components/FilePreview.svelte` | Neu |
| `frontend/src/App.svelte` | Ändern |
| `frontend/package.json` | Dependency hinzufügen |
