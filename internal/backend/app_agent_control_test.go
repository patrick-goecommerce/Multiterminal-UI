package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

func newTestAgentControlService() *AppService {
	return NewAppService(nil, config.DefaultConfig(), true)
}

func TestBuildAgentArgv(t *testing.T) {
	a := newTestAgentControlService()

	cases := []struct {
		name  string
		tool  string
		model string
		want  []string
	}{
		{"claude no model", "claude", "", []string{"claude"}},
		{"claude with model", "claude", "opus", []string{"claude", "--model", "opus"}},
		{"codex no model", "codex", "", []string{"codex"}},
		{"gemini with model", "gemini", "gemini-2.5-pro", []string{"gemini", "--model", "gemini-2.5-pro"}},
		{"unsupported tool", "bogus", "", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := a.buildAgentArgv(tc.tool, tc.model)
			if len(got) != len(tc.want) {
				t.Fatalf("buildAgentArgv(%q, %q) = %v, want %v", tc.tool, tc.model, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("buildAgentArgv(%q, %q) = %v, want %v", tc.tool, tc.model, got, tc.want)
				}
			}
		})
	}
}

func TestSpawnAgentSessionRejectsUnsupportedTool(t *testing.T) {
	a := newTestAgentControlService()
	if _, err := a.SpawnAgentSession("notreal", "C:\\tmp", "", ""); err == nil {
		t.Fatal("expected error for unsupported tool, got nil")
	}
}

func TestSendAgentInputUnknownSession(t *testing.T) {
	a := newTestAgentControlService()
	if err := a.SendAgentInput(999, "hello"); err == nil {
		t.Fatal("expected error for unknown session, got nil")
	}
}

func TestCloseAgentSessionUnknownSession(t *testing.T) {
	a := newTestAgentControlService()
	if err := a.CloseAgentSession(999, "cleanup"); err == nil {
		t.Fatal("expected error for unknown session, got nil")
	}
}

func TestListAgentSessionsEmpty(t *testing.T) {
	a := newTestAgentControlService()
	got := a.ListAgentSessions()
	if len(got) != 0 {
		t.Fatalf("expected no agent sessions, got %v", got)
	}
}
