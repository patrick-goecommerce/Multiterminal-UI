// Package backend provides global activity tracking across sessions.
package backend

import (
	"time"
)

// GetGlobalLastActivityUnix returns the Unix timestamp (seconds) of the most
// recent PTY output across all active sessions. Returns 0 if no sessions exist
// or if no output has been received yet.
func (a *AppService) GetGlobalLastActivityUnix() int64 {
	var latest time.Time
	for _, s := range a.sessionSummaries() {
		t := s.LastOutputAt
		if t.After(latest) {
			latest = t
		}
	}
	if latest.IsZero() {
		return 0
	}
	return latest.Unix()
}
