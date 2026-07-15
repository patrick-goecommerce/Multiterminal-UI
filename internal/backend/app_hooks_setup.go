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
		func(sessionID int, activity string, cost string) {
			log.Printf("[hooks] session %d: %s", sessionID, activity)
			if a.app != nil {
				a.app.Event.Emit("terminal:activity", ActivityInfo{
					ID:       sessionID,
					Activity: activity,
					Cost:     cost,
				})
			}
			if activity == "done" {
				a.processQueue(sessionID)
			}
			a.onActivityChangeForIssue(sessionID, activity, cost)
		},
	)
	a.hookMgr.onPrompt = a.maybeGeneratePaneName
	a.hookMgr.onWorktreeChange = a.onWorktreeChange
	a.hookMgr.onPathBlocked = a.onWorktreePathBlocked
	a.hookMgr.Start(ctx)
}
