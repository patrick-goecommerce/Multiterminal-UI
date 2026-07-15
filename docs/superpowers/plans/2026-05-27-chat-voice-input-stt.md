# Chat Voice Input (STT) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Einen Push-to-talk-Mikrofon-Button neben dem Chat-Absenden-Button hinzufügen, der aufgenommenes Audio über eine in Settings wählbare STT-Engine (cloud-whisper / whisper.cpp / parakeet) transkribiert und das Ergebnis editierbar ins Eingabefeld schreibt.

**Architecture:** Frontend nimmt per `getUserMedia`+`MediaRecorder` auf (WebView2 kann das; Web Speech API kann es NICHT). Audio geht base64 an die Wails-Binding `TranscribeAudio`. Im Go-Backend wählt eine Factory anhand `config.stt.provider` einen `Transcriber` (Interface). Cloud-Whisper postet an einen OpenAI-kompatiblen Endpoint; lokale Engines (whisper.cpp, sherpa-onnx/Parakeet) laufen als Subprozess über on-demand heruntergeladene Binary+Modell. Parakeet ist der letzte, isolierte Task.

**Tech Stack:** Go 1.21+ (`net/http` multipart, `os/exec`), Wails v3 (Events, Bindings), TypeScript/Svelte 4, MediaRecorder API.

**Spec:** `docs/superpowers/specs/2026-05-25-chat-voice-input-stt-design.md`

**Worktree:** `D:\repos\Multiterminal\.worktrees\chat-pane-display-mode` (branch `feat/chat-pane-display-mode`).
**Go PATH:** `export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"`
**Builds:** Go `go build ./... && go test ./internal/...`; Frontend `cd frontend && npm run build` (no FE unit tests → build = TS compile gate; FE tasks tagged needs-e2e-testing).

---

## Phase 1 — Config + Provider-Gerüst

### Task 1: STT-Config-Block + Default + Wails-Sync

**Files:**
- Modify: `internal/config/config.go` (struct + DefaultConfig + Load validation)
- Modify: `frontend/wailsjs/go/models.ts` (Config class + 2 new nested classes)
- Test: `internal/config/config_stt_test.go`

- [ ] **Step 1: Failing test** — `internal/config/config_stt_test.go`:

```go
package config

import "testing"

func TestDefaultConfig_STTDefaults(t *testing.T) {
	c := DefaultConfig()
	if c.STT.Provider != "cloud-whisper" {
		t.Errorf("provider = %q, want cloud-whisper", c.STT.Provider)
	}
	if c.STT.Language != "de" {
		t.Errorf("language = %q, want de", c.STT.Language)
	}
	if c.STT.Cloud.Model != "whisper-1" {
		t.Errorf("cloud.model = %q, want whisper-1", c.STT.Cloud.Model)
	}
}

func TestLoad_InvalidSTTProviderResetsToCloud(t *testing.T) {
	c := Config{STT: STTSettings{Provider: "bogus"}}
	normalizeSTT(&c)
	if c.STT.Provider != "cloud-whisper" {
		t.Errorf("invalid provider should reset to cloud-whisper, got %q", c.STT.Provider)
	}
}
```

- [ ] **Step 2: Run, expect FAIL**

Run: `go test ./internal/config/ -run "STT" -v`
Expected: FAIL (undefined: STTSettings, normalizeSTT).

- [ ] **Step 3: Implement**

In `internal/config/config.go` add to the `Config` struct (near `ChatStyle`):
```go
	STT                   STTSettings    `yaml:"stt" json:"stt"`
```
Add the nested types (near other settings structs):
```go
// STTSettings configures speech-to-text voice input.
type STTSettings struct {
	Provider string           `yaml:"provider" json:"provider"` // cloud-whisper | whisper-cpp | parakeet
	Language string           `yaml:"language" json:"language"` // ISO code or "auto"
	Cloud    STTCloudSettings `yaml:"cloud" json:"cloud"`
}

// STTCloudSettings configures the cloud Whisper-compatible endpoint.
type STTCloudSettings struct {
	BaseURL string `yaml:"base_url" json:"base_url"` // empty = OpenAI default
	Model   string `yaml:"model" json:"model"`       // default whisper-1
	APIKey  string `yaml:"api_key" json:"api_key"`   // empty = $OPENAI_API_KEY
}

// normalizeSTT applies defaults/validation to STT settings.
func normalizeSTT(c *Config) {
	valid := map[string]bool{"cloud-whisper": true, "whisper-cpp": true, "parakeet": true}
	if !valid[c.STT.Provider] {
		c.STT.Provider = "cloud-whisper"
	}
	if c.STT.Language == "" {
		c.STT.Language = "de"
	}
	if c.STT.Cloud.Model == "" {
		c.STT.Cloud.Model = "whisper-1"
	}
}
```
In `DefaultConfig()` add the STT default (near `ChatStyle: "claude-code"`):
```go
		STT: STTSettings{Provider: "cloud-whisper", Language: "de", Cloud: STTCloudSettings{Model: "whisper-1"}},
```
In `Load(...)`, after the existing `chat_style`/other validation, call `normalizeSTT(&cfg)` (so loaded configs without an stt block get defaults).

- [ ] **Step 4: Run, expect PASS**

