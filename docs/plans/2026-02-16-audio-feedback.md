# F-028: Terminal Bell / Audio-Feedback — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add optional audio feedback when Claude finishes or needs input, using Web Audio API with configurable custom sounds.

**Architecture:** Frontend-only approach. New `lib/audio.ts` module synthesizes tones via Web Audio API OscillatorNode. Hooks into existing `terminal:activity` Wails event in `App.svelte`. Config extended in Go backend and frontend store. Footer gets mute toggle.

**Tech Stack:** Web Audio API (OscillatorNode), Svelte stores, Go YAML config

---

### Task 1: Add AudioSettings to Go Config

**Files:**
- Modify: `internal/config/config.go:14-31` (Config struct)
- Modify: `internal/config/config.go:58-88` (DefaultConfig)
- Modify: `internal/config/config.go:124-169` (Load validation)

**Step 1: Add AudioSettings struct and Config field**

In `internal/config/config.go`, add after the `CommandEntry` struct (line 52):

```go
// AudioSettings holds audio feedback configuration.
type AudioSettings struct {
	Enabled     *bool  `yaml:"enabled" json:"enabled"`
	Volume      int    `yaml:"volume" json:"volume"`
	WhenFocused *bool  `yaml:"when_focused" json:"when_focused"`
	DoneSound   string `yaml:"done_sound" json:"done_sound"`
	InputSound  string `yaml:"input_sound" json:"input_sound"`
	ErrorSound  string `yaml:"error_sound" json:"error_sound"`
}
```

Add to `Config` struct after `Commands`:

```go
Audio AudioSettings `yaml:"audio" json:"audio"`
```

**Step 2: Set defaults in DefaultConfig**

Add to `DefaultConfig()` return, after `Commands`:

```go
Audio: AudioSettings{
	Enabled:     boolPtr(true),
	Volume:      50,
	WhenFocused: boolPtr(true),
},
```

**Step 3: Add validation in Load**

Add after the `CommitReminderMinutes` validation block (after line 163):

```go
if cfg.Audio.Volume < 0 {
	cfg.Audio.Volume = 0
}
if cfg.Audio.Volume > 100 {
	cfg.Audio.Volume = 100
}
if cfg.Audio.Enabled == nil {
	cfg.Audio.Enabled = boolPtr(true)
}
if cfg.Audio.WhenFocused == nil {
	cfg.Audio.WhenFocused = boolPtr(true)
}
```

**Step 4: Build to verify Go compiles**

Run: `cd /d/repos/Multiterminal && export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH" && go build ./...`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add AudioSettings for terminal bell (F-028)"
```

---

### Task 2: Add audio field to frontend AppConfig

**Files:**
- Modify: `frontend/src/stores/config.ts:13-27` (AppConfig interface)
- Modify: `frontend/src/stores/config.ts:29-47` (default value)

**Step 1: Add AudioConfig interface and field**

In `frontend/src/stores/config.ts`, add after `CommandEntry` interface (after line 11):

```typescript
export interface AudioConfig {
  enabled?: boolean;
  volume: number;
  when_focused?: boolean;
  done_sound: string;
  input_sound: string;
  error_sound: string;
}
```

Add to `AppConfig` interface after `commands`:

```typescript
audio: AudioConfig;
```

**Step 2: Add default value**

Add to the `config` writable default after `commands`:

```typescript
audio: {
  enabled: true,
  volume: 50,
  when_focused: true,
  done_sound: '',
  input_sound: '',
  error_sound: '',
},
```

**Step 3: Commit**

```bash
git add frontend/src/stores/config.ts
git commit -m "feat(config): add AudioConfig to frontend store (F-028)"
```

---

### Task 3: Create audio.ts module

**Files:**
- Create: `frontend/src/lib/audio.ts`

**Step 1: Create the audio module**

Create `frontend/src/lib/audio.ts` with the following content:

```typescript
import { writable } from 'svelte/store';

export const audioMuted = writable(false);

let ctx: AudioContext | null = null;

function getContext(): AudioContext | null {
  if (!ctx) {
    try {
      ctx = new AudioContext();
    } catch {
      return null;
    }
  }
  if (ctx.state === 'suspended') ctx.resume();
  return ctx;
}

