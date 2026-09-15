package hub

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

// maxFramePayload caps how much output goes into one binary frame. Replay of a
// full ring would otherwise arrive as a single multi-megabyte message, which
// both peers would have to hold in memory at once.
const maxFramePayload = 128 << 10

// streamReadLimit bounds an inbound message. Input frames are keystrokes and
// pastes, so a megabyte is generous; without a limit a peer could make the
// daemon allocate without bound.
const streamReadLimit = 1 << 20

// streamClient is one connected client: a socket, the subscriptions it asked
// for, and a mutex making writes sequential.
type streamClient struct {
	srv  *Server
	conn *websocket.Conn
	ctx  context.Context

	writeMu sync.Mutex

	subsMu sync.Mutex
	subs   map[int]*Subscription
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept already answered the request
	}
	conn.SetReadLimit(streamReadLimit)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	c := &streamClient{srv: s, conn: conn, ctx: ctx, subs: make(map[int]*Subscription)}
	s.mu.Lock()
	s.clients[c] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, c)
		s.mu.Unlock()
		c.closeSubs()
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	c.readLoop()
}

// readLoop serves one client until it disconnects.
func (c *streamClient) readLoop() {
	for {
		typ, data, err := c.conn.Read(c.ctx)
		if err != nil {
			return
		}
		switch typ {
		case websocket.MessageText:
			var msg control
			if err := json.Unmarshal(data, &msg); err != nil {
				c.sendControl(control{Op: opError, Message: "bad control frame"})
				continue
			}
			c.handleControl(msg)
		case websocket.MessageBinary:
			c.handleBinary(data)
		}
	}
}

func (c *streamClient) handleControl(msg control) {
	switch msg.Op {
	case opAttach:
		c.attach(msg.ID, msg.From)
	case opDetach:
		c.detach(msg.ID)
		c.sendControl(control{Op: opDetached, ID: msg.ID})
	default:
		c.sendControl(control{Op: opError, ID: msg.ID, Message: "unknown op " + msg.Op})
	}
}

func (c *streamClient) handleBinary(data []byte) {
	typ, _, id, _, payload, err := decodeFrame(data)
	if err != nil || typ != frameInput {
		return
	}
	if err := c.srv.host.Write(id, payload); err != nil {
		c.sendControl(control{Op: opError, ID: id, Message: err.Error()})
	}
}

// attach subscribes this client to a session. Re-attaching an already attached
// session replaces the subscription, which is what a client does after it lost
// track of its offset.
func (c *streamClient) attach(id int, from int64) {
	sub, err := c.srv.host.Attach(id, from)
	if err != nil {
		c.sendControl(control{Op: opError, ID: id, Message: err.Error()})
		return
	}

	c.subsMu.Lock()
	if old := c.subs[id]; old != nil {
		old.Close()
	}
	c.subs[id] = sub
	c.subsMu.Unlock()

	summary, err := c.srv.host.Get(id)
	offset := from
	if err == nil {
		offset = summary.Offset
	}
	c.sendControl(control{Op: opAttached, ID: id, Offset: offset})
	go c.forward(id, sub)
}

func (c *streamClient) detach(id int) {
	c.subsMu.Lock()
	sub := c.subs[id]
	delete(c.subs, id)
	c.subsMu.Unlock()
	if sub != nil {
		sub.Close()
	}
}

// forward copies one subscription onto the socket.
func (c *streamClient) forward(id int, sub *Subscription) {
	defer sub.Close()
	for chunk := range sub.C {
		flags := byte(0)
		if chunk.Truncated {
			flags |= flagTruncated
		}
		offset := chunk.Offset
		data := chunk.Data
		for len(data) > 0 {
			n := len(data)
			if n > maxFramePayload {
				n = maxFramePayload
			}
			if err := c.sendBinary(encodeFrame(frameOutput, flags, id, offset, data[:n])); err != nil {
				return
			}
			// Only the first frame of a split chunk carries the gap: the rest
			// follow it contiguously.
			flags = 0
			offset += int64(n)
			data = data[n:]
		}
	}
}

func (c *streamClient) sendControl(msg control) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[hub] marshalling %s: %v", msg.Op, err)
		return
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.Write(c.ctx, websocket.MessageText, data)
}

func (c *streamClient) sendBinary(frame []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.Write(c.ctx, websocket.MessageBinary, frame)
}

func (c *streamClient) closeSubs() {
	c.subsMu.Lock()
	subs := c.subs
	c.subs = make(map[int]*Subscription)
	c.subsMu.Unlock()
	for _, sub := range subs {
		sub.Close()
	}
}
