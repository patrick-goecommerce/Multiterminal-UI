package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunWrappedPassesThroughStdoutAndExit(t *testing.T) {
	var out bytes.Buffer
	// `cmd /c echo hi` prints "hi"; exit 0.
	code := runWrapped([]string{"cmd", "/c", "echo", "hi"}, []byte("{}"), &out, io.Discard)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "hi") {
		t.Fatalf("stdout = %q, want it to contain %q", out.String(), "hi")
	}
}

func TestRunWrappedNoArgsIsNoop(t *testing.T) {
	var out bytes.Buffer
	if code := runWrapped(nil, []byte("{}"), &out, io.Discard); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestForwardPostsSessionIdAndPayload(t *testing.T) {
	type got struct {
		SessionID int             `json:"sessionId"`
		Payload   json.RawMessage `json:"payload"`
	}
	ch := make(chan got, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var g got
		_ = json.NewDecoder(r.Body).Decode(&g)
		ch <- g
	}))
	defer srv.Close()
	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")

	forward([]byte(`{"cost":{"total_cost_usd":0.42}}`), port, "7")

	select {
	case g := <-ch:
		if g.SessionID != 7 {
			t.Fatalf("sessionId = %d, want 7", g.SessionID)
		}
		if !strings.Contains(string(g.Payload), "0.42") {
			t.Fatalf("payload = %s, want it to contain 0.42", g.Payload)
		}
	default:
		t.Fatal("server received no request")
	}
}

func TestForwardDeadPortIsSilent(t *testing.T) {
	// Port 1 is not listening; forward must return promptly without panic.
	forward([]byte(`{}`), "1", "7")
}

func TestForwardMissingEnvIsNoop(t *testing.T) {
	forward([]byte(`{}`), "", "") // no port/sid -> no request, no panic
}
