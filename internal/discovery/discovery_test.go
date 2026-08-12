package discovery

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// useTempDir points the discovery directory at a per-test temp dir so tests
// never touch the real %LOCALAPPDATA%\mtui.
func useTempDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "runtime")
	t.Setenv(EnvDirOverride, dir)
	return dir
}

func TestPublishThenResolveRoundTrip(t *testing.T) {
	useTempDir(t)

	pub, err := Publish(ServiceFocus, 45123)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if pub.Port != 45123 {
		t.Fatalf("published port = %d, want 45123", pub.Port)
	}
	if pub.PID != os.Getpid() {
		t.Fatalf("published pid = %d, want %d", pub.PID, os.Getpid())
	}
	if len(pub.Token) != 32 {
		t.Fatalf("token = %q, want 32 hex chars", pub.Token)
	}

	got, err := Resolve(ServiceFocus)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Port != pub.Port || got.Token != pub.Token || got.PID != pub.PID {
		t.Fatalf("resolved %+v, want %+v", got, pub)
	}
	if got.Addr() != "127.0.0.1:45123" {
		t.Fatalf("Addr() = %q", got.Addr())
	}
}

func TestPublishIsPerServiceAndOverwrites(t *testing.T) {
	useTempDir(t)

	if _, err := Publish(ServiceFocus, 40001); err != nil {
		t.Fatalf("Publish focus: %v", err)
	}
	if _, err := Publish(ServiceMCP, 40002); err != nil {
		t.Fatalf("Publish mcp: %v", err)
	}

	focus, err := Resolve(ServiceFocus)
	if err != nil {
		t.Fatalf("Resolve focus: %v", err)
	}
	mcp, err := Resolve(ServiceMCP)
	if err != nil {
		t.Fatalf("Resolve mcp: %v", err)
	}
	if focus.Port != 40001 || mcp.Port != 40002 {
		t.Fatalf("services leaked into each other: focus=%d mcp=%d", focus.Port, mcp.Port)
	}
	if focus.Token == mcp.Token {
		t.Fatal("both services share a token; tokens must be per listener")
	}

	// A restart republishes over the existing record instead of piling up.
	republished, err := Publish(ServiceFocus, 40003)
	if err != nil {
		t.Fatalf("republish: %v", err)
	}
	again, err := Resolve(ServiceFocus)
	if err != nil {
		t.Fatalf("Resolve after republish: %v", err)
	}
	if again.Port != 40003 || again.Token != republished.Token {
		t.Fatalf("stale record survived republish: %+v", again)
	}
}

func TestPublishRejectsUnboundPort(t *testing.T) {
	useTempDir(t)

	for _, port := range []int{0, -1, 70000} {
		if _, err := Publish(ServiceMCP, port); err == nil {
			t.Fatalf("Publish(%d) succeeded, want error", port)
		}
	}
	if _, err := Read(ServiceMCP); !errors.Is(err, ErrNotPublished) {
		t.Fatalf("Read after rejected publish = %v, want ErrNotPublished", err)
	}
}

func TestResolveWithoutRecord(t *testing.T) {
	useTempDir(t)

	if _, err := Resolve(ServiceFocus); !errors.Is(err, ErrNotPublished) {
		t.Fatalf("Resolve = %v, want ErrNotPublished", err)
	}
}

// A record left behind by a crashed process (or by the updater's os.Exit(0),
// issue #184) names a port that nothing of ours owns any more. Resolve must
// reject it rather than letting a caller connect to whatever took the port.
func TestResolveRejectsStaleRecordOfDeadProcess(t *testing.T) {
	dir := useTempDir(t)

	if _, err := Publish(ServiceFocus, 40010); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	writeRawRecord(t, dir, ServiceFocus, Record{
		Service:   string(ServiceFocus),
		Port:      40010,
		PID:       deadPID(t),
		Token:     "deadbeefdeadbeefdeadbeefdeadbeef",
		StartedAt: time.Now().UTC(),
	})

	// Read still returns it — Read makes no liveness claim.
	if _, err := Read(ServiceFocus); err != nil {
		t.Fatalf("Read: %v", err)
	}
	_, err := Resolve(ServiceFocus)
	if !errors.Is(err, ErrStale) {
		t.Fatalf("Resolve = %v, want ErrStale", err)
	}
}

