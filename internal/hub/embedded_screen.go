package hub

import "github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"

// The screen and the agent-state fields, reachable without holding the session
// object. These are what the callers that used to reach into *terminal.Session
// need, expressed so that a client in another process can ask the same
// questions.

// PlainText implements Host.
func (h *Embedded) PlainText(id int) (string, error) {
	m, err := h.lookup(id)
	if err != nil {
		return "", err
	}
	return m.sess.Screen.PlainText(), nil
}

// PlainTextRows implements Host.
func (h *Embedded) PlainTextRows(id, startRow, endRow int) ([]string, error) {
	m, err := h.lookup(id)
	if err != nil {
		return nil, err
	}
	return m.sess.Screen.PlainTextRows(startRow, endRow), nil
}

// SetStatusline implements Host.
func (h *Embedded) SetStatusline(id int, cost float64, contextPct int, model string) error {
	m, err := h.lookup(id)
	if err != nil {
		return err
	}
	m.sess.SetStatuslineData(cost, contextPct, model)
	return nil
}

// SetHookActivity implements Host.
func (h *Embedded) SetHookActivity(id int, activity Activity) error {
	m, err := h.lookup(id)
	if err != nil {
		return err
	}
	m.sess.SetHookActivity(terminalActivity(activity))
	return nil
}

// SetHookSessionID implements Host.
func (h *Embedded) SetHookSessionID(id int, agentSessionID string) error {
	m, err := h.lookup(id)
	if err != nil {
		return err
	}
	m.sess.SetHookSessionID(agentSessionID)
	return nil
}

// ClearHookData implements Host.
func (h *Embedded) ClearHookData(id int) error {
	m, err := h.lookup(id)
	if err != nil {
		return err
	}
	m.sess.ClearHookData()
	return nil
}

// ResetActivity implements Host.
func (h *Embedded) ResetActivity(id int) error {
	m, err := h.lookup(id)
	if err != nil {
		return err
	}
	m.sess.ResetActivity()
	return nil
}

// terminalActivity is the inverse of activityOf.
func terminalActivity(a Activity) terminal.ActivityState {
	switch a {
	case ActivityActive:
		return terminal.ActivityActive
	case ActivityDone:
		return terminal.ActivityDone
	case ActivityWaitingPermission:
		return terminal.ActivityWaitingPermission
	case ActivityWaitingAnswer:
		return terminal.ActivityWaitingAnswer
	case ActivityError:
		return terminal.ActivityError
	default:
		return terminal.ActivityIdle
	}
}
