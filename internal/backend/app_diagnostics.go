package backend

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
)

// RuntimeStats are cheap runtime counters carried along with CheckHealth.
//
// They exist because MTUI had no runtime introspection at all: diagnosing why
// one instance burned a core for five days took hours of external process
// tooling, and the decisive question — which call site created 3327 OS threads —
// could only be inferred, never shown (issue #192). Each of these is a few
// microseconds to read.
type RuntimeStats struct {
	// Goroutines is the live goroutine count.
	Goroutines int `json:"goroutines" yaml:"goroutines"`
	// ThreadsCreated is how many OS threads the runtime has ever created. Go
	// never destroys an M, so this only grows — a large value means something
	// blocked in syscalls often enough to keep starving the scheduler.
	ThreadsCreated int `json:"threads_created" yaml:"threads_created"`
	// HookFiles counts .jsonl files in the polled hooks directory. This is the
	// number that would have exposed issue #192 on day one: the poller opens
	// every one of them ten times a second.
	HookFiles int `json:"hook_files" yaml:"hook_files"`
	// Sessions is how many terminal sessions the app is tracking.
	Sessions int `json:"sessions" yaml:"sessions"`
}

// hookFileWarnThreshold is where the hooks directory stops being healthy.
//
// A full pass over ~1200 files took longer than the 100 ms poll interval, at
// which point the poller runs back to back with no idle time at all. 200 is far
// below that and still well above any plausible number of live sessions.
const hookFileWarnThreshold = 200

// RuntimeStats returns the current counters. Safe to call from the frontend on
// a timer; nothing here allocates or locks anything expensive.
func (a *AppService) RuntimeStats() RuntimeStats {
	a.mu.Lock()
	sessions := len(a.sessions)
	a.mu.Unlock()

	return RuntimeStats{
		Goroutines:     runtime.NumGoroutine(),
		ThreadsCreated: pprof.Lookup("threadcreate").Count(),
		HookFiles:      a.countHookFiles(),
		Sessions:       sessions,
	}
}

// countHookFiles counts .jsonl files in the hooks directory, or -1 when the
// directory is unknown or unreadable — -1 rather than 0 so the UI can tell
// "nothing there" from "could not look".
func (a *AppService) countHookFiles() int {
	if a.hookMgr == nil || a.hookMgr.dir == "" {
		return -1
	}
	entries, err := os.ReadDir(a.hookMgr.dir)
	if err != nil {
		return -1
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			n++
		}
	}
	return n
}

// WriteDiagnosticDump writes goroutine, threadcreate and heap profiles into the
// log directory and returns the file path.
//
// Deliberately a file, not an HTTP endpoint. net/http/pprof would mean binding
// a port, and on a multi-user terminal server 127.0.0.1 is not per-user — any
// other logged-in account can reach it. That is how the focus listener and the
// MCP server handed one account control over another's sessions (#183), and a
// goroutine dump leaks more than either: full stack traces, file paths and the
// command line. A file in the log directory inherits the profile's NTFS ACL,
// the same isolation the config and session files already rely on.
//
// The goroutine profile is written with debug=2, which renders every stack in
// full — that is the format that answers "which call site is this".
func (a *AppService) WriteDiagnosticDump() string {
	dir := logDir()
	if dir == "" {
		log.Printf("[diag] no log directory available, dump skipped")
		return ""
	}
	path := filepath.Join(dir, fmt.Sprintf("mtui-diag-%s.txt", time.Now().Format("20060102-150405")))
	f, err := os.Create(path)
	if err != nil {
		log.Printf("[diag] could not create %s: %v", path, err)
		return ""
	}
	defer f.Close()

	stats := a.RuntimeStats()
	fmt.Fprintf(f, "mtui diagnostic dump %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, "goroutines=%d threads_created=%d hook_files=%d sessions=%d gomaxprocs=%d\n\n",
		stats.Goroutines, stats.ThreadsCreated, stats.HookFiles, stats.Sessions, runtime.GOMAXPROCS(0))

	for _, p := range []struct {
		name  string
		debug int
	}{
		{"goroutine", 2},
		{"threadcreate", 1},
		{"heap", 0},
	} {
		prof := pprof.Lookup(p.name)
		if prof == nil {
			continue
		}
		fmt.Fprintf(f, "\n===== %s =====\n", p.name)
		if err := prof.WriteTo(f, p.debug); err != nil {
			fmt.Fprintf(f, "(failed: %v)\n", err)
		}
	}

	log.Printf("[diag] wrote %s", path)
	return path
}
