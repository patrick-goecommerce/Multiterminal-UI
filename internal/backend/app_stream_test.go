package backend

import "testing"

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