func TestReadIgnoresCorruptRecord(t *testing.T) {
	dir := useTempDir(t)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "focus.port.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Read(ServiceFocus); !errors.Is(err, ErrNotPublished) {
		t.Fatalf("Read(corrupt) = %v, want ErrNotPublished", err)
	}

	// A syntactically valid record with a nonsense port is equally unusable.
	writeRawRecord(t, dir, ServiceFocus, Record{Service: "focus", Port: 0, PID: os.Getpid()})
	if _, err := Read(ServiceFocus); !errors.Is(err, ErrNotPublished) {
		t.Fatalf("Read(port 0) = %v, want ErrNotPublished", err)
	}
}

func TestRemoveIsIdempotent(t *testing.T) {
	useTempDir(t)

	if _, err := Publish(ServiceMCP, 40020); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := Remove(ServiceMCP); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := Remove(ServiceMCP); err != nil {
		t.Fatalf("second Remove: %v", err)
	}
	if _, err := Resolve(ServiceMCP); !errors.Is(err, ErrNotPublished) {
		t.Fatalf("Resolve after Remove = %v, want ErrNotPublished", err)
	}
}

func TestDirIsPerUserAndOverridable(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom")
	t.Setenv(EnvDirOverride, custom)
	got, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if got != custom {
		t.Fatalf("Dir() = %q, want %q", got, custom)
	}

	// Without the override the directory must live under the user-scoped cache
	// root, never in a machine-wide location.
	t.Setenv(EnvDirOverride, "")
	got, err = Dir()
	if err != nil {
		t.Fatalf("Dir without override: %v", err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Skipf("no user cache dir on this platform: %v", err)
	}
	if !strings.HasPrefix(got, cache) {
		t.Fatalf("Dir() = %q, want a path under the per-user cache root %q", got, cache)
	}
}

// Publish must survive a listener that binds port 0: the caller resolves the
// real port from the listener before publishing, and that is what readers get.
func TestPublishResolvedEphemeralPort(t *testing.T) {
	useTempDir(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	bound := ln.Addr().(*net.TCPAddr).Port
	if bound == 0 {
		t.Fatal("OS assigned port 0")
	}
	if _, err := Publish(ServiceFocus, bound); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	rec, err := Resolve(ServiceFocus)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if rec.Port != bound {
		t.Fatalf("resolved port %d, want %d", rec.Port, bound)
	}
	if !Probe(rec, 2*time.Second) {
		t.Fatal("Probe could not reach the published listener")
	}

	ln.Close()
	if Probe(rec, 200*time.Millisecond) {
		t.Fatal("Probe reached a closed listener")
	}
}

func TestProcessAlive(t *testing.T) {
	if !ProcessAlive(os.Getpid()) {
		t.Fatal("ProcessAlive(self) = false")
	}
	if ProcessAlive(0) || ProcessAlive(-1) {
		t.Fatal("ProcessAlive accepted a non-positive pid")
	}
	if ProcessAlive(deadPID(t)) {
		t.Fatal("ProcessAlive(exited process) = true")
	}
}

// writeRawRecord plants a record file directly, bypassing Publish, so tests can
// construct states that only a crashed or foreign process could leave behind.
func writeRawRecord(t *testing.T, dir string, svc Service, rec Record) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, string(svc)+".port.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// deadPID returns the PID of a process that has certainly exited. It re-runs
// the test binary with a -test.run pattern that matches nothing, so the child
// exits immediately; that keeps the helper dependency-free and portable.
func deadPID(t *testing.T) int {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("cannot locate the test binary: %v", err)
	}
	cmd := exec.Command(exe, "-test.run=ZZZ_discovery_no_such_test")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start helper process: %v", err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Wait()
	return pid
}
