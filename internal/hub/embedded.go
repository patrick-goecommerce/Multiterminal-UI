package hub

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
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
	// KillTree ends a process subtree before the session is closed. It is
	// injected rather than implemented here because it is platform code that
	// lives in the backend (killProcessTree); the hub must not grow a second
	// copy of it. Nil skips the step, which leaves the same orphans the
	// ordinary close path used to leave (#185).
	KillTree func(pid int)
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
	startedAt time.Time
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
	return &Embedded{
		sessions:  make(map[int]*managed),
		hubID:     id,
		version:   opts.Version,
		ringBytes: ring,
		sink:      opts.Sink,
		killTree:  opts.KillTree,
		startedAt: time.Now(),
	}
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
		Protocol:  Protocol,
		Version:   h.version,
		PID:       os.Getpid(),
		StartedAt: h.startedAt,
	}
}

// Create implements Host.
func (h *Embedded) Create(spec CreateSpec) (int, error) {
	if spec.Rows < 5 {
		spec.Rows = 24
	}
	if spec.Cols < 20 {
		spec.Cols = 80
	}
	if spec.Dir == "" {
		spec.Dir, _ = os.Getwd()
	}

	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return 0, ErrClosed
	}
	h.nextID++
	id := h.nextID
	h.mu.Unlock()

	sess := terminal.NewSession(id, spec.Rows, spec.Cols)
	if err := sess.Start(spec.Argv, spec.Dir, spec.Env); err != nil {
		return 0, fmt.Errorf("hub: start session %d: %w", id, err)
	}
	m := &managed{
		sess:      sess,
		ring:      NewRing(h.ringBytes),
		spec:      spec,
		startedAt: time.Now(),
	}

	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		sess.Close()
		return 0, ErrClosed
	}
	h.sessions[id] = m
	h.mu.Unlock()

	// Report the session before starting its watchers. A process that exits
	// at once (a typo in argv, a one-line script) would otherwise race its own
	// creation, and a client that receives an exit for a session it has never
	// heard of has no way to make sense of it. Output is not at risk from the
	// later start: RawOutputCh buffers, and nothing is read from it elsewhere.
	h.emit(EventSessionCreated, SessionCreated{Session: summarize(id, m)})

	go h.pump(m)
	go h.watchExit(id, sess)
	return id, nil
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
	return SessionSummary{
		ID:           id,
		Name:         m.sess.Name(),
		Dir:          m.spec.Dir,
		Mode:         m.spec.Mode,
		Status:       statusOf(m.sess),
		ExitCode:     m.sess.GetExitCode(),
		PID:          m.sess.Pid(),
		StartedAt:    m.startedAt,
		LastOutputAt: m.sess.GetLastOutputAt(),
		Offset:       m.ring.End(),
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
