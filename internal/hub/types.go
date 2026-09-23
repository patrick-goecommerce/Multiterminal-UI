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
	// Launch asks the host to work out Argv and Env itself, from the tool
	// name. It is how a caller with no access to the configuration (the CLI,
	// an agent over MCP, anything on the far side of the socket) starts a
	// session at all: it names what it wants, not how to run it.
	//
	// When it is set, Argv and Env are ignored. A host without a Launcher
	// rejects it rather than starting something half-configured: a pane
	// launched without MULTITERMINAL_SESSION_ID looks fine and silently has
	// no hook wiring.
	Launch *LaunchRequest `json:"launch,omitempty" yaml:"launch,omitempty"`
	// Origin says who asked for this session. The host does nothing with it;
	// it is carried so a client can tell a session it started itself from one
	// something else started, which over a socket it has no other way of
	// knowing. Empty means the UI, OriginAgent means another agent.
	Origin string `json:"origin,omitempty" yaml:"origin,omitempty"`
}

// Origins. An empty origin is a window's own UI, and every other value means
// some other client asked for the session: a window that sees one draws a pane
// for it, because nobody else is going to.
const (
	// OriginAgent is another agent, through the MCP server. The idle-suspend
	// gate also keys off this one: the agent that opened the session is about
	// to write to it.
	OriginAgent = "agent"
	// OriginCLI is a person at a terminal, through mt new.
	OriginCLI = "cli"
)

// LaunchRequest names an agent to start, in the caller's terms.
type LaunchRequest struct {
	// Tool is the agent CLI: "claude", "codex", "gemini".
	Tool string `json:"tool" yaml:"tool"`
	// Model is passed to the CLI when set, otherwise its own default applies.
	Model string `json:"model,omitempty" yaml:"model,omitempty"`
}

// Launcher turns a launch request into a command and an environment.
//
// It is an interface rather than a concrete type because deciding what a
// session's environment contains is policy, and this package owns sessions,
// not policy. internal/launch implements it; a host that is handed one can
// start an agent by name, and a host without one cannot. That is the whole
// difference between a daemon an agent can delegate to and a daemon that can
// only hold what a window gave it.
type Launcher interface {
	// Argv builds the command line for a tool, optionally pinning a model.
	Argv(tool, model string) ([]string, error)
	// Env builds the PTY environment for a session that has already been
	// assigned an ID.
	Env(sessionID int, dir, mode string) []string
	// ResumeArgv rewrites a launch command line into one that continues the
	// agent's existing conversation. It is part of this interface rather than
	// something the host does with a string append, because --session-id and
	// --resume are mutually exclusive and a command line carrying both is
	// rejected by the CLI.
	ResumeArgv(argv []string, resumeID string) []string
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
	// ActivitySleeping and ActivityResuming are lifecycle labels, not screen
	// classifications: a suspended pane has a frozen screen and nothing to
	// classify. A scan never produces them. A caller writes one with
	// ForceActivity so the badge shows it and so the next real state reads as
	// a change instead of re-confirming what the pane was doing before it
	// fell asleep.
	ActivitySleeping Activity = "sleeping"
	ActivityResuming Activity = "resuming"
)

// SessionSummary is what a client learns about a session without attaching.
//
// It is deliberately everything a caller might want short of the screen
// itself: every field here is a field nobody has to hold a *terminal.Session
// to read, and holding one is what a remote client cannot do.
type SessionSummary struct {
	ID   int    `json:"id" yaml:"id"`
	Name string `json:"name" yaml:"name"`
	// AgentName is the name the agent gave the session itself, empty until
	// it reported one. Name falls back to the terminal title.
	AgentName    string    `json:"agent_name" yaml:"agent_name"`
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
	// ContextPct comes from the agent's own status line when it reports one,
	// and is zero otherwise.
	ContextPct int `json:"context_pct" yaml:"context_pct"`
	// Model is what the agent's status line reports, falling back to the model
	// the session was launched with. The fallback matters because the status
	// line arrives seconds after the session does, and a caller that asked for
	// a specific model should not be told "none" in between.
	Model string `json:"model" yaml:"model"`
	// Origin is CreateSpec.Origin, handed back so a client can recognise the
	// sessions it did not start.
	Origin string `json:"origin" yaml:"origin"`
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
	Asleep bool `json:"asleep" yaml:"asleep"`
	// Activity is the CONFIRMED state, not this tick's raw observation: a
	// state has to hold for a debounce window before it is reported as the
	// session's. Until the first confirmation it is the raw reading, because
	// an empty activity is outside the documented set and a caller has to be
	// able to show something.
	Activity Activity `json:"activity" yaml:"activity"`
	// Changed reports that Activity is a confirmed transition on this tick.
	// Everything a caller does about a change (emit an event, report progress,
	// advance a queue) keys off this and not off comparing Activity itself,
	// which would react to a repaint (#188).
	Changed bool `json:"changed" yaml:"changed"`
	// Since is when the confirmed state began, zero while unknown. It is the
	// first observation of that state, not the moment it survived the window,
	// or every duration would be short by up to one window.
	Since      time.Time `json:"since" yaml:"since"`
	Cost       float64   `json:"cost" yaml:"cost"`
	Title      string    `json:"title" yaml:"title"`
	ContextPct int       `json:"context_pct" yaml:"context_pct"`
	Model      string    `json:"model" yaml:"model"`
	// Name is the agent's own name for the session (Claude Code's
	// session_name), empty until it reports one. Title is the terminal title
	// with any spinner glyph in front removed.
	Name string `json:"name" yaml:"name"`
}

// Info describes a Host to a client that just connected.
type Info struct {
	HubID string `json:"hub_id" yaml:"hub_id"`
	// ShimPort is where MTUI's helper binaries post (MTUI_PORT in a session's
	// environment), or 0 when this host serves no shim endpoints. It has to
	// come from the host because the value is baked into a session at launch
	// and outlives any one window.
	ShimPort  int       `json:"shim_port" yaml:"shim_port"`
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