function playTone(freq: number, duration: number, volume: number, startTime: number, ac: AudioContext) {
  const osc = ac.createOscillator();
  const gain = ac.createGain();
  osc.frequency.value = freq;
  osc.type = 'sine';
  gain.gain.value = volume;
  // Fade out to avoid click
  gain.gain.setValueAtTime(volume, startTime + duration - 0.02);
  gain.gain.linearRampToValueAtTime(0, startTime + duration);
  osc.connect(gain);
  gain.connect(ac.destination);
  osc.start(startTime);
  osc.stop(startTime + duration);
}

function playDone(volume: number) {
  const ac = getContext();
  if (!ac) return;
  const v = volume / 100 * 0.3;
  const now = ac.currentTime;
  playTone(523.25, 0.1, v, now, ac);       // C5
  playTone(659.25, 0.12, v, now + 0.12, ac); // E5
}

function playNeedsInput(volume: number) {
  const ac = getContext();
  if (!ac) return;
  const v = volume / 100 * 0.3;
  const now = ac.currentTime;
  playTone(440, 0.08, v, now, ac);           // A4
  playTone(440, 0.08, v, now + 0.15, ac);    // A4
  playTone(440, 0.08, v, now + 0.30, ac);    // A4
}

function playError(volume: number) {
  const ac = getContext();
  if (!ac) return;
  const v = volume / 100 * 0.3;
  const now = ac.currentTime;
  // Descending tone E4 -> C4
  const osc = ac.createOscillator();
  const gain = ac.createGain();
  osc.type = 'sine';
  osc.frequency.setValueAtTime(329.63, now);    // E4
  osc.frequency.linearRampToValueAtTime(261.63, now + 0.2); // C4
  gain.gain.value = v;
  gain.gain.setValueAtTime(v, now + 0.18);
  gain.gain.linearRampToValueAtTime(0, now + 0.2);
  osc.connect(gain);
  gain.connect(ac.destination);
  osc.start(now);
  osc.stop(now + 0.2);
}

export function playCustomSound(path: string, volume: number) {
  const audio = new Audio(path);
  audio.volume = Math.min(1, Math.max(0, volume / 100));
  audio.play().catch(() => {});
}

