package hub

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
)

// The shim endpoints are what MTUI's helper binaries post to: the statusline
// forwarder reports an agent's cost and context window, and the tmux shim logs
// what a tool tried to run.
//
// They belong to the host rather than to the window because the port is baked
// into a session's environment at launch (MTUI_PORT) and can never be changed
// afterwards. Served from the window, a session outliving that window would go
// on posting to a port that is gone, for as long as it lives. Served by the
// daemon, the address stays valid for the session's whole life.
//
// There is no token here, deliberately: the helpers are spawned by the agent
// with an environment MTUI controls, they carry no credential, and the
// endpoints only accept a session id that has to already exist. That is the
// same trust model these two endpoints have always had.

// startShim binds a loopback port and serves the helper endpoints on it.
func (h *Embedded) startShim() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("hub: shim listen: %w", err)
	}
	h.shimPort = ln.Addr().(*net.TCPAddr).Port
	h.shimListener = ln

	mux := http.NewServeMux()
	mux.HandleFunc("/api/statusline", h.handleShimStatusline)
	mux.HandleFunc("/api/tmux/log", h.handleShimTmuxLog)

	log.Printf("[hub] shim endpoints on port %d", h.shimPort)
	go func() { _ = http.Serve(ln, mux) }()
	return nil
}

// statuslinePayload is MTUI's wrapper around the agent's raw statusLine JSON,
// posted by mtui-statusline. Only the fields MTUI consumes are typed.
type statuslinePayload struct {
	SessionID int `json:"sessionId"`
	Payload   struct {
		// SessionName is Claude Code's name for the session: the one set with
		// /rename or --name, otherwise its own generated title. Absent until
		// one of those exists.
		SessionName string `json:"session_name"`
		Cost        struct {
			TotalCostUSD  float64 `json:"total_cost_usd"`
			TotalDuration int     `json:"total_duration_ms"`
		} `json:"cost"`
		ContextWindow struct {
			UsedPercentage float64 `json:"used_percentage"`
		} `json:"context_window"`
		Model struct {
			DisplayName string `json:"display_name"`
		} `json:"model"`
	} `json:"payload"`
}

func (h *Embedded) handleShimStatusline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var p statuslinePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// int(UsedPercentage) truncates on purpose: 40.9 becomes 40. Exact decimals
	// are not required for a progress bar.
	err := h.SetStatusline(
		p.SessionID,
		p.Payload.Cost.TotalCostUSD,
		int(p.Payload.ContextWindow.UsedPercentage),
		p.Payload.Model.DisplayName,
	)
	if err == nil {
		h.setAgentName(p.SessionID, p.Payload.SessionName)
	}
	log.Printf("[hub] statusline session %d cost=%.4f ctx=%d%% model=%q found=%t",
		p.SessionID, p.Payload.Cost.TotalCostUSD,
		int(p.Payload.ContextWindow.UsedPercentage), p.Payload.Model.DisplayName, err == nil)

	w.WriteHeader(http.StatusOK)
}

func (h *Embedded) handleShimTmuxLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var entry TmuxCommand
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("[hub] tmux shim args=%v dir=%q", entry.Args, entry.Dir)
	h.emit(EventTmuxCommand, entry)

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "logged"})
}

// setAgentName records the agent's own name for a session, if it has one.
func (h *Embedded) setAgentName(id int, name string) {
	if m, err := h.lookup(id); err == nil {
		m.sess.SetAgentName(strings.TrimSpace(name))
	}
}
