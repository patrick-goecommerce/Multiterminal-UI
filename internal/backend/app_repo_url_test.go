package backend

import "testing"

func TestNormalizeGitHubURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"git@github.com:patrick-goecommerce/Multiterminal-UI.git", "https://github.com/patrick-goecommerce/Multiterminal-UI"},
		{"https://github.com/patrick-goecommerce/Multiterminal-UI.git", "https://github.com/patrick-goecommerce/Multiterminal-UI"},
		{"https://github.com/patrick-goecommerce/Multiterminal-UI", "https://github.com/patrick-goecommerce/Multiterminal-UI"},
		{"ssh://git@github.com/patrick-goecommerce/Multiterminal-UI.git", "https://github.com/patrick-goecommerce/Multiterminal-UI"},
		{"https://gitlab.com/owner/repo.git", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeGitHubURL(tc.in); got != tc.want {
			t.Errorf("normalizeGitHubURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGetRepoURLEmptyDir(t *testing.T) {
	a := newTestAgentControlService()
	if got := a.GetRepoURL(""); got != "" {
		t.Fatalf("GetRepoURL(\"\") = %q, want \"\"", got)
	}
}
