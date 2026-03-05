# Auto-Session-Start & Keep-Alive Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Automatically open a Claude session on startup and periodically ping it to keep the 5-hour Claude Code token window alive.

**Architecture:** Frontend-driven: after `restoreSession()`, a keep-alive loop checks for Claude panes and creates one if absent, then fires a configurable ping message when no activity has occurred in any session for the configured interval. Backend exposes `GetGlobalLastActivityUnix()` to give the frontend a global last-activity timestamp.

**Tech Stack:** Go 1.21 (backend binding), TypeScript/Svelte 4 (frontend loop + settings UI), Wails v3 bindings (auto-regenerated via `wails dev`)

---

## Task 1: Add `KeepAliveSettings` to Config

**Files:**
- Modify: `internal/config/config.go`

**Step 1: Add the struct and field**

In `config.go`, after the `AudioSettings` struct, insert:

```go
// KeepAliveSettings controls the automatic Claude session keep-alive feature.
type KeepAliveSettings struct {
	Enabled         *bool  `yaml:"enabled" json:"enabled"`
	IntervalMinutes int    `yaml:"interval_minutes" json:"interval_minutes"`
	Message         string `yaml:"message" json:"message"`
}
```

Add the field to `Config` struct (after `Audio AudioSettings`):
```go
KeepAlive KeepAliveSettings `yaml:"keep_alive" json:"keep_alive"`
```

**Step 2: Add defaults in `DefaultConfig()`**

In `DefaultConfig()`, add (after the `Audio:` block):
```go
KeepAlive: KeepAliveSettings{
    Enabled:         boolPtr(true),
    IntervalMinutes: 300,
    Message:         "Hi!",
},
```

**Step 3: Add nil-guard in `Load()` after the audio nil-guards**

After the `if cfg.Audio.WhenFocused == nil` block:
```go
if cfg.KeepAlive.Enabled == nil {
    cfg.KeepAlive.Enabled = boolPtr(true)
}
if cfg.KeepAlive.IntervalMinutes <= 0 {
    cfg.KeepAlive.IntervalMinutes = 300
}
if cfg.KeepAlive.Message == "" {
    cfg.KeepAlive.Message = "Hi!"
}
```

**Step 4: Run config tests**

```bash
go test ./internal/config/... -v
```
Expected: all PASS

**Step 5: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add KeepAliveSettings struct with defaults"
```

---

## Task 2: Add `GetLastOutputAt()` method to Session

The backend package can't access `session.mu` directly (private field). A thread-safe accessor method is needed.

**Files:**
- Modify: `internal/terminal/session_helpers.go`

**Step 1: Add the method**

Open `internal/terminal/session_helpers.go` and add:

```go
// GetLastOutputAt returns when the last PTY output was received, under lock.
func (s *Session) GetLastOutputAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.LastOutputAt
}
```

**Step 2: Run terminal tests**

```bash
go test ./internal/terminal/... -v
```
Expected: all PASS

**Step 3: Commit**

```bash
git add internal/terminal/session_helpers.go
git commit -m "feat(terminal): add GetLastOutputAt() accessor with lock"
```

---

## Task 3: Add `GetGlobalLastActivityUnix()` backend binding

**Files:**
- Modify: `internal/backend/app.go`

**Step 1: Add the method** (append to `app.go`, before the last blank line)

```go
// GetGlobalLastActivityUnix returns the Unix timestamp (seconds) of the most
// recent PTY output across all active sessions. Returns 0 if no sessions exist.
func (a *AppService) GetGlobalLastActivityUnix() int64 {
	a.mu.Lock()
	sessions := make([]*terminal.Session, 0, len(a.sessions))
	for _, s := range a.sessions {
		sessions = append(sessions, s)
	}
	a.mu.Unlock()

	var latest time.Time
	for _, s := range sessions {
		t := s.GetLastOutputAt()
		if t.After(latest) {
			latest = t
		}
	}
	if latest.IsZero() {
		return 0
	}
	return latest.Unix()
}
```

Add `"time"` to the imports in `app.go` if not already present.

**Step 2: Verify it compiles**

```bash
go build ./internal/backend/...
```
Expected: no errors

**Step 3: Regenerate Wails bindings**

Run `wails dev` briefly (Ctrl+C after frontend starts) OR:

```bash
go run github.com/wailsapp/wails/v3/cmd/wails3@latest generate bindings -f internal/backend
```

This regenerates `frontend/wailsjs/go/backend/App.js` and `App.d.ts` with a new `GetGlobalLastActivityUnix` export.

**Step 4: Commit**

```bash
git add internal/backend/app.go frontend/wailsjs/go/backend/App.js frontend/wailsjs/go/backend/App.d.ts
git commit -m "feat(backend): add GetGlobalLastActivityUnix binding"
```

---

## Task 4: Create `frontend/src/lib/keepalive.ts`

**Files:**
- Create: `frontend/src/lib/keepalive.ts`

**Step 1: Write the module**

```typescript
import { tabStore } from '../stores/tabs';
import { buildClaudeArgv, encodeForPty } from './claude';
import * as App from '../../wailsjs/go/backend/App';
import { isMainWindow } from './window';

