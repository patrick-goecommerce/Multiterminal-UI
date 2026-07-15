// Package backend provides global activity tracking across sessions.
package backend

import (
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// GetGlobalLastActivityUnix returns the Unix timestamp (seconds) of the most
// recent PTY output across all active sessions. Returns 0 if no sessions exist
// or if no output has been received yet.
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
