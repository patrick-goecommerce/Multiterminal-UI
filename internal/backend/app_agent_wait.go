package backend

import (
	"context"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// Waiting for another agent, as an MCP tool.
//
// The logic is hub.WaitForAgent: the CLI asks the same question of the same
// Host, and one vocabulary for "done" is the point. What is left here is the
// binding.

const (
	agentWaitPoll    = hub.AgentWaitPoll
	agentWaitDefault = hub.AgentWaitDefault
	agentWaitMax     = hub.AgentWaitMax
)

// WaitForAgent blocks until a session reaches one of the given states and
// returns the state it reached. An empty list means "done or blocked".
func (a *AppService) WaitForAgent(ctx context.Context, sessionID int, until []string, timeout time.Duration) (string, error) {
	return hub.WaitForAgent(ctx, a.host, sessionID, until, timeout)
}

// agentWaitState collapses a session's summary into the wait vocabulary.
func (a *AppService) agentWaitState(sessionID int) string {
	return hub.AgentStateOf(a.host, sessionID)
}
