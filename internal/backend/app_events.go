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
// CleanupFailed marks the special case where the merge already went through
// but the worktree removal failed — the frontend then offers "Cleanup erneut
// versuchen" (routes to FinishWorktree/resume), not "Erneut vorbereiten".
type WorktreeFinishBlockedEvent struct {
	SessionID     int    `json:"sessionId"`
	Phase         string `json:"phase"`
	Reason        string `json:"reason"`
	CleanupFailed bool   `json:"cleanupFailed"`
}

// WorktreeFinishDoneEvent: merge+cleanup finished; frontend relaunches the pane.
type WorktreeFinishDoneEvent struct {
	SessionID    int    `json:"sessionId"`
	MainRoot     string `json:"mainRoot"`
	TargetBranch string `json:"targetBranch"`
	Mode         string `json:"mode"`
}

// WorktreeDetectedEvent is emitted when a Claude session enters a native
// EnterWorktree-created worktree.
type WorktreeDetectedEvent struct {
	ID             int    `json:"id"`
	WorktreePath   string `json:"worktreePath"`
	WorktreeBranch string `json:"worktreeBranch"`
	TargetBranch   string `json:"targetBranch"`
}

// WorktreeClearedEvent is emitted when a session's cwd is no longer inside a
// previously detected worktree (Claude called ExitWorktree, or otherwise left).
type WorktreeClearedEvent struct {
	ID int `json:"id"`
}
