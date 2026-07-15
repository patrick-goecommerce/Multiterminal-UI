// internal/backend/app_statusline_api.go
package backend

import (
	"encoding/json"
	"log"
	"net/http"
)

// statuslinePayload is MTUI's wrapper around Claude's raw statusLine JSON,
// posted by mtui-statusline. Only the fields MTUI consumes are typed;
// unknown fields are ignored.
type statuslinePayload struct {
	SessionID int `json:"sessionId"`
	Payload   struct {
		Cost struct {
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

// handleStatusline records cost/context/model from a forwarded statusLine update.
// Loopback-only, POST-only, no auth (same trust model as /api/tmux/log).
func (a *AppService) handleStatusline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var p statuslinePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	sess := a.sessions[p.SessionID]
	a.mu.Unlock()
	if sess != nil {
		// int(UsedPercentage) intentionally truncates: 40.9 → 40. Exact decimal
		// precision is not required for the progress-bar display.
		sess.SetStatuslineData(
			p.Payload.Cost.TotalCostUSD,
			int(p.Payload.ContextWindow.UsedPercentage),
			p.Payload.Model.DisplayName,
		)
	}
	log.Printf("[statusline] session %d cost=%.4f ctx=%d%% model=%q found=%t",
		p.SessionID, p.Payload.Cost.TotalCostUSD,
		int(p.Payload.ContextWindow.UsedPercentage), p.Payload.Model.DisplayName, sess != nil)

	w.WriteHeader(http.StatusOK)
}
