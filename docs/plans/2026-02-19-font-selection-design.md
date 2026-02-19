# F-016: Font Selection in Settings Dialog

Closes #55

## Summary

Add font family dropdown and font size setting to the Settings dialog with live preview on all terminals. Persist per-pane zoom delta in session state.

## Requirements

- Font family dropdown with curated monospace font list
- Font size setting (8-32px) with persistence
- Live application to all terminals (revert on cancel)
- Per-pane Ctrl+Wheel zoom delta persisted in session JSON

## Config Changes

### Go (`internal/config/config.go`)

New fields on `Config` struct:

```go
FontFamily string `yaml:"font_family" json:"font_family"`
FontSize   int    `yaml:"font_size"   json:"font_size"`
```

- `font_family` default: `""` (empty = fallback chain)
- `font_size` default: `14`
- Validation: `font_size` must be 8-32

### TypeScript (`frontend/src/stores/config.ts`)

Mirror fields in `AppConfig` interface:

```typescript
font_family: string;
font_size: number;
```

## Curated Font List

Defined as constant in `frontend/src/lib/terminal.ts`:

```
Cascadia Code, Cascadia Mono, Fira Code, JetBrains Mono,
Source Code Pro, IBM Plex Mono, Ubuntu Mono, Hack,
Inconsolata, Consolas, Courier New, monospace
```

Frontend checks availability via `document.fonts.check()`. Unavailable fonts shown as disabled in dropdown. `monospace` always available as "System Monospace".

## Terminal Integration

### Font Application

- `createTerminal()` receives font config or reads from config store
- `TerminalPane.svelte` reactively watches `$appConfig.font_family` and `$appConfig.font_size`
- On change: sets `terminal.options.fontFamily`, `terminal.options.fontSize`, calls `fitAddon.fit()`

### Zoom Persistence

- Ctrl+Wheel sets pane-local `zoomDelta` (existing behavior)
- `zoomDelta` saved per pane in `~/.multiterminal-session.json`
- Effective size = `config.font_size + zoomDelta`
- Restored on session load

## Settings Dialog

New "Font" section in SettingsDialog:

- **Font Family Dropdown**: Curated list, unavailable fonts marked. Empty = "Standard (Cascadia Code, ...)"
- **Font Size Slider/Input**: 8-32px, step 1
- **Live Application**: Changes applied immediately to all terminals. Reverted on cancel.

## Data Flow

```
Settings Dialog -> Config Store -> TerminalPane reactive watch
    -> terminal.options.fontFamily/fontSize -> fitAddon.fit()

Ctrl+Wheel Zoom -> pane-local zoomDelta -> session auto-save (debounced)
    -> effective size = config.font_size + zoomDelta

Session restore -> load zoomDelta per pane -> apply on mount
```

## Files to Modify

1. `internal/config/config.go` - Add FontFamily, FontSize fields + validation
2. `frontend/src/stores/config.ts` - Mirror new fields
3. `frontend/src/lib/terminal.ts` - Font list constant, read config in createTerminal
4. `frontend/src/components/TerminalPane.svelte` - Reactive font watch, zoom persistence
5. `frontend/src/components/SettingsDialog.svelte` - Font section UI
6. `frontend/src/stores/tabs.ts` - Add zoomDelta to pane state for session persistence
