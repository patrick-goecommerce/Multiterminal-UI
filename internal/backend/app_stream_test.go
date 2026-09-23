package backend

import (
	"context"
	"testing"
	"time"
)

// The output batcher is a required collaborator of CreateSession→collectOutput,
// which can run before ServiceStartup (e.g. scheduled tasks in tests). It must
// therefore never be nil regardless of how the AppService was constructed.
func TestOutputBatch_NeverNilAndStable(t *testing.T) {
	app := newTestApp() // direct construction, no ServiceStartup

	b1 := app.outputBatch()
	if b1 == nil {
		t.Fatal("outputBatch() returned nil before ServiceStartup")
	}

	// Repeated calls must return the same instance, not a fresh one that would
	// drop already-accumulated bytes.
	b2 := app.outputBatch()
	if b1 != b2 {
		t.Error("outputBatch() returned different instances across calls")
	}

	// The returned batcher must be usable without panicking.
	b1.add(1, []byte("hello"))
	if got := b1.swap()[1]; string(got) != "hello" {
		t.Errorf("batcher add/swap roundtrip: got %q, want %q", got, "hello")
	}
}

// runLoop starts runBatchLoop with an emit that reports on a channel.
func runLoop(t *testing.T, b *outputBatcher) <-chan map[int][]byte {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	got := make(chan map[int][]byte, 16)
	go runBatchLoop(ctx, b, func(batch map[int][]byte) { got <- batch })
	return got
}

// An idle app used to wake the loop 62 times a second through empty frames.
// With nothing pending there is nothing to do and nothing is emitted.
func TestBatchLoop_IdleEmitsNothing(t *testing.T) {
	b := newOutputBatcher()
	got := runLoop(t, b)
	select {
	case batch := <-got:
		t.Fatalf("idle loop emitted %v", batch)
	case <-time.After(10 * outputFrame):
	}
	if b.swap() != nil {
		t.Error("an empty swap allocated a batch")
	}
}

// Output still leaves within about a frame, and a burst from several sessions
// becomes one event.
func TestBatchLoop_CoalescesABurstIntoOneFrame(t *testing.T) {
	b := newOutputBatcher()
	got := runLoop(t, b)

	b.add(1, []byte("a"))
	b.add(2, []byte("b"))
	b.add(1, []byte("c"))

	select {
	case batch := <-got:
		if string(batch[1]) != "ac" || string(batch[2]) != "b" {
			t.Errorf("batch = %q", batch)
		}
	case <-time.After(time.Second):
		t.Fatal("pending output was never emitted")
	}
	select {
	case extra := <-got:
		t.Errorf("one burst produced a second event: %q", extra)
	case <-time.After(5 * outputFrame):
	}

	// And the loop wakes again for the next burst.
	b.add(3, []byte("d"))
	select {
	case batch := <-got:
		if string(batch[3]) != "d" {
			t.Errorf("second batch = %q", batch)
		}
	case <-time.After(time.Second):
		t.Fatal("the loop did not wake for the second burst")
	}
}
