package backend

import (
	"strings"
	"testing"
)

// TestGitCmdPrependsSuppressionFlags verifies that gitCmd inserts
// --no-optional-locks and -c core.fsmonitor=false before the subcommand.
// Args[0] is the executable ("git"), so subcommand starts at Args[1].
func TestGitCmdPrependsSuppressionFlags(t *testing.T) {
	cmd := gitCmd("/x", "status")
	args := cmd.Args
	// Expected: git --no-optional-locks -c core.fsmonitor=false status
	if len(args) < 5 {
		t.Fatalf("expected at least 5 args, got %d: %v", len(args), args)
	}
	if args[1] != "--no-optional-locks" {
		t.Errorf("args[1] = %q, want --no-optional-locks", args[1])
	}
	if args[2] != "-c" {
		t.Errorf("args[2] = %q, want -c", args[2])
	}
	if args[3] != "core.fsmonitor=false" {
		t.Errorf("args[3] = %q, want core.fsmonitor=false", args[3])
	}
	if args[4] != "status" {
		t.Errorf("args[4] = %q, want status (the subcommand)", args[4])
	}
}

// TestGitCmdMultipleSubcmdArgs verifies that extra subcommand arguments are
// appended after the suppression flags in the correct order.
func TestGitCmdMultipleSubcmdArgs(t *testing.T) {
	cmd := gitCmd("/repo", "log", "-1", "--format=%H")
	args := cmd.Args
	// Expected: git --no-optional-locks -c core.fsmonitor=false log -1 --format=%H
	if len(args) < 7 {
		t.Fatalf("expected at least 7 args, got %d: %v", len(args), args)
	}
	if args[4] != "log" {
		t.Errorf("args[4] = %q, want log", args[4])
	}
	if args[5] != "-1" {
		t.Errorf("args[5] = %q, want -1", args[5])
	}
	if args[6] != "--format=%H" {
		t.Errorf("args[6] = %q, want --format=%%H", args[6])
	}
}

// TestGitCmdSetsDirAndEnv verifies that gitCmd sets Dir and populates Env with
// the required suppression variables.
func TestGitCmdSetsDirAndEnv(t *testing.T) {
	cmd := gitCmd("/x", "status")
	if cmd.Dir != "/x" {
		t.Errorf("Dir = %q, want /x", cmd.Dir)
	}
	if cmd.Env == nil {
		t.Fatal("Env must not be nil")
	}
	wantVars := []string{
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
	}
	for _, want := range wantVars {
		found := false
		for _, e := range cmd.Env {
			if e == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Env missing %q; Env = %v", want, cmd.Env)
		}
	}
}

// TestGitCmdEnvContainsOSEnviron verifies that gitCmd inherits the OS environment
// (not an empty env), so git can find its config and PATH.
func TestGitCmdEnvContainsOSEnviron(t *testing.T) {
	cmd := gitCmd("/x", "status")
	if len(cmd.Env) < 1 {
		t.Fatal("Env is empty; expected at least OS environ to be inherited")
	}
	// The PATH variable should be present (inherited from os.Environ()).
	pathFound := false
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "PATH=") || strings.HasPrefix(e, "Path=") {
			pathFound = true
			break
		}
	}
	if !pathFound {
		t.Error("PATH not found in Env — os.Environ() not inherited")
	}
}
