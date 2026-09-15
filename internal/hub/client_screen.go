package hub

import (
	"fmt"
	"net/http"
)

// The screen and agent-state half of the client. All of it is control traffic,
// so it goes over HTTP rather than the stream socket: these are questions and
// one-off updates, not the byte-per-keystroke path.

// PlainText implements Host.
func (r *Remote) PlainText(id int) (string, error) {
	var out struct {
		Text string `json:"text"`
	}
	err := r.call(http.MethodGet, fmt.Sprintf("/v1/sessions/%d/text", id), nil, &out)
	return out.Text, err
}

// PlainTextRows implements Host.
func (r *Remote) PlainTextRows(id, startRow, endRow int) ([]string, error) {
	var out struct {
		Rows []string `json:"rows"`
	}
	path := fmt.Sprintf("/v1/sessions/%d/text?start=%d&end=%d", id, startRow, endRow)
	err := r.call(http.MethodGet, path, nil, &out)
	return out.Rows, err
}

// SetStatusline implements Host.
func (r *Remote) SetStatusline(id int, cost float64, contextPct int, model string) error {
	body := struct {
		Cost       float64 `json:"cost"`
		ContextPct int     `json:"context_pct"`
		Model      string  `json:"model"`
	}{cost, contextPct, model}
	return r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/statusline", id), body, nil)
}

// SetHookActivity implements Host.
func (r *Remote) SetHookActivity(id int, activity Activity) error {
	body := struct {
		Activity Activity `json:"activity"`
	}{activity}
	return r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/hook-activity", id), body, nil)
}

// SetHookSessionID implements Host.
func (r *Remote) SetHookSessionID(id int, agentSessionID string) error {
	body := struct {
		AgentSessionID string `json:"agent_session_id"`
	}{agentSessionID}
	return r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/hook-session", id), body, nil)
}

// ClearHookData implements Host.
func (r *Remote) ClearHookData(id int) error {
	return r.call(http.MethodDelete, fmt.Sprintf("/v1/sessions/%d/hook-session", id), nil, nil)
}