Run: `go test ./internal/config/ -run "STT" -v && go build ./...`
Expected: PASS + build ok.

- [ ] **Step 5: Wails models.ts sync (MANDATORY — recurring bug)**

In `frontend/wailsjs/go/models.ts`, inside `export namespace config { ... }` add two classes (mirror `AudioSettings` style):
```ts
	export class STTCloudSettings {
	    base_url: string;
	    model: string;
	    api_key: string;
	    static createFrom(source: any = {}) { return new STTCloudSettings(source); }
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.base_url = source["base_url"];
	        this.model = source["model"];
	        this.api_key = source["api_key"];
	    }
	}
	export class STTSettings {
	    provider: string;
	    language: string;
	    cloud: STTCloudSettings;
	    static createFrom(source: any = {}) { return new STTSettings(source); }
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.language = source["language"];
	        this.cloud = this.convertValues(source["cloud"], STTCloudSettings);
	    }
	    convertValues(a: any, classs: any, asMap: boolean = false): any {
	        if (!a) return a;
	        if (a.slice && a.map) return (a as any[]).map(elem => this.convertValues(elem, classs));
	        else if ("object" === typeof a) {
	            if (asMap) { for (const key of Object.keys(a)) a[key] = new classs(a[key]); return a; }
	            return new classs(a);
	        }
	        return a;
	    }
	}
```
In `class Config`: add field declaration near `chat_style: string;`:
```ts
    stt: STTSettings;
```
and in the Config constructor near `this.chat_style = source["chat_style"];`:
```ts
        this.stt = this.convertValues(source["stt"], STTSettings);
```

- [ ] **Step 6: Verify + commit**

Run: `cd "D:/repos/Multiterminal/.worktrees/chat-pane-display-mode/frontend" && npm run build` (success), and `cd .. && go test ./internal/config/...` (pass).
```bash
git add internal/config/config.go internal/config/config_stt_test.go frontend/wailsjs/go/models.ts
git commit -m "feat(voice): add STT config block with wails models sync"
```

---

### Task 2: Transcriber-Interface + Factory + TranscribeAudio-Binding

**Files:**
- Create: `internal/backend/stt.go` (interface, factory, binding)
- Test: `internal/backend/stt_test.go`

- [ ] **Step 1: Failing test** — `internal/backend/stt_test.go`:

```go
package backend

import (
	"context"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

func TestSelectTranscriber_ByProvider(t *testing.T) {
	a := &AppService{cfg: config.Config{STT: config.STTSettings{Provider: "cloud-whisper", Cloud: config.STTCloudSettings{Model: "whisper-1"}}}}
	tr, err := a.selectTranscriber()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if _, ok := tr.(*cloudWhisperTranscriber); !ok {
		t.Fatalf("got %T, want *cloudWhisperTranscriber", tr)
	}
}

func TestSelectTranscriber_Unknown(t *testing.T) {
	a := &AppService{cfg: config.Config{STT: config.STTSettings{Provider: "nope"}}}
	if _, err := a.selectTranscriber(); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

var _ Transcriber = (*cloudWhisperTranscriber)(nil)
```

- [ ] **Step 2: Run, expect FAIL**

Run: `go test ./internal/backend/ -run TestSelectTranscriber -v`
Expected: FAIL (undefined: Transcriber, cloudWhisperTranscriber, selectTranscriber).

- [ ] **Step 3: Implement** — `internal/backend/stt.go`:

```go
// Package backend — speech-to-text (voice input) transcription.
package backend

import (
	"context"
	"encoding/base64"
	"fmt"
)

// Transcriber turns recorded audio into text.
type Transcriber interface {
	// Transcribe converts raw recorded audio (mime e.g. "audio/webm") to text.
	// lang is an ISO code (e.g. "de") or "auto".
	Transcribe(ctx context.Context, audio []byte, mime, lang string) (string, error)
}

// selectTranscriber returns the configured Transcriber.
func (a *AppService) selectTranscriber() (Transcriber, error) {
	switch a.cfg.STT.Provider {
	case "cloud-whisper":
		return &cloudWhisperTranscriber{cfg: a.cfg.STT.Cloud}, nil
	case "whisper-cpp":
		return &whisperCppTranscriber{a: a}, nil
	case "parakeet":
		return &parakeetTranscriber{a: a}, nil
	default:
		return nil, fmt.Errorf("unknown STT provider: %q", a.cfg.STT.Provider)
	}
}

// TranscribeAudio is the Wails binding: base64 audio in, recognized text out.
func (a *AppService) TranscribeAudio(audioB64, mime string) (string, error) {
	audio, err := base64.StdEncoding.DecodeString(audioB64)
	if err != nil {
		return "", fmt.Errorf("decode audio: %w", err)
	}
	tr, err := a.selectTranscriber()
	if err != nil {
		return "", err
	}
	return tr.Transcribe(context.Background(), audio, mime, a.cfg.STT.Language)
}
```

