package backend

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// setupHooks deploys the hook script, registers hooks in ~/.claude/settings.json,
// and starts the HookManager polling loop.
func (a *AppService) setupHooks(ctx context.Context) {
	appDataDir := os.Getenv("APPDATA")
	if appDataDir == "" {
		log.Println("[hooks] APPDATA not set — hook integration skipped")
		return
	}

	hooksDir := filepath.Join(appDataDir, "Multiterminal", "hooks")

	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		log.Printf("[hooks] could not create app dir: %v", err)
		return
	}

	// Best-effort: delete a stale PS1 hook handler left over from an older build.
	_ = os.Remove(filepath.Join(appDataDir, "Multiterminal", "hook_handler.ps1"))

	// Register the GUI-subsystem hook binary directly (no powershell → no console
	// window flash). There is no PowerShell fallback: if the binary cannot be
	// resolved, skip hook integration entirely.
	hookExe := resolveBundledBinary("mtui-hook", hookBin)
	if hookExe == "" {
		log.Printf("[hooks] mtui-hook binary not found — hook integration skipped")
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

	// Start the HookManager
	a.hookMgr = newHookManager(hooksDir,
		func(mtID int) *terminal.Session {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.sessions[mtID]
		},
		a.onHookActivity,
	)
	a.hookMgr.onPrompt = a.maybeGeneratePaneName
	a.hookMgr.onWorktreeChange = a.onWorktreeChange
	a.hookMgr.onPathBlocked = a.onWorktreePathBlocked
	a.hookMgr.Start(ctx)
}

// onHookActivity is the HookManager's activity callback. It repaints the badge
// as soon as a hook event lands — roughly a debounce window before the scan
// loop confirms the same state — and that low latency is the only reason this
// second emit path exists.
//
// It deliberately triggers *no* side effects. Queue advance, orchestrator
// notification and issue reporting all hang off the one confirmed change in
// scanAllSessions (see confirmActivity). Firing them here as well meant every
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
		ActivitySince: activitySinceUnixIfState(sessionID, activity),
	})
}
