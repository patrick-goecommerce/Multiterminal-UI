package main

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// startDaemon runs the daemon in this process and waits for its record.
func startDaemon(t *testing.T, args ...string) (discovery.Record, chan int) {
	t.Helper()
	exit := make(chan int, 1)
	go func() { exit <- run(args) }()

	deadline := time.Now().Add(15 * time.Second)
	for {
		rec, err := discovery.Resolve(discovery.ServiceHub)
		if err == nil {
			return rec, exit
		}
		if time.Now().After(deadline) {
			t.Fatalf("daemon never published a record: %v", err)
		}
		select {
		case code := <-exit:
			t.Fatalf("daemon exited with %d before publishing", code)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// askShutdown posts the shutdown request the way a client would.
func askShutdown(t *testing.T, rec discovery.Record) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "http://"+rec.Addr()+"/v1/hub/shutdown", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("shutdown request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("shutdown status = %d, want 202", resp.StatusCode)
	}
}

func awaitExit(t *testing.T, exit chan int, want int) {
	t.Helper()
	select {
	case code := <-exit:
		if code != want {
			t.Errorf("exit code = %d, want %d", code, want)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("daemon did not stop")
	}
}

// The whole point, end to end: a client connects to a daemon it did not start,
// runs a session, goes away, and the session is still there for the next one.
func TestDaemon_ServesSessionsAcrossClients(t *testing.T) {
	t.Setenv(discovery.EnvDirOverride, filepath.Join(t.TempDir(), "runtime"))
	rec, exit := startDaemon(t, "-idle", "0")

	first, err := hub.Dial(rec.Addr(), rec.Token, hub.DialOptions{})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if !first.WaitReady(10 * time.Second) {
		t.Fatal("stream socket never came up")
	}

	argv := []string{"/bin/sh", "-c", "sleep 60"}
	if runtime.GOOS == "windows" {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = "cmd.exe"
		}
		argv = []string{comspec, "/c", "ping -n 60 127.0.0.1 > nul"}
	}
	id, err := first.Create(hub.CreateSpec{Argv: argv, Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The client leaves, as it would when its window is closed.
	first.Release()

	second, err := hub.Dial(rec.Addr(), rec.Token, hub.DialOptions{})
	if err != nil {
		t.Fatalf("second Dial: %v", err)
	}
	defer second.Release()

	summary, err := second.Get(id)
	if err != nil {
		t.Fatalf("the session did not survive the first client: %v", err)
	}
	if summary.Status != hub.StatusRunning {
		t.Errorf("status = %q, want %q", summary.Status, hub.StatusRunning)
	}

	askShutdown(t, rec)
	awaitExit(t, exit, exitOK)
}

// A second daemon must not start: one hub per user, decided by the lock.
func TestDaemon_SecondInstanceStandsDown(t *testing.T) {
	t.Setenv(discovery.EnvDirOverride, filepath.Join(t.TempDir(), "runtime"))
	rec, exit := startDaemon(t, "-idle", "0")

	before, err := discovery.Read(discovery.ServiceHub)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}

	if code := run([]string{"-idle", "0"}); code != exitOK {
		t.Errorf("second instance exit code = %d, want %d", code, exitOK)
	}

	after, err := discovery.Read(discovery.ServiceHub)
	if err != nil {
		t.Fatalf("read record after second instance: %v", err)
	}
	if after.Port != before.Port || after.Token != before.Token {
		t.Error("the second instance overwrote the running daemon's record")
	}

	askShutdown(t, rec)
	awaitExit(t, exit, exitOK)
}

// Stopping the daemon removes its record: a stale record is normal after a
// crash, but a clean exit must not leave one behind for a client to dial.
func TestDaemon_CleanExitWithdrawsItsRecord(t *testing.T) {
	t.Setenv(discovery.EnvDirOverride, filepath.Join(t.TempDir(), "runtime"))
	rec, exit := startDaemon(t, "-idle", "0")
	askShutdown(t, rec)
	awaitExit(t, exit, exitOK)

	if _, err := discovery.Read(discovery.ServiceHub); err == nil {
		t.Error("the record is still there after a clean shutdown")
	}
}

func TestDaemon_VersionFlagPrintsAndExits(t *testing.T) {
	if code := run([]string{"-version"}); code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
}

func TestHubID_SurvivesARestart(t *testing.T) {
	t.Setenv(discovery.EnvDirOverride, filepath.Join(t.TempDir(), "runtime"))

	if id, err := loadHubID(); err != nil || id != "" {
		t.Fatalf("loadHubID on a fresh dir = (%q, %v), want empty and no error", id, err)
	}
	if err := saveHubID("abc123"); err != nil {
		t.Fatalf("saveHubID: %v", err)
	}
	id, err := loadHubID()
	if err != nil {
		t.Fatalf("loadHubID: %v", err)
	}
	if id != "abc123" {
		t.Errorf("hub id = %q, want %q", id, "abc123")
	}
}

// A garbled file is corruption, not an identity. Treating it as "none" makes
// the daemon generate a fresh one instead of reporting nonsense to clients.
func TestHubID_RejectsGarbage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "runtime")
	t.Setenv(discovery.EnvDirOverride, dir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, hubIDFile)
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), hubIDMaxLen+1), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if id, err := loadHubID(); err != nil || id != "" {
		t.Errorf("loadHubID on garbage = (%q, %v), want empty and no error", id, err)
	}
}

// --- idle watcher ---------------------------------------------------------

type fakeSessions struct{ n int }

func (f fakeSessions) List() []hub.SessionSummary {
	return make([]hub.SessionSummary, f.n)
}

type fakeClients struct{ n int }

func (f fakeClients) Clients() int { return f.n }

func withFastIdleTicks(t *testing.T) {
	t.Helper()
	previous := idleCheckInterval
	idleCheckInterval = 5 * time.Millisecond
	t.Cleanup(func() { idleCheckInterval = previous })
}

func TestWatchIdle_StopsWhenNobodyNeedsTheDaemon(t *testing.T) {
	withFastIdleTicks(t)
	stopped := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go watchIdle(ctx, fakeSessions{0}, fakeClients{0}, 20*time.Millisecond,
		func() { close(stopped) })

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("an idle daemon did not stop")
	}
}

func TestWatchIdle_AnAttachedClientKeepsItAlive(t *testing.T) {
	withFastIdleTicks(t)
	stopped := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go watchIdle(ctx, fakeSessions{0}, fakeClients{1}, 20*time.Millisecond,
		func() { close(stopped) })

	select {
	case <-stopped:
		t.Fatal("stopped while a client was attached")
	case <-time.After(300 * time.Millisecond):
	}
}

// A session with no client attached is still a running agent: the daemon
// exists precisely so that nobody has to be watching it.
func TestWatchIdle_ASessionAloneKeepsItAlive(t *testing.T) {
	withFastIdleTicks(t)
	stopped := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go watchIdle(ctx, fakeSessions{1}, fakeClients{0}, 20*time.Millisecond,
		func() { close(stopped) })

	select {
	case <-stopped:
		t.Fatal("stopped while a session was running")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestWatchIdle_ZeroTimeoutDisablesIt(t *testing.T) {
	withFastIdleTicks(t)
	stopped := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go watchIdle(ctx, fakeSessions{0}, fakeClients{0}, 0, func() { close(stopped) })

	select {
	case <-stopped:
		t.Fatal("stopped although idle shutdown was disabled")
	case <-time.After(200 * time.Millisecond):
	}
}
