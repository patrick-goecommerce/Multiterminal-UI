package hub

import (
	"errors"
	"time"
)

var (
	// ErrNoSession means the ID does not name a session this Host owns.
	ErrNoSession = errors.New("hub: no such session")
	// ErrNoResumeID means the session has no agent conversation ID, so it
	// could be put to sleep but never woken up again.
	ErrNoResumeID = errors.New("hub: session has no resume id")
	// ErrNotIdle means the session is doing something. Only a finished pane
	// may be suspended; killing a working one would throw away the work.
	ErrNotIdle = errors.New("hub: session is not idle")
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

	// ScanActivity re-evaluates every session's agent state and returns what
	// it found. It is a call rather than a stream because the caller decides
	// how often to look, and because what follows from a change (debouncing,
	// advancing a queue, reporting progress) is the caller's business.
	ScanActivity() []ScanResult

	// ConfirmedActivity returns the session's debounced state and when it
	// began, zero values while it has never confirmed one.
	ConfirmedActivity(id int) (Activity, time.Time)

	// ForceActivity sets the confirmed state and its start together, for a
	// transition that does not come from the screen: a queue reset, a suspend,
	// a resume. Setting the state alone would leave the start pointing at the
	// previous one and leave an armed candidate that can confirm on a single
	// unrelated tick (#188 follow-up).
	ForceActivity(id int, state Activity, at time.Time) error

	// SeedActivity restores a state's start timestamp after a restart, so a
	// pane keeps its duration instead of starting over. The state is part of
	// it: the seed is honoured only if the pane confirms that same state
	// first, because a restored pane's CLI boot reads as "active" for longer
	// than a debounce window and would otherwise swallow it (#189).
	SeedActivity(id int, state Activity, at time.Time) error

	// ResetActivity puts the session back to idle. A caller that has just sent
	// it work uses this so the next "done" reads as a change rather than as
	// the previous item's leftover state.
	ResetActivity(id int) error

	// Suspend puts a finished session to sleep: its process tree is killed and
	// the session object stays, ready to be resumed. It returns as soon as the
	// suspend is armed, because the kill takes long enough that no caller
	// should wait on it; EventSessionSuspended reports the finish.
	Suspend(id int) error

	// Resume starts a fresh process into a sleeping session. argv, dir and env
	// are built by the caller exactly as for Create, because what a resumed
	// agent needs in its environment is the caller's policy.
	Resume(id int, argv []string, dir string, env []string) error

	// QueueAdd puts a prompt at the end of a session's queue. When nothing is
	// in flight it is sent at once rather than waiting for a transition that
	// has already happened.
	QueueAdd(id int, prompt string) (QueueItem, error)

	// QueueList returns a session's queue, never nil.
	QueueList(id int) []QueueItem

	// QueueRemove drops one item and reports whether it was there.
	//
	// An item already in flight is normally kept: the agent has it, so
	// removing the row would only hide what is happening. force overrides
	// that, for a caller that owns the item outright and is tearing down the
	// flow it belongs to.
	QueueRemove(id, itemID int, force bool) (bool, error)

	// QueueClear empties a session's queue, or only its finished items.
	QueueClear(id int, doneOnly bool) error

	// QueueAdvance marks the item in flight done and sends the next one. The
	// host calls it itself on every confirmed transition; a caller needs it
	// only to push a queue along for a reason of its own.
	QueueAdvance(id int)

	// Wake resumes a sleeping session, working out the command line and the
	// environment from what the host already knows. It is the counterpart to
	// Suspend and the reason a daemon can pick up a pane nobody is watching:
	// Resume needs a caller that remembers how the session was launched, and
	// with no window open there is no such caller.
	//
	// It returns nil for a session that is already awake, so two callers
	// racing a wake do not both have to care which one won.
	Wake(id int) error

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
	// C carries chunks in order. It is closed when the SESSION is closed, or
	// when the subscription is.
	//
	// Not when the session's process exits. A session outlives its process: a
	// suspend kills the process on purpose and a resume starts a new one into
	// the same session, so a channel that closed on exit would end every
	// subscription the first time a pane went to sleep. A reader that wants to
	// stop when the process ends watches EventSessionExited instead.
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