> `cloudWhisperTranscriber` is implemented in Task 3, `whisperCppTranscriber` in Task 8, `parakeetTranscriber` in Task 9. To make THIS task compile + pass its test, add minimal stubs for the two not-yet-built ones at the bottom of `stt.go` (Task 8/9 will move them to their own files and flesh them out):
```go
// stubs (replaced in later tasks)
type whisperCppTranscriber struct{ a *AppService }
func (t *whisperCppTranscriber) Transcribe(ctx context.Context, audio []byte, mime, lang string) (string, error) {
	return "", fmt.Errorf("whisper.cpp not yet implemented")
}
type parakeetTranscriber struct{ a *AppService }
func (t *parakeetTranscriber) Transcribe(ctx context.Context, audio []byte, mime, lang string) (string, error) {
	return "", fmt.Errorf("parakeet not yet implemented")
}
```
(The `cloudWhisperTranscriber` type is created in Task 3; for Task 2 add a minimal stub for it too so the package compiles, then Task 3 replaces it.)
```go
type cloudWhisperTranscriber struct{ cfg config.STTCloudSettings }
func (t *cloudWhisperTranscriber) Transcribe(ctx context.Context, audio []byte, mime, lang string) (string, error) {
	return "", fmt.Errorf("cloud whisper not yet implemented")
}
```
Add the `config` import.

- [ ] **Step 4: Run, expect PASS + build**

Run: `go test ./internal/backend/ -run TestSelectTranscriber -v && go build ./...`
Expected: PASS + build ok.

- [ ] **Step 5: Commit**

```bash
git add internal/backend/stt.go internal/backend/stt_test.go
git commit -m "feat(voice): add Transcriber interface, factory, TranscribeAudio binding"
```

---

## Phase 2 — Cloud Whisper (voll funktionsfähig)

### Task 3: cloudWhisperTranscriber (OpenAI-kompatibel)

**Files:**
- Create: `internal/backend/stt_cloud.go` (move + implement cloudWhisperTranscriber; remove its stub from stt.go)
- Test: `internal/backend/stt_cloud_test.go`

- [ ] **Step 1: Failing test** (httptest server verifies request shape + parses response) — `internal/backend/stt_cloud_test.go`:

```go
package backend

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

func TestCloudWhisper_PostsMultipartAndParsesText(t *testing.T) {
	var gotAuth, gotModel string
	var gotFile bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = r.ParseMultipartForm(10 << 20)
		gotModel = r.FormValue("model")
		if f, _, err := r.FormFile("file"); err == nil {
			gotFile = true
			io.Copy(io.Discard, f)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"hallo welt"}`))
	}))
	defer srv.Close()

	tr := &cloudWhisperTranscriber{cfg: config.STTCloudSettings{BaseURL: srv.URL, Model: "whisper-1", APIKey: "sk-test"}}
	txt, err := tr.Transcribe(context.Background(), []byte("AUDIODATA"), "audio/webm", "de")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if txt != "hallo welt" {
		t.Errorf("text = %q, want 'hallo welt'", txt)
	}
	if !strings.Contains(gotAuth, "sk-test") {
		t.Errorf("auth header missing key: %q", gotAuth)
	}
	if gotModel != "whisper-1" {
		t.Errorf("model = %q, want whisper-1", gotModel)
	}
	if !gotFile {
		t.Error("no file part sent")
	}
}

func TestCloudWhisper_NoKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	tr := &cloudWhisperTranscriber{cfg: config.STTCloudSettings{Model: "whisper-1"}}
	if _, err := tr.Transcribe(context.Background(), []byte("x"), "audio/webm", "de"); err == nil {
		t.Fatal("expected error when no API key available")
	}
}
```

- [ ] **Step 2: Run, expect FAIL**

Run: `go test ./internal/backend/ -run TestCloudWhisper -v`
Expected: FAIL (stub returns not-implemented / wrong behavior).

- [ ] **Step 3: Implement** — remove the `cloudWhisperTranscriber` stub from `stt.go`, create `internal/backend/stt_cloud.go`:

```go
// Package backend — cloud Whisper-compatible transcription.
package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

type cloudWhisperTranscriber struct{ cfg config.STTCloudSettings }

