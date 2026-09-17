package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

// The agent-control tools themselves are internal/mcpsrv now, and tested
// there. What is still this package's is the launch policy the window hands
// its host: which command each tool name resolves to.

func newTestAgentControlService() *AppService {
	return NewAppService(nil, config.DefaultConfig(), true)
}

func TestLaunchPolicyArgv(t *testing.T) {
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.launchPolicy().Argv(tc.tool, tc.model)
			if err != nil {
				t.Fatalf("Argv(%q, %q): %v", tc.tool, tc.model, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("Argv(%q, %q) = %v, want %v", tc.tool, tc.model, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("Argv(%q, %q) = %v, want %v", tc.tool, tc.model, got, tc.want)
				}
			}
		})
	}
}

func TestLaunchPolicyRejectsAnUnknownTool(t *testing.T) {
	a := newTestAgentControlService()
	if _, err := a.launchPolicy().Argv("bogus", ""); err == nil {
		t.Fatal("expected an error for an unsupported tool, got nil")
	}
}
