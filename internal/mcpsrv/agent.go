package mcpsrv

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/launch"
)

// What the tools actually do. The MCP glue is in handlers.go; these are plain
// methods so they can be tested without building a tool call.

// SessionInfo is one delegated session, as list_sessions reports it.
type SessionInfo struct {
	ID      int    `json:"id"`
	Tool    string `json:"tool"`
	Dir     string `json:"dir"`
	Model   string `json:"model"`
	Running bool   `json:"running"`
}

// promptDelay is how long open_session waits before queueing its initial
// prompt.
//
// The queue sends a prompt at once when the session has no confirmed activity
// yet, which is exactly the state a session two milliseconds old is in. Typed
// that early the prompt lands in a CLI that has not drawn its input box, and
// the characters go nowhere. The pause is until the CLI is up, after which the
// queue's own readiness check takes over.
const promptDelay = 2 * time.Second

// OpenSession starts an agent CLI in its own session.
//
// The session is marked OriginAgent so that whoever is watching can tell it
// apart from the panes a person opened: a window puts a pane in front of it,
// and the idle-suspend gate leaves it alone.
func (s *Server) OpenSession(tool, dir, model, prompt string) (int, error) {
	if !launch.IsAgentTool(tool) {
		return -1, fmt.Errorf("unsupported tool %q (must be %s)", tool,
			strings.Join(launch.KnownAgents(), ", "))
	}

	id, err := s.host.Create(hub.CreateSpec{
		Dir:    dir,
		Rows:   24,
		Cols:   80,
		Origin: hub.OriginAgent,
		// By name, not by argv: the caller is another agent, which knows what
		// it wants run and nothing about how this machine runs it.
		Launch: &hub.LaunchRequest{Tool: tool, Model: model},
	})
	if err != nil {
		return -1, fmt.Errorf("failed to start %s session in %q: %w", tool, dir, err)
	}

	if prompt != "" {
		go func() {
			time.Sleep(promptDelay)
			if _, err := s.host.QueueAdd(id, prompt); err != nil {
				log.Printf("[mcp-server] session %d: initial prompt not queued: %v", id, err)
			}
		}()
	}

	log.Printf("[mcp-server] opened %s session %d in %q", tool, id, dir)
	return id, nil
}

// SendInput queues text as the next prompt for a running session.
func (s *Server) SendInput(sessionID int, text string) error {
	if _, err := s.host.QueueAdd(sessionID, text); err != nil {
		return sessionError(sessionID, err)
	}
	return nil
}

// ReadOutput returns the session's current visible screen as plain text.
// This is the only way a delegating agent sees what the other one produced;
// list_sessions only reports run state.
func (s *Server) ReadOutput(sessionID int) (string, error) {
	text, err := s.host.PlainText(sessionID)
	if err != nil {
		return "", sessionError(sessionID, err)
	}
	return text, nil
}

// CloseSession ends a session. reason is logged only.
func (s *Server) CloseSession(sessionID int, reason string) error {
	if _, err := s.host.Get(sessionID); err != nil {
		return sessionError(sessionID, err)
	}
	log.Printf("[mcp-server] closing session %d: %s", sessionID, reason)
	if err := s.host.Close(sessionID); err != nil {
		return sessionError(sessionID, err)
	}
	return nil
}

// WaitForAgent blocks until a session reaches one of the given states.
func (s *Server) WaitForAgent(ctx context.Context, sessionID int, until []string, timeout time.Duration) (string, error) {
	return hub.WaitForAgent(ctx, s.host, sessionID, until, timeout)
}

// ListSessions returns the still-running sessions that were opened through
// this server.
//
// The marker is on the session itself rather than in a map here, because the
// two are not in the same process in daemon mode: a window restarted since the
// delegation still has to see that a session belongs to an agent, and so does
// a second client asking the same question.
func (s *Server) ListSessions() []SessionInfo {
	all := s.host.List()
	out := make([]SessionInfo, 0, len(all))
	for _, summary := range all {
		if summary.Origin != hub.OriginAgent || summary.Status != hub.StatusRunning {
			continue
		}
		out = append(out, SessionInfo{
			ID:      summary.ID,
			Tool:    summary.Mode,
			Dir:     summary.Dir,
			Model:   summary.Model,
			Running: true,
		})
	}
	return out
}

// sessionError keeps the message an agent sees the same whichever host
// answered: a remote host reports a missing session over HTTP, and "404" is
// not something to hand to a model.
func sessionError(id int, err error) error {
	if errors.Is(err, hub.ErrNoSession) {
		return fmt.Errorf("session %d not found", id)
	}
	return err
}
