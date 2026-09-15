package hub

import (
	"encoding/binary"
	"encoding/json"
	"errors"
)

// The stream socket carries two kinds of frame. Control is JSON in text
// frames, terminal bytes are binary frames with a fixed header. Splitting them
// that way keeps the hot path free of base64: PTY output is the one thing on
// this connection that is large and constant, and encoding it as text would
// cost a third more bytes and a copy in both directions.

// Frame types.
const (
	frameOutput byte = 1 // daemon → client: PTY bytes
	frameInput  byte = 2 // client → daemon: keystrokes and pastes
)

// Frame flags.
const flagTruncated byte = 1 << 0

// frameHeaderLen is type(1) + flags(1) + session(4) + offset(8).
const frameHeaderLen = 14

// errShortFrame means the peer sent fewer bytes than a header needs.
var errShortFrame = errors.New("hub: short frame")

// encodeFrame builds a binary frame. offset is the position of the first
// payload byte in the session's output stream; it is unused for input frames.
func encodeFrame(typ byte, flags byte, id int, offset int64, payload []byte) []byte {
	buf := make([]byte, frameHeaderLen+len(payload))
	buf[0] = typ
	buf[1] = flags
	binary.BigEndian.PutUint32(buf[2:6], uint32(id))
	binary.BigEndian.PutUint64(buf[6:14], uint64(offset))
	copy(buf[frameHeaderLen:], payload)
	return buf
}

// decodeFrame splits a binary frame. The payload aliases buf, so a caller that
// keeps it beyond the read must copy.
func decodeFrame(buf []byte) (typ byte, flags byte, id int, offset int64, payload []byte, err error) {
	if len(buf) < frameHeaderLen {
		return 0, 0, 0, 0, nil, errShortFrame
	}
	typ = buf[0]
	flags = buf[1]
	id = int(binary.BigEndian.Uint32(buf[2:6]))
	offset = int64(binary.BigEndian.Uint64(buf[6:14]))
	return typ, flags, id, offset, buf[frameHeaderLen:], nil
}

// Control operations, sent as JSON text frames.
const (
	opAttach   = "attach"
	opDetach   = "detach"
	opAttached = "attached"
	opEvent    = "event"
	opError    = "error"
	opDetached = "detached"
)

// control is the envelope of every JSON frame in either direction. Fields not
// relevant to an op stay zero.
type control struct {
	Op string `json:"op"`
	ID int    `json:"id,omitempty"`
	// From is the offset an attach resumes at, or ReplayAll.
	From int64 `json:"from,omitempty"`
	// Offset is the stream position an attach was granted at.
	Offset int64 `json:"offset,omitempty"`
	// Name and Payload carry a Host event. Payload stays raw in both
	// directions: the daemon marshals it once, and a client that only
	// forwards it into a Wails event never has to know its shape.
	Name    string          `json:"name,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	// Message explains an opError.
	Message string `json:"message,omitempty"`
}
