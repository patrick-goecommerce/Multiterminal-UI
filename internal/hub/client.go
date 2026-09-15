package hub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ErrProtocol means the daemon speaks a version this client does not.
var ErrProtocol = errors.New("hub: protocol mismatch")

// Remote is a Host backed by a daemon reached over loopback.
//
// It holds no session state of its own. Everything it knows it asked the
// daemon for, which is what keeps the two implementations from drifting: the
// logic lives in Embedded, and this is transport.
type Remote struct {
	base   string
	token  string
	http   *http.Client
	info   Info
	sink   EventSink
	cancel context.CancelFunc
	ctx    context.Context

	mu     sync.Mutex
	conn   connWriter
	subs   map[int]*remoteSub
	closed bool
	// ready is closed once the socket has been up at least once, so callers
	// that attach immediately after Dial do not race the first connection.
	ready     chan struct{}
	readyOnce sync.Once
}

// connWriter is the part of a live socket the rest of the client needs. It is
// an interface so the reconnect loop can swap it out under the lock.
type connWriter interface {
	sendControl(msg control) error
	sendBinary(frame []byte) error
}

// DialOptions configures a Remote.
type DialOptions struct {
	// Sink receives the daemon's events. Nil discards them.
	Sink EventSink
	// Timeout bounds a control request. Zero means 10 seconds.
	Timeout time.Duration
}

// Dial connects to a daemon at addr (host:port) using the discovery token.
//
// It checks the protocol version before returning: a client that talks to a
// daemon it does not understand would misread sessions, and the fix is a
// daemon restart, which the caller must decide on because it ends the agents.
func Dial(addr, token string, opts DialOptions) (*Remote, error) {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &Remote{
		base:   "http://" + addr,
		token:  token,
		http:   &http.Client{Timeout: timeout},
		sink:   opts.Sink,
		ctx:    ctx,
		cancel: cancel,
		subs:   make(map[int]*remoteSub),
		ready:  make(chan struct{}),
	}

	var info Info
	if err := r.call(http.MethodGet, "/v1/hub", nil, &info); err != nil {
		cancel()
		return nil, err
	}
	if info.Protocol != Protocol {
		cancel()
		return nil, fmt.Errorf("%w: daemon speaks %d, this build speaks %d",
			ErrProtocol, info.Protocol, Protocol)
	}
	r.info = info

	go r.run()
	return r, nil
}

// Info implements Host. It is the description fetched when the connection was
// established, not a fresh request.
func (r *Remote) Info() Info { return r.info }

// WaitReady blocks until the output socket is up or the deadline passes. It is
// for callers that attach right after Dial; the client works without it, but
// the first attach would then queue until the socket connects.
func (r *Remote) WaitReady(timeout time.Duration) bool {
	select {
	case <-r.ready:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Create implements Host.
func (r *Remote) Create(spec CreateSpec) (int, error) {
	var out struct {
		ID int `json:"id"`
	}
	if err := r.call(http.MethodPost, "/v1/sessions", spec, &out); err != nil {
		return 0, err
	}
	return out.ID, nil
}

// Write implements Host. Keystrokes go over the socket when it is up, because
// a round trip per keypress is the one thing a terminal cannot afford. The
// HTTP path is the fallback while reconnecting.
func (r *Remote) Write(id int, data []byte) error {
	r.mu.Lock()
	conn := r.conn
	r.mu.Unlock()
	if conn != nil {
		if err := conn.sendBinary(encodeFrame(frameInput, 0, id, 0, data)); err == nil {
			return nil
		}
	}
	body := struct {
		Data []byte `json:"data"`
	}{Data: data}
	return r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/input", id), body, nil)
}

// Resize implements Host.
func (r *Remote) Resize(id, rows, cols int) error {
	body := struct {
		Rows int `json:"rows"`
		Cols int `json:"cols"`
	}{rows, cols}
	return r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/resize", id), body, nil)
}

// Close implements Host: it ends the session on the daemon.
func (r *Remote) Close(id int) error {
	return r.call(http.MethodDelete, fmt.Sprintf("/v1/sessions/%d", id), nil, nil)
}

// List implements Host. A transport failure returns no sessions rather than an
// error, because Host.List has no error to return; callers that need to tell
// "none" from "could not ask" use Get or Info.
func (r *Remote) List() []SessionSummary {
	var out struct {
		Sessions []SessionSummary `json:"sessions"`
	}
	if err := r.call(http.MethodGet, "/v1/sessions", nil, &out); err != nil {
		return nil
	}
	return out.Sessions
}

// Get implements Host.
func (r *Remote) Get(id int) (SessionSummary, error) {
	var out SessionSummary
	err := r.call(http.MethodGet, fmt.Sprintf("/v1/sessions/%d", id), nil, &out)
	return out, err
}

// Repaint implements Host.
func (r *Remote) Repaint(id int) ([]byte, error) {
	req, err := r.request(http.MethodGet, fmt.Sprintf("/v1/sessions/%d/repaint", id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp)
	}
	return io.ReadAll(resp.Body)
}

// Release implements Host: it drops the connection and leaves every session
// running on the daemon. That asymmetry with Embedded.Release is the point of
// the daemon.
func (r *Remote) Release() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	subs := r.subs
	r.subs = make(map[int]*remoteSub)
	r.mu.Unlock()

	r.cancel()
	for _, sub := range subs {
		sub.close()
	}
}

func (r *Remote) request(method, path string, body any) (*http.Request, error) {
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(r.ctx, method, r.base+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// call performs a control request and decodes the response into out (which may
// be nil).
func (r *Remote) call(method, path string, body, out any) error {
	req, err := r.request(method, path, body)
	if err != nil {
		return err
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w (%s)", ErrNoSession, path)
	}
	if resp.StatusCode == http.StatusServiceUnavailable {
		return ErrClosed
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusError(resp)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func statusError(resp *http.Response) error {
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("hub: %s %s: %s: %s",
		resp.Request.Method, resp.Request.URL.Path, resp.Status, bytes.TrimSpace(msg))
}

// streamURL is the WebSocket endpoint derived from the control base.
func (r *Remote) streamURL() string {
	u, err := url.Parse(r.base + "/v1/stream")
	if err != nil {
		return r.base + "/v1/stream"
	}
	u.Scheme = "ws"
	return u.String()
}

var _ Host = (*Remote)(nil)
