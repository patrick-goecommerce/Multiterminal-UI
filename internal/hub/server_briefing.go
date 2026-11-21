package hub

import (
	"encoding/json"
	"net/http"
)

// The briefing routes of /v1/sessions/{id}. Out of server.go, which is over
// the file-size rule and must not grow (CLAUDE.md).

// briefingRead is the body of GET /v1/sessions/{id}/briefing. A session with
// no briefing yet answers 200 with a null, not 404: a 404 already means "no
// such session" on the client.
type briefingRead struct {
	Briefing *Briefing `json:"briefing"`
}

// targetWrite is the body of POST /v1/sessions/{id}/target.
type targetWrite struct {
	Branch string `json:"branch"`
}

// handleBriefing serves GET /v1/sessions/{id}/briefing.
func (s *Server) handleBriefing(w http.ResponseWriter, id int) {
	b, ok, err := s.host.Briefing(id)
	if err != nil {
		writeHostError(w, err)
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, briefingRead{})
		return
	}
	writeJSON(w, http.StatusOK, briefingRead{Briefing: &b})
}

// handleTarget serves POST /v1/sessions/{id}/target.
func (s *Server) handleTarget(w http.ResponseWriter, r *http.Request, id int) {
	var body targetWrite
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.host.SetTarget(id, body.Branch); err != nil {
		writeHostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
