# F-020: Favoriten/Bookmarks für Dateien

**Issue:** #63
**Date:** 2026-02-19

## Problem

Häufig genutzte Dateien und Verzeichnisse müssen jedes Mal durch die Verzeichnisstruktur navigiert werden.

## Design-Entscheidungen

- **Scope:** Pro Arbeitsverzeichnis (beim Tab-Wechsel sieht man andere Favoriten)
- **Klick-Aktion:** Pfad ins fokussierte Terminal einfügen (wie Drag & Drop)
- **Typen:** Dateien und Verzeichnisse können gepinnt werden
- **Persistierung:** In `~/.multiterminal.yaml` als Map
- **UI-Position:** Collapsible Sektion oben im Explorer-View der Sidebar

## Datenmodell

Neues Feld in Config:

```yaml
favorites:
  "D:\\repos\\Multiterminal":
    - "D:\\repos\\Multiterminal\\internal\\backend\\app.go"
    - "D:\\repos\\Multiterminal\\frontend\\src"
  "D:\\repos\\OtherProject":
    - "D:\\repos\\OtherProject\\main.go"
```

Go struct: `Favorites map[string][]string` mit `yaml:"favorites" json:"favorites"` Tags.

## Backend-Bindings

- `GetFavorites(dir string) []string` — gibt Favoriten für ein Verzeichnis zurück
- `AddFavorite(dir, path string) error` — fügt Favorit hinzu, speichert Config
- `RemoveFavorite(dir, path string) error` — entfernt Favorit, speichert Config

## UI-Komponenten

### FavoritesSection.svelte (neu)

Collapsible Sektion im Explorer-View:
- Stern-Icon + "Favorites" Header mit Collapse-Toggle
- Jeder Eintrag: Datei/Ordner-Icon, Name, Tooltip mit vollem Pfad
- Klick dispatcht `selectFile` (Pfad ins Terminal)
- Drag & Drop unterstützt
- Remove-Button (Unpin) beim Hover
- "Keine Favoriten" Platzhalter wenn leer

### FileTreeItem.svelte (erweitert)

- Neuer Star-Button neben Copy-Button, erscheint bei Hover
- Gefüllter Stern = ist Favorit, leerer Stern = kein Favorit
- Klick toggled Favorit-Status via `toggleFavorite` Event

### Sidebar.svelte (erweitert)

- Lädt Favoriten beim Mount und bei dir-Wechsel via `App.GetFavorites(dir)`
- Rendert FavoritesSection oberhalb der Suchleiste im Explorer-View
- Leitet `toggleFavorite` Events an Backend weiter

## Datenfluss

```
Star-Klick → FileTreeItem dispatcht toggleFavorite
  → Sidebar ruft App.AddFavorite/RemoveFavorite
  → Go aktualisiert Config, speichert YAML
  → Sidebar aktualisiert lokale favorites-Liste

Favorit-Klick → FavoritesSection dispatcht selectFile
  → App.svelte fügt Pfad ins fokussierte Terminal ein
```
