package hub

import (
	"encoding/json"
	"testing"
)

// The same event arrives as a struct from an in-process host and as JSON from
// a daemon. A handler that only type-asserts hears one of the two, which is
// how a client can miss every exit the daemon reports.
func TestDecodePayload_AcceptsBothShapes(t *testing.T) {
	want := SessionExited{ID: 7, ExitCode: 3}

	got, ok := DecodePayload[SessionExited](want)
	if !ok || got != want {
		t.Errorf("struct payload = (%+v, %v), want (%+v, true)", got, ok, want)
	}

	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, ok = DecodePayload[SessionExited](json.RawMessage(raw))
	if !ok || got != want {
		t.Errorf("json payload = (%+v, %v), want (%+v, true)", got, ok, want)
	}
}

func TestDecodePayload_RejectsSomethingElse(t *testing.T) {
	if _, ok := DecodePayload[SessionExited](42); ok {
		t.Error("an int decoded as a session event")
	}
	if _, ok := DecodePayload[SessionExited](json.RawMessage("not json")); ok {
		t.Error("garbage decoded as a session event")
	}
}