func (t *cloudWhisperTranscriber) Transcribe(ctx context.Context, audio []byte, mime, lang string) (string, error) {
	key := t.cfg.APIKey
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}
	if key == "" {
		return "", fmt.Errorf("kein API-Key gesetzt (Settings → Spracheingabe, oder $OPENAI_API_KEY)")
	}
	base := strings.TrimRight(t.cfg.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	model := t.cfg.Model
	if model == "" {
		model = "whisper-1"
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "audio"+extForMime(mime))
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(audio); err != nil {
		return "", err
	}
	_ = mw.WriteField("model", model)
	if lang != "" && lang != "auto" {
		_ = mw.WriteField("language", lang)
	}
	_ = mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/audio/transcriptions", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("transcription request: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("transcription failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	return strings.TrimSpace(out.Text), nil
}

// extForMime maps a recording mime to a file extension the API understands.
func extForMime(mime string) string {
	switch {
	case strings.Contains(mime, "webm"):
		return ".webm"
	case strings.Contains(mime, "ogg"):
		return ".ogg"
	case strings.Contains(mime, "mp4"), strings.Contains(mime, "mp4a"), strings.Contains(mime, "m4a"):
		return ".m4a"
	case strings.Contains(mime, "wav"):
		return ".wav"
	default:
		return ".webm"
	}
}
```

- [ ] **Step 4: Run, expect PASS**

Run: `go test ./internal/backend/ -run "TestCloudWhisper|TestSelectTranscriber" -v && go build ./...`
Expected: PASS + build ok.

- [ ] **Step 5: Commit**

```bash
git add internal/backend/stt.go internal/backend/stt_cloud.go internal/backend/stt_cloud_test.go
git commit -m "feat(voice): cloud Whisper-compatible transcriber"
```

---

## Phase 3 — Frontend: Capture + Mic-Button

### Task 4: Voice-Capture-Helfer

**Files:**
- Create: `frontend/src/lib/voice.ts`

- [ ] **Step 1: Implement** — `frontend/src/lib/voice.ts`:

```ts
// Push-to-talk audio capture for chat voice input (WebView2-compatible).
// Web Speech API is NOT available in WebView2; we record audio and send it
// to the backend for transcription.

export interface VoiceRecorder {
  stop: () => Promise<{ base64: string; mime: string }>;
  cancel: () => void;
}

/** Starts microphone recording. Resolves once recording has actually begun. */
export async function startRecording(): Promise<VoiceRecorder> {
  const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
  const mime = pickMime();
  const rec = new MediaRecorder(stream, mime ? { mimeType: mime } : undefined);
  const chunks: Blob[] = [];
  rec.ondataavailable = (e) => { if (e.data && e.data.size > 0) chunks.push(e.data); };
  rec.start();

  const cleanup = () => stream.getTracks().forEach((t) => t.stop());

  return {
    stop: () =>
      new Promise((resolve, reject) => {
        rec.onstop = async () => {
          cleanup();
          try {
            const blob = new Blob(chunks, { type: rec.mimeType || 'audio/webm' });
            const base64 = await blobToBase64(blob);
            resolve({ base64, mime: blob.type });
          } catch (e) { reject(e); }
        };
        rec.stop();
      }),
    cancel: () => { try { rec.stop(); } catch {} cleanup(); },
  };
}

function pickMime(): string {
  const candidates = ['audio/webm;codecs=opus', 'audio/webm', 'audio/ogg;codecs=opus', 'audio/mp4'];
  for (const c of candidates) {
    if (typeof MediaRecorder !== 'undefined' && MediaRecorder.isTypeSupported(c)) return c;
  }
  return '';
}

function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onloadend = () => {
      const s = String(r.result || '');
      const comma = s.indexOf(',');
      resolve(comma >= 0 ? s.slice(comma + 1) : s); // strip data: prefix
    };
    r.onerror = () => reject(r.error);
    r.readAsDataURL(blob);
  });
}
```

- [ ] **Step 2: Build**

Run: `cd "D:/repos/Multiterminal/.worktrees/chat-pane-display-mode/frontend" && npm run build`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/voice.ts
git commit -m "feat(voice): push-to-talk audio capture helper"
```

---

### Task 5: Mic-Button in ChatInput (push-to-talk)

**Files:**
- Modify: `frontend/src/components/ChatInput.svelte`

- [ ] **Step 1: Implement** — add to `ChatInput.svelte` `<script>`:

```ts
  import { startRecording, type VoiceRecorder } from '../lib/voice';
  import * as App from '../../wailsjs/go/backend/App';

  let voiceState: 'idle' | 'recording' | 'transcribing' = 'idle';
  let recorder: VoiceRecorder | null = null;
  let voiceError = '';

  async function startVoice() {
    if (voiceState !== 'idle' || disabled) return;
    voiceError = '';
    try {
      recorder = await startRecording();
      voiceState = 'recording';
    } catch (e) {
      voiceError = 'Mikrofon nicht verfügbar oder verweigert.';
      voiceState = 'idle';
      recorder = null;
    }
  }

  async function stopVoice() {
    if (voiceState !== 'recording' || !recorder) return;
    voiceState = 'transcribing';
    try {
      const { base64, mime } = await recorder.stop();
      const transcript = (await App.TranscribeAudio(base64, mime)).trim();
      if (transcript) {
        text = text ? text + ' ' + transcript : transcript;
        // resize after value change
        setTimeout(autoResize, 0);
      }
    } catch (e) {
      voiceError = 'Transkription fehlgeschlagen.';
    } finally {
      voiceState = 'idle';
      recorder = null;
    }
  }
```

Add the button to the markup, **left of** the send button (inside `.chat-input`, before `<button class="send-btn" ...>`):
```svelte
  <button
    class="mic-btn"
    class:recording={voiceState === 'recording'}
    disabled={disabled || voiceState === 'transcribing'}
    on:pointerdown|preventDefault={startVoice}
    on:pointerup|preventDefault={stopVoice}
    on:pointerleave={() => voiceState === 'recording' && stopVoice()}
    title="Gedrückt halten zum Diktieren"
  >
    {#if voiceState === 'transcribing'}…{:else}&#127908;{/if}
  </button>
```
Add an error line under the input row (after the `.chat-input` div, or inside it as a sibling — keep it minimal):
```svelte
{#if voiceError}
  <div class="voice-error">{voiceError}</div>
{/if}
```
Add CSS in the `<style>`:
```css
  .mic-btn {
    width: 36px; height: 36px; border-radius: 8px; flex-shrink: 0;
    background: var(--bg-tertiary, #313244); border: 1px solid var(--border, #45475a);
    color: var(--fg, #cdd6f4); cursor: pointer; font-size: 1rem;
    display: flex; align-items: center; justify-content: center; transition: background .15s;
    user-select: none; touch-action: none;
  }
  .mic-btn:hover { background: var(--surface, #1e1e2e); }
  .mic-btn.recording { background: #e53935; color: #fff; animation: micpulse 1s infinite; }
  .mic-btn:disabled { opacity: .4; cursor: not-allowed; }
  @keyframes micpulse { 50% { opacity: .6; } }
  .voice-error { padding: 2px 16px 6px; font-size: .7rem; color: #e53935; }
```

