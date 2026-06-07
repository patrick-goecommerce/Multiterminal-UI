# Voice Input (Speech-to-Text) im Chat — Design Spec

**Datum:** 2026-05-25
**Status:** Approved (Design), bereit für Implementierungsplanung
**Branch-Kontext:** `feat/chat-pane-display-mode` (Worktree); baut auf der Chat-Pane-Implementierung auf.
**Verwandt:** `docs/superpowers/specs/2026-05-24-chat-pane-display-mode-design.md`

## Problem

Im Chat-Pane soll man per Mikrofon-Button (neben dem Absenden-Button) diktieren können (Speech-to-Text).

Claude Code hat zwar `/voice` (Push-to-Talk-Diktat, ab v2.1.69), **aber nur im interaktiven TUI**. Unsere
Chat-Panes treiben `claude` **headless** (`-p --output-format stream-json`), wo Slash-Commands wie `/voice`
nicht ausgeführt werden (sie gelten als reiner Text). Außerdem unterstützt **WebView2 die Web Speech API
(`SpeechRecognition`) nicht** (offenes MS-Issue seit 2021). Daher müssen wir STT selbst integrieren.

Quellen: [Voice dictation – Claude Code Docs](https://code.claude.com/docs/en/voice-dictation),
[Headless mode](https://code.claude.com/docs/en/headless),
[WebView2Feedback #1613](https://github.com/MicrosoftEdge/WebView2Feedback/issues/1613).

## Was in WebView2 funktioniert

`getUserMedia` + `MediaRecorder` (Mikrofon-Aufnahme) sind verfügbar. Das aufgenommene Audio wird an eine
STT-Engine geschickt. Drei Engines sind in den Einstellungen wählbar.

## Recherche: Engines

- **Cloud-Whisper:** Whisper-kompatible HTTP-API (OpenAI `/v1/audio/transcriptions`, Groq, Azure). Top
  Deutsch-Genauigkeit, schnell, wenig Code; braucht Internet + API-Key.
- **whisper.cpp:** C++-Engine für OpenAIs Whisper-Modelle (GGML). Kleine Windows-Binary + Modell
  (~150–500 MB). Multilingual inkl. Deutsch. Einfachster lokaler Embed.
- **Parakeet v3 (NVIDIA):** anderes Modell als Whisper; CPU-optimiert, Auto-Sprache, 25 Sprachen inkl.
  Deutsch. Läuft via **sherpa-onnx** (ONNX-Runtime + ONNX-Modell) — **nicht** whisper.cpp. Höchstes
  Distributions-/Integrationsrisiko (größere Runtime). Die Open-Source-App **Handy** (handy.computer) nutzt
  beide Modelle lokal und arbeitet push-to-talk → fügt Text ins fokussierte Feld ein (sofortiger Workaround
  für den Nutzer, unabhängig von dieser Integration).

Quellen: [Handy (GitHub)](https://github.com/cjpais/Handy),
[Parakeet V3 vs Whisper](https://whispernotes.app/blog/parakeet-v3-default-mac-model).

## Designentscheidungen

| Entscheidung | Wahl |
|---|---|
| Engines | **Drei**, in Settings wählbar: `cloud-whisper`, `whisper-cpp`, `parakeet` |
| Abstraktion | Go-Interface `Transcriber`; Factory nach `config.stt.provider`; Frontend kennt nur EINE Binding |
| Aufnahme-UX | **Push-to-talk** (drücken→aufnehmen, loslassen→transkribieren) |
| Ziel des Transkripts | An die Textarea **anhängen, editierbar, KEIN Auto-Send** |
| Cloud-Key | Settings-Feld; leer ⇒ Fallback auf `OPENAI_API_KEY`-Env |
| Lokale Binaries/Modelle | **On-demand-Download** nach `~/.multiterminal/stt/<engine>/`, nicht im Installer gebündelt |
| Audio-Format | Backend normalisiert nach WAV 16 kHz mono (ffmpeg falls vorhanden); MediaRecorder liefert sonst webm/opus |
| Phasing | cloud-whisper + whisper.cpp = sicher v1; **parakeet = letzter, abtrennbarer Task** (Risiko) |

## Architektur

### 1. Backend — `Transcriber`-Interface + Provider

Neue Dateien unter `internal/backend/` (300-Zeilen-Regel beachten):

```go
// Transcriber turns recorded audio into text.
type Transcriber interface {
    // Transcribe takes raw recorded audio bytes (mime, e.g. "audio/webm")
    // and returns recognized text. lang is an ISO code or "auto".
    Transcribe(ctx context.Context, audio []byte, mime, lang string) (string, error)
}
```

- `app_stt.go` — Wails-Binding `TranscribeAudio(audioBase64, mime string) (string, error)`: decodiert base64,
  wählt via `a.cfg.STT.Provider` den `Transcriber`, ruft `Transcribe` auf, gibt Text zurück.
- `stt_cloud.go` — `cloudWhisperTranscriber`: baut multipart/form-data POST an `base_url` (default
  `https://api.openai.com/v1`) `/audio/transcriptions`, `model` (default `whisper-1`), `language`,
  Bearer-Key (Config oder `OPENAI_API_KEY`). Parsed `{"text": "..."}`.
- `stt_local.go` — gemeinsame Helfer: `ensureEngine(engine)` (Binary+Modell sicherstellen, sonst Download
  mit Fortschritts-Event `stt:download`), `toWav16kMono(audio, mime)` (ffmpeg falls vorhanden, sonst
  Audio direkt durchreichen wenn Engine das Format akzeptiert), `runCLI(bin, args, wavPath) (string,error)`.
- `stt_whispercpp.go` — `whisperCppTranscriber`: ruft das whisper.cpp-CLI mit Modellpfad + WAV; parsed stdout.
- `stt_parakeet.go` — `parakeetTranscriber`: ruft das sherpa-onnx-CLI (Parakeet-Modell) mit WAV; parsed stdout.
  **Letzter Task; hinter dem Interface isoliert.**

Windows-Gotchas: Binaries via direktem Pfad (kein `.cmd`-Shim hier, das sind echte `.exe`); Subprozess-Env
unkritisch. Downloads über HTTPS mit Größen-/SHA-Check.

### 2. Frontend — Mic-Button & Capture

`frontend/src/components/ChatInput.svelte`:
- 🎙️-Button **links neben** dem Senden-Button.
- Push-to-talk: `on:pointerdown` → `navigator.mediaDevices.getUserMedia({audio:true})` + `new MediaRecorder(stream)`,
  `start()`; Aufnahme-Indikator (roter Puls am Button). `on:pointerup`/`on:pointerleave` → `stop()`; im
  `ondataavailable`/`onstop` Blob sammeln → `FileReader` → base64 → `App.TranscribeAudio(b64, blob.type)`.
- Ergebnis: `text = (text ? text + ' ' : '') + transcript;` (anhängen, editierbar). Danach Textarea fokussieren
  + Höhe neu berechnen (vorhandenes `autoResize`).
- Zustände: `idle | recording | transcribing` (Button-Icon/Spinner). Fehler (kein Mic, Permission denied,
  Transcribe-Fehler) → kurze Inline-Meldung unter dem Input.
- Optionaler Capture-Helfer ausgelagert nach `frontend/src/lib/voice.ts` (getUserMedia/MediaRecorder-Kapselung,
  testbarer/klar abgegrenzt).

### 3. Lokale Engines — Binary & Modell

- Speicherort: `~/.multiterminal/stt/whisper-cpp/` bzw. `.../parakeet/` (Binary + Modell).
- `ensureEngine`: existiert Binary+Modell → ok; sonst Download von definierter URL (Größe/Hash geprüft),
  Fortschritt via Event `stt:download` `{engine, pct}`. Settings zeigt „Modell herunterladen"-Button + Fortschritt.
- Audio-Pipeline: MediaRecorder (webm/opus) → Backend → WAV 16 kHz mono. ffmpeg bevorzugt (PATH-Detection wie
  bei anderen CLI-Tools); falls nicht vorhanden, dokumentierter Hinweis + Versuch, das Engine-CLI direkt mit dem
  gelieferten Format zu füttern (whisper.cpp/sherpa-onnx akzeptieren WAV; ohne ffmpeg ist webm→wav nötig →
  ffmpeg als Voraussetzung für lokale Engines dokumentieren).

### 4. Config + Settings

`internal/config/config.go` — neuer Block (yaml+json-Tags) und Default:

```go
type STTSettings struct {
    Provider string          `yaml:"provider" json:"provider"` // cloud-whisper | whisper-cpp | parakeet
    Language string          `yaml:"language" json:"language"` // "de" | "auto" | ...
    Cloud    STTCloudSettings `yaml:"cloud" json:"cloud"`
}
type STTCloudSettings struct {
    BaseURL string `yaml:"base_url" json:"base_url"` // leer = OpenAI default
    Model   string `yaml:"model" json:"model"`       // default "whisper-1"
    APIKey  string `yaml:"api_key" json:"api_key"`   // leer = $OPENAI_API_KEY
}
```
Default: `Provider: "cloud-whisper"`, `Language: "de"`, `Cloud.Model: "whisper-1"`.

**Wails-Sync (`frontend/wailsjs/go/models.ts`) PFLICHT:** Klassen `STTSettings` + `STTCloudSettings` und das Feld
`stt` an `Config` (Deklaration + Konstruktor mit `convertValues`) — sonst Silent-Stripping (Recurring-Bug).

`frontend/src/components/SettingsDialog.svelte`: Sektion „Spracheingabe (STT)" — Provider-Dropdown; bei
`cloud-whisper` Felder base_url/model/api_key; bei lokalen Engines Sprache + „Modell herunterladen"-Button mit
Fortschritt. **`$:`-Reactivity-Falle** vermeiden — Init im bestehenden Init-Block-Muster (write-only Variablen).

## Scope

**v1 (sicher):** `Transcriber`-Interface + Factory; `TranscribeAudio`-Binding; Mic-Button + Capture (`voice.ts`);
**cloud-whisper** (voll funktionsfähig); **whisper.cpp** (on-demand Binary+Modell); Config+`models.ts`-Sync;
Settings-Sektion; Fehler-/Permission-Handling.

**v1 wenn glatt, sonst Fast-Follow (letzter, abtrennbarer Task):** **parakeet/sherpa-onnx** hinter demselben
`Transcriber`-Interface.

**Draußen (v2, YAGNI):** Echtzeit-Streaming-Transkription, Auto-Send, VAD/Silence-Trim, GPU-Beschleunigung.

## Risiken & Gotchas

- **Lokale Distribution** ist der Hauptaufwand: Binaries+Modelle on-demand laden; ffmpeg als Voraussetzung für
  webm→WAV-Konvertierung (sonst Format-Mismatch). Parakeet/sherpa-onnx ist am riskantesten → isoliert, zuletzt.
- **WebView2 Mic-Permission:** `getUserMedia` muss in WebView2 erlaubt sein; ggf. WebView2-Permission-Handler
  prüfen. Bei Verweigerung klare Inline-Meldung.
- **Wails `models.ts`-Sync** für den neuen `stt`-Config-Block (Recurring-Bug).
- **Svelte `$:`-Reset-Bug** im SettingsDialog.
- **300-Zeilen-Regel** pro Go-Datei: STT auf `app_stt.go`/`stt_cloud.go`/`stt_local.go`/`stt_whispercpp.go`/
  `stt_parakeet.go` aufteilen.
- **MediaRecorder-Format:** Browser liefert meist `audio/webm;codecs=opus`; Cloud-Whisper akzeptiert webm direkt,
  lokale Engines brauchen WAV → ffmpeg.
