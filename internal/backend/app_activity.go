// Package backend provides global activity tracking across sessions.
package backend

import (
	"time"
)

// GetGlobalLastActivityUnix returns the Unix timestamp (seconds) of the most
// recent PTY output across all active sessions. Returns 0 if no sessions exist
// or if no output has been received yet.
func (a *AppService) GetGlobalLastActivityUnix() int64 {
	sessions := a.liveSessions()

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
