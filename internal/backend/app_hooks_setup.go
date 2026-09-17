package backend

import (
	"context"
	"fmt"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"log"
	"os"
	"path/filepath"
)

// resolveHookBinary resolves the hook helper and, when it cannot be found,
// records a bind warning so the failure reaches the UI.
//
// Skipping hook integration disables activity detection, pane auto-naming and
// worktree detection, and leaves the hook directory to grow unswept (#192).
// Until now the only trace was a single log line, which is how a two-week
// outage went unnoticed on a real installation.
func (a *AppService) resolveHookBinary(name string, embedded []byte) string {
	exe := resolveBundledBinary(name, embedded)
	if exe == "" {
		a.recordBindWarning("hooks", fmt.Errorf(
			"%s nicht gefunden — Hook-Integration übersprungen: "+
				"Aktivitätserkennung, Pane-Namen und Worktree-Anzeige bleiben aus", name))
	}
	return exe
}

// setupHooks deploys the hook binary and registers it in
// ~/.claude/settings.json.
//
// It no longer reads the resulting files: that is the session host's job
// (internal/hub), because the events keep arriving while no window is open.
// What stays here is the registration, which only a running app can do.
func (a *AppService) setupHooks(ctx context.Context) {
	hooksDir := config.HooksDir()
	if hooksDir == "" {
		log.Println("[hooks] APPDATA not set — hook integration skipped")
		return
	}
	appDataDir := filepath.Dir(hooksDir)

	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		log.Printf("[hooks] could not create app dir: %v", err)
		return
	}

	// Best-effort: delete a stale PS1 hook handler left over from an older build.
	_ = os.Remove(filepath.Join(appDataDir, "Multiterminal", "hook_handler.ps1"))

	// Register the GUI-subsystem hook binary directly (no powershell → no console
	// window flash). There is no PowerShell fallback: if the binary cannot be
	// resolved, skip hook integration entirely.
	hookExe := a.resolveHookBinary("mtui-hook", hookBin)
	if hookExe == "" {
		return
	}

	// Register hooks in ~/.claude/settings.json (idempotent)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[hooks] could not get home dir: %v", err)
		return
	}
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	command := fmt.Sprintf(`"%s"`, hookExe)
	installer := newHookInstaller(settingsPath, command)
	if err := installer.Install(); err != nil {
		log.Printf("[hooks] could not install hooks: %v", err)
		// Non-fatal: hooks just won't fire, PTY scan fallback still works
	} else {
		log.Println("[hooks] hooks registered in ~/.claude/settings.json")
	}

	a.hooksDir = hooksDir
}

// onHookActivity is the HookManager's activity callback. It repaints the badge
// as soon as a hook event lands — roughly a debounce window before the scan
// loop confirms the same state — and that low latency is the only reason this
// second emit path exists.
//
// It deliberately triggers *no* side effects. Queue advance, orchestrator
// notification and issue reporting all hang off the one confirmed change in
// applyScanResults (see confirmActivity). Firing them here as well meant every
// hook-driven completion ran them twice about two seconds apart, and
// reportIssueProgress has no deduplication: with auto_comment_on_done that was
// two GitHub comments per completion, with auto_close_issue two close attempts
// (issue #188).
func (a *AppService) onHookActivity(sessionID int, activity string, cost string) {
	log.Printf("[hooks] session %d: %s", sessionID, activity)
	if a.app == nil {
		return
	}
	a.app.Event.Emit("terminal:activity", ActivityInfo{
		ID:       sessionID,
		Activity: activity,
		Cost:     cost,
		// This path runs a debounce window ahead of confirmActivity, so the
		// recorded timestamp still belongs to the previous state unless the
		// confirmed state already *is* this one. Sending it anyway would pair
		// a fresh label with a stale start time and make the duration jump
		// backwards a moment later.
		ActivitySince: a.activitySinceIfState(sessionID, activity),
	})
}

// activitySinceIfState returns the confirmed state's start as unix seconds,
// but only when that confirmed state is already the one being announced.
//
// The hook path runs a debounce window ahead of the scan's confirmation, so at
// that moment the recorded start still belongs to the PREVIOUS state. Pairing
// it with the fresh label would render e.g. "fertig · 3 Std 20" for a second
// or two and then snap to "fertig · gerade eben". Zero means "show the state
// without a duration", which never jumps backwards.
func (a *AppService) activitySinceIfState(sessionID int, activity string) int64 {
	state, since := a.host.ConfirmedActivity(sessionID)
	if string(state) != activity {
		return 0
	}
	return unixOrZero(since)
}
