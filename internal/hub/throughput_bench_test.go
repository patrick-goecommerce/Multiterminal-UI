package hub

import (
	"fmt"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// The measurement the daemon default is waiting on.
//
// The design spec makes a throughput comparison the condition for flipping
// session_host to "daemon" by default, and the question it has to answer is
// narrow: does routing a session's output through a socket cost enough to
// notice, with the pane count a real user has open?
//
// Run it where that matters, which is a real Windows machine (ConPTY, not a
// Unix PTY, and the WebView is the consumer):
//
//	go test ./internal/hub/ -run '^$' -bench Throughput -benchtime 5x
//
// The two lines to compare are Embedded (sessions in this process, today's
// default) and Remote (sessions in a daemon, reached over loopback). The
// MB/s column is the answer.
//
// What it does NOT answer: GUI CPU. The consumer here is a Go goroutine, not
// a WebView writing to xterm.js, and the coalescing that sits between them
// lives in internal/backend. For that half, run the app with eight busy panes
// and watch the process. This benchmark isolates the part the daemon changed.

// spewEnv switches the test binary into its producer role: print a fixed
// number of bytes, then exit. Re-running the test binary is the portable way
// to get the same child process on every platform, which matters here because
// the number being compared must not also depend on which shell is installed.
const spewEnv = "MTUI_BENCH_SPEW_BYTES"

// spewLine is 64 bytes including the newline, so the byte count is exact
// rather than approximate. A PTY read is 64 KiB, so this fills one in 1024
// lines and the measurement is not dominated by per-read overhead.
const spewLine = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abc\n"

// bytesPerSession is how much each session produces per iteration. Large
// enough that process startup is not what is being measured, small enough
// that eight sessions still finish in well under a second.
const bytesPerSession = 4 << 20 // 4 MiB

func TestMain(m *testing.M) {
	if raw := os.Getenv(spewEnv); raw != "" {
		total, err := strconv.Atoi(raw)
		if err != nil {
			os.Exit(2)
		}
		for written := 0; written < total; written += len(spewLine) {
			if _, err := os.Stdout.WriteString(spewLine); err != nil {
				os.Exit(1)
			}
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// benchPanes is the pane count the spec names. Eight busy agents is a heavy
// but real day.
const benchPanes = 8

func BenchmarkThroughput_Embedded(b *testing.B) {
	benchThroughput(b, func(b *testing.B) (Host, func()) {
		h := NewEmbedded(Options{Version: "bench"})
		return h, h.Release
	})
}

func BenchmarkThroughput_Remote(b *testing.B) {
	benchThroughput(b, func(b *testing.B) (Host, func()) {
		h := NewEmbedded(Options{Version: "bench"})
		srv := NewServer(h, testToken)
		ts := httptest.NewServer(srv.Handler())
		r, err := Dial(strings.TrimPrefix(ts.URL, "http://"), testToken, DialOptions{})
		if err != nil {
			b.Fatalf("Dial: %v", err)
		}
		if !r.WaitReady(10 * time.Second) {
			b.Fatal("the stream socket never came up")
		}
		return r, func() {
			r.Release()
			ts.Close()
			h.Release()
		}
	})
}

// benchThroughput streams bytesPerSession from each of benchPanes sessions and
// reports the delivered rate.
//
// It counts what a client actually received, not what the PTY produced: the
// whole point of the comparison is the path between them.
func benchThroughput(b *testing.B, setup func(*testing.B) (Host, func())) {
	if testing.Short() {
		b.Skip("the throughput benchmark starts real processes")
	}
	b.SetBytes(int64(benchPanes * bytesPerSession))
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		host, cleanup := setup(b)
		dir := b.TempDir()
		env := append(os.Environ(), fmt.Sprintf("%s=%d", spewEnv, bytesPerSession))

		ids := make([]int, 0, benchPanes)
		subs := make([]*Subscription, 0, benchPanes)
		for p := 0; p < benchPanes; p++ {
			id, err := host.Create(CreateSpec{
				Argv: []string{os.Args[0]}, Dir: dir, Rows: 24, Cols: 80, Env: env,
			})
			if err != nil {
				cleanup()
				b.Fatalf("Create: %v", err)
			}
			sub, err := host.Attach(id, ReplayAll)
			if err != nil {
				cleanup()
				b.Fatalf("Attach: %v", err)
			}
			ids = append(ids, id)
			subs = append(subs, sub)
		}
		b.StartTimer()

		// Wait on the byte count, not on the channel closing: a subscription
		// closes with the SESSION, not with its process, so a producer that
		// has exited leaves its channel open. Counting what arrived is also
		// the honest measure of what was delivered.
		var wg sync.WaitGroup
		var mu sync.Mutex
		var truncated int
		for _, sub := range subs {
			wg.Add(1)
			go func(s *Subscription) {
				defer wg.Done()
				got := 0
				for chunk := range s.C {
					if chunk.Truncated {
						mu.Lock()
						truncated++
						mu.Unlock()
					}
					got += len(chunk.Data)
					if got >= bytesPerSession {
						return
					}
				}
			}(sub)
		}
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Minute):
			b.Fatal("the producers never finished")
		}

		b.StopTimer()
		if truncated > 0 {
			// A truncation means the ring lapped before the reader drained it,
			// so the rate above is for less data than was produced and the two
			// implementations are no longer being compared on equal terms.
			b.Logf("WARNING: %d truncated chunk(s); the reader fell behind the ring", truncated)
		}
		for _, sub := range subs {
			sub.Close()
		}
		for _, id := range ids {
			_ = host.Close(id)
		}
		cleanup()
		b.StartTimer()
	}
}
