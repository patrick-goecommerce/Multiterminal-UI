package mcpsrv

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// sleepArgv is a command that stays alive without producing output.
func sleepArgv() []string {
	if runtime.GOOS == "windows" {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = "cmd.exe"
		}
		return []string{comspec, "/c", "ping -n 60 127.0.0.1 > nul"}
	}
	return []string{"/bin/sh", "-c", "sleep 60"}
}

// sessionDir returns a working directory for a test session. Not t.TempDir():
// on Windows a directory that is a live process's working directory cannot be
// removed, and t.TempDir's cleanup runs before the host has let go of it.
func sessionDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "mtui-mcp-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		for i := 0; i < 40; i++ {
			if os.RemoveAll(dir) == nil {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	})
	return dir
}

// testServer builds a server over a host of its own.
func testServer(t *testing.T) (*Server, *hub.Embedded) {
	t.Helper()
	h := hub.NewEmbedded(hub.Options{Version: "test"})
	t.Cleanup(h.Release)
	return &Server{host: h}, h
}

// startSession puts a running session on the host, with the given origin.
func startSession(t *testing.T, h *hub.Embedded, origin string) int {
	t.Helper()
	id, err := h.Create(hub.CreateSpec{Argv: sleepArgv(), Dir: sessionDir(t), Origin: origin})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = h.Close(id) })
	return id
}

func TestOpenSessionRejectsAnUnsupportedTool(t *testing.T) {
	s, _ := testServer(t)
	if _, err := s.OpenSession("notreal", t.TempDir(), "", ""); err == nil {
		t.Fatal("expected an error for an unsupported tool, got nil")
	}
}

// A host with no launcher cannot start an agent by name. The error has to say
// so rather than leaving a half-configured session behind.
func TestOpenSessionWithoutALauncherFails(t *testing.T) {
	s, h := testServer(t)
	if _, err := s.OpenSession("claude", sessionDir(t), "", ""); err == nil {
		t.Fatal("expected an error from a host without a launcher, got nil")
	}
	if got := len(h.List()); got != 0 {
		t.Errorf("host holds %d sessions after a failed open, want 0", got)
	}
}

func TestSendInputUnknownSession(t *testing.T) {
	s, _ := testServer(t)
	if err := s.SendInput(999, "hello"); err == nil {
		t.Fatal("expected an error for an unknown session, got nil")
	}
}

func TestReadOutputUnknownSession(t *testing.T) {
	s, _ := testServer(t)
	if _, err := s.ReadOutput(999); err == nil {
		t.Fatal("expected an error for an unknown session, got nil")
	}
}

func TestCloseSessionUnknownSession(t *testing.T) {
	s, _ := testServer(t)
	if err := s.CloseSession(999, "cleanup"); err == nil {
		t.Fatal("expected an error for an unknown session, got nil")
	}
}

func TestCloseSessionEndsIt(t *testing.T) {
	s, h := testServer(t)
	id := startSession(t, h, hub.OriginAgent)
	if err := s.CloseSession(id, "done"); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if got := len(h.List()); got != 0 {
		t.Errorf("host holds %d sessions after the close, want 0", got)
	}
}

// The list is what a delegating agent uses to find its own work again, so it
// must not report the panes a person opened.
func TestListSessionsOnlyReportsDelegatedOnes(t *testing.T) {
	s, h := testServer(t)
	mine := startSession(t, h, hub.OriginAgent)
	startSession(t, h, "") // a pane the user opened

	got := s.ListSessions()
	if len(got) != 1 {
		t.Fatalf("ListSessions returned %d sessions, want 1: %+v", len(got), got)
	}
	if got[0].ID != mine {
		t.Errorf("ListSessions returned session %d, want %d", got[0].ID, mine)
	}
	if !got[0].Running {
		t.Error("a running session was reported as not running")
	}
}

func TestListSessionsEmpty(t *testing.T) {
	s, _ := testServer(t)
	if got := s.ListSessions(); len(got) != 0 {
		t.Fatalf("expected no delegated sessions, got %+v", got)
	}
}

func TestStartBindsLoopbackAndPublishesThePort(t *testing.T) {
	// Start publishes its port; without the redirect the test would overwrite
	// the record of the developer's own running instance.
	t.Setenv(discovery.EnvDirOverride, filepath.Join(t.TempDir(), "runtime"))

	h := hub.NewEmbedded(hub.Options{Version: "test"})
	t.Cleanup(h.Release)

	port, err := Start(Options{Host: h, Version: "test"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if port <= 0 {
		t.Fatalf("expected a bound port, got %d", port)
	}
	rec, err := discovery.Resolve(discovery.ServiceMCP)
	if err != nil {
		t.Fatalf("the port was not published: %v", err)
	}
	if rec.Port != port {
		t.Errorf("published port %d, server is on %d", rec.Port, port)
	}
}

func TestStartWithoutAHostFails(t *testing.T) {
	if _, err := Start(Options{}); err == nil {
		t.Fatal("expected an error without a host, got nil")
	}
}
