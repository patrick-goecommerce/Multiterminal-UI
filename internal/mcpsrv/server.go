// Package mcpsrv serves MTUI's agent-control MCP server: the endpoint an
// agent running in one pane uses to open, feed, read and close sessions in
// other panes.
//
// It lives next to the sessions rather than in the window, for the same reason
// the shim endpoints and the prompt queue do. An agent that delegates a task
// asks for a session to be created and then comes back to it later; with the
// server in the window, both halves end the moment somebody closes the app,
// and the delegating agent is left holding a session ID that answers to
// nobody. Served by whoever owns the sessions, the endpoint is up for as long
// as the sessions are: mtuid in daemon mode, the window in embedded mode.
//
// Everything here goes through hub.Host, which is what makes that possible:
// there is one implementation, and it does not care whether the sessions are
// in this process.
package mcpsrv

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/launch"
)

// Options configures the server.
type Options struct {
	// Host owns the sessions the tools act on.
	Host hub.Host
	// Port is the TCP port to bind on 127.0.0.1. Zero asks the OS for a free
	// one, which is the default and the right answer: see Start.
	Port int
	// Version is reported to MCP clients.
	Version string
}

// Server implements the tools over a Host.
type Server struct {
	host hub.Host
}

// URL builds the endpoint URL clients use to reach a local MCP server.
func URL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
}

// Start binds the server and serves it in the background. It returns the bound
// port.
//
// The server is bound to 127.0.0.1 and carries no auth layer, so reaching it
// *is* the authorisation. That was defensible while the port was a fixed
// machine-wide constant on a single-user desktop, and wrong the moment two
// Windows accounts shared a machine: user B's agents could drive user A's
// sessions through open_session / send_input / close_session (issue #183).
// Port 0 (the default) therefore asks the OS for a free ephemeral port and the
// result is published per-user via the discovery package, so only processes of
// the same account can find it. An explicitly configured port is still
// honoured: the user asked for it, and it is the only way to pin a URL.
func Start(opts Options) (int, error) {
	if opts.Host == nil {
		return 0, fmt.Errorf("mcpsrv: no host")
	}
	s := &Server{host: opts.Host}

	mcpSrv := server.NewMCPServer("multiterminal-ui", opts.Version,
		server.WithToolCapabilities(false))
	s.register(mcpSrv)

	mux := http.NewServeMux()
	mux.Handle("/mcp", server.NewStreamableHTTPServer(mcpSrv, server.WithStateLess(true)))

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", opts.Port))
	if err != nil {
		return 0, fmt.Errorf("mcpsrv: listen on port %d: %w", opts.Port, err)
	}
	boundPort := listener.Addr().(*net.TCPAddr).Port

	if _, err := discovery.Publish(discovery.ServiceMCP, boundPort); err != nil {
		// Without the record nothing can find this server, including MTUI's
		// own registration with the claude CLI, so this is a startup failure
		// and not a cosmetic one.
		listener.Close()
		return 0, fmt.Errorf("mcpsrv: publish port: %w", err)
	}
	log.Printf("[mcp-server] listening on %s", URL(boundPort))

	go func() {
		if err := http.Serve(listener, mux); err != nil {
			log.Printf("[mcp-server] serve error: %v", err)
		}
	}()
	return boundPort, nil
}

// register declares the tools. The descriptions are the only documentation a
// delegating agent gets, so they say when to reach for each one, not just what
// it does.
func (s *Server) register(m *server.MCPServer) {
	tools := strings.Join(launch.KnownAgents(), ", ")

	m.AddTool(mcp.NewTool("open_session",
		mcp.WithDescription("Open a new MTUI session running "+tools+" in a directory, visible as a pane in the running MTUI window. Use this to delegate a dedicated task to another CLI/model."),
		mcp.WithString("tool", mcp.Required(), mcp.Description("Which CLI to launch: "+tools)),
		mcp.WithString("dir", mcp.Required(), mcp.Description("Absolute working directory for the new session")),
		mcp.WithString("model", mcp.Description("Optional model id passed via --model")),
		mcp.WithString("prompt", mcp.Description("Optional initial prompt sent once the CLI has started")),
	), s.handleOpenSession)

	m.AddTool(mcp.NewTool("send_input",
		mcp.WithDescription("Send text as the next prompt to a running MTUI session (queued until the session is idle)."),
		mcp.WithNumber("session_id", mcp.Required(), mcp.Description("Session id returned by open_session or list_sessions")),
		mcp.WithString("text", mcp.Required(), mcp.Description("Text to send")),
	), s.handleSendInput)

	m.AddTool(mcp.NewTool("read_output",
		mcp.WithDescription("Read the current visible terminal output of an MTUI session as plain text. Use this after send_input or open_session (with a prompt) to see what the delegated CLI actually produced — list_sessions only reports whether it's still running."),
		mcp.WithNumber("session_id", mcp.Required(), mcp.Description("Session id returned by open_session or list_sessions")),
	), s.handleReadOutput)

	m.AddTool(mcp.NewTool("close_session",
		mcp.WithDescription("Close a running MTUI session."),
		mcp.WithNumber("session_id", mcp.Required(), mcp.Description("Session id to close")),
		mcp.WithString("reason", mcp.Description("Optional reason, logged only")),
	), s.handleCloseSession)

	m.AddTool(mcp.NewTool("wait_for_agent",
		mcp.WithDescription("Block until a delegated session finishes or needs a human, then return the state it reached. Use this instead of polling read_output: it returns as soon as the other agent is done (\"done\"), is waiting for a permission or an answer (\"blocked\"), or its process is gone (\"exited\")."),
		mcp.WithNumber("session_id", mcp.Required(), mcp.Description("Session id returned by open_session or list_sessions")),
		mcp.WithString("until", mcp.Description("Comma-separated states to wait for: done, blocked, idle, exited. Defaults to \"done,blocked\".")),
		mcp.WithNumber("timeout_seconds", mcp.Description("How long to wait before giving up. Defaults to 300, capped at 1800.")),
	), s.handleWaitForAgent)

	m.AddTool(mcp.NewTool("list_sessions",
		mcp.WithDescription("List sessions currently open in MTUI that were started via open_session."),
	), s.handleListSessions)
}
