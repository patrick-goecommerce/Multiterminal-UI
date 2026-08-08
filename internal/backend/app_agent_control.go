package backend

import (
	"fmt"
	"log"
	"time"
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

var agentTools = map[string]bool{"claude": true, "codex": true, "gemini": true}

// buildAgentArgv builds a plain (non-yolo/non-auto) launch command for one of
// the three supported CLI tools. Mirrors the relevant cases of
// frontend/src/lib/claude.ts's buildClaudeArgv, ported to Go since agent
// -control sessions are spawned without any frontend involvement.
func (a *AppService) buildAgentArgv(tool, model string) []string {
	var cmd string
	switch tool {
	case "claude":
		cmd = a.resolvedClaudePath
		if cmd == "" {
			cmd = a.cfg.ClaudeCommand
		}
	case "codex":
		cmd = a.resolvedCodexPath
		if cmd == "" {
			cmd = a.cfg.CodexCommand
		}
	case "gemini":
		cmd = a.resolvedGeminiPath
		if cmd == "" {
			cmd = a.cfg.GeminiCommand
		}
	default:
		return nil
	}
	if cmd == "" {
		cmd = tool
	}
	if model != "" {
		return []string{cmd, "--model", model}
	}
	return []string{cmd}
}

func agentToolDisplayName(tool string) string {
	switch tool {
	case "claude":
		return "Claude"
	case "codex":
		return "Codex"
	case "gemini":
		return "Gemini"
	default:
		return tool
	}
}

// SpawnAgentSession opens a new session running claude, codex, or gemini in
// dir, optionally queuing prompt once the CLI has started. It is the
// Go-native entry point the local MCP server (app_mcp_server.go) calls on
// behalf of an agent delegating a task to another tool; the frontend attaches
// a visible pane in response to the emitted "mtui:session-spawned" event.
func (a *AppService) SpawnAgentSession(tool, dir, model, prompt string) (int, error) {
	if !agentTools[tool] {
		return -1, fmt.Errorf("unsupported tool %q (must be claude, codex, or gemini)", tool)
	}
	if dir == "" {
		dir = a.GetWorkingDir()
	}

	argv := a.buildAgentArgv(tool, model)
	id := a.CreateSession(argv, dir, 24, 80, tool)
	if id < 0 {
		return -1, fmt.Errorf("failed to start %s session in %q", tool, dir)
	}

	a.mu.Lock()
	a.agentSessions[id] = AgentSessionInfo{ID: id, Tool: tool, Dir: dir, Model: model, Running: true}
	a.mu.Unlock()

	name := agentToolDisplayName(tool)
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
	a.mu.Lock()
	_, exists := a.sessions[sessionID]
	a.mu.Unlock()
	if !exists {
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
	a.mu.Lock()
	sess, exists := a.sessions[sessionID]
	a.mu.Unlock()
	if !exists {
		return "", fmt.Errorf("session %d not found", sessionID)
	}
	return sess.Screen.PlainText(), nil
}

// CloseAgentSession closes a running session. reason is logged only.
func (a *AppService) CloseAgentSession(sessionID int, reason string) error {
	a.mu.Lock()
	_, exists := a.sessions[sessionID]
	a.mu.Unlock()
	if !exists {
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
		if a.sessions[id] == nil {
			continue
		}
		info.Running = true
		result = append(result, info)
	}
	return result
}
