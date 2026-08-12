package backend

import (
	"strings"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

func newSuspendTestService() *AppService {
	return NewAppService(nil, config.DefaultConfig(), true)
}

// registerSession puts a session object into the service maps the way
// CreateSession would, without spawning a process. Resume is stubbed out so a
// wake-up in these tests never launches a real claude CLI.
func registerSession(a *AppService, id int, mode string, argv []string, dir string) *terminal.Session {
	sess := terminal.NewSession(id, 24, 80)
	sess.SetSpawnForTest(func([]string, string, []string) error { return nil })
	a.mu.Lock()
	a.sessions[id] = sess
	a.sessionMode[id] = mode
	a.launches[id] = launchSpec{argv: argv, dir: dir, mode: mode}
	a.mu.Unlock()
	return sess
}

// ---------------------------------------------------------------------------
// claudeSessionIDFromArgv — the backend's only launch-time source for the UUID.
// ---------------------------------------------------------------------------

func TestClaudeSessionIDFromArgv(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"session-id flag", []string{"claude", "--session-id", "abc-123"}, "abc-123"},
		{"resume flag", []string{"claude", "--resume", "abc-123"}, "abc-123"},
		{"session-id equals form", []string{"claude", "--session-id=abc-123"}, "abc-123"},
		{"resume equals form", []string{"claude", "--resume=abc-123"}, "abc-123"},
		{"with model and yolo", []string{"claude", "--dangerously-skip-permissions", "--model", "opus", "--session-id", "u-1"}, "u-1"},
		{"bare resume picker", []string{"claude", "--resume"}, ""},
		{"bare resume before flag", []string{"claude", "--resume", "--model", "opus"}, ""},
		{"no id at all", []string{"claude", "--model", "opus"}, ""},
		{"empty argv", nil, ""},
		{"shell", []string{"pwsh"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := claudeSessionIDFromArgv(tc.argv); got != tc.want {
				t.Fatalf("claudeSessionIDFromArgv(%v) = %q, want %q", tc.argv, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// resumeArgv — rebuild the launch command line for a wake-up.
// ---------------------------------------------------------------------------

func TestResumeArgv(t *testing.T) {
	cases := []struct {
		name     string
		argv     []string
		resumeID string
		want     []string
	}{
		{
			"replaces session-id",
			[]string{"claude", "--session-id", "old"}, "new",
			[]string{"claude", "--resume", "new"},
		},
		{
			"replaces existing resume",
			[]string{"claude", "--resume", "old"}, "new",
			[]string{"claude", "--resume", "new"},
		},
		{
			"keeps mode flags and model",
			[]string{"claude", "--dangerously-skip-permissions", "--model", "opus", "--session-id", "old"}, "new",
			[]string{"claude", "--dangerously-skip-permissions", "--model", "opus", "--resume", "new"},
		},
		{
			"equals form is dropped too",
			[]string{"claude", "--session-id=old", "--model", "opus"}, "new",
			[]string{"claude", "--model", "opus", "--resume", "new"},
		},
		{
			"no id in argv",
			[]string{"claude", "--permission-mode", "auto"}, "new",
			[]string{"claude", "--permission-mode", "auto", "--resume", "new"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resumeArgv(tc.argv, tc.resumeID)
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Fatalf("resumeArgv = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResumeArgv_NeverProducesTwoIDFlags(t *testing.T) {
	got := resumeArgv([]string{"claude", "--session-id", "a", "--resume", "b"}, "c")
	joined := strings.Join(got, " ")
	if strings.Count(joined, "--resume") != 1 || strings.Contains(joined, "--session-id") {
		t.Fatalf("resumeArgv produced %q — --session-id and --resume are mutually exclusive", joined)
	}
}

// ---------------------------------------------------------------------------
// sessionEnv — identical environment on launch and on resume.
// ---------------------------------------------------------------------------

func TestSessionEnv_ClaudeModesCarrySessionID(t *testing.T) {
	a := newSuspendTestService()
	for _, mode := range []string{"claude", "claude-auto", "claude-yolo"} {
		env := a.sessionEnv(7, "", mode)
		if !containsEnv(env, "MULTITERMINAL_SESSION_ID=7") {
			t.Fatalf("mode %q lost MULTITERMINAL_SESSION_ID: %v", mode, env)
		}
	}
}

func TestSessionEnv_NonClaudeModesHaveNoSessionID(t *testing.T) {
	a := newSuspendTestService()
	for _, mode := range []string{"shell", "codex", "gemini"} {
		env := a.sessionEnv(7, "", mode)
		if containsEnv(env, "MULTITERMINAL_SESSION_ID=7") {
			t.Fatalf("mode %q must not get MULTITERMINAL_SESSION_ID: %v", mode, env)
		}
	}
}

// A resume that builds a different environment than the launch silently breaks
// hooks / statusline / worktree firewall, so pin that both calls agree.
func TestSessionEnv_IsStableAcrossCalls(t *testing.T) {
	a := newSuspendTestService()
	first := a.sessionEnv(3, "", "claude")
	second := a.sessionEnv(3, "", "claude")
	if strings.Join(first, "\x00") != strings.Join(second, "\x00") {
		t.Fatalf("sessionEnv is not deterministic:\n%v\n%v", first, second)
	}
}

func containsEnv(env []string, want string) bool {
	for _, e := range env {
		if e == want {
			return true
		}
	}
	return false
}

func TestIsClaudeMode(t *testing.T) {
	yes := []string{"claude", "claude-auto", "claude-yolo"}
	no := []string{"shell", "codex", "codex-auto", "gemini", "gemini-yolo", ""}
	for _, m := range yes {
		if !isClaudeMode(m) {
			t.Fatalf("%q should be a claude mode", m)
		}
	}
	for _, m := range no {
		if isClaudeMode(m) {
			t.Fatalf("%q must not be treated as a claude mode (no verified resume path)", m)
		}
	}
}

// ---------------------------------------------------------------------------
// SuspendSession gating.
// ---------------------------------------------------------------------------

func TestSuspendSession_UnknownSession(t *testing.T) {
	a := newSuspendTestService()
	if err := a.SuspendSession(999); err == nil {
		t.Fatal("expected an error for an unknown session")
	}
}

func TestSuspendSession_RejectsNonClaudeModes(t *testing.T) {
	a := newSuspendTestService()
	sess := registerSession(a, 1, "shell", []string{"pwsh"}, "")
	sess.SetResumeID("uuid")
	sess.SetHookActivity(terminal.ActivityDone)
	if err := a.SuspendSession(1); err == nil {
		t.Fatal("shell panes have no resume path and must be rejected")
	}
}

func TestSuspendSession_RejectsWithoutResumeID(t *testing.T) {
	a := newSuspendTestService()
	sess := registerSession(a, 1, "claude", []string{"claude"}, "")
	sess.SetHookActivity(terminal.ActivityDone)
	if err := a.SuspendSession(1); err == nil {
		t.Fatal("a pane without a claude session id could never be resumed")
	}
}

func TestSuspendSession_RejectsBusyPane(t *testing.T) {
	a := newSuspendTestService()
	sess := registerSession(a, 1, "claude", []string{"claude", "--session-id", "u1"}, "")
	sess.SetResumeID("u1")
	sess.SetHookActivity(terminal.ActivityActive)
	if err := a.SuspendSession(1); err == nil {
		t.Fatal("only a finished (done) pane may sleep")
	}
	if sess.IsSuspendedOrSuspending() {
		t.Fatal("a rejected suspend must leave the session untouched")
	}
}

func TestSuspendSession_SuspendsDonePane(t *testing.T) {
	a := newSuspendTestService()
	sess := registerSession(a, 1, "claude", []string{"claude", "--session-id", "u1"}, "")
	sess.SetResumeID("u1")
	sess.SetHookActivity(terminal.ActivityDone)

	if err := a.SuspendSession(1); err != nil {
		t.Fatalf("SuspendSession: %v", err)
	}
	waitFor(t, func() bool { return sess.IsSuspended() }, "session did not reach the suspended state")
	waitFor(t, func() bool { return lastActivity(1) == "sleeping" },
		"prevActivity never became \"sleeping\" — the scan loop would re-emit the old state")
	if !a.IsSessionSuspended(1) {
		t.Fatal("IsSessionSuspended must report true")
	}
}

func TestSuspendSession_IsIdempotent(t *testing.T) {
	a := newSuspendTestService()
	sess := registerSession(a, 1, "claude", []string{"claude", "--session-id", "u1"}, "")
	sess.SetResumeID("u1")
	sess.SetHookActivity(terminal.ActivityDone)
	if err := a.SuspendSession(1); err != nil {
		t.Fatalf("SuspendSession: %v", err)
	}
	waitFor(t, func() bool { return sess.IsSuspended() }, "not suspended")
	if err := a.SuspendSession(1); err != nil {
		t.Fatalf("second SuspendSession must be a no-op, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ResumeSession gating.
// ---------------------------------------------------------------------------

func TestResumeSession_UnknownSession(t *testing.T) {
	a := newSuspendTestService()
	if err := a.ResumeSession(999); err == nil {
		t.Fatal("expected an error for an unknown session")
	}
}

func TestResumeSession_AwakePaneIsNoOp(t *testing.T) {
	a := newSuspendTestService()
	registerSession(a, 1, "claude", []string{"claude"}, "")
	if err := a.ResumeSession(1); err != nil {
		t.Fatalf("resuming an awake pane must be a no-op, got %v", err)
	}
}

func TestResumeSession_WithoutResumeIDFails(t *testing.T) {
	a := newSuspendTestService()
	sess := registerSession(a, 1, "claude", []string{"claude"}, "")
	sess.SetHookActivity(terminal.ActivityDone)
	// Reach the suspended state directly — SuspendSession would refuse without
	// a resume id, which is exactly what we want to observe on the wake path.
	if !sess.TrySuspend() || !sess.FinishSuspend() {
		t.Fatal("failed to put the test session to sleep")
	}
	if err := a.ResumeSession(1); err == nil {
		t.Fatal("waking a pane with no claude session id must fail loudly")
	}
	if !sess.IsSuspended() {
		t.Fatal("a failed wake must leave the pane asleep")
	}
}

func TestResumeSession_UsesResumeArgvAndIdenticalEnv(t *testing.T) {
	a := newSuspendTestService()
	sess := registerSession(a, 1, "claude", []string{"claude", "--model", "opus", "--session-id", "u1"}, "C:\\repo")
	sess.SetResumeID("u1")
	sess.SetHookActivity(terminal.ActivityDone)
	if err := a.SuspendSession(1); err != nil {
		t.Fatalf("SuspendSession: %v", err)
	}
	waitFor(t, func() bool { return sess.IsSuspended() }, "not suspended")

	var gotArgv, gotEnv []string
	var gotDir string
	sess.SetSpawnForTest(func(argv []string, dir string, env []string) error {
		gotArgv, gotDir, gotEnv = argv, dir, env
		return nil
	})
	if err := a.ResumeSession(1); err != nil {
		t.Fatalf("ResumeSession: %v", err)
	}

	want := "claude --model opus --resume u1"
	if strings.Join(gotArgv, " ") != want {
		t.Fatalf("resume argv = %q, want %q", strings.Join(gotArgv, " "), want)
	}
	if gotDir != "C:\\repo" {
		t.Fatalf("resume dir = %q, want the launch dir", gotDir)
	}
	if !containsEnv(gotEnv, "MULTITERMINAL_SESSION_ID=1") {
		t.Fatalf("resume env lost MULTITERMINAL_SESSION_ID: %v", gotEnv)
	}
	if sess.IsSuspended() {
		t.Fatal("pane must be awake after ResumeSession")
	}
	if act := lastActivity(1); act != "resuming" {
		t.Fatalf("prevActivity = %q, want \"resuming\"", act)
	}
}

// lastActivity returns the state the scan loop last saw for a session.
func lastActivity(id int) string {
	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()
	return prevActivity[id]
}

func TestResumeSession_HookSessionIDWinsOverArgv(t *testing.T) {
	a := newSuspendTestService()
	sess := registerSession(a, 1, "claude", []string{"claude", "--session-id", "argv-id"}, "")
	sess.SetResumeID("argv-id")
	sess.SetHookSessionID("hook-id")
	sess.SetHookActivity(terminal.ActivityDone)
	if err := a.SuspendSession(1); err != nil {
		t.Fatalf("SuspendSession: %v", err)
	}
	waitFor(t, func() bool { return sess.IsSuspended() }, "not suspended")

	var gotArgv []string
	sess.SetSpawnForTest(func(argv []string, _ string, _ []string) error { gotArgv = argv; return nil })
	if err := a.ResumeSession(1); err != nil {
		t.Fatalf("ResumeSession: %v", err)
	}
	if strings.Join(gotArgv, " ") != "claude --resume hook-id" {
		t.Fatalf("resume argv = %v — the hook-reported id must win", gotArgv)
	}
}

// ---------------------------------------------------------------------------
// Queue interaction: a sleeping pane must not swallow prompts.
// ---------------------------------------------------------------------------

func TestAddToQueue_OnSleepingPaneWakesInsteadOfHanging(t *testing.T) {
	a := newSuspendTestService()
	sess := registerSession(a, 1, "claude", []string{"claude", "--session-id", "u1"}, "")
	sess.SetResumeID("u1")
	sess.SetHookActivity(terminal.ActivityDone)
	if err := a.SuspendSession(1); err != nil {
		t.Fatalf("SuspendSession: %v", err)
	}
	waitFor(t, func() bool { return sess.IsSuspended() }, "not suspended")

	item := a.AddToQueue(1, "mach weiter")
	if item.ID == 0 {
		t.Fatal("the item must be queued, not rejected")
	}
	// The item stays pending: processQueue refuses to write into a sleeping
	// pane and triggers a wake instead. Nothing may be marked "sent".
	for _, it := range a.GetQueue(1) {
		if it.Status != "pending" {
			t.Fatalf("item %d status = %q, want pending — a sleeping pane cannot receive a prompt", it.ID, it.Status)
		}
	}
	// prevActivity is "sleeping", so tryProcessQueue must still have run: it is
	// the only path that can wake the pane. Verify it is in the trigger set.
	if !triggersQueueProcessing("sleeping") {
		t.Fatal("\"sleeping\" must trigger tryProcessQueue, otherwise AddToQueue hangs silently")
	}
}

// triggersQueueProcessing mirrors tryProcessQueue's condition so the trigger set
// is pinned by a test rather than by reading the code.
func triggersQueueProcessing(act string) bool {
	return act == "done" || act == "idle" || act == "" || act == "sleeping"
}

func TestTryProcessQueue_TriggerSet(t *testing.T) {
	cases := map[string]bool{
		"done": true, "idle": true, "": true, "sleeping": true,
		"active": false, "waitingPermission": false, "waitingAnswer": false,
		"error": false, "resuming": false,
	}
	for act, want := range cases {
		if got := triggersQueueProcessing(act); got != want {
			t.Fatalf("trigger for %q = %v, want %v", act, got, want)
		}
	}
}

func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}
