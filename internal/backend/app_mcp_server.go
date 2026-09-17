package backend

import (
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/mcpsrv"
)

// The agent-control MCP server itself is internal/mcpsrv, served by whoever
// owns the sessions. This window serves it only when it owns them; in daemon
// mode mtuid does, and all that is left here is telling the user (and the
// claude CLI) where it is.

// startMCPServer starts the MCP server against this window's host.
func (a *AppService) startMCPServer(port int) (int, error) {
	return mcpsrv.Start(mcpsrv.Options{Host: a.host, Port: port, Version: Version})
}

// GetMCPServerPort returns the port an agent reaches MTUI's MCP server on:
// this window's own when it serves one, otherwise whatever the per-user
// discovery record names, which in daemon mode is mtuid's.
//
// Resolving rather than reporting zero matters for the settings dialog: "no
// MCP server" and "the MCP server is in the daemon" look identical to a user,
// and only one of them is a problem.
func (a *AppService) GetMCPServerPort() int {
	if a.mcpServerPort != 0 {
		return a.mcpServerPort
	}
	rec, err := discovery.Resolve(discovery.ServiceMCP)
	if err != nil {
		return 0
	}
	return rec.Port
}

// mcpURL is where that server answers.
func mcpURL(port int) string { return mcpsrv.URL(port) }
