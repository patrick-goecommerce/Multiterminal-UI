package backend

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/launch"
)

// AgentSessionInfo describes a session that was spawned via SpawnAgentSession
// (i.e. by another agent through the local MCP server) rather than by the
// user through the UI.
type AgentSessionInfo struct {
	ID      int    `json:"id" yaml:"id"`
	Tool    string `json:"tool" yaml:"tool"`
	Dir     string `json:"dir" yaml:"dir"`
	Model   string `json:"model" yaml:"model"`
	Running bool   `json:"running" yaml:"running"`
}

// AgentSessionSpawnedEvent is emitted so the frontend can attach a visible
// pane to a session that was created outside the UI.
type AgentSessionSpawnedEvent struct {
	ID    int    `json:"id" yaml:"id"`
	Tool  string `json:"tool" yaml:"tool"`
	Model string `json:"model" yaml:"model"`
	Dir   string `json:"dir" yaml:"dir"`
	Name  string `json:"name" yaml:"name"`
}

// The agent table is internal/launch: the daemon starts the same three CLIs
// without a window, and two tables would be two answers to "what is claude".
// What stays here is the resolved path, which only this process works out by
// searching the well-known install locations at startup.

// agentArgv builds a plain (non-yolo, non-auto) launch command for one of the
// supported CLIs, preferring the path this process resolved over the bare
// command from the config.
func (a *AppService) agentArgv(tool, model string) []string {
	argv, err := a.launchPolicy().Argv(tool, model)
	if err != nil {
		return nil
	}
	return argv
}

// launchCommands maps each tool to the command to start it with: the resolved
// absolute path when startup found one, the configured command otherwise.
func (a *AppService) launchCommands() launch.Commands {
	pick := func(resolved, configured string) string {
		if resolved != "" {
			return resolved
		}
		return configured
	}
	return launch.Commands{
		"claude": pick(a.resolvedClaudePath, a.cfg.ClaudeCommand),
		"codex":  pick(a.resolvedCodexPath, a.cfg.CodexCommand),
		"gemini": pick(a.resolvedGeminiPath, a.cfg.GeminiCommand),
	}
}

func (a *AppService) SpawnAgentSession(tool, dir, model, prompt string) (int, error) {
	if !launch.IsAgentTool(tool) {
		return -1, fmt.Errorf("unsupported tool %q (must be %s)", tool,
			strings.Join(launch.KnownAgents(), ", "))
	}
	if dir == "" {
		dir = a.GetWorkingDir()
	}

	argv := a.agentArgv(tool, model)
	id := a.CreateSession(argv, dir, 24, 80, tool)
	if id < 0 {
		return -1, fmt.Errorf("failed to start %s session in %q", tool, dir)
	}

	a.mu.Lock()
	a.agentSessions[id] = AgentSessionInfo{ID: id, Tool: tool, Dir: dir, Model: model, Running: true}
	a.mu.Unlock()

	name := launch.AgentDisplayName(tool)
	if model != "" {
		name = fmt.Sprintf("%s (%s)", name, model)
	}
	if a.app != nil {
		a.app.Event.Emit("mtui:session-spawned", AgentSessionSpawnedEvent{ID: id, Tool: tool, Model: model, Dir: dir, Name: name})
	}

	if prompt != "" {
		go func() {
			time.Sleep(2 * time.Second)
			a.AddToQueue(id, prompt)
		}()
	}

	log.Printf("[agent-control] spawned %s session %d in %q", tool, id, dir)
	return id, nil
}

// SendAgentInput queues text as the next prompt for a running session.
func (a *AppService) SendAgentInput(sessionID int, text string) error {
	if !a.hasSession(sessionID) {
		return fmt.Errorf("session %d not found", sessionID)
	}
	a.AddToQueue(sessionID, text)
	return nil
}

// ReadAgentSessionOutput returns the session's current visible screen
// content as plain text (VT100 buffer rendered without escape sequences).
// This is the only way an MCP-driven agent can see what a delegated session
// actually produced — list_sessions only reports run state.
func (a *AppService) ReadAgentSessionOutput(sessionID int) (string, error) {
	text, err := a.host.PlainText(sessionID)
	if err != nil {
		return "", fmt.Errorf("session %d not found", sessionID)
	}
	return text, nil
}

// CloseAgentSession closes a running session. reason is logged only.
func (a *AppService) CloseAgentSession(sessionID int, reason string) error {
	if !a.hasSession(sessionID) {
		return fmt.Errorf("session %d not found", sessionID)
	}
	log.Printf("[agent-control] closing session %d: %s", sessionID, reason)
	a.CloseSession(sessionID)

	a.mu.Lock()
	delete(a.agentSessions, sessionID)
	a.mu.Unlock()
	return nil
}

// ListAgentSessions returns all still-running sessions that were spawned via
// SpawnAgentSession.
func (a *AppService) ListAgentSessions() []AgentSessionInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]AgentSessionInfo, 0, len(a.agentSessions))
	for id, info := range a.agentSessions {
		if !a.hasSession(id) {
			continue
		}
		info.Running = true
		result = append(result, info)
	}
	return result
}
