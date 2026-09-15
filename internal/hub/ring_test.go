package hub

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestRing_ReadFromReturnsEverythingWritten(t *testing.T) {
	r := NewRing(64)
	r.Write([]byte("hello "))
	r.Write([]byte("world"))

	data, next, truncated := r.ReadFrom(0)
	if truncated {
		t.Error("nothing was dropped, truncated must be false")
	}
	if got := string(data); got != "hello world" {
		t.Errorf("data = %q, want %q", got, "hello world")
	}
	if next != 11 {
		t.Errorf("next = %d, want 11", next)
	}
}

func TestRing_ReadFromOffsetSkipsWhatTheReaderHas(t *testing.T) {
	r := NewRing(64)
	r.Write([]byte("hello world"))

	data, next, truncated := r.ReadFrom(6)
	if truncated {
		t.Error("offset 6 is still in the ring, truncated must be false")
	}
	if got := string(data); got != "world" {
		t.Errorf("data = %q, want %q", got, "world")
	}
	if next != 11 {
		t.Errorf("next = %d, want 11", next)
	}
}

func TestRing_UpToDateReaderGetsNothing(t *testing.T) {
	r := NewRing(64)
	r.Write([]byte("abc"))

	data, next, truncated := r.ReadFrom(3)
	if len(data) != 0 || next != 3 || truncated {
		t.Errorf("ReadFrom(end) = (%q, %d, %v), want (empty, 3, false)", data, next, truncated)
	}
}

// Wrapping is the case where an off-by-one costs real bytes: the newest data
// sits at the start of the buffer while the oldest still held sits at the end.
func TestRing_WrapKeepsTheNewestBytesInOrder(t *testing.T) {
	r := NewRing(8)
	r.Write([]byte("abcde"))  // no wrap yet
	r.Write([]byte("fghijk")) // wraps, drops "abc"

	data, next, truncated := r.ReadFrom(0)
	if !truncated {
		t.Error("offset 0 fell out of the ring, truncated must be true")
	}
	if got := string(data); got != "defghijk" {
		t.Errorf("data = %q, want %q", got, "defghijk")
	}
	if next != 11 {
		t.Errorf("next = %d, want 11", next)
	}
}

func TestRing_WriteLargerThanRingKeepsTheTail(t *testing.T) {
	r := NewRing(4)
	r.Write([]byte("0123456789"))

	data, next, truncated := r.ReadFrom(0)
	if !truncated {
		t.Error("most of the write is gone, truncated must be true")
	}
	if got := string(data); got != "6789" {
		t.Errorf("data = %q, want %q", got, "6789")
	}
	// Offsets count every byte the session produced, not just the kept ones.
	if next != 10 {
		t.Errorf("next = %d, want 10", next)
	}
}

// ReplayAll means "whatever you still have", which is not the same as asking
// for byte 0 and being told the rest is gone.
func TestRing_ReplayAllIsNotATruncation(t *testing.T) {
	r := NewRing(4)
	r.Write([]byte("0123456789"))

	data, _, truncated := r.ReadFrom(ReplayAll)
	if truncated {
		t.Error("ReplayAll must not report truncation")
	}
	if got := string(data); got != "6789" {
		t.Errorf("data = %q, want %q", got, "6789")
	}
}

func TestRing_StartAndEndTrackTheWindow(t *testing.T) {
	r := NewRing(4)
	if r.Start() != 0 || r.End() != 0 {
		t.Fatalf("fresh ring = [%d,%d), want [0,0)", r.Start(), r.End())
	}
	r.Write([]byte("abc"))
	if r.Start() != 0 || r.End() != 3 {
		t.Errorf("after 3 bytes = [%d,%d), want [0,3)", r.Start(), r.End())
	}
	r.Write([]byte("de"))
	if r.Start() != 1 || r.End() != 5 {
		t.Errorf("after 5 bytes into a 4-byte ring = [%d,%d), want [1,5)", r.Start(), r.End())
	}
}

// A ring of size zero is replay switched off. It must still count, because the
// offsets are what a reconnecting client resumes from.
func TestRing_ZeroSizeStillCounts(t *testing.T) {
	r := NewRing(0)
	r.Write([]byte("abcdef"))

	data, next, _ := r.ReadFrom(0)
	if len(data) != 0 {
		t.Errorf("zero-size ring returned %q, want nothing", data)
	}
	if next != 6 {
		t.Errorf("next = %d, want 6", next)
	}
}

func TestRing_WaitFiresOnWrite(t *testing.T) {
	r := NewRing(16)
	wait := r.Wait()
	select {
	case <-wait:
		t.Fatal("Wait fired before anything was written")
	default:
	}

	r.Write([]byte("x"))
	select {
	case <-wait:
	case <-time.After(time.Second):
		t.Fatal("Wait did not fire after a write")
	}
}

func TestRing_WaitFiresOnClose(t *testing.T) {
	r := NewRing(16)
	wait := r.Wait()
	r.Close()
	select {
	case <-wait:
	case <-time.After(time.Second):
		t.Fatal("Wait did not fire on Close")
	}
	if !r.Closed() {
		t.Error("Closed() = false after Close()")
	}
}

// A reader that takes Wait after the ring is already closed must not block:
// there will be no further notification to wake it.
func TestRing_WaitOnClosedRingIsAlreadyClosed(t *testing.T) {
	r := NewRing(16)
	r.Close()
	select {
	case <-r.Wait():
	case <-time.After(time.Second):
		t.Fatal("Wait on a closed ring blocked")
	}
}

// Data written before Close stays readable: a pane whose process exited still
// shows its last words.
func TestRing_ClosedRingKeepsItsData(t *testing.T) {
	r := NewRing(16)
	r.Write([]byte("last words"))
	r.Close()

	data, _, _ := r.ReadFrom(0)
	if got := string(data); got != "last words" {
		t.Errorf("data after Close = %q, want %q", got, "last words")
	}
}

func TestRing_ConcurrentWritersKeepByteCount(t *testing.T) {
	r := NewRing(1 << 16)
	const writers, perWriter = 8, 200
	chunk := bytes.Repeat([]byte("z"), 16)

	done := make(chan struct{})
	for i := 0; i < writers; i++ {
		go func() {
			for j := 0; j < perWriter; j++ {
				r.Write(chunk)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < writers; i++ {
		<-done
	}

	want := int64(writers * perWriter * len(chunk))
	if r.End() != want {
		t.Errorf("End() = %d, want %d", r.End(), want)
	}
	data, _, _ := r.ReadFrom(ReplayAll)
	if strings.Trim(string(data), "z") != "" {
		t.Error("ring content is not a clean run of the written bytes")
	}
}
