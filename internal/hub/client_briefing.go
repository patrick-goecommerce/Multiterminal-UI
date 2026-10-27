package hub

import (
	"fmt"
	"net/http"
)

// Briefing returns the session's last briefing; false when none was built yet.
func (r *Remote) Briefing(id int) (Briefing, bool, error) {
	var body briefingRead
	if err := r.call(http.MethodGet, fmt.Sprintf("/v1/sessions/%d/briefing", id), nil, &body); err != nil {
		return Briefing{}, false, err
	}
	if body.Briefing == nil {
		return Briefing{}, false, nil
	}
	return *body.Briefing, true, nil
}

// SetTarget tells the host which branch is the session's target; empty clears it.
func (r *Remote) SetTarget(id int, branch string) error {
	return r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/target", id), targetWrite{Branch: branch}, nil)
}
