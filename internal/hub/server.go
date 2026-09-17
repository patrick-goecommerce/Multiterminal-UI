package hub

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Server exposes a Host over loopback HTTP.
//
// Control is plain JSON over HTTP/1.1; terminal output and input ride a
// WebSocket at /v1/stream. The transport is deliberately the one the rest of
// MTUI already uses for its loopback listeners: bind port 0, publish the port
// and a token through internal/discovery, and check the token on every
// request. Reachability is not identity, so the token is not optional.
type Server struct {
	host  Host
	token string

	mu       sync.Mutex
	clients  map[*streamClient]struct{}
	shutdown func()
}

// NewServer wraps a Host. token must be the value published in the discovery
// record; an empty token is refused by every request, which fails closed.
func NewServer(host Host, token string) *Server {
	return &Server{
		host:    host,
		token:   token,
		clients: make(map[*streamClient]struct{}),
	}
}

// Handler returns the HTTP handler to serve.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/hub", s.guard(s.handleHub))
	mux.HandleFunc("/v1/sessions", s.guard(s.handleSessions))
	mux.HandleFunc("/v1/sessions/", s.guard(s.handleSession))
	mux.HandleFunc("/v1/hub/shutdown", s.guard(s.handleShutdown))
	mux.HandleFunc("/v1/scan", s.guard(s.handleScan))
	mux.HandleFunc("/v1/stream", s.guard(s.handleStream))
	return mux
}

// SetShutdown registers what POST /v1/hub/shutdown does. Without it the
// endpoint answers 501: a daemon that cannot be asked to stop is better than
// one that pretends it stopped.
func (s *Server) SetShutdown(fn func()) {
	s.mu.Lock()
	s.shutdown = fn
	s.mu.Unlock()
}

// Clients reports how many clients currently hold a stream socket. The daemon
// uses it to decide whether anyone is still watching.
func (s *Server) Clients() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients)
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	fn := s.shutdown
	s.mu.Unlock()
	if fn == nil {
		http.Error(w, "shutdown not supported", http.StatusNotImplemented)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	// Answer first: the caller asked the daemon to stop, and stopping tears
	// down this very connection.
	go fn()
}

// Sink returns an EventSink that fans Host events out to every connected
// client. Pass it to the Host so its events reach the clients.
func (s *Server) Sink() EventSink {
	return SinkFunc(func(name string, payload any) {
		raw, err := json.Marshal(payload)
		if err != nil {
			return
		}
		s.broadcast(control{Op: opEvent, Name: name, Payload: raw})
	})
}

func (s *Server) broadcast(msg control) {
	s.mu.Lock()
	targets := make([]*streamClient, 0, len(s.clients))
	for c := range s.clients {
		targets = append(targets, c)
	}
	s.mu.Unlock()
	for _, c := range targets {
		c.sendControl(msg)
	}
}

// guard rejects any request that does not present the discovery token.
func (s *Server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if s.token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleHub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.host.Info())
}