export interface KeepAliveConfig {
  enabled: boolean | null;
  interval_minutes: number;
  message: string;
}

/** Find the first running Claude or claude-yolo pane across all tabs. */
function findFirstClaudePane(): { sessionId: number; tabId: string } | null {
  const state = tabStore.getState();
  for (const tab of state.tabs) {
    for (const pane of tab.panes) {
      if ((pane.mode === 'claude' || pane.mode === 'claude-yolo') && pane.running) {
        return { sessionId: pane.sessionId, tabId: tab.id };
      }
    }
  }
  return null;
}

/**
 * Start the keep-alive loop after session restore.
 * Returns a cleanup function to stop the loop (call in onDestroy).
 *
 * Behaviour:
 * 1. If no Claude pane exists after restore → create one in the first tab.
 * 2. Every `interval_minutes` minutes: if no activity in any session for that
 *    interval, write the keep-alive message to the first Claude pane found.
 */
export async function startKeepAliveLoop(
  cfg: KeepAliveConfig,
  claudePath: string,
): Promise<() => void> {
  if (!isMainWindow()) return () => {};
  if (!cfg.enabled || cfg.interval_minutes <= 0) return () => {};

  // Auto-start: create a Claude pane if none exists
  if (!findFirstClaudePane()) {
    const state = tabStore.getState();
    if (state.tabs.length > 0) {
      const firstTab = state.tabs[0];
      const argv = buildClaudeArgv('claude', '', claudePath);
      try {
        const sessionId = await App.CreateSession(argv, firstTab.dir || '', 24, 80, 'claude');
        if (sessionId > 0) {
          tabStore.addPane(firstTab.id, sessionId, 'Claude', 'claude', '');
        }
      } catch (err) {
        console.error('[keepalive] auto-start failed:', err);
      }
    }
  }

  const intervalMs = cfg.interval_minutes * 60 * 1000;
  const intervalSec = cfg.interval_minutes * 60;

  const timer = setInterval(async () => {
    try {
      const lastActivity = await App.GetGlobalLastActivityUnix();
      const nowSec = Math.floor(Date.now() / 1000);

      if (lastActivity > 0 && nowSec - lastActivity < intervalSec) {
        return; // activity within window
      }

      const pane = findFirstClaudePane();
      if (!pane) return; // no Claude pane to ping

      await App.WriteToSession(pane.sessionId, encodeForPty(cfg.message + '\n'));
    } catch (err) {
      console.error('[keepalive] ping failed:', err);
    }
  }, intervalMs);

  return () => clearInterval(timer);
}
```

**Step 2: Commit**

```bash
git add frontend/src/lib/keepalive.ts
git commit -m "feat(frontend): add keep-alive loop module"
```

---

## Task 5: Wire keep-alive into `App.svelte`

**Files:**
- Modify: `frontend/src/App.svelte`

**Step 1: Import `startKeepAliveLoop`**

Add to the existing import block (after the `restoreSession` import):
```typescript
import { startKeepAliveLoop } from './lib/keepalive';
```

**Step 2: Add cleanup variable**

After the `let storeUnsubscribe` declaration, add:
```typescript
let keepAliveCleanup: (() => void) | null = null;
```

**Step 3: Call `startKeepAliveLoop` after `restoreSession`**

In `onMount`, the current code is:
```typescript
const restored = await restoreSession(resolvedClaudePath);
if (!restored) {
  let workDir = '';
  try { workDir = await App.GetWorkingDir(); } catch {}
  tabStore.addTab('Workspace', workDir);
}
```

Change to:
```typescript
const restored = await restoreSession(resolvedClaudePath);
if (!restored) {
  let workDir = '';
  try { workDir = await App.GetWorkingDir(); } catch {}
  tabStore.addTab('Workspace', workDir);
}

