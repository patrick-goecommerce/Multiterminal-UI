package hub

// How a host is configured.
//
// Split out of embedded.go for the file-size rule. Every field here answers
// the same question: which of the jobs that belong to whoever owns the
// sessions does this particular host do? A daemon says yes to all of them; a
// window says yes only while the sessions are its own; a test says no to all
// of them and drives the host by hand.

// Options configures an Embedded host.
type Options struct {
	// HubID identifies this host to clients. Pass a persisted value so a
	// restarted daemon keeps its identity; empty generates a fresh one.
	HubID string
	// Version is reported to clients, for the protocol/version check.
	Version string
	// RingBytes is the per-session replay buffer. Zero means DefaultRingBytes;
	// a negative value switches replay off while still counting offsets.
	RingBytes int
	// Sink receives session events. Nil discards them.
	Sink EventSink
	// Scan turns on the host's own activity scan. It has to be on wherever
	// the sessions are: a host whose client has gone away still has agents
	// working, and their state has to keep being written down or the next
	// client finds yesterday's picture. Off by default so a caller that
	// drives the scan itself (or a test) is not surprised by a ticker.
	Scan bool
	// Shim turns on the loopback endpoints MTUI's helper binaries post to.
	// It belongs wherever the sessions are, because a session's environment
	// names that port for as long as the session lives.
	Shim bool
	// HooksDir turns on the lifecycle-hook reader over that directory. Like
	// Scan, it belongs wherever the sessions are: the hook events are how an
	// agent says what it is doing, and they keep arriving while no client is
	// connected. Empty leaves the reader off.
	HooksDir string
	// KillTree ends a process subtree before the session is closed. It is
	// injected rather than implemented here because it is platform code that
	// lives in the backend (killProcessTree); the hub must not grow a second
	// copy of it. Nil skips the step, which leaves the same orphans the
	// ordinary close path used to leave (#185).
	KillTree func(pid int)
	// Launcher lets this host start an agent from a tool name, working out
	// argv and environment itself. Without one, only a caller that already
	// knows both can create a session, which is what kept every client but
	// the window from starting anything. See CreateSpec.Launch.
	Launcher Launcher
	// KeepAlive supplies the keep-alive policy. It is a function rather than a
	// value because the policy comes from the config and the user can change
	// it while the host runs; it is read per tick. Nil leaves the keep-alive
	// off, and so does a policy that returns a zero interval or no message.
	KeepAlive func() KeepAlive
}
