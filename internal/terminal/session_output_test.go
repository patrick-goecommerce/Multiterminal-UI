package terminal

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// bulkEnv switches the test binary into its helper role: print a lot, then a
// marker, then exit. Re-running the test binary is the portable way to get the
// same child process on every platform.
const bulkEnv = "MTUI_TEST_BULK_PRINT"

const bulkMarker = "BULK-END-MARKER"

// bulkLines is chosen to exceed one 64 KiB PTY read several times over, so
// that reads continue after the child has already exited.
const bulkLines = 8000

func TestMain(m *testing.M) {
	if os.Getenv(bulkEnv) != "" {
		for i := 0; i < bulkLines; i++ {
			fmt.Printf("line-%05d-padding-padding-padding\n", i)
		}
		fmt.Println(bulkMarker)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// Output still queued in the PTY when the process exits must not be dropped.
//
// readLoop used to select over the send and the process-exit guard in one
// statement. A child that writes more than the reader has drained keeps the
// loop reading after it has exited, and from that point both cases are ready
// on every chunk: Go picks a ready case at random, so roughly half of the
// remaining output went nowhere. The tail of a command's output is exactly the
// part a user reads, and nothing anywhere reported a loss.
func TestReadLoop_OutputQueuedAtExitIsNotDropped(t *testing.T) {
	for attempt := 0; attempt < 3; attempt++ {
		s := NewSession(1, 24, 80)
		env := append(os.Environ(), bulkEnv+"=1")
		if err := s.Start([]string{os.Args[0]}, t.TempDir(), env); err != nil {
			t.Fatalf("attempt %d: Start: %v", attempt, err)
		}

		var got strings.Builder
		deadline := time.After(30 * time.Second)
	collect:
		for {
			select {
			case chunk, ok := <-s.RawOutputCh:
				if !ok {
					break collect
				}
				got.Write(chunk)
				if strings.Contains(got.String(), bulkMarker) {
					break collect
				}
			case <-deadline:
				break collect
			}
		}
		text := got.String()
		s.Close()

		if !strings.Contains(text, bulkMarker) {
			t.Fatalf("attempt %d: the last line never arrived (%d bytes received)",
				attempt, len(text))
		}
		// The marker alone would not catch a hole in the middle, so check the
		// run is complete.
		for _, i := range []int{0, bulkLines / 2, bulkLines - 1} {
			want := fmt.Sprintf("line-%05d", i)
			if !strings.Contains(text, want) {
				t.Fatalf("attempt %d: %q missing from %d bytes of output",
					attempt, want, len(text))
			}
		}
	}
}
