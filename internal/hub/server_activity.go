package hub

import (
	"encoding/json"
	"net/http"
	"time"
)

// The agent-state routes of /v1/sessions/{id}: the confirmed activity and the
// seen flag. Out of server.go, which is over the file-size rule and must not
// grow (CLAUDE.md).

// confirmedActivity is the body of GET /v1/sessions/{id}/activity.
type confirmedActivity struct {
	Activity Activity  `json:"activity"`
	Since    time.Time `json:"since"`
}

// activityWrite carries both ways of setting a confirmed state. Seed picks
// which: a seed is refused once the session has confirmed something, a force
// always applies.
type activityWrite struct {
	Activity Activity  `json:"activity"`
	At       time.Time `json:"at"`
	Seed     bool      `json:"seed,omitempty"`
}

// seenWrite is the body of POST /v1/sessions/{id}/seen.
type seenWrite struct {
	Seen bool `json:"seen"`
}

// handleActivity serves GET and POST /v1/sessions/{id}/activity.
func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request, id int) {
	switch r.Method {
	case http.MethodGet:
		state, began := s.host.ConfirmedActivity(id)
		writeJSON(w, http.StatusOK, confirmedActivity{Activity: state, Since: began})
	case http.MethodPost:
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
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleResetActivity serves POST /v1/sessions/{id}/reset-activity.
func (s *Server) handleResetActivity(w http.ResponseWriter, id int) {
	if err := s.host.ResetActivity(id); err != nil {
		writeHostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleKeepAlive serves POST /v1/sessions/{id}/keepalive.
func (s *Server) handleKeepAlive(w http.ResponseWriter, id int) {
	if err := s.host.Nudge(id); err != nil {
		writeHostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSeen serves POST /v1/sessions/{id}/seen.
func (s *Server) handleSeen(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body seenWrite
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	fn := s.host.MarkUnseen
	if body.Seen {
		fn = s.host.MarkSeen
	}
	if err := fn(id); err != nil {
		writeHostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
