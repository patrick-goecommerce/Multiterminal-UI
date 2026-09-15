package hub

import "time"

// Protocol is the wire version a client and a daemon must agree on. It is
// bumped whenever a change would make an older client misread a newer daemon.
// Mismatching peers do not talk: reconnecting is cheap, guessing is not.
const Protocol = 1

// DefaultRingBytes is how much raw PTY output a session keeps for replay.
// It is the only thing standing between a reattach and an empty pane, so it is
// sized for "the last screenful and then some" rather than for a full log.
const DefaultRingBytes = 2 << 20 // 2 MiB

// ReplayAll asks Attach for everything the ring still holds. It is not a
// truncation: the client asked for whatever is there, and got it.
const ReplayAll int64 = -1

// Status is the lifecycle state of a session, as seen from outside the
// process that owns it. It mirrors terminal.SessionStatus but is a string on
// purpose: the numeric constants are an internal ordering that must not leak
// into a wire format where a reordering would silently change meaning.
type Status string

const (
	StatusRunning    Status = "running"
	StatusExited     Status = "exited"
	StatusError      Status = "error"
	StatusSuspending Status = "suspending"
	StatusSuspended  Status = "suspended"
)

// Ref addresses one session across hubs. Hosts speak in plain IDs; the hub
// half is filled in at the wire boundary (see package doc).
type Ref struct {
	Hub string `json:"hub" yaml:"hub"`
	ID  int    `json:"id" yaml:"id"`
}

// CreateSpec is everything needed to start a session. Env is passed in rather
// than assembled here: which variables a Claude pane needs (hook wiring,
// worktree firewall, session ID) is policy that belongs to the caller, and the
// hub must not grow a second opinion about it.
type CreateSpec struct {
	// ID is a previously reserved session ID, or zero to allocate one now.
	//
	// It exists because part of the environment names the session it belongs
	// to (MULTITERMINAL_SESSION_ID, which the hook and the statusline shim
	// report back with), so the caller has to know the ID before it can build
	// Env. A callback would solve that locally and not at all over the wire,
	// hence Reserve plus this field.
	ID int `json:"id,omitempty" yaml:"id,omitempty"`
	// ResumeID is the agent's own conversation ID, when the caller already
	// knows it from argv. Setting it here rather than after the session starts
	// keeps it from racing the first hook event.
	ResumeID string   `json:"resume_id,omitempty" yaml:"resume_id,omitempty"`
	Argv     []string `json:"argv" yaml:"argv"`
	Dir      string   `json:"dir" yaml:"dir"`
	Rows     int      `json:"rows" yaml:"rows"`
	Cols     int      `json:"cols" yaml:"cols"`
	Mode     string   `json:"mode" yaml:"mode"`
	Env      []string `json:"env" yaml:"env"`
}

// Activity is what a session is doing, as the agent-state detection sees it.
// The strings match what the frontend already listens for, so a summary can be
// handed to the UI without a second vocabulary in between.
type Activity string

const (
	ActivityIdle              Activity = "idle"
	ActivityActive            Activity = "active"
	ActivityDone              Activity = "done"
	ActivityWaitingPermission Activity = "waitingPermission"
	ActivityWaitingAnswer     Activity = "waitingAnswer"
	ActivityError             Activity = "error"
)

// SessionSummary is what a client learns about a session without attaching.
//
// It is deliberately everything a caller might want short of the screen
// itself: every field here is a field nobody has to hold a *terminal.Session
// to read, and holding one is what a remote client cannot do.
type SessionSummary struct {
	ID           int       `json:"id" yaml:"id"`
	Name         string    `json:"name" yaml:"name"`
	Dir          string    `json:"dir" yaml:"dir"`
	Mode         string    `json:"mode" yaml:"mode"`
	Status       Status    `json:"status" yaml:"status"`
	ExitCode     int       `json:"exit_code" yaml:"exit_code"`
	PID          int       `json:"pid" yaml:"pid"`
	StartedAt    time.Time `json:"started_at" yaml:"started_at"`
	LastOutputAt time.Time `json:"last_output_at" yaml:"last_output_at"`
	// Activity and the fields under it are the agent's state, not the
	// process's: a shell pane simply stays idle.
	Activity Activity `json:"activity" yaml:"activity"`
	Title    string   `json:"title" yaml:"title"`
	Cost     float64  `json:"cost" yaml:"cost"`
	// ContextPct and Model come from the agent's own status line when it
	// reports one; both are zero-valued otherwise.
	ContextPct int    `json:"context_pct" yaml:"context_pct"`
	Model      string `json:"model" yaml:"model"`
	// ResumeID is the agent's own conversation ID, for waking it up again.
	// It is the effective one: what a lifecycle hook reported wins over what
	// was parsed out of argv at launch, because the agent may have picked a
	// different ID internally.
	ResumeID string `json:"resume_id" yaml:"resume_id"`
	// HookSessionID is the agent's own session ID exactly as a hook reported
	// it, empty until one has. ResumeID prefers it; this is the raw value,
	// which is what "have we heard from this session yet" asks about.
	HookSessionID string `json:"hook_session_id" yaml:"hook_session_id"`
	// HasHookData reports whether lifecycle hooks have been heard from. Until
	// they have, the session's state is guesswork from screen patterns, which
	// is not a good enough basis for putting a pane to sleep.
	HasHookData bool `json:"has_hook_data" yaml:"has_hook_data"`
	// Offset is how many bytes this session has produced since it started.
	// A client that reconnects passes the last offset it applied back to
	// Attach and continues without a hole.
	Offset int64 `json:"offset" yaml:"offset"`
}

