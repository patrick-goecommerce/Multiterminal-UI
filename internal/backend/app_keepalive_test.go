package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// fakeSession registers a session in the maps GetFirstClaudeSessionID reads,
// without spawning a real PTY process.
func fakeSession(t *testing.T, a *AppService, id int, mode string) {
	t.Helper()
	adopt(t, a, id, terminal.NewSession(id, 24, 80))
	a.mu.Lock()
	a.sessionMode[id] = mode
	a.mu.Unlock()
}

func TestGetFirstClaudeSessionID_NoneRunning(t *testing.T) {
	a := newTestAgentControlService()
	if got := a.GetFirstClaudeSessionID(); got != -1 {
		t.Fatalf("GetFirstClaudeSessionID() = %d, want -1", got)
	}
}

func TestGetFirstClaudeSessionID_IgnoresNonClaudeModes(t *testing.T) {
	a := newTestAgentControlService()
	fakeSession(t, a, 1, "shell")
	fakeSession(t, a, 2, "codex")
	fakeSession(t, a, 3, "gemini")
	if got := a.GetFirstClaudeSessionID(); got != -1 {
		t.Fatalf("GetFirstClaudeSessionID() = %d, want -1 (no claude-mode session)", got)
	}
}

func TestGetFirstClaudeSessionID_ReturnsOldestClaudeSession(t *testing.T) {
	a := newTestAgentControlService()
	fakeSession(t, a, 5, "shell")
	fakeSession(t, a, 7, "claude-yolo")
	fakeSession(t, a, 3, "claude") // lower ID, i.e. created earlier — should win
	fakeSession(t, a, 9, "claude-auto")

	if got := a.GetFirstClaudeSessionID(); got != 3 {
		t.Fatalf("GetFirstClaudeSessionID() = %d, want 3 (oldest claude-mode session)", got)
	}
}

func TestGetFirstClaudeSessionID_SkipsClosedSessions(t *testing.T) {
	a := newTestAgentControlService()
	// Mode recorded but session already removed from a.sessions (closed).
	a.mu.Lock()
	a.sessionMode[1] = "claude"
	a.mu.Unlock()
	fakeSession(t, a, 4, "claude")

	if got := a.GetFirstClaudeSessionID(); got != 4 {
		t.Fatalf("GetFirstClaudeSessionID() = %d, want 4 (id 1 has no live session)", got)
	}
}
