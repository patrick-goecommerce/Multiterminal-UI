package backend

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/backend/hooks"
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
	scriptPath := filepath.Join(appDataDir, "Multiterminal", "hook_handler.ps1")

	// Deploy/update the hook script from embedded bytes
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		log.Printf("[hooks] could not create app dir: %v", err)
		return
	}
	if err := os.WriteFile(scriptPath, []byte(hooks.HookHandlerScript), 0644); err != nil {
		log.Printf("[hooks] could not write hook script: %v", err)
		return
	}

	// Register hooks in ~/.claude/settings.json (idempotent)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[hooks] could not get home dir: %v", err)
		return
	}
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	command := fmt.Sprintf(`powershell -NonInteractive -File "%s"`, scriptPath)
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
	a.hookMgr.Start(ctx)
}