> Note: `autoResize`, `text`, `disabled` already exist in ChatInput. Place the mic button so it sits left of `.send-btn`. Keep existing send/keydown behavior unchanged.

- [ ] **Step 2: Build**

Run: `cd "D:/repos/Multiterminal/.worktrees/chat-pane-display-mode/frontend" && npm run build`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/ChatInput.svelte
git commit -m "feat(voice): push-to-talk mic button in chat input"
```

---

## Phase 4 — Settings

### Task 6: STT-Sektion im SettingsDialog

**Files:**
- Modify: `frontend/src/components/SettingsDialog.svelte`

- [ ] **Step 1: Implement** — Read the file's init pattern first. Add locals initialized from `$config.stt` in the SAME init block other fields use (write-only assignments — safe):
```ts
  let sttProvider = ($config as any).stt?.provider || 'cloud-whisper';
  let sttLanguage = ($config as any).stt?.language || 'de';
  let sttBaseUrl = ($config as any).stt?.cloud?.base_url || '';
  let sttModel = ($config as any).stt?.cloud?.model || 'whisper-1';
  let sttApiKey = ($config as any).stt?.cloud?.api_key || '';
```
Re-init the same five inside the existing `$: if (visible) { ... }` block (mirroring the other fields).
In the `save()` assembled config object, add:
```ts
      stt: {
        provider: sttProvider,
        language: sttLanguage,
        cloud: { base_url: sttBaseUrl, model: sttModel, api_key: sttApiKey },
      },
```
In `resetDefault()` add: `sttProvider='cloud-whisper'; sttLanguage='de'; sttBaseUrl=''; sttModel='whisper-1'; sttApiKey='';`
Add a settings section (near the Chat-Stil group) with German labels:
```svelte
      <div class="setting-group">
        <label class="setting-label" for="stt-provider">Spracheingabe (STT)</label>
        <p class="setting-desc">Engine für das Mikrofon-Diktat im Chat.</p>
        <select id="stt-provider" class="theme-select" bind:value={sttProvider}>
          <option value="cloud-whisper">Cloud (Whisper-API)</option>
          <option value="whisper-cpp">Lokal: whisper.cpp</option>
          <option value="parakeet">Lokal: Parakeet (sherpa-onnx)</option>
        </select>
        <label class="setting-label" for="stt-lang">Sprache</label>
        <select id="stt-lang" class="theme-select" bind:value={sttLanguage}>
          <option value="de">Deutsch</option>
          <option value="en">Englisch</option>
          <option value="auto">Automatisch</option>
        </select>
        {#if sttProvider === 'cloud-whisper'}
          <label class="setting-label" for="stt-key">API-Key (leer = $OPENAI_API_KEY)</label>
          <input id="stt-key" type="password" bind:value={sttApiKey} placeholder="sk-…" />
          <label class="setting-label" for="stt-url">Base-URL (leer = OpenAI)</label>
          <input id="stt-url" type="text" bind:value={sttBaseUrl} placeholder="https://api.openai.com/v1" />
          <label class="setting-label" for="stt-model">Modell</label>
          <input id="stt-model" type="text" bind:value={sttModel} placeholder="whisper-1" />
        {:else}
          <p class="setting-desc">Lokale Engine lädt Binary + Modell beim ersten Gebrauch nach <code>~/.multiterminal/stt/</code>. Benötigt <code>ffmpeg</code> im PATH.</p>
        {/if}
      </div>
```
**Reactivity trap:** Do NOT add any `$:` block that reads these vars; only the init block writes them (consistent with existing fields).

- [ ] **Step 2: Build**

Run: `cd "D:/repos/Multiterminal/.worktrees/chat-pane-display-mode/frontend" && npm run build`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/SettingsDialog.svelte
git commit -m "feat(voice): STT settings section (provider, language, cloud key/url/model)"
```

---

## Phase 5 — Lokale Engine: whisper.cpp

### Task 7: Lokale Engine-Helfer (download / wav / runCLI)

**Files:**
- Create: `internal/backend/stt_local.go`
- Test: `internal/backend/stt_local_test.go`

- [ ] **Step 1: Failing test** (pure helpers: dir resolution + ffmpeg detection are testable; download/exec are integration) — `internal/backend/stt_local_test.go`:

```go
package backend

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSttEngineDir_UnderHome(t *testing.T) {
	p := sttEngineDir("whisper-cpp")
	if !strings.HasSuffix(filepath.ToSlash(p), ".multiterminal/stt/whisper-cpp") {
		t.Errorf("dir = %q, want …/.multiterminal/stt/whisper-cpp", p)
	}
}

func TestWavArgsForLang(t *testing.T) {
	// whisperCppArgs builds CLI args; "auto" must omit -l
	got := strings.Join(whisperCppArgs("/m.bin", "/a.wav", "de"), " ")
	if !strings.Contains(got, "-l de") {
		t.Errorf("expected -l de, got %q", got)
	}
	got2 := strings.Join(whisperCppArgs("/m.bin", "/a.wav", "auto"), " ")
	if strings.Contains(got2, "-l ") {
		t.Errorf("auto should omit -l, got %q", got2)
	}
}
```

- [ ] **Step 2: Run, expect FAIL**

Run: `go test ./internal/backend/ -run "TestSttEngineDir|TestWavArgs" -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Implement** — `internal/backend/stt_local.go`:

```go
// Package backend — shared helpers for local STT engines (download, wav, exec).
package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// sttEngineDir returns the on-demand storage dir for an engine's binary+model.
func sttEngineDir(engine string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".multiterminal", "stt", engine)
}

