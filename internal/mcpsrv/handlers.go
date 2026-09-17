package mcpsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// The MCP glue: unpack the arguments, call the method, wrap the answer. Every
// failure comes back as a tool error rather than a transport error, because a
// model can read the first and only sees a stack trace of the second.

func (s *Server) handleOpenSession(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tool, err := req.RequireString("tool")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	dir, err := req.RequireString("dir")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	id, err := s.OpenSession(tool, dir, req.GetString("model", ""), req.GetString("prompt", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Opened %s session %d in %q", tool, id, dir)), nil
}

func (s *Server) handleSendInput(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := req.RequireInt("session_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	text, err := req.RequireString("text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := s.SendInput(sessionID, text); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Queued input for session %d", sessionID)), nil
}

func (s *Server) handleReadOutput(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := req.RequireInt("session_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	output, err := s.ReadOutput(sessionID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(output), nil
}

func (s *Server) handleCloseSession(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := req.RequireInt("session_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := s.CloseSession(sessionID, req.GetString("reason", "")); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Closed session %d", sessionID)), nil
}

func (s *Server) handleWaitForAgent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := req.RequireInt("session_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var until []string
	if raw := strings.TrimSpace(req.GetString("until", "")); raw != "" {
		until = strings.Split(raw, ",")
	}
	timeout := time.Duration(req.GetInt("timeout_seconds", 0)) * time.Second

	state, err := s.WaitForAgent(ctx, sessionID, until, timeout)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(state), nil
}

func (s *Server) handleListSessions(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(s.ListSessions())
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
