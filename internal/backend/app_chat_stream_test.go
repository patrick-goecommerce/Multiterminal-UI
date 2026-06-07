package backend

import "testing"

// ---------------------------------------------------------------------------
// filterEnv — removes named env var
// ---------------------------------------------------------------------------

func TestFilterEnv_Removes(t *testing.T) {
	env := []string{"PATH=/usr/bin", "CLAUDECODE=xxx", "HOME=/home/user"}
	got := filterEnv(env, "CLAUDECODE")
	if len(got) != 2 {
		t.Fatalf("expected 2 vars, got %d", len(got))
	}
	for _, e := range got {
		if e == "CLAUDECODE=xxx" {
			t.Error("CLAUDECODE should be removed")
		}
	}
}

func TestFilterEnv_NoMatch(t *testing.T) {
	env := []string{"PATH=/usr/bin", "HOME=/home/user"}
	got := filterEnv(env, "NONEXISTENT")
	if len(got) != 2 {
		t.Errorf("expected 2 vars unchanged, got %d", len(got))
	}
}

func TestFilterEnv_Empty(t *testing.T) {
	got := filterEnv(nil, "FOO")
	if len(got) != 0 {
		t.Errorf("expected empty result, got %d", len(got))
	}
}
