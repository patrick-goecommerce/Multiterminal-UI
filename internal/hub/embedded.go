package hub

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

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
}

var _ Host = (*Embedded)(nil)

// Embedded is a Host that runs its sessions in the calling process.
type Embedded struct {
	mu       sync.Mutex
	sessions map[int]*managed
	nextID   int
	closed   bool

	hubID     string
	version   string
	ringBytes int
	sink      EventSink
	killTree  func(pid int)
	launcher  Launcher
	activity  *activityDebouncer
	queueMu   sync.Mutex
	queues    map[int]*sessionQueue
	startedAt time.Time

	// stop ends the background loops this host runs (the scan, the hook
	// reader). Closed exactly once, by Release.
	stop     chan struct{}
	stopOnce sync.Once

	shimPort     int
	shimListener net.Listener
}

type managed struct {
	sess      *terminal.Session
	ring      *Ring
	spec      CreateSpec
	startedAt time.Time
}

// NewEmbedded returns a Host owning sessions in this process.
func NewEmbedded(opts Options) *Embedded {
	ring := opts.RingBytes
	switch {
	case ring == 0:
		ring = DefaultRingBytes
	case ring < 0:
		ring = 0
	}
	id := opts.HubID
	if id == "" {
		id = newHubID()
	}
	h := &Embedded{
		sessions:  make(map[int]*managed),
		hubID:     id,
		version:   opts.Version,
		ringBytes: ring,
		sink:      opts.Sink,
		killTree:  opts.KillTree,
		launcher:  opts.Launcher,
		activity:  newActivityDebouncer(),
		queues:    make(map[int]*sessionQueue),
		startedAt: time.Now(),
		stop:      make(chan struct{}),
	}
	if opts.Scan {
		go h.scanLoop()
	}
	if opts.Shim {
		if err := h.startShim(); err != nil {
			// Not fatal: without it an agent loses its cost readout and the
			// tmux shim logs nowhere, but its pane runs.
			log.Printf("[hub] shim endpoints unavailable: %v", err)
		}
	}
	if opts.HooksDir != "" {
		h.startHookReader(opts.HooksDir)
	}
	return h
}

// newHubID returns a random identifier. It is not a secret and not a token:
// it only has to be different from other hubs a client might reach.
func newHubID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("pid%d", os.Getpid())
	}
	return hex.EncodeToString(b[:])
}

// HubID returns this host's identity.
func (h *Embedded) HubID() string { return h.hubID }

// Info implements Host.
func (h *Embedded) Info() Info {
	return Info{
		HubID:     h.hubID,
		ShimPort:  h.shimPort,
		Protocol:  Protocol,
		Version:   h.version,
		PID:       os.Getpid(),
		StartedAt: h.startedAt,
	}
}

// Reserve implements Host.
func (h *Embedded) Reserve() (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return 0, ErrClosed
	}
	h.nextID++
	return h.nextID, nil
}

// Write implements Host.
//
// A suspended session has no PTY and returns terminal.ErrSuspended. Waking it
// is not decided here: "typing into a sleeping pane means wake it" is a UI
// policy, and a scripted client may well prefer the error.
func (h *Embedded) Write(id int, data []byte) error {
	m, err := h.lookup(id)
	if err != nil {
		return err
	}
	_, err = m.sess.Write(data)
	return err
}

// Resize implements Host.
func (h *Embedded) Resize(id, rows, cols int) error {
	m, err := h.lookup(id)
	if err != nil {
		return err
	}
	m.sess.Resize(rows, cols)
	return nil
}

// Close implements Host. It blocks until the process is gone so that the
// output pump has drained and every subscriber saw the last bytes.
func (h *Embedded) Close(id int) error {
	m, err := h.lookup(id)
	if err != nil {
		return err
	}
	// Kill the tree before Close(): Close only ends the root process, and once
	// that is gone its descendants can no longer be reached from its PID.
	if h.killTree != nil {
		h.killTree(m.sess.Pid())
	}
	m.sess.Close()

	h.mu.Lock()
	delete(h.sessions, id)
	h.mu.Unlock()
	// The debounce state goes with the session. Without this every closed pane
	// leaves five map entries behind for the life of the process, and a reused
	// ID would inherit a stranger's confirmed state.
	h.activity.forget(id)
	h.forgetQueue(id)
	return nil
}

