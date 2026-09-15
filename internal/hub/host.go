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

	// Reserve allocates a session ID without starting anything. Use it when
	// the environment has to name the session before the session can exist;
	// pass the result back as CreateSpec.ID.
	Reserve() (int, error)

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

	// PlainText returns the screen as text, without escape sequences.
	PlainText(id int) (string, error)

	// PlainTextRows returns a row range of the screen as text. endRow may be
	// -1 for "to the bottom".
	PlainTextRows(id, startRow, endRow int) ([]string, error)

	// SetStatusline records what the agent's own status line reported.
	SetStatusline(id int, cost float64, contextPct int, model string) error

	// SetHookActivity records an agent state reported by a lifecycle hook.
	// Hook events are authoritative: once one has arrived for a session, the
	// screen-pattern detection no longer overrides it.
	SetHookActivity(id int, activity Activity) error

	// SetHookSessionID records the agent's own session ID as the hook reported
	// it, which is what resuming the conversation needs.
	SetHookSessionID(id int, agentSessionID string) error

	// ClearHookData forgets what the hooks reported, returning the session to
	// screen-pattern detection.
	ClearHookData(id int) error

	// Attach subscribes to a session's output starting at from, which may be
	// ReplayAll. The caller must Close the subscription.
	Attach(id int, from int64) (*Subscription, error)

	// Release lets go of this handle.
	//
	// What that costs depends on who owns the sessions, and the difference is
	// the point of this whole package: an Embedded owns them and therefore
	// ends them, while a Remote only drops the connection and leaves the
	// daemon's sessions running. A caller that means "end these agents" closes
	// them by ID; Release is for shutting down the handle.
	Release()
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

// NewSubscriptionForTest wraps a channel as a Subscription so a consumer can be
// driven with chunks a test wrote by hand, including the truncation case that
// is otherwise hard to provoke.
func NewSubscriptionForTest(ch <-chan Chunk) *Subscription {
	return &Subscription{C: ch}
}
