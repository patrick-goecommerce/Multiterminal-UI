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

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

var _ Host = (*Embedded)(nil)

// Embedded is a Host that runs its sessions in the calling process.
type Embedded struct {
	mu       sync.Mutex
	sessions map[int]*managed
	nextID   int
	closed   bool

	hubID       string
	version     string
	ringBytes   int
	sink        EventSink
	killTree    func(pid int)
	launcher    Launcher
	activity    *activityDebouncer
	queueMu     sync.Mutex
	queues      map[int]*sessionQueue
	keepAliveFn func() KeepAlive
	startedAt   time.Time
	hookWatcher *hooks.Watcher // nil without a hooks directory

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
		sessions:    make(map[int]*managed),
		hubID:       id,
		version:     opts.Version,
		ringBytes:   ring,
		sink:        opts.Sink,
		killTree:    opts.KillTree,
		launcher:    opts.Launcher,
		activity:    newActivityDebouncer(),
		queues:      make(map[int]*sessionQueue),
		keepAliveFn: opts.KeepAlive,
		startedAt:   time.Now(),
		stop:        make(chan struct{}),
	}
	if opts.Scan {
		go h.scanLoop()
	}
	if opts.KeepAlive != nil {
		go h.keepAliveLoop()
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
	// Its hook file goes with it. Only a SessionEnd event removed one so far,
	// and a pane that is closed rather than exited never writes one, so every
	// closed pane left its file behind in a directory the reader lists several
	// times a second.
	if w, agent := h.hookWatcher, m.sess.HookSessionID(); w != nil && agent != "" {
		w.Forget(agent)
	}

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