// ffmpegPath returns the ffmpeg executable path, or "" if not found in PATH.
func ffmpegPath() string {
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		return ""
	}
	return p
}

// toWav16kMono converts recorded audio bytes to a temp 16kHz mono WAV file via
// ffmpeg, returning the wav path (caller deletes). Requires ffmpeg in PATH.
func toWav16kMono(ctx context.Context, audio []byte, mime string) (string, error) {
	ff := ffmpegPath()
	if ff == "" {
		return "", fmt.Errorf("ffmpeg nicht im PATH gefunden (für lokale STT-Engines benötigt)")
	}
	in, err := os.CreateTemp("", "mtui-stt-in*"+extForMime(mime))
	if err != nil {
		return "", err
	}
	defer os.Remove(in.Name())
	if _, err := in.Write(audio); err != nil {
		in.Close()
		return "", err
	}
	in.Close()
	out := in.Name() + ".wav"
	cmd := exec.CommandContext(ctx, ff, "-y", "-i", in.Name(), "-ar", "16000", "-ac", "1", "-f", "wav", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg: %v: %s", err, strings.TrimSpace(string(b)))
	}
	return out, nil
}

// whisperCppArgs builds the whisper.cpp CLI args (no executable).
func whisperCppArgs(modelPath, wavPath, lang string) []string {
	args := []string{"-m", modelPath, "-f", wavPath, "-nt", "-otxt", "-of", wavPath}
	if lang != "" && lang != "auto" {
		args = append(args, "-l", lang)
	}
	return args
}

// emitSttDownload reports model/binary download progress to the frontend.
func (a *AppService) emitSttDownload(engine string, pct int) {
	if a.app == nil {
		return
	}
	a.app.Event.Emit("stt:download", map[string]interface{}{"engine": engine, "pct": pct})
}
```

> NOTE on `-otxt -of <wavPath>`: whisper.cpp writes `<wavPath>.txt`. Task 8 reads that file. If the installed whisper.cpp build uses different flags, Task 8 adjusts. Keep `whisperCppArgs` as the single source of truth for flags.

- [ ] **Step 4: Run, expect PASS + build**

Run: `go test ./internal/backend/ -run "TestSttEngineDir|TestWavArgs" -v && go build ./...`
Expected: PASS + build ok.

- [ ] **Step 5: Commit**

```bash
git add internal/backend/stt_local.go internal/backend/stt_local_test.go
git commit -m "feat(voice): local STT helpers (engine dir, ffmpeg wav, whisper args)"
```

---

### Task 8: whisperCppTranscriber + on-demand Download

**Files:**
- Create: `internal/backend/stt_whispercpp.go` (remove the whisperCppTranscriber stub from stt.go)
- Test: extend `internal/backend/stt_local_test.go` (parse-output helper)

**Download sources (verify current URLs at implementation time — these are external and version-dependent):**
- whisper.cpp Windows binary: from `https://github.com/ggml-org/whisper.cpp/releases` (asset like `whisper-bin-x64.zip`, contains `whisper-cli.exe`/`main.exe`).
- GGML model: `https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin` (~148 MB) — multilingual incl. German.

- [ ] **Step 1: Failing test** (output parsing is the unit-testable part) — append to `internal/backend/stt_local_test.go`:

```go
func TestParseWhisperTxt(t *testing.T) {
	raw := "  hallo welt \n"
	if got := parseTranscriptText(raw); got != "hallo welt" {
		t.Errorf("got %q, want 'hallo welt'", got)
	}
}
```

- [ ] **Step 2: Run, expect FAIL**

Run: `go test ./internal/backend/ -run TestParseWhisperTxt -v`
Expected: FAIL (undefined: parseTranscriptText).

- [ ] **Step 3: Implement** — remove the `whisperCppTranscriber` stub from `stt.go`; create `internal/backend/stt_whispercpp.go`:

