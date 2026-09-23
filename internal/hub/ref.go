package hub

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Ref addresses one session across hubs. Hosts speak in plain IDs; the hub
// half is filled in at the wire boundary (see package doc).
//
// Outside Go a Ref is the string "hub:id", and the zero Ref is "". That is
// the form the frontend holds, saves and compares: an opaque string, so that
// a second hub changes nothing in the code that only passes a session around.
type Ref struct {
	Hub string
	ID  int
}

// Local builds a Ref for a session on the named hub.
func Local(hub string, id int) Ref { return Ref{Hub: hub, ID: id} }

// IsZero reports whether r names no session. Hosts hand out IDs from 1.
func (r Ref) IsZero() bool { return r.ID <= 0 }

// Legacy reports whether r came from a file written before sessions carried
// their hub: a bare number, which only the one hub of that time could mean.
func (r Ref) Legacy() bool { return !r.IsZero() && r.Hub == "" }

// String renders "hub:id", or "" for the zero Ref.
func (r Ref) String() string {
	if r.IsZero() {
		return ""
	}
	if r.Hub == "" {
		return strconv.Itoa(r.ID)
	}
	return r.Hub + ":" + strconv.Itoa(r.ID)
}

// ParseRef is the inverse of String. A bare number parses as a legacy Ref.
// The split is at the last colon: the ID is always a number, the hub is only
// guaranteed to be non-empty.
func ParseRef(s string) (Ref, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Ref{}, nil
	}
	hub, num := "", s
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		hub, num = s[:i], s[i+1:]
		if hub == "" {
			return Ref{}, fmt.Errorf("hub: ref %q has no hub", s)
		}
	}
	id, err := strconv.Atoi(num)
	if err != nil || id <= 0 {
		return Ref{}, fmt.Errorf("hub: ref %q has no session id", s)
	}
	return Ref{Hub: hub, ID: id}, nil
}

// MarshalText implements encoding.TextMarshaler, which is what makes a Ref a
// JSON string, a JSON map key, and a Wails binding argument the frontend can
// pass as a plain string.
func (r Ref) MarshalText() ([]byte, error) { return []byte(r.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler.
func (r *Ref) UnmarshalText(b []byte) error {
	v, err := ParseRef(string(b))
	if err != nil {
		return err
	}
	*r = v
	return nil
}

// UnmarshalJSON accepts the string form and, for files written before the
// switch, a bare number. null and 0 are the zero Ref.
func (r *Ref) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		return r.UnmarshalText([]byte(s))
	}
	if string(b) == "null" {
		*r = Ref{}
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("hub: ref must be a string or a number: %w", err)
	}
	*r = Ref{ID: n}
	if n < 0 {
		*r = Ref{}
	}
	return nil
}
