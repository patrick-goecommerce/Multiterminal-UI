# F-028: Terminal Bell / Audio-Feedback

**Date:** 2026-02-16
**Issue:** #46
**Status:** Approved

## Problem

Visuelle Indikatoren (Glow/Pulse) sind bei Multitasking auf anderen Screens nicht sichtbar. Es fehlt ein Audio-Signal wenn Claude fertig ist oder Input braucht.

## Approach

Rein Frontend-basiert (Ansatz A). Audio-Logik lebt komplett im Frontend als `lib/audio.ts` Modul. Hakt sich in das bestehende `terminal:activity` Event ein. Keine neue Go-Dependency nötig.

## Config

Neues `AudioSettings`-Struct im Go-Backend (`internal/config/config.go`):

```go
type AudioSettings struct {
    Enabled     *bool  `yaml:"enabled" json:"enabled"`           // Default: true
    Volume      int    `yaml:"volume" json:"volume"`             // 0-100, Default: 50
    WhenFocused *bool  `yaml:"when_focused" json:"when_focused"` // Default: true
    DoneSound   string `yaml:"done_sound" json:"done_sound"`     // Custom .wav/.mp3 path or ""
    InputSound  string `yaml:"input_sound" json:"input_sound"`   // Custom .wav/.mp3 path or ""
    ErrorSound  string `yaml:"error_sound" json:"error_sound"`   // Custom .wav/.mp3 path or ""
}
```

Config-Feld: `Audio AudioSettings` in `Config` struct.

YAML example:
```yaml
audio:
  enabled: true
  volume: 50
  when_focused: true
  done_sound: ""
  input_sound: ""
  error_sound: ""
```

Frontend `AppConfig` interface gets matching `audio` field.

## Audio Module

**New file:** `frontend/src/lib/audio.ts` (~80-100 lines)

### Synthesized Default Sounds (Web Audio API)

- **done**: Two ascending tones (C5 -> E5, 100ms each) — friendly "ding-ding"
- **needsInput**: Three short equal tones (A4, 80ms each with pauses) — attention-grabbing
- **error**: One descending tone (E4 -> C4, 200ms) — unmistakably negative

### Custom Sounds

When a path is set in config, `new Audio(path)` is used instead of synthesis.

### API

```typescript
export function initAudio(): void
export function playBell(event: 'done' | 'needsInput' | 'error', volume: number): void
export function playCustomSound(path: string, volume: number): void
export const audioMuted: Writable<boolean>  // Svelte store for mute toggle
```

`AudioContext` is lazily created after first user interaction (browser autoplay policy).

## Event Integration

In `App.svelte`, inside the existing `terminal:activity` listener:

- Only triggers for Claude/YOLO panes (`pane.mode === 'claude' || pane.mode === 'claude-yolo'`)
- Respects `audioMuted` store and `config.audio.enabled`
- Respects `config.audio.when_focused` — if false, only plays when window is unfocused
- Activity `done` -> done sound
- Activity `needsInput` -> input sound
- `terminal:error` event -> error sound (separate listener)

## Footer Mute Button

Speaker icon in `Footer.svelte` between `footer-center` and `footer-right`:

- Click toggles `audioMuted` store
- Visual: SVG speaker icon (strikethrough line when muted)
- State is not persisted (starts unmuted each session) — config `enabled` is the persistent switch

## Files Changed

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `AudioSettings` struct + `Audio` field to `Config` + defaults |
| `frontend/src/stores/config.ts` | Add `audio` field to `AppConfig` interface |
| `frontend/src/lib/audio.ts` | **New** — Web Audio API synthesis + custom sound support + muted store |
| `frontend/src/App.svelte` | Import audio module, call `playBell()` in activity listener, call `initAudio()` |
| `frontend/src/components/Footer.svelte` | Add mute toggle button with speaker SVG icon |

## Out of Scope

- Volume slider in UI (config only)
- Per-pane audio settings
- Sound preview/test in settings dialog
- Persisting mute state across sessions