// List implements Host.
func (h *Embedded) List() []SessionSummary {
	h.mu.Lock()
	out := make([]SessionSummary, 0, len(h.sessions))
	for id, m := range h.sessions {
		out = append(out, summarize(id, m))
	}
	h.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Get implements Host.
func (h *Embedded) Get(id int) (SessionSummary, error) {
	m, err := h.lookup(id)
	if err != nil {
		return SessionSummary{}, err
	}
	return summarize(id, m), nil
}

// Repaint implements Host.
func (h *Embedded) Repaint(id int) ([]byte, error) {
	m, err := h.lookup(id)
	if err != nil {
		return nil, err
	}
	return []byte(m.sess.Screen.Repaint()), nil
}

// Release implements Host. An Embedded owns its sessions, so releasing it ends
// them. They are closed concurrently: each one waits for its process, and
// eight panes should not take eight timeouts in a row.
func (h *Embedded) Release() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	h.stopOnce.Do(func() { close(h.stop) })
	if h.shimListener != nil {
		_ = h.shimListener.Close()
	}
	ids := make([]int, 0, len(h.sessions))
	for id := range h.sessions {
		ids = append(ids, id)
	}
	h.mu.Unlock()

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = h.Close(id)
		}(id)
	}
	wg.Wait()
}

// AdoptForTest registers a session object the caller built itself, without
// starting a process. It exists for tests that need a session in a known state
// (a frozen screen, a given activity) and must not be used in production: a
// session adopted this way has no output pump and no exit watcher.
func (h *Embedded) AdoptForTest(id int, sess *terminal.Session) {
	m := &managed{
		sess:      sess,
		ring:      NewRing(h.ringBytes),
		spec:      CreateSpec{ID: id},
		startedAt: time.Now(),
	}
	h.mu.Lock()
	h.sessions[id] = m
	if id > h.nextID {
		h.nextID = id
	}
	h.mu.Unlock()
}

// sessions returns every session object, paired with its ID.
//
// It is unexported on purpose: handing out the session itself is the one thing
// a host in another process cannot do, so the only caller is this package's
// own activity scan, which runs where the screens are.
func (h *Embedded) sessionObjects() (ids []int, sessions []*terminal.Session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ids = make([]int, 0, len(h.sessions))
	sessions = make([]*terminal.Session, 0, len(h.sessions))
	for id, m := range h.sessions {
		ids = append(ids, id)
		sessions = append(sessions, m.sess)
	}
	return ids, sessions
}

func (h *Embedded) lookup(id int) (*managed, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	m := h.sessions[id]
	if m == nil {
		return nil, fmt.Errorf("%w: %d", ErrNoSession, id)
	}
	return m, nil
}

func (h *Embedded) emit(name string, payload any) {
	if h.sink != nil {
		h.sink.Emit(name, payload)
	}
}

func summarize(id int, m *managed) SessionSummary {
	contextPct, model, _ := m.sess.StatuslineInfo()
	return SessionSummary{
		ID:            id,
		Name:          m.sess.Name(),
		Dir:           m.spec.Dir,
		Mode:          m.spec.Mode,
		Status:        statusOf(m.sess),
		ExitCode:      m.sess.GetExitCode(),
		PID:           m.sess.Pid(),
		StartedAt:     m.startedAt,
		LastOutputAt:  m.sess.GetLastOutputAt(),
		Activity:      activityOf(m.sess.GetActivity()),
		Title:         m.sess.GetTitle(),
		Cost:          m.sess.GetTokens().TotalCost,
		ContextPct:    contextPct,
		Model:         model,
		ResumeID:      effectiveResumeID(m.sess),
		HookSessionID: m.sess.HookSessionID(),
		HasHookData:   m.sess.HasHookData(),
		Offset:        m.ring.End(),
	}
}

// effectiveResumeID is the ID to resume a session with: the hook-reported one
// wins, the one parsed out of argv at launch is the fallback.
func effectiveResumeID(s *terminal.Session) string {
	if id := s.HookSessionID(); id != "" {
		return id
	}
	return s.ResumeID()
}

// activityOf maps the terminal package's numeric state onto the wire strings.
// The numbers are an internal ordering; letting them cross a protocol boundary
// would make a reordering change meaning silently.
func activityOf(a terminal.ActivityState) Activity {
	switch a {
	case terminal.ActivityActive:
		return ActivityActive
	case terminal.ActivityDone:
		return ActivityDone
	case terminal.ActivityWaitingPermission:
		return ActivityWaitingPermission
	case terminal.ActivityWaitingAnswer:
		return ActivityWaitingAnswer
	case terminal.ActivityError:
		return ActivityError
	default:
		return ActivityIdle
	}
}

func statusOf(s *terminal.Session) Status {
	switch s.GetStatus() {
	case terminal.StatusExited:
		return StatusExited
	case terminal.StatusError:
		return StatusError
	case terminal.StatusSuspending:
		return StatusSuspending
	case terminal.StatusSuspended:
		return StatusSuspended
	default:
		return StatusRunning
	}
}