export function playBell(event: 'done' | 'needsInput' | 'error', volume: number, customPath?: string) {
  if (customPath) {
    playCustomSound(customPath, volume);
    return;
  }
  if (event === 'done') playDone(volume);
  else if (event === 'needsInput') playNeedsInput(volume);
  else if (event === 'error') playError(volume);
}
```

**Step 2: Verify TypeScript compiles**

Run: `cd /d/repos/Multiterminal/frontend && npx tsc --noEmit --skipLibCheck`
Expected: No errors (or only pre-existing ones)

**Step 3: Commit**

```bash
git add frontend/src/lib/audio.ts
git commit -m "feat(audio): add Web Audio API bell module (F-028)"
```

---

### Task 4: Integrate audio into App.svelte event listeners

**Files:**
- Modify: `frontend/src/App.svelte:1-22` (imports)
- Modify: `frontend/src/App.svelte:159-176` (event listeners)

**Step 1: Add imports**

Add after the `sendNotification` import (line 20):

```typescript
import { playBell, audioMuted } from './lib/audio';
```

**Step 2: Add audio to terminal:activity listener**

Replace the `EventsOn('terminal:activity', ...)` block (lines 159-171) with:

```typescript
EventsOn('terminal:activity', (info: any) => {
  tabStore.updateActivity(info.id, info.activity, info.cost);

  // Audio feedback for Claude panes
  if (!$audioMuted && $config.audio?.enabled !== false) {
    const shouldPlay = $config.audio?.when_focused !== false || !document.hasFocus();
    if (shouldPlay && (info.activity === 'done' || info.activity === 'needsInput')) {
      for (const tab of $allTabs) {
        const pane = tab.panes.find(p => p.sessionId === info.id);
        if (pane && (pane.mode === 'claude' || pane.mode === 'claude-yolo')) {
          const event = info.activity as 'done' | 'needsInput';
          const customPath = event === 'done' ? $config.audio?.done_sound : $config.audio?.input_sound;
          playBell(event, $config.audio?.volume ?? 50, customPath || undefined);
          break;
        }
      }
    }
  }

  // Notify when an issue-linked agent finishes
  if (info.activity === 'done') {
    for (const tab of $allTabs) {
      const pane = tab.panes.find(p => p.sessionId === info.id);
      if (pane?.issueNumber) {
        sendNotification(`Agent fertig – #${pane.issueNumber}`, pane.issueTitle || pane.name);
        break;
      }
    }
  }
});
```

**Step 3: Add error sound to terminal:error listener**

Replace the `EventsOn('terminal:error', ...)` block (lines 173-176) with:

```typescript
EventsOn('terminal:error', (id: number, msg: string) => {
  console.error('[terminal:error]', id, msg);
  if (!$audioMuted && $config.audio?.enabled !== false) {
    playBell('error', $config.audio?.volume ?? 50, $config.audio?.error_sound || undefined);
  }
  alert(`Terminal-Fehler (Session ${id}): ${msg}`);
});
```

**Step 4: Verify the app compiles**

Run: `cd /d/repos/Multiterminal/frontend && npx tsc --noEmit --skipLibCheck`
Expected: No errors

**Step 5: Commit**

```bash
git add frontend/src/App.svelte
git commit -m "feat(audio): integrate bell into activity/error events (F-028)"
```

---

### Task 5: Add mute toggle to Footer

**Files:**
- Modify: `frontend/src/components/Footer.svelte`
- Modify: `frontend/src/App.svelte:484` (Footer usage)

**Step 1: Add mute toggle to Footer.svelte**

Add to `<script>` section at the top of Footer.svelte, after the existing exports:

```typescript
import { audioMuted } from '../lib/audio';

function toggleMute() {
  audioMuted.update(m => !m);
}
```

Add a mute button in the template, between `footer-update` and `footer-right`:

```svelte
<button class="mute-btn" on:click={toggleMute} title={$audioMuted ? 'Audio aktivieren' : 'Audio stumm'}>
  {#if $audioMuted}
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" />
      <line x1="23" y1="9" x2="17" y2="15" />
      <line x1="17" y1="9" x2="23" y2="15" />
    </svg>
  {:else}
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" />
      <path d="M15.54 8.46a5 5 0 0 1 0 7.07" />
      <path d="M19.07 4.93a10 10 0 0 1 0 14.14" />
    </svg>
  {/if}
</button>
```

Add CSS for the mute button:

```css
.mute-btn {
  background: none;
  border: none;
  color: var(--fg-muted);
  cursor: pointer;
  padding: 2px 4px;
  display: flex;
  align-items: center;
  border-radius: 3px;
}

.mute-btn:hover {
  color: var(--fg);
  background: var(--hover-bg);
}
```

**Step 2: Verify it compiles**

Run: `cd /d/repos/Multiterminal/frontend && npx tsc --noEmit --skipLibCheck`
Expected: No errors

**Step 3: Commit**

```bash
git add frontend/src/components/Footer.svelte
git commit -m "feat(ui): add audio mute toggle to footer (F-028)"
```

---

### Task 6: Build and Manual Test

**Step 1: Full build**

Run: `cd /d/repos/Multiterminal && export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH" && wails build -debug`
Expected: Build succeeds, binary at `build/bin/multiterminal.exe`

**Step 2: Manual test checklist**

1. Launch app, open a Claude pane
2. Wait for Claude to finish → should hear ascending "ding-ding"
3. Trigger a needsInput state → should hear three short beeps
4. Click mute button in footer → icon should change to muted
5. Trigger another Claude event → should NOT hear sound
6. Click mute button again → unmuted
7. Edit `~/.multiterminal.yaml` and set `audio.enabled: false` → restart → no sounds
8. Set `audio.when_focused: false` → sounds only when window unfocused
9. Set `audio.volume: 10` → quieter sounds

**Step 3: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix(audio): address manual test findings (F-028)"
```