// Asleep reports whether the session is suspended, or on its way there. A
// sleeping pane has no process, so writing to it fails and waking it is the
// only way to reach it.
func (s SessionSummary) Asleep() bool {
	return s.Status == StatusSuspended || s.Status == StatusSuspending
}

// ScanResult is one session's agent state as of a scan tick.
type ScanResult struct {
	ID int `json:"id" yaml:"id"`
	// Asleep marks a suspended pane: its screen is frozen, so there is nothing
	// to classify and the other fields are not filled in.
	Asleep     bool     `json:"asleep" yaml:"asleep"`
	Activity   Activity `json:"activity" yaml:"activity"`
	Cost       float64  `json:"cost" yaml:"cost"`
	Title      string   `json:"title" yaml:"title"`
	ContextPct int      `json:"context_pct" yaml:"context_pct"`
	Model      string   `json:"model" yaml:"model"`
}

// Info describes a Host to a client that just connected.
type Info struct {
	HubID     string    `json:"hub_id" yaml:"hub_id"`
	Protocol  int       `json:"protocol" yaml:"protocol"`
	Version   string    `json:"version" yaml:"version"`
	PID       int       `json:"pid" yaml:"pid"`
	StartedAt time.Time `json:"started_at" yaml:"started_at"`
}

// Chunk is one run of terminal output.
type Chunk struct {
	// Offset is the position of the first byte of Data in the session's
	// output stream.
	Offset int64  `json:"offset" yaml:"offset"`
	Data   []byte `json:"data" yaml:"data"`
	// Truncated reports that bytes before Offset were dropped from the ring
	// before this client could read them. The stream has a hole, so the
	// receiver must repaint rather than append: a VT100 stream missing a run
	// of bytes stays garbled for good (#157).
	Truncated bool `json:"truncated" yaml:"truncated"`
}

// Event names emitted by a Host. They are deliberately not the Wails event
// names: a client translates them into whatever its UI listens for.
const (
	EventSessionCreated   = "session.created"
	EventSessionExited    = "session.exited"
	EventSessionSuspended = "session.suspended"
	EventSessionResumed   = "session.resumed"
)

// SessionCreated is the payload of EventSessionCreated.
type SessionCreated struct {
	Session SessionSummary `json:"session" yaml:"session"`
}

// SessionExited is the payload of EventSessionExited.
type SessionExited struct {
	ID       int `json:"id" yaml:"id"`
	ExitCode int `json:"exit_code" yaml:"exit_code"`
}

// SessionSuspended is the payload of EventSessionSuspended: the pane's process
// tree is gone on purpose and the session object is waiting to be resumed.
type SessionSuspended struct {
	ID       int    `json:"id" yaml:"id"`
	ResumeID string `json:"resume_id" yaml:"resume_id"`
}

// SessionResumed is the payload of EventSessionResumed.
type SessionResumed struct {
	ID       int    `json:"id" yaml:"id"`
	ResumeID string `json:"resume_id" yaml:"resume_id"`
}

// EventSink receives Host events. The daemon's sink fans them out to connected
// clients; the GUI's sink re-emits them as Wails events so the frontend keeps
// listening for exactly what it listens for today.
type EventSink interface {
	Emit(name string, payload any)
}

// SinkFunc adapts a function to EventSink.
type SinkFunc func(name string, payload any)

// Emit implements EventSink.
func (f SinkFunc) Emit(name string, payload any) { f(name, payload) }
