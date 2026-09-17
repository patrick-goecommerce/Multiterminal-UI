package hub

import "testing"

// Who asked for a session, and what it is running.
//
// Both answers have to survive the trip to a client that did not create the
// session: in daemon mode the window that shows a delegated pane may not even
// have been running when the agent asked for it.

// sleeperLauncher starts a long-lived command whatever it is asked for, so the
// launch path can be exercised without a real agent CLI on the machine.
type sleeperLauncher struct{}

func (sleeperLauncher) Argv(tool, model string) ([]string, error) { return sleepArgv(), nil }
func (sleeperLauncher) Env(int, string, string) []string          { return nil }
func (sleeperLauncher) ResumeArgv(argv []string, _ string) []string {
	return argv
}

func TestSummary_CarriesTheOriginAndTheLaunchModel(t *testing.T) {
	h := NewEmbedded(Options{Version: "test", Launcher: sleeperLauncher{}})
	t.Cleanup(h.Release)

	id, err := h.Create(CreateSpec{
		Dir:    sessionDir(t),
		Origin: OriginAgent,
		Launch: &LaunchRequest{Tool: "claude", Model: "opus"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	s, err := h.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if s.Origin != OriginAgent {
		t.Errorf("Origin = %q, want %q", s.Origin, OriginAgent)
	}
	if s.Mode != "claude" {
		t.Errorf("Mode = %q, want %q", s.Mode, "claude")
	}
	// The status line arrives seconds later; until then the model that was
	// asked for is the answer.
	if s.Model != "opus" {
		t.Errorf("Model = %q, want %q", s.Model, "opus")
	}
}

func TestSummary_AUserPaneHasNoOrigin(t *testing.T) {
	h := newTestHost(t, nil)
	id, err := h.Create(CreateSpec{Argv: sleepArgv(), Dir: sessionDir(t)})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	s, err := h.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if s.Origin != "" {
		t.Errorf("Origin = %q, want empty for a pane the user opened", s.Origin)
	}
}

// The marker is only worth anything if it reaches the other side of the
// socket: that is where the window asking "is this one mine to draw?" is.
func TestRemote_OriginSurvivesTheWire(t *testing.T) {
	host := newTestHost(t, nil)
	addr, _ := serveHost(t, host)
	r := dialTest(t, addr, nil)

	id, err := r.Create(CreateSpec{Argv: sleepArgv(), Dir: sessionDir(t), Origin: OriginAgent})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = r.Close(id) })

	s, err := r.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if s.Origin != OriginAgent {
		t.Errorf("Origin over the wire = %q, want %q", s.Origin, OriginAgent)
	}
}
