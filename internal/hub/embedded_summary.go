package hub

import (
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// Turning a live session into the flat picture a client gets.
//
// Split out of embedded.go for the file-size rule, and it is the right seam:
// everything here answers "what would someone who cannot hold a
// *terminal.Session see", which is exactly the question the Host interface
// exists to answer.

func summarize(id int, m *managed) SessionSummary {
	contextPct, model, _ := m.sess.StatuslineInfo()
	if model == "" && m.spec.Launch != nil {
		// The status line takes a few seconds to arrive. Until it does, the
		// model the session was asked for is a better answer than none.
		model = m.spec.Launch.Model
	}
	return SessionSummary{
		ID:            id,
		Name:          m.sess.Name(),
		Dir:           m.spec.Dir,
		Mode:          m.spec.Mode,
		Status:        statusOf(m.sess),
		ExitCode:      m.sess.GetExitCode(),
		PID:           m.sess.Pid(),
		StartedAt:     m.startedAt,
		LastOutputAt:  m.sess.GetLastOutputAt(),
		Activity:      activityOf(m.sess.GetActivity()),
		Title:         m.sess.GetTitle(),
		Cost:          m.sess.GetTokens().TotalCost,
		ContextPct:    contextPct,
		Model:         model,
		ResumeID:      effectiveResumeID(m.sess),
		HookSessionID: m.sess.HookSessionID(),
		HasHookData:   m.sess.HasHookData(),
		Origin:        m.spec.Origin,
		Offset:        m.ring.End(),
	}
}

// effectiveResumeID is the ID to resume a session with: the hook-reported one
// wins, the one parsed out of argv at launch is the fallback.
func effectiveResumeID(s *terminal.Session) string {
	if id := s.HookSessionID(); id != "" {
		return id
	}
	return s.ResumeID()
}

// activityOf maps the terminal package's numeric state onto the wire strings.
// The numbers are an internal ordering; letting them cross a protocol boundary
// would make a reordering change meaning silently.
func activityOf(a terminal.ActivityState) Activity {
	switch a {
	case terminal.ActivityActive:
		return ActivityActive
	case terminal.ActivityDone:
		return ActivityDone
	case terminal.ActivityWaitingPermission:
		return ActivityWaitingPermission
	case terminal.ActivityWaitingAnswer:
		return ActivityWaitingAnswer
	case terminal.ActivityError:
		return ActivityError
	default:
		return ActivityIdle
	}
}

func statusOf(s *terminal.Session) Status {
	switch s.GetStatus() {
	case terminal.StatusExited:
		return StatusExited
	case terminal.StatusError:
		return StatusError
	case terminal.StatusSuspending:
		return StatusSuspending
	case terminal.StatusSuspended:
		return StatusSuspended
	default:
		return StatusRunning
	}
}
