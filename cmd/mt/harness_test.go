package main

import (
	"bytes"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// These tests drive the real command against a real daemon: a real Embedded
// host behind a real listener, found through a real discovery record. The
// point is that nothing about the path from `mt ls` to a PTY is stubbed,
// because every bug this CLI can have lives in that path.

// testDaemon starts a hub server and publishes it where the CLI will look.
func testDaemon(t *testing.T) hub.Host {
	return testDaemonWith(t, hub.Options{Version: "test"})
}

// testDaemonWith is testDaemon with the host configured, for the tests that
// care what the host can do rather than only that it is there.
func testDaemonWith(t *testing.T, opts hub.Options) hub.Host {
	t.Helper()
	// Per-test runtime directory, so a developer's own daemon is neither
	// found by the test nor disturbed by it.
	t.Setenv(discovery.EnvDirOverride, t.TempDir())

	host := hub.NewEmbedded(opts)
	t.Cleanup(host.Release)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	rec, err := discovery.Publish(discovery.ServiceHub, ln.Addr().(*net.TCPAddr).Port)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	srv := hub.NewServer(host, rec.Token)
	httpSrv := &http.Server{Handler: srv.Handler()}
	go func() { _ = httpSrv.Serve(ln) }()
	t.Cleanup(func() { _ = httpSrv.Close() })
	return host
}

// cli runs the command and returns its exit code with what it printed.
func cli(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

// startSession puts a shell on the host and waits until it has produced its
// prompt, so that a read has something to find.
func startSession(t *testing.T, host hub.Host) int {
	t.Helper()
	argv := []string{"/bin/sh"}
	if runtime.GOOS == "windows" {
		argv = []string{os.Getenv("COMSPEC")}
	}
	id, err := host.Create(hub.CreateSpec{Argv: argv, Dir: t.TempDir(), Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = host.Close(id) })
	return id
}

// waitForScreen polls a session's screen until it contains want, or fails.
// Polling rather than sleeping: a loaded CI box is slower than a laptop, and a
// fixed sleep is either flaky there or wasted here.
func waitForScreen(t *testing.T, host hub.Host, id int, want string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var screen string
	for {
		screen, _ = host.PlainText(id)
		if strings.Contains(screen, want) {
			return screen
		}
		if time.Now().After(deadline) {
			t.Fatalf("%q never appeared on session %d's screen:\n%s", want, id, screen)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
