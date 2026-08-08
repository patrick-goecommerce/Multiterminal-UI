package backend

var claudeKeepAliveModes = map[string]bool{"claude": true, "claude-auto": true, "claude-yolo": true}

// GetFirstClaudeSessionID returns the oldest still-running Claude session
// (mode claude/claude-auto/claude-yolo), or -1 if none exists. Session IDs
// are monotonically increasing, so the lowest ID among tracked sessions is
// the first one created. Unlike scanning a single window's local tab state,
// this looks across every session the backend knows about — including ones
// living in a detached secondary window — since AppService is shared by the
// whole process regardless of how many OS windows are open.
func (a *AppService) GetFirstClaudeSessionID() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	best := -1
	for id, mode := range a.sessionMode {
		if !claudeKeepAliveModes[mode] {
			continue
		}
		if a.sessions[id] == nil {
			continue
		}
		if best == -1 || id < best {
			best = id
		}
	}
	return best
}
