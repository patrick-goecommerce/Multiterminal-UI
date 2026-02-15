# GitHub Issues Integration - Implementierungsplan

## Übersicht

Integration von GitHub Issues in Multiterminal-UI: Issues lesen, erstellen und bearbeiten direkt aus der App heraus. Nutzt die `gh` CLI (GitHub CLI) im Backend und erweitert die bestehende Sidebar im Frontend um einen dritten View-Tab.

---

## 1. Backend: `internal/backend/app_issues.go` (Go)

Neue Datei mit Wails-Bindings, die `gh` CLI-Befehle ausführen (analog zu `app_git.go` mit `exec.Command`).

### Datentypen

```go
type Issue struct {
    Number    int      `json:"number"`
    Title     string   `json:"title"`
    State     string   `json:"state"`      // "open" / "closed"
    Author    string   `json:"author"`
    Labels    []string `json:"labels"`
    CreatedAt string   `json:"createdAt"`
    UpdatedAt string   `json:"updatedAt"`
    Comments  int      `json:"comments"`
    URL       string   `json:"url"`
}

type IssueDetail struct {
    Issue
    Body     string          `json:"body"`
    Comments []IssueComment  `json:"commentList"`
    Assignees []string       `json:"assignees"`
    Milestone string         `json:"milestone"`
}

type IssueComment struct {
    Author    string `json:"author"`
    Body      string `json:"body"`
    CreatedAt string `json:"createdAt"`
}

type IssueLabel struct {
    Name  string `json:"name"`
    Color string `json:"color"`
}
```

### API-Methoden (Wails-Bindings)

| Methode | Beschreibung | `gh` Befehl |
|---------|-------------|-------------|
| `GetIssues(dir, state string) []Issue` | Issues auflisten (open/closed/all) | `gh issue list --state X --json ...` |
| `GetIssueDetail(dir string, number int) IssueDetail` | Einzelnes Issue mit Body + Kommentaren | `gh issue view N --json ...` + `gh api` für Kommentare |
| `CreateIssue(dir, title, body string, labels []string) (Issue, error)` | Neues Issue erstellen | `gh issue create --title X --body Y --label Z` |
| `UpdateIssue(dir string, number int, title, body, state string) error` | Issue bearbeiten (Titel, Body, Status) | `gh issue edit N --title X --body Y` + `gh issue close/reopen` |
| `AddIssueComment(dir string, number int, body string) error` | Kommentar hinzufügen | `gh issue comment N --body X` |
| `GetIssueLabels(dir string) []IssueLabel` | Verfügbare Labels abrufen | `gh label list --json name,color` |
| `CheckGitHubCLI() bool` | Prüft ob `gh` installiert und authentifiziert ist | `gh auth status` |

### Voraussetzung
- `gh` CLI muss installiert und authentifiziert sein (`gh auth login`)
- Wird beim ersten Aufruf geprüft, Frontend zeigt Hinweis wenn nicht verfügbar

---

## 2. Frontend: Sidebar-Erweiterung

### 2a. Sidebar.svelte anpassen

Die bestehende `view-toggle` Leiste um einen dritten Button "Issues" erweitern:

```
[Explorer] [Source Control] [Issues (3)]
```

- `activeView` erhält neuen Typ: `'explorer' | 'source-control' | 'issues'`
- Bei `activeView === 'issues'` wird `<IssuesView>` gerendert
- Badge zeigt Anzahl offener Issues

### 2b. Neue Komponente: `IssuesView.svelte` (~250 Zeilen)

Wird innerhalb der Sidebar gerendert wenn Issues-Tab aktiv ist.

**Ansichten:**
1. **Listenansicht** (Standard)
   - Filter-Buttons: `Open` | `Closed` | `All`
   - Suchfeld zum Filtern nach Titel
   - Issue-Liste mit: Nummer, Titel, Labels (farbige Badges), Autor, Kommentar-Anzahl
   - Klick auf Issue → Detailansicht
   - "+" Button → Neues Issue erstellen (öffnet Dialog)

2. **Detailansicht**
   - Zurück-Button zur Liste
   - Issue-Titel (editierbar via Klick)
   - Status-Badge (open/closed) mit Toggle-Button
   - Labels (farbige Badges)
   - Body (Markdown-formatiert als Text)
   - Kommentare chronologisch (Autor, Datum, Text)
   - Textarea + Button zum Hinzufügen neuer Kommentare
   - "Bearbeiten" Button → öffnet IssueDialog

### 2c. Neue Komponente: `IssueDialog.svelte` (~200 Zeilen)

Modal-Dialog für Issue erstellen/bearbeiten (analog zu `LaunchDialog.svelte`).

**Felder:**
- Titel (Textfeld, required)
- Body/Beschreibung (Textarea, Markdown)
- Labels (Multi-Select aus verfügbaren Labels)
- Status (nur beim Bearbeiten: Open/Closed Dropdown)
- Buttons: Speichern / Abbrechen

### 2d. App.svelte Integration

- Neue Event-Handler für Issue-Aktionen
- Sidebar erhält Events für Issue-Interaktionen (z.B. Issue-Nummer in fokussiertes Terminal einfügen)

---

## 3. Dateistruktur (Neue/Geänderte Dateien)

```
Neue Dateien:
  internal/backend/app_issues.go        # Go Backend: GitHub Issues API (~250 Zeilen)
  frontend/src/components/IssuesView.svelte    # Issue-Liste + Detailansicht (~250 Zeilen)
  frontend/src/components/IssueDialog.svelte   # Issue erstellen/bearbeiten Dialog (~200 Zeilen)

Geänderte Dateien:
  frontend/src/components/Sidebar.svelte       # Dritter View-Tab "Issues" hinzufügen
  frontend/src/App.svelte                      # IssueDialog einbinden, Event-Handler
```

---

## 4. Implementierungsreihenfolge

### Schritt 1: Backend - `app_issues.go`
- `CheckGitHubCLI()` implementieren
- `GetIssues()` implementieren (gh issue list)
- `GetIssueDetail()` implementieren (gh issue view + Kommentare)
- `CreateIssue()` implementieren
- `UpdateIssue()` implementieren
- `AddIssueComment()` implementieren
- `GetIssueLabels()` implementieren

### Schritt 2: Frontend - IssuesView.svelte
- Issue-Listenansicht mit Filter und Suche
- Issue-Detailansicht mit Kommentaren
- Kommentar-Eingabe
- Status-Toggle (Open/Close)

### Schritt 3: Frontend - IssueDialog.svelte
- Create-Modus: Neues Issue
- Edit-Modus: Bestehendes Issue bearbeiten
- Label-Auswahl

### Schritt 4: Sidebar + App Integration
- Sidebar.svelte: Dritten Tab hinzufügen
- App.svelte: Dialog einbinden und Events verdrahten

### Schritt 5: Wails Bindings generieren
- `wails generate module` um TypeScript-Bindings für neue Go-Methoden zu erzeugen

---

## 5. UX-Details

- **Polling:** Issues werden beim Tab-Wechsel zu "Issues" geladen, nicht periodisch (anders als Git-Status)
- **Refresh-Button:** Manueller Refresh in der Issues-Ansicht
- **Fehlerbehandlung:** Wenn `gh` nicht installiert → Info-Nachricht mit Installationsanweisung
- **Sprache:** UI-Texte auf Deutsch (konsistent mit bestehendem Code, z.B. "Keine Ergebnisse", "Suchen...")
- **Keyboard Shortcut:** Kein neuer Shortcut nötig, Issues sind über die Sidebar (Ctrl+B) erreichbar