// handleScan is a POST because it is not free: it re-reads every screen.
func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": s.host.ScanActivity()})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"sessions": s.host.List()})
	case http.MethodPost:
		var spec CreateSpec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		id, err := s.host.Create(spec)
		if err != nil {
			writeHostError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSession routes /v1/sessions/{id} and /v1/sessions/{id}/{action}.
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
	idPart, action, _ := strings.Cut(rest, "/")

	// "reserve" sits under /v1/sessions/ but names an operation, not a
	// session. It is spelled this way so the whole session API stays under one
	// prefix and one guard.
	if idPart == "reserve" && action == "" {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id, err := s.host.Reserve()
		if err != nil {
			writeHostError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id})
		return
	}

	id, err := strconv.Atoi(idPart)
	if err != nil {
		http.Error(w, "bad session id", http.StatusBadRequest)
		return
	}

	switch {
	case action == "" && r.Method == http.MethodGet:
		summary, err := s.host.Get(id)
		if err != nil {
			writeHostError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, summary)
	case action == "" && r.Method == http.MethodDelete:
		if err := s.host.Close(id); err != nil {
			writeHostError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case action == "input" && r.Method == http.MethodPost:
		s.handleInput(w, r, id)
	case action == "resize" && r.Method == http.MethodPost:
		s.handleResize(w, r, id)
	case action == "reset-activity" && r.Method == http.MethodPost:
		if err := s.host.ResetActivity(id); err != nil {
			writeHostError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case action == "activity" && r.Method == http.MethodGet:
		state, began := s.host.ConfirmedActivity(id)
		writeJSON(w, http.StatusOK, confirmedActivity{Activity: state, Since: began})
	case action == "activity" && r.Method == http.MethodPost:
		var body activityWrite
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		fn := s.host.ForceActivity
		if body.Seed {
			fn = s.host.SeedActivity
		}
		if err := fn(id, body.Activity, body.At); err != nil {
			writeHostError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case action == "suspend" && r.Method == http.MethodPost:
		if err := s.host.Suspend(id); err != nil {
			writeHostError(w, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	case action == "queue" || strings.HasPrefix(action, "queue/"):
		s.handleQueue(w, r, id, strings.TrimPrefix(strings.TrimPrefix(action, "queue"), "/"))
	case action == "wake" && r.Method == http.MethodPost:
		if err := s.host.Wake(id); err != nil {
			writeHostError(w, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	case action == "resume" && r.Method == http.MethodPost:
		s.handleResume(w, r, id)
	case action == "text" && r.Method == http.MethodGet:
		s.handleText(w, r, id)
	case action == "statusline" && r.Method == http.MethodPost:
		s.handleStatusline(w, r, id)
	case action == "hook-activity" && r.Method == http.MethodPost:
		s.handleHookActivity(w, r, id)
	case action == "hook-session" && r.Method == http.MethodPost:
		s.handleHookSession(w, r, id)
	case action == "hook-session" && r.Method == http.MethodDelete:
		if err := s.host.ClearHookData(id); err != nil {
			writeHostError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case action == "repaint" && r.Method == http.MethodGet:
		painted, err := s.host.Repaint(id)
		if err != nil {
			writeHostError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(painted)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// handleInput takes keystrokes over HTTP. The stream socket carries them as
// binary frames instead; this exists for scripted clients that do not want to
// open a socket for a single line of input.
func (s *Server) handleInput(w http.ResponseWriter, r *http.Request, id int) {
	var body struct {
		Data []byte `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.host.Write(id, body.Data); err != nil {
		writeHostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleResize(w http.ResponseWriter, r *http.Request, id int) {
	var body struct {
		Rows int `json:"rows"`
		Cols int `json:"cols"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.host.Resize(id, body.Rows, body.Cols); err != nil {
		writeHostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleText answers either the whole screen or a row range, depending on
// whether the caller asked for one.
func (s *Server) handleText(w http.ResponseWriter, r *http.Request, id int) {
	q := r.URL.Query()
	if !q.Has("start") && !q.Has("end") {
		text, err := s.host.PlainText(id)
		if err != nil {
			writeHostError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"text": text})
		return
	}
	start, err := strconv.Atoi(q.Get("start"))
	if err != nil {
		http.Error(w, "bad start row", http.StatusBadRequest)
		return
	}
	end, err := strconv.Atoi(q.Get("end"))
	if err != nil {
		http.Error(w, "bad end row", http.StatusBadRequest)
		return
	}
	rows, hostErr := s.host.PlainTextRows(id, start, end)
	if hostErr != nil {
		writeHostError(w, hostErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows})
}

func (s *Server) handleStatusline(w http.ResponseWriter, r *http.Request, id int) {
	var body struct {
		Cost       float64 `json:"cost"`
		ContextPct int     `json:"context_pct"`
		Model      string  `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.host.SetStatusline(id, body.Cost, body.ContextPct, body.Model); err != nil {
		writeHostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHookActivity(w http.ResponseWriter, r *http.Request, id int) {
	var body struct {
		Activity Activity `json:"activity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.host.SetHookActivity(id, body.Activity); err != nil {
		writeHostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHookSession(w http.ResponseWriter, r *http.Request, id int) {
	var body struct {
		AgentSessionID string `json:"agent_session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.host.SetHookSessionID(id, body.AgentSessionID); err != nil {
		writeHostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request, id int) {
	var body struct {
		Argv []string `json:"argv"`
		Dir  string   `json:"dir"`
		Env  []string `json:"env"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.host.Resume(id, body.Argv, body.Dir, body.Env); err != nil {
		writeHostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeHostError maps a Host error onto a status a client can act on: a gone
// session is a 404 and not worth retrying, a closed host is a 503 and is.
func writeHostError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNoSession):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, ErrClosed):
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, ErrNotIdle), errors.Is(err, ErrNoResumeID):
		// The session is fine; the request was not applicable to it.
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// confirmedActivity is the wire shape of a debounced state.
type confirmedActivity struct {
	Activity Activity  `json:"activity"`
	Since    time.Time `json:"since"`
}

// activityWrite carries both ways of setting a confirmed state. Seed picks
// which: a seed is refused once the session has confirmed something, a force
// always wins, and they are one endpoint because they write the same field.
type activityWrite struct {
	Activity Activity  `json:"activity"`
	At       time.Time `json:"at"`
	Seed     bool      `json:"seed,omitempty"`
}

// handleQueue routes /v1/sessions/{id}/queue and its sub-paths.
func (s *Server) handleQueue(w http.ResponseWriter, r *http.Request, id int, rest string) {
	switch {
	case rest == "" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"items": s.host.QueueList(id)})
	case rest == "" && r.Method == http.MethodPost:
		var body struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		item, err := s.host.QueueAdd(id, body.Prompt)
		if err != nil {
			writeHostError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case rest == "" && r.Method == http.MethodDelete:
		if err := s.host.QueueClear(id, r.URL.Query().Get("done") == "1"); err != nil {
			writeHostError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case rest == "advance" && r.Method == http.MethodPost:
		s.host.QueueAdvance(id)
		w.WriteHeader(http.StatusAccepted)
	case r.Method == http.MethodDelete:
		itemID, err := strconv.Atoi(rest)
		if err != nil {
			http.Error(w, "bad queue item id", http.StatusBadRequest)
			return
		}
		removed, err := s.host.QueueRemove(id, itemID, r.URL.Query().Get("force") == "1")
		if err != nil {
			writeHostError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
