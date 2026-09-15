package hub

import "errors"

var (
	// ErrNoSession means the ID does not name a session this Host owns.
	ErrNoSession = errors.New("hub: no such session")
	// ErrClosed means the Host is shutting down and takes no new work.
	ErrClosed = errors.New("hub: host is closed")
)

// Host owns a set of terminal sessions. IDs are local to the Host.
//
// Every method is safe for concurrent use: a daemon serves several clients at
// once, and input from any of them goes into the same PTY.
type Host interface {
	// Info describes this Host.
	Info() Info

	// Create starts a session and returns its ID.
	Create(spec CreateSpec) (int, error)

	// Write sends raw bytes to a session's PTY.
	Write(id int, data []byte) error

	// Resize changes the PTY and screen dimensions.
	Resize(id, rows, cols int) error

	// Close terminates a session and forgets it. It blocks until the process
	// is gone, so that the output pump has drained first.
	Close(id int) error

	// List returns a summary of every session, ordered by ID.
	List() []SessionSummary

	// Get returns one session's summary.
	Get(id int) (SessionSummary, error)

	// Repaint renders the session's current screen as a self-contained ANSI
	// sequence. It is what a client applies when its stream has a hole.
	Repaint(id int) ([]byte, error)

	// Attach subscribes to a session's output starting at from, which may be
	// ReplayAll. The caller must Close the subscription.
	Attach(id int, from int64) (*Subscription, error)

	// Shutdown closes every session and releases the Host.
	Shutdown()
}

// Subscription delivers one session's output to one reader.
type Subscription struct {
	// C carries chunks in order. It is closed when the session's output ends
	// or the subscription is closed.
	C <-chan Chunk

	closeOnce func()
}

// Close stops the subscription. It is safe to call more than once.
func (s *Subscription) Close() {
	if s != nil && s.closeOnce != nil {
		s.closeOnce()
	}
}
