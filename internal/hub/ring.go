package hub

import "sync"

// Ring is a fixed-size buffer of raw PTY output with absolute offsets.
//
// It exists so a client can leave and come back. Every byte a session produces
// is counted, and the last RingBytes of them are kept, so an attaching client
// can ask "give me everything from offset N" and either get it or be told
// truthfully that it is gone.
//
// Readers are not pushed to. They wait on Wait and then pull whatever has
// accumulated at their own offset. That is what makes a slow client harmless:
// it cannot block the PTY reader, it just falls behind and eventually reads a
// truncated range instead of a contiguous one. A per-reader queue would have
// to choose between blocking the producer and dropping bytes silently, and
// silently is the one thing a VT100 stream cannot survive.
type Ring struct {
	mu sync.Mutex
	// buf holds the last len(buf) bytes written; w is the next write index.
	buf []byte
	w   int
	// end is the total number of bytes ever written, i.e. the offset one past
	// the newest byte. It never wraps.
	end    int64
	wait   chan struct{}
	closed bool
}

// NewRing returns a ring holding at most size bytes. A size of zero keeps no
// bytes but still counts offsets, which is what a session wants when replay is
// switched off.
func NewRing(size int) *Ring {
	if size < 0 {
		size = 0
	}
	return &Ring{buf: make([]byte, size)}
}

// Write appends p. It never blocks and never fails: the oldest bytes are
// overwritten once the ring is full.
func (r *Ring) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	r.mu.Lock()
	if size := len(r.buf); size > 0 {
		src := p
		if len(src) > size {
			// Only the tail can survive; the rest is overwritten by this same
			// write anyway.
			src = src[len(src)-size:]
		}
		n := copy(r.buf[r.w:], src)
		if n < len(src) {
			copy(r.buf, src[n:])
		}
		r.w = (r.w + len(src)) % size
	}
	r.end += int64(len(p))
	r.notifyLocked()
	r.mu.Unlock()
	return len(p), nil
}

// End returns the offset one past the newest byte, i.e. the total number of
// bytes the session has produced.
func (r *Ring) End() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.end
}

// Start returns the offset of the oldest byte still held.
func (r *Ring) Start() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.startLocked()
}

func (r *Ring) startLocked() int64 {
	if buffered := int64(len(r.buf)); r.end > buffered {
		return r.end - buffered
	}
	return 0
}

// ReadFrom returns everything held from off onwards, the offset to ask for
// next, and whether bytes before the returned data were already gone.
//
// off may be ReplayAll for "whatever you still have", which is not a
// truncation. An off beyond End returns no data and is not an error either:
// the reader is simply up to date.
func (r *Ring) ReadFrom(off int64) (data []byte, next int64, truncated bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	start := r.startLocked()
	if off == ReplayAll || off < 0 {
		off = start
	} else if off < start {
		off = start
		truncated = true
	}
	n := int(r.end - off)
	if n <= 0 {
		return nil, r.end, truncated
	}

	out := make([]byte, n)
	size := len(r.buf)
	idx := (r.w - n + size) % size
	first := copy(out, r.buf[idx:])
	if first < n {
		copy(out[first:], r.buf[:n-first])
	}
	return out, r.end, truncated
}

// Wait returns a channel that is closed on the next write or on Close. It is
// re-armed per notification, so a reader takes it, drains what it can, and
// takes a fresh one.
func (r *Ring) Wait() <-chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		// A closed ring never notifies again; hand back a channel that is
		// already closed so a reader's select fires immediately instead of
		// blocking forever.
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	if r.wait == nil {
		r.wait = make(chan struct{})
	}
	return r.wait
}

// Close marks the stream finished and wakes every waiter. Data already in the
// ring stays readable: a pane whose process exited still shows its last words.
func (r *Ring) Close() {
	r.mu.Lock()
	if !r.closed {
		r.closed = true
		r.notifyLocked()
	}
	r.mu.Unlock()
}

// Closed reports whether the producer is done.
func (r *Ring) Closed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

func (r *Ring) notifyLocked() {
	if r.wait != nil {
		close(r.wait)
		r.wait = nil
	}
}
