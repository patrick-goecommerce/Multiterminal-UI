package backend

// TerminalOutputEvent is emitted when PTY output is available.
type TerminalOutputEvent struct {
	ID   int    `json:"id"`
	Data string `json:"data"` // base64-encoded
}

// TerminalExitEvent is emitted when a PTY session exits.
type TerminalExitEvent struct {
	ID       int `json:"id"`
	ExitCode int `json:"exitCode"`
}

// TerminalErrorEvent is emitted when a session fails to start.
type TerminalErrorEvent struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
}

// WorktreeFinishReadyEvent: checks green, show confirm overlay.
type WorktreeFinishReadyEvent struct {
	SessionID    int      `json:"sessionId"`
	TargetBranch string   `json:"targetBranch"`
	Commits      []string `json:"commits"`
	Stat         string   `json:"stat"`
	Untracked    []string `json:"untracked"`
	CleanupOnly  bool     `json:"cleanupOnly"`
}

// WorktreeFinishBlockedEvent: a check failed or the flow was reset/informed.
type WorktreeFinishBlockedEvent struct {
	SessionID int    `json:"sessionId"`
	Phase     string `json:"phase"`
	Reason    string `json:"reason"`
}

// WorktreeFinishDoneEvent: merge+cleanup finished; frontend relaunches the pane.
type WorktreeFinishDoneEvent struct {
	SessionID    int    `json:"sessionId"`
	MainRoot     string `json:"mainRoot"`
	TargetBranch string `json:"targetBranch"`
	Mode         string `json:"mode"`
}
