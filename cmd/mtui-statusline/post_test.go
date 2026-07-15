package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostCaptureSendsSessionAndPayload(t *testing.T) {
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
	postCapture([]byte(`{"cost":{"total_cost_usd":0.42}}`), port, "7")
	g := <-ch
	if g.SessionID != 7 || !strings.Contains(string(g.Payload), "0.42") {
		t.Fatalf("bad post: %+v", g)
	}
}

func TestPostCaptureNoEnvIsNoop(t *testing.T) { postCapture([]byte(`{}`), "", "") }
func TestPostCaptureDeadPortIsSilent(t *testing.T) { postCapture([]byte(`{}`), "1", "7") }
