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
