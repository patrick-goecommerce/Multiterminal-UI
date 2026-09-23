package main

import (
	"log"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/mcpsrv"
)

// startMCP serves the agent-control MCP server against the daemon's sessions
// and returns the cleanup for its port record.
//
// This is the whole point of the daemon for a delegating agent: it asks for a
// session, goes away, and comes back to it. Served from a window, both the
// session and the endpoint end when somebody closes the app, and the agent is
// left holding an ID that answers to nobody.
//
// A failure here is not fatal. The daemon's job is holding sessions; an agent
// that cannot delegate is a smaller loss than a daemon that refuses to start.
func startMCP(host hub.Host) func() {
	cfg := config.Load()
	if !cfg.ShouldRunMCPServer() {
		return func() {}
	}
	port, err := mcpsrv.Start(mcpsrv.Options{
		Host:    host,
		Port:    cfg.MCPServer.Port,
		Version: Version,
	})
	if err != nil {
		log.Printf("[mtuid] mcp server unavailable: %v", err)
		return func() {}
	}
	log.Printf("[mtuid] mcp server on %s", mcpsrv.URL(port))
	return func() { _ = discovery.Remove(discovery.ServiceMCP) }
}
