package terminal

// SetHookActivity updates the activity state from a hook event and marks
// the session as having authoritative hook data.
func (s *Session) SetHookActivity(state ActivityState) {
	s.mu.Lock()
	s.Activity = state
	s.hasHookData = true
	s.mu.Unlock()
}

// HasHookData reports whether hook events have set the activity state.
// When true, the PTY scan loop skips DetectActivity() for this session.
func (s *Session) HasHookData() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasHookData
}

// ClearHookData resets the hook data flag (e.g. when a session ends).
func (s *Session) ClearHookData() {
	s.mu.Lock()
	s.hasHookData = false
	s.mu.Unlock()
}

// SetHookSessionID stores the Claude Code session UUID for this session.
func (s *Session) SetHookSessionID(id string) {
	s.mu.Lock()
	s.hookSessionID = id
	s.mu.Unlock()
}

// HookSessionID returns the Claude Code session UUID, empty if not yet set.
func (s *Session) HookSessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hookSessionID
}
