package backend

import (
	"net"
	"testing"
)

func focusRequest(t *testing.T, send string) (bool, string) {
	t.Helper()
	server, client := net.Pipe()
	go func() {
		_, _ = client.Write([]byte(send))
		_ = client.Close()
	}()
	ok, ref := readFocusRequest(server, "tok")
	_ = server.Close()
	if ref.IsZero() {
		return ok, ""
	}
	return ok, ref.String()
}

func TestReadFocusRequest(t *testing.T) {
	cases := []struct {
		send    string
		wantOK  bool
		wantRef string
	}{
		{"tok\nab12:5\n", true, "ab12:5"},
		{"tok\n", true, ""},            // a plain focus click, as before
		{"tok\nnot a ref\n", true, ""}, // garbage names no pane
		{"tok\nab12:0\n", true, ""},    // a zero id names no pane
		{"wrong\nab12:5\n", false, ""},
	}
	for _, c := range cases {
		ok, ref := focusRequest(t, c.send)
		if ok != c.wantOK || ref != c.wantRef {
			t.Errorf("%q: got (%v, %q), want (%v, %q)", c.send, ok, ref, c.wantOK, c.wantRef)
		}
	}
}
