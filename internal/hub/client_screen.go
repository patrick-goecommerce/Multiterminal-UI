package hub

import (
	"fmt"
	"net/http"
	"time"
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

// ResetActivity implements Host.
func (r *Remote) ResetActivity(id int) error {
	return r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/reset-activity", id), nil, nil)
}

// ConfirmedActivity implements Host. A failure reads as "no confirmed state
// yet", which is the same thing a fresh session reports and therefore the
// answer a caller already handles.
func (r *Remote) ConfirmedActivity(id int) (Activity, time.Time) {
	var out confirmedActivity
	if err := r.call(http.MethodGet, fmt.Sprintf("/v1/sessions/%d/activity", id), nil, &out); err != nil {
		return "", time.Time{}
	}
	return out.Activity, out.Since
}

// ForceActivity implements Host.
func (r *Remote) ForceActivity(id int, state Activity, at time.Time) error {
	return r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/activity", id),
		activityWrite{Activity: state, At: at}, nil)
}

// SeedActivity implements Host.
func (r *Remote) SeedActivity(id int, state Activity, at time.Time) error {
	return r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/activity", id),
		activityWrite{Activity: state, At: at, Seed: true}, nil)
}

// ScanActivity implements Host.
func (r *Remote) ScanActivity() []ScanResult {
	var out struct {
		Results []ScanResult `json:"results"`
	}
	if err := r.call(http.MethodPost, "/v1/scan", nil, &out); err != nil {
		return nil
	}
	return out.Results
}

// Suspend implements Host.
func (r *Remote) Suspend(id int) error {
	return r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/suspend", id), nil, nil)
}

// Resume implements Host.
func (r *Remote) Resume(id int, argv []string, dir string, env []string) error {
	body := struct {
		Argv []string `json:"argv"`
		Dir  string   `json:"dir"`
		Env  []string `json:"env"`
	}{argv, dir, env}
	return r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/resume", id), body, nil)
}
