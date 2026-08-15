package backend

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRuntimeStats_CountsLiveState(t *testing.T) {
	a := newTestApp()
	dir := t.TempDir()
	a.hookMgr = &HookManager{dir: dir, offsets: map[string]int64{}}

	for _, n := range []string{"a.jsonl", "b.jsonl", "ignored.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("{}\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", n, err)
		}
	}

	st := a.RuntimeStats()
	if st.HookFiles != 2 {
		t.Errorf("HookFiles = %d, want 2 (non-.jsonl must not count)", st.HookFiles)
	}
	if st.Goroutines < 1 {
		t.Errorf("Goroutines = %d, want at least 1", st.Goroutines)
	}
	// Go never destroys an M, so this can only ever be >= 1 in a running test.
	if st.ThreadsCreated < 1 {
		t.Errorf("ThreadsCreated = %d, want at least 1", st.ThreadsCreated)
	}
	if st.Sessions != 0 {
		t.Errorf("Sessions = %d, want 0 for a fresh app", st.Sessions)
	}
}

// -1 rather than 0 so the UI can distinguish "no files" from "could not look".
func TestRuntimeStats_UnknownHookDirReportsMinusOne(t *testing.T) {
	a := newTestApp()

	a.hookMgr = nil
	if got := a.RuntimeStats().HookFiles; got != -1 {
		t.Errorf("HookFiles = %d without a hook manager, want -1", got)
	}

	a.hookMgr = &HookManager{dir: filepath.Join(t.TempDir(), "missing"), offsets: map[string]int64{}}
	if got := a.RuntimeStats().HookFiles; got != -1 {
		t.Errorf("HookFiles = %d for an unreadable directory, want -1", got)
	}
}

// The whole point of the counter: it has to trip before the poller does.
// A full pass over ~1200 files exceeded the 100 ms interval (issue #192).
func TestHookFileWarnThreshold_TripsWellBeforeTheP0ller(t *testing.T) {
	if hookFileWarnThreshold >= 1200 {
		t.Errorf("threshold %d does not warn before the poll interval is exceeded", hookFileWarnThreshold)
	}
	if hookFileWarnThreshold < 50 {
		t.Errorf("threshold %d is low enough to fire on normal use", hookFileWarnThreshold)
	}
}

func TestCheckHealth_CarriesRuntimeStats(t *testing.T) {
	a := newTestApp()
	dir := t.TempDir()
	a.hookMgr = &HookManager{dir: dir, offsets: map[string]int64{}}

	h := a.CheckHealth()
	if h.Runtime.Goroutines < 1 {
		t.Error("CheckHealth did not fill in the runtime counters")
	}
	if h.HookFilesHigh {
		t.Error("HookFilesHigh is set for an empty hooks directory")
	}

	for i := 0; i < hookFileWarnThreshold+1; i++ {
		name := filepath.Join(dir, "sess-"+strconv.Itoa(i)+".jsonl")
		if err := os.WriteFile(name, []byte("{}\n"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if !a.CheckHealth().HookFilesHigh {
		t.Errorf("HookFilesHigh not set with %d files", hookFileWarnThreshold+1)
	}
}

func TestWriteDiagnosticDump_WritesAProfile(t *testing.T) {
	a := newTestApp()
	a.hookMgr = &HookManager{dir: t.TempDir(), offsets: map[string]int64{}}

	// logDir() resolves against the user profile; redirect it for the test.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	path := a.WriteDiagnosticDump()
	if path == "" {
		t.Fatal("WriteDiagnosticDump returned no path")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dump: %v", err)
	}
	text := string(body)

	for _, want := range []string{"goroutines=", "===== goroutine =====", "===== threadcreate =====", "===== heap ====="} {
		if !strings.Contains(text, want) {
			t.Errorf("dump is missing %q", want)
		}
	}
	// debug=2 renders full stacks; without them the dump cannot answer
	// "which call site created these".
	if !strings.Contains(text, "goroutine ") || !strings.Contains(text, "[running]") {
		t.Error("the goroutine profile carries no stack traces — was it written with debug<2?")
	}
}
