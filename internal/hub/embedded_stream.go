package hub

import (
	"sync"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// pump moves raw PTY output into the session's ring.
//
// RawOutputCh is closed exactly once, when the session is closed for good; a
// suspend/resume cycle leaves it open (see Session.closeRawOutput). Ranging
// over it is therefore the session's true end of output.
func (h *Embedded) pump(m *managed) {
	for data := range m.sess.RawOutputCh {
		_, _ = m.ring.Write(data)
	}
	m.ring.Close()
}

// watchExit reports a session's process exit.
//
// A suspend kills the process on purpose (#180). Reporting that as an exit
// would tell every client a merely sleeping pane is dead, so the exit of a
// suspended generation is swallowed and the watcher re-arms on the next one.
func (h *Embedded) watchExit(id int, sess *terminal.Session) {
	for {
		<-sess.Done()
		if !sess.IsSuspendedOrSuspending() {
			break
		}
		if !sess.WaitAwake() {
			return // closed for good while asleep, nothing to report
		}
	}
	h.emit(EventSessionExited, SessionExited{ID: id, ExitCode: sess.GetExitCode()})
}

// Attach implements Host.
func (h *Embedded) Attach(id int, from int64) (*Subscription, error) {
	m, err := h.lookup(id)
	if err != nil {
		return nil, err
	}
	ch := make(chan Chunk, 8)
	done := make(chan struct{})
	var once sync.Once
	go feed(m.ring, from, ch, done)
	return &Subscription{
		C:         ch,
		closeOnce: func() { once.Do(func() { close(done) }) },
	}, nil
}

// feed copies a ring's contents to one subscriber, starting at from.
//
// The wait channel is taken before reading, not after: a write landing between
// the read and the wait would otherwise go unnoticed until the next one, and
// for a pane that just printed its last line there is no next one.
//
// Sending blocks when the subscriber is slow. That is deliberate: the producer
// is the ring, which never blocks, so a slow client only delays itself and
// eventually reads a truncated range instead of stalling the PTY.
func feed(ring *Ring, from int64, ch chan<- Chunk, done <-chan struct{}) {
	defer close(ch)
	off := from
	for {
		wait := ring.Wait()
		data, next, truncated := ring.ReadFrom(off)
		if len(data) > 0 {
			select {
			case ch <- Chunk{Offset: next - int64(len(data)), Data: data, Truncated: truncated}:
			case <-done:
				return
			}
			off = next
			continue
		}
		if ring.Closed() {
			return
		}
		select {
		case <-wait:
		case <-done:
			return
		}
	}
}