```go
// Package backend — local whisper.cpp transcriber.
package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type whisperCppTranscriber struct{ a *AppService }

// whisperCppBinURL / whisperCppModelURL are the on-demand download sources.
// VERIFY current release URLs at implementation time.
const (
	whisperCppModelURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin"
)

func (t *whisperCppTranscriber) Transcribe(ctx context.Context, audio []byte, mime, lang string) (string, error) {
	dir := sttEngineDir("whisper-cpp")
	bin := filepath.Join(dir, binName("whisper-cli"))
	model := filepath.Join(dir, "ggml-base.bin")
	if err := t.a.ensureWhisperCpp(ctx, dir, bin, model); err != nil {
		return "", err
	}
	wav, err := toWav16kMono(ctx, audio, mime)
	if err != nil {
		return "", err
	}
	defer os.Remove(wav)
	defer os.Remove(wav + ".txt")

	cmd := commandCtx(ctx, bin, whisperCppArgs(model, wav, lang)...)
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("whisper.cpp: %v: %s", err, strings.TrimSpace(string(b)))
	}
	out, err := os.ReadFile(wav + ".txt")
	if err != nil {
		return "", fmt.Errorf("read transcript: %w", err)
	}
	return parseTranscriptText(string(out)), nil
}

// ensureWhisperCpp downloads the binary+model on first use.
func (a *AppService) ensureWhisperCpp(ctx context.Context, dir, bin, model string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if !fileExists(model) {
		a.emitSttDownload("whisper-cpp", 0)
		if err := downloadFile(ctx, whisperCppModelURL, model, func(p int) { a.emitSttDownload("whisper-cpp", p) }); err != nil {
			return fmt.Errorf("modell-download: %w", err)
		}
	}
	if !fileExists(bin) {
		return fmt.Errorf("whisper.cpp-Binary fehlt unter %s — bitte whisper-cli(.exe) dort ablegen (Auto-Download des Binaries folgt)", bin)
	}
	return nil
}

func parseTranscriptText(s string) string { return strings.TrimSpace(s) }
```

Add small shared helpers to `internal/backend/stt_local.go` (used by both local engines):
```go
import "io" // add to existing imports
import "net/http"
import "runtime"
import "time"

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func binName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func commandCtx(ctx context.Context, bin string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, bin, args...)
}

// downloadFile streams url to dst, reporting integer percent via progress.
func downloadFile(ctx context.Context, url, dst string, progress func(pct int)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	total := resp.ContentLength
	var written int64
	buf := make([]byte, 256*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				return werr
			}
			written += int64(n)
			if total > 0 && progress != nil {
				progress(int(written * 100 / total))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			return rerr
		}
	}
	f.Close()
	return os.Rename(tmp, dst)
}
```
(Consolidate imports in `stt_local.go`; remove duplicates.)

- [ ] **Step 4: Run, expect PASS + build**

Run: `go test ./internal/backend/ -run "TestParseWhisperTxt|TestSttEngineDir|TestWavArgs|TestSelectTranscriber" -v && go build ./...`
Expected: PASS + build ok.

- [ ] **Step 5: Commit**

```bash
git add internal/backend/stt.go internal/backend/stt_whispercpp.go internal/backend/stt_local.go internal/backend/stt_local_test.go
git commit -m "feat(voice): local whisper.cpp transcriber with on-demand model download"
```

---

## Phase 6 — Lokale Engine: Parakeet (letzter, isolierter Task)

### Task 9: parakeetTranscriber (sherpa-onnx)

**Files:**
- Create: `internal/backend/stt_parakeet.go` (remove parakeet stub from stt.go)
- Test: extend `internal/backend/stt_local_test.go`

**Risk note:** This is the highest-risk task (sherpa-onnx runtime + ONNX model distribution). It is isolated behind the `Transcriber` interface — if its download/runtime integration cannot be completed cleanly, leave the `parakeet` provider returning a clear "noch nicht verfügbar" error and ship cloud-whisper + whisper.cpp. Do NOT let Parakeet block the rest.

**Download sources (verify at implementation time):**
- sherpa-onnx prebuilt binaries: `https://github.com/k2-fsa/sherpa-onnx/releases` (CLI like `sherpa-onnx-offline`).
- Parakeet v3 ONNX model: from the sherpa-onnx model zoo / HuggingFace (`csukuangfj` / k2-fsa pre-converted Parakeet TDT v3 int8). 

- [ ] **Step 1: Failing test** — append to `internal/backend/stt_local_test.go`:

```go
func TestParakeetArgs_OmitsLangWhenAuto(t *testing.T) {
	a := strings.Join(parakeetArgs("/model", "/a.wav", "auto"), " ")
	if strings.Contains(a, "--language") {
		t.Errorf("auto should omit --language, got %q", a)
	}
}
```

- [ ] **Step 2: Run, expect FAIL**

Run: `go test ./internal/backend/ -run TestParakeetArgs -v`
Expected: FAIL (undefined: parakeetArgs).

- [ ] **Step 3: Implement** — remove parakeet stub from `stt.go`; create `internal/backend/stt_parakeet.go`:

```go
// Package backend — local Parakeet (sherpa-onnx) transcriber.
package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type parakeetTranscriber struct{ a *AppService }

func (t *parakeetTranscriber) Transcribe(ctx context.Context, audio []byte, mime, lang string) (string, error) {
	dir := sttEngineDir("parakeet")
	bin := filepath.Join(dir, binName("sherpa-onnx-offline"))
	model := filepath.Join(dir, "model.onnx")
	if !fileExists(bin) || !fileExists(model) {
		return "", fmt.Errorf("Parakeet-Engine nicht installiert unter %s (sherpa-onnx + Parakeet-Modell). Provider in den Einstellungen wechseln oder Engine bereitstellen.", dir)
	}
	wav, err := toWav16kMono(ctx, audio, mime)
	if err != nil {
		return "", err
	}
	defer os.Remove(wav)
	cmd := commandCtx(ctx, bin, parakeetArgs(model, wav, lang)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sherpa-onnx: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return parseSherpaOutput(string(out)), nil
}

// parakeetArgs builds sherpa-onnx-offline CLI args. VERIFY exact flag names
// against the installed sherpa-onnx version at implementation time.
func parakeetArgs(modelPath, wavPath, lang string) []string {
	args := []string{"--parakeet=" + modelPath, wavPath}
	if lang != "" && lang != "auto" {
		args = append(args, "--language", lang)
	}
	return args
}

// parseSherpaOutput extracts the recognized text line from sherpa-onnx stdout.
// VERIFY actual output format at implementation time and adjust.
func parseSherpaOutput(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
```

> The sherpa-onnx CLI flags/output format and the Parakeet model packaging MUST be verified against the actual installed version; `parakeetArgs`/`parseSherpaOutput` are the single places to adjust. On-demand download of sherpa-onnx + model can reuse `downloadFile`; if the runtime needs extra shared libs (onnxruntime DLLs), document the requirement and place them in the engine dir. If clean integration isn't achievable, keep the not-installed error path (do not block the feature).

- [ ] **Step 4: Run, expect PASS + build**

Run: `go test ./internal/backend/ -run "TestParakeetArgs|TestSelectTranscriber" -v && go build ./...`
Expected: PASS + build ok.

- [ ] **Step 5: Commit**

```bash
git add internal/backend/stt.go internal/backend/stt_parakeet.go internal/backend/stt_local_test.go
git commit -m "feat(voice): local Parakeet (sherpa-onnx) transcriber"
```

---

## Phase 7 — Verifikation

### Task 10: Voll-Build + E2E-Checkliste

- [ ] **Step 1: All Go tests + vet**

Run: `cd "D:/repos/Multiterminal/.worktrees/chat-pane-display-mode" && export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH" && go test ./internal/backend/... ./internal/config/... && go vet ./...`
Expected: pass + clean.

- [ ] **Step 2: Full build**

Run: `cd frontend && npm run build && cd .. && go build -o build/bin/multiterminal.exe -tags desktop .`
Expected: success.

- [ ] **Step 3: Manual E2E checklist** (real mic, real claude):
  1. Settings → Spracheingabe → Cloud (Whisper), Key gesetzt (oder `$OPENAI_API_KEY`).
  2. Chat-Pane → Mic-Button gedrückt halten, deutsch sprechen, loslassen → Text erscheint editierbar im Eingabefeld (kein Auto-Send).
  3. Senden → Antwort streamt normal.
  4. Mic ohne Key/ohne Mikrofon → klare Inline-Fehlermeldung.
  5. Settings → whisper.cpp → erster Gebrauch lädt Modell (Fortschritt), dann Transkription (ffmpeg im PATH).
  6. (falls implementiert) Parakeet analog; sonst klare "nicht installiert"-Meldung.

- [ ] **Step 4: Tag** `needs-e2e-testing` bis Schritt 3-6 real bestätigt.

---

## Self-Review

**Spec-Abdeckung:** Provider-Interface+Factory (Task 2) ✓; cloud-whisper inkl. Key-Env-Fallback (Task 3) ✓; Mic-Button push-to-talk + append editierbar (Task 4/5) ✓; whisper.cpp on-demand (Task 7/8) ✓; Parakeet isoliert/letzter (Task 9) ✓; Config + models.ts-Sync (Task 1) ✓; Settings-Sektion (Task 6) ✓; ffmpeg-WAV (Task 7) ✓; Download-Fortschritt-Event (Task 7/8) ✓; Fehler-/Permission-Handling (Task 5) ✓.

**Placeholder-Scan:** Externe Download-URLs (whisper.cpp-Binary, sherpa-onnx, Parakeet-Modell) sind als „beim Implementieren aktuelle Release-URL verifizieren" markiert — sie sind versions-/host-abhängig und können nicht fix gepinnt werden; das ist bewusst, kein Plan-Versäumnis. Binary-Auto-Download für whisper.cpp/sherpa-onnx ist als Folgeschritt im jeweiligen `ensure…` markiert; v1-Minimum ist Modell-Download + klare Meldung, falls Binary fehlt.

**Typ-Konsistenz:** `Transcriber.Transcribe(ctx, audio, mime, lang)` einheitlich über alle Impls; `cloudWhisperTranscriber{cfg}`, `whisperCppTranscriber{a}`, `parakeetTranscriber{a}` konsistent zwischen stt.go-Factory und ihren Dateien; `extForMime`/`toWav16kMono`/`whisperCppArgs`/`parseTranscriptText`/`downloadFile`/`fileExists`/`binName`/`commandCtx` an einer Stelle definiert, mehrfach genutzt.

**Offene Annahme (beim Ausführen verifizieren):** exakte CLI-Flags + Output-Format von whisper.cpp und sherpa-onnx; aktuelle Binary-/Modell-Download-URLs.
