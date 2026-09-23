package backend

import (
	"log"
	"sync"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

// ServiceShutdown implements the Wails v3 Service interface.
func (a *AppService) ServiceShutdown() error {
	if a.cancelAll != nil {
		a.cancelAll()
	}
	// Releasing the host means different things by design: the embedded host
	// owns its sessions and ends them (killing each process tree first, which
	// the old shutdown loop did not, leaving descendants holding handles
	// inside worktrees, #185), while a daemon host is merely disconnected and
	// keeps every agent running for the next window.
	a.host.Release()

	// Chat panes run claude outside the host, so releasing it does not reach
	// them. Without this they outlived the app, MCP servers and all.
	a.closeAllChatSessions()

	// Withdraw the published loopback ports so no helper process dials a port
	// this instance no longer owns.
	a.releaseDiscoveryRecords()

	// Mark clean shutdown and auto-disable logging if stable
	config.MarkCleanShutdown(&a.health)
	if config.ShouldAutoDisableLogging(&a.health) {
		config.DisableAutoLogging(&a.health)
		a.cfg.LoggingEnabled = false
		_ = config.Save(a.cfg)
		log.Println("[Shutdown] Auto-logging disabled after 3 clean shutdowns")
	}
	_ = config.SaveHealth(a.health)
	log.Println("[Shutdown] Clean shutdown recorded")

	if a.safeMode {
		if a.sessionBackup != nil {
			if err := config.SaveSession(*a.sessionBackup); err != nil {
				log.Printf("[SafeMode] failed to restore session backup: %v", err)
			}
		} else {
			config.ClearSession()
		}
	}
	return nil
}

// closeAllChatSessions ends every chat's process tree and waits until they
// are gone, all of them at once so a slow one does not hold up the rest.
func (a *AppService) closeAllChatSessions() {
	a.mu.Lock()
	sessions := make([]*ChatSession, 0, len(a.chatSessions))
	for id, s := range a.chatSessions {
		sessions = append(sessions, s)
		delete(a.chatSessions, id)
	}
	a.mu.Unlock()

	var wg sync.WaitGroup
	for _, s := range sessions {
		wg.Add(1)
		go func(s *ChatSession) {
			defer wg.Done()
			s.close(true)
		}(s)
	}
	wg.Wait()
}
