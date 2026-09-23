package hub

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// reconnect backoff bounds. A daemon restart or a dropped socket must not turn
// into a tight dial loop, and it must not take a minute to come back either.
const (
	reconnectMin = 250 * time.Millisecond
	reconnectMax = 5 * time.Second
)

// subQueue is how many chunks a subscription buffers before the socket reader
// waits for its consumer. A slow pane then delays itself and, once the buffer
// is full, the whole connection; the alternative is dropping chunks, and a
// dropped chunk is a hole the consumer cannot even see. The GUI's consumer
// hands bytes straight to xterm.js, so the buffer exists to absorb bursts, not
// to paper over a stalled reader.
const subQueue = 64

// remoteSub is one attached session on the client side.
type remoteSub struct {
	id int
	ch chan Chunk

	mu     sync.Mutex
	offset int64 // next byte expected; what a reconnect resumes from
	closed bool
}

func (s *remoteSub) resumeFrom() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.offset
}

// push hands a chunk to the consumer, flagging a gap the sender did not
// already report. Offsets are checked as well as trusted: the ring knows what
// it dropped, but a reconnect can skip bytes the ring never saw the reader
// miss, and appending across either kind of gap garbles the pane for good.
func (s *remoteSub) push(ctx context.Context, offset int64, truncated bool, data []byte) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	if s.offset >= 0 && offset > s.offset {
		truncated = true
	}
	s.offset = offset + int64(len(data))
	ch := s.ch
	s.mu.Unlock()

	select {
	case ch <- Chunk{Offset: offset, Data: data, Truncated: truncated}:
	case <-ctx.Done():
	}
}

func (s *remoteSub) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.ch)
}

// Attach implements Host. The subscription survives a reconnect: the client
// re-attaches at the offset it last delivered, so a dropped socket costs a
// round trip rather than the pane's history.
func (r *Remote) Attach(id int, from int64) (*Subscription, error) {
	sub := &remoteSub{id: id, ch: make(chan Chunk, subQueue), offset: from}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, ErrClosed
	}
	if old := r.subs[id]; old != nil {
		old.close()
	}
	r.subs[id] = sub
	conn := r.conn
	r.mu.Unlock()

	if conn != nil {
		_ = conn.sendControl(control{Op: opAttach, ID: id, From: from})
	}

	var once sync.Once
	return &Subscription{
		C: sub.ch,
		closeOnce: func() {
			once.Do(func() {
				r.mu.Lock()
				if r.subs[id] == sub {
					delete(r.subs, id)
				}
				conn := r.conn
				r.mu.Unlock()
				if conn != nil {
					_ = conn.sendControl(control{Op: opDetach, ID: id})
				}
				sub.close()
			})
		},
	}, nil
}

// run keeps the output socket up for the life of the client.
func (r *Remote) run() {
	backoff := reconnectMin
	for {
		if err := r.serve(); err == nil {
			backoff = reconnectMin
		}
		select {
		case <-r.ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff *= 2; backoff > reconnectMax {
			backoff = reconnectMax
		}
	}
}

// serve holds one connection until it fails.
func (r *Remote) serve() error {
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, r.streamURL(), &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{"Bearer " + r.token}},
	})
	if err != nil {
		return err
	}
	conn.SetReadLimit(streamReadLimit)
	defer conn.Close(websocket.StatusNormalClosure, "")

	w := &wsConn{conn: conn, ctx: ctx}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.conn = w
	pending := make([]*remoteSub, 0, len(r.subs))
	for _, sub := range r.subs {
		pending = append(pending, sub)
	}
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		if r.conn == w {
			r.conn = nil
		}
		r.mu.Unlock()
	}()

	// Re-attach whatever was attached before the socket went away.
	for _, sub := range pending {
		if err := w.sendControl(control{Op: opAttach, ID: sub.id, From: sub.resumeFrom()}); err != nil {
			return err
		}
	}
	r.readyOnce.Do(func() { close(r.ready) })

	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		switch typ {
		case websocket.MessageBinary:
			r.onBinary(ctx, data)
		case websocket.MessageText:
			var msg control
			if json.Unmarshal(data, &msg) == nil {
				r.onControl(msg)
			}
		}
	}
}

func (r *Remote) onBinary(ctx context.Context, data []byte) {
	typ, flags, id, offset, payload, err := decodeFrame(data)
	if err != nil || typ != frameOutput {
		return
	}
	r.mu.Lock()
	sub := r.subs[id]
	r.mu.Unlock()
	if sub == nil {
		return
	}
	// payload aliases the read buffer, which the next Read reuses.
	sub.push(ctx, offset, flags&flagTruncated != 0, append([]byte(nil), payload...))
}

func (r *Remote) onControl(msg control) {
	switch msg.Op {
	case opEvent:
		if r.sink != nil {
			r.sink.Emit(msg.Name, msg.Payload)
		}
	case opError:
		// An error carrying a session ID is that session's: the usual case is
		// re-attaching to a session the daemon no longer has. Ending the
		// subscription tells the consumer the pane is gone instead of leaving
		// it waiting for output that will never come.
		if msg.ID != 0 {
			r.mu.Lock()
			sub := r.subs[msg.ID]
			delete(r.subs, msg.ID)
			r.mu.Unlock()
			if sub != nil {
				sub.close()
			}
		}
	}
}

// wsConn serialises writes on one socket.
type wsConn struct {
	conn *websocket.Conn
	ctx  context.Context
	mu   sync.Mutex
}

func (w *wsConn) sendControl(msg control) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.Write(w.ctx, websocket.MessageText, data)
}

func (w *wsConn) sendBinary(frame []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.Write(w.ctx, websocket.MessageBinary, frame)
}
