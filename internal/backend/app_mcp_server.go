package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// startMCPServer starts the local MCP server that exposes session
// open/send/close/list tools to any MCP-capable agent (Claude Code, Codex,
// Gemini CLI, ...) configured to connect to it, letting an agent running in
// one MTUI pane delegate a task to a fresh session in another. Bound to
// 127.0.0.1 only; there is no auth layer, since MTUI is a single-user local
// desktop app. Returns the bound port (0 on failure).
func (a *AppService) startMCPServer(port int) (int, error) {
	mcpSrv := server.NewMCPServer(
		"multiterminal-ui",
		Version,
		server.WithToolCapabilities(false),
	)

	mcpSrv.AddTool(mcp.NewTool("open_session",
		mcp.WithDescription("Open a new MTUI session running claude, codex, or gemini in a directory, visible as a pane in the running MTUI window. Use this to delegate a dedicated task to another CLI/model."),
		mcp.WithString("tool", mcp.Required(), mcp.Description("Which CLI to launch: claude, codex, or gemini")),
		mcp.WithString("dir", mcp.Required(), mcp.Description("Absolute working directory for the new session")),
		mcp.WithString("model", mcp.Description("Optional model id passed via --model")),
		mcp.WithString("prompt", mcp.Description("Optional initial prompt sent once the CLI has started")),
	), a.handleOpenSession)

	mcpSrv.AddTool(mcp.NewTool("send_input",
		mcp.WithDescription("Send text as the next prompt to a running MTUI session (queued until the session is idle)."),
		mcp.WithNumber("session_id", mcp.Required(), mcp.Description("Session id returned by open_session or list_sessions")),
		mcp.WithString("text", mcp.Required(), mcp.Description("Text to send")),
	), a.handleSendInput)

	mcpSrv.AddTool(mcp.NewTool("close_session",
		mcp.WithDescription("Close a running MTUI session."),
		mcp.WithNumber("session_id", mcp.Required(), mcp.Description("Session id to close")),
		mcp.WithString("reason", mcp.Description("Optional reason, logged only")),
	), a.handleCloseSession)

	mcpSrv.AddTool(mcp.NewTool("list_sessions",
		mcp.WithDescription("List sessions currently open in MTUI that were started via open_session."),
	), a.handleListSessions)

	httpSrv := server.NewStreamableHTTPServer(mcpSrv, server.WithStateLess(true))

	mux := http.NewServeMux()
	mux.Handle("/mcp", httpSrv)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return 0, fmt.Errorf("mcp server listen: %w", err)
	}
	boundPort := listener.Addr().(*net.TCPAddr).Port
	log.Printf("[mcp-server] listening on http://127.0.0.1:%d/mcp", boundPort)

	go func() {
		if err := http.Serve(listener, mux); err != nil {
			log.Printf("[mcp-server] serve error: %v", err)
		}
	}()
	return boundPort, nil
}

// GetMCPServerPort returns the local MCP server's bound port (0 if disabled
// or not yet started).
func (a *AppService) GetMCPServerPort() int {
	return a.mcpServerPort
}

func (a *AppService) handleOpenSession(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tool, err := req.RequireString("tool")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	dir, err := req.RequireString("dir")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	model := req.GetString("model", "")
	prompt := req.GetString("prompt", "")

	id, err := a.SpawnAgentSession(tool, dir, model, prompt)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Opened %s session %d in %q", tool, id, dir)), nil
}

func (a *AppService) handleSendInput(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := req.RequireInt("session_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	text, err := req.RequireString("text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := a.SendAgentInput(sessionID, text); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Queued input for session %d", sessionID)), nil
}

func (a *AppService) handleCloseSession(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := req.RequireInt("session_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	reason := req.GetString("reason", "")
	if err := a.CloseAgentSession(sessionID, reason); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Closed session %d", sessionID)), nil
}

func (a *AppService) handleListSessions(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(a.ListAgentSessions())
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