// Start keep-alive loop (auto-start + periodic ping)
const cfg = $config;
if (cfg.keep_alive) {
  keepAliveCleanup = await startKeepAliveLoop(cfg.keep_alive, resolvedClaudePath);
}
```

**Step 4: Clean up in `onDestroy`**

In `onDestroy`, add:
```typescript
if (keepAliveCleanup) keepAliveCleanup();
```

**Step 5: Verify the app compiles**

```bash
cd frontend && npm run check
```
Expected: no TypeScript errors

**Step 6: Commit**

```bash
git add frontend/src/App.svelte
git commit -m "feat(app): wire keep-alive loop into startup"
```

---

## Task 6: Add Keep-Alive section to SettingsDialog

**Files:**
- Modify: `frontend/src/components/SettingsDialog.svelte`

**Step 1: Add local state variables**

In the `<script>` block, after the audio variables, add:
```typescript
let keepAliveEnabled = $config.keep_alive?.enabled ?? true;
let keepAliveInterval = $config.keep_alive?.interval_minutes ?? 300;
let keepAliveMessage = $config.keep_alive?.message ?? 'Hi!';
```

**Step 2: Load from config when dialog opens**

In the `$: if (visible)` reactive block, after the audio assignments, add:
```typescript
keepAliveEnabled = c.keep_alive?.enabled ?? true;
keepAliveInterval = c.keep_alive?.interval_minutes ?? 300;
keepAliveMessage = c.keep_alive?.message ?? 'Hi!';
```

**Step 3: Include in `save()` function**

In the `save()` function, the `updated` object spread already captures `$config`. Add `keep_alive` explicitly:
```typescript
const updated = {
  ...$config,
  terminal_color: colorValue,
  theme: selectedTheme,
  logging_enabled: loggingEnabled,
  claude_command: claudeCommand,
  font_family: fontFamily,
  font_size: fontSize,
  audio: {
    enabled: audioEnabled,
    volume: audioVolume,
    when_focused: audioWhenFocused,
    done_sound: audioDoneSound,
    input_sound: audioInputSound,
    error_sound: audioErrorSound,
  },
  keep_alive: {
    enabled: keepAliveEnabled,
    interval_minutes: keepAliveInterval,
    message: keepAliveMessage,
  },
};
```

**Step 4: Add UI section**

Find the last `</div>` before the closing `</div>` of the dialog content (before the save/cancel buttons area). Add this new settings group right before the logging section or at the end of settings groups:

```html
<div class="setting-group">
  <label class="setting-label">Session Keep-Alive</label>
  <p class="setting-desc">Sendet automatisch eine Nachricht an Claude, wenn das Token-Fenster ausläuft.</p>
  <label class="toggle-row">
    <input type="checkbox" bind:checked={keepAliveEnabled} />
    Aktiviert
  </label>
  {#if keepAliveEnabled}
    <div class="keepalive-fields">
      <label for="keepalive-interval">Intervall (Minuten)</label>
      <input
        id="keepalive-interval"
        type="number"
        min="1"
        max="1440"
        bind:value={keepAliveInterval}
        class="text-input"
      />
      <label for="keepalive-message">Nachricht</label>
      <input
        id="keepalive-message"
        type="text"
        bind:value={keepAliveMessage}
        class="text-input"
        placeholder="Hi!"
      />
    </div>
  {/if}
</div>
```

**Step 5: Add minimal CSS** (in the `<style>` block)

```css
.toggle-row {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  margin-top: 6px;
}
.keepalive-fields {
  display: grid;
  grid-template-columns: 140px 1fr;
  gap: 6px 12px;
  align-items: center;
  margin-top: 8px;
}
```

**Step 6: Commit**

```bash
git add frontend/src/components/SettingsDialog.svelte
git commit -m "feat(settings): add Keep-Alive configuration section"
```

---

## Task 7: Manual smoke test

Run the app in dev mode and verify:

```bash
wails dev
```

1. **Auto-start**: Close mtui, delete `~/.multiterminal-session.json`, restart → a Claude pane should appear automatically in Tab 1
2. **Existing pane**: Restore a session with Claude panes → no extra pane is added
3. **Settings**: Open Settings → "Session Keep-Alive" section visible, values editable and saved
4. **Keep-alive ping** (short test): Set interval to `1` minute in settings, wait 1 minute without typing anything → check that the keep-alive message appears in the Claude pane
5. **Disable**: Toggle off in settings → no ping after restart

**Step 2: Commit final**

```bash
git add -A
git commit -m "chore: verify keep-alive smoke test passed"
```

---

## Task 8: Push and open PR

```bash
git push origin issue/96-auto-session-start-beim-starten-von-mtui
gh pr create \
  --title "feat: auto-session-start and keep-alive for Claude token window (#96)" \
  --body "$(cat <<'EOF'
## Summary
- Automatically opens a Claude session on startup if none exists after session restore
- Sends a configurable keep-alive ping every N minutes (default 5h) when no terminal activity has occurred
- Fully configurable via Settings: enable/disable toggle, interval (minutes), message text

## Config
```yaml
keep_alive:
  enabled: true
  interval_minutes: 300
  message: "Hi!"
```

## Test plan
- [ ] Fresh start (no session file): Claude pane auto-created in Tab 1
- [ ] Session restore with Claude panes: no extra pane added
- [ ] Settings section visible and saves correctly
- [ ] Keep-alive ping fires after configured interval with no activity
- [ ] Disabled toggle stops all keep-alive behaviour

Closes #96
EOF
)"
```
