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
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
)

// mcpURL builds the endpoint URL clients use to reach the local MCP server.
func mcpURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
}

// startMCPServer starts the local MCP server that exposes session
// open/send/close/list tools to any MCP-capable agent (Claude Code, Codex,
// Gemini CLI, ...) configured to connect to it, letting an agent running in
// one MTUI pane delegate a task to a fresh session in another. Returns the
// bound port (0 on failure).
//
// The server is bound to 127.0.0.1 and carries no auth layer, so reaching it
// *is* the authorisation. That was defensible while the port was a fixed
// machine-wide constant on a single-user desktop, and wrong the moment two
// Windows accounts shared a machine: user B's agents could drive user A's
// sessions through open_session / send_input / close_session (issue #183).
// port 0 (the default) therefore asks the OS for a free ephemeral port and the
// result is published per-user via the discovery package, so only processes of
// the same Windows account can find it. An explicitly configured port is still
// honoured — the user asked for it, and it is the only way to pin a URL.
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

	mcpSrv.AddTool(mcp.NewTool("read_output",
		mcp.WithDescription("Read the current visible terminal output of an MTUI session as plain text. Use this after send_input or open_session (with a prompt) to see what the delegated CLI actually produced — list_sessions only reports whether it's still running."),
		mcp.WithNumber("session_id", mcp.Required(), mcp.Description("Session id returned by open_session or list_sessions")),
	), a.handleReadOutput)

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
		return 0, fmt.Errorf("mcp server listen on port %d: %w", port, err)
	}
	boundPort := listener.Addr().(*net.TCPAddr).Port

	if _, err := discovery.Publish(discovery.ServiceMCP, boundPort); err != nil {
		// Without the record nothing can find this server — including our own
		// registration with the claude CLI — so this is a startup failure, not
		// a cosmetic one.
		listener.Close()
		return 0, fmt.Errorf("mcp server publish port: %w", err)
	}
	log.Printf("[mcp-server] listening on %s", mcpURL(boundPort))

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

func (a *AppService) handleReadOutput(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := req.RequireInt("session_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	output, err := a.ReadAgentSessionOutput(sessionID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(output), nil
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
