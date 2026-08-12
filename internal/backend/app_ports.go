package backend

import (
	"log"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
)

// BindWarning reports a local listener that failed to come up during startup.
//
// These used to be logged and swallowed, which is how issue #183 stayed
// invisible for so long: on a shared machine the second instance lost the race
// for the old fixed ports and simply had no focus listener and no MCP server,
// with nothing in the UI to say so. Warnings are collected here and handed to
// the frontend by CheckHealth, the same pull-on-mount channel the crash/logging
// hint already uses — an event would race the frontend, which is not listening
// yet while ServiceStartup runs.
type BindWarning struct {
	// Service is a stable identifier ("focus", "mcp"), not display text.
	Service string `json:"service" yaml:"service"`
	// Detail is the underlying Go error, for the log and for the UI to append.
	Detail string `json:"detail" yaml:"detail"`
}

// startLocalListeners brings up the loopback listeners this instance owns and
// publishes their OS-assigned ports. Each failure is recorded rather than
// swallowed, so a half-started instance is visible instead of merely quiet.
func (a *AppService) startLocalListeners() {
	if err := a.startFocusListener(); err != nil {
		a.recordBindWarning("focus", err)
	}

	// The MCP server lets an agent in a pane delegate tasks by opening,
	// feeding and closing other MTUI sessions. Opt-out via config.
	if !a.cfg.ShouldRunMCPServer() {
		return
	}
	port, err := a.startMCPServer(a.cfg.MCPServer.Port)
	if err != nil {
		a.recordBindWarning("mcp", err)
		return
	}
	a.mcpServerPort = port
	go a.ensureMCPRegisteredWithClaude()
}

// recordBindWarning logs a failed listener and queues it for the UI.
func (a *AppService) recordBindWarning(service string, err error) {
	if err == nil {
		return
	}
	log.Printf("[%s] failed to start: %v", service, err)
	a.bindWarningsMu.Lock()
	defer a.bindWarningsMu.Unlock()
	a.bindWarnings = append(a.bindWarnings, BindWarning{Service: service, Detail: err.Error()})
}

// bindWarningsSnapshot returns a copy of the collected warnings.
func (a *AppService) bindWarningsSnapshot() []BindWarning {
	a.bindWarningsMu.Lock()
	defer a.bindWarningsMu.Unlock()
	if len(a.bindWarnings) == 0 {
		return []BindWarning{}
	}
	out := make([]BindWarning, len(a.bindWarnings))
	copy(out, a.bindWarnings)
	return out
}

// releaseDiscoveryRecords removes this instance's published port records on
// clean shutdown. Readers do not depend on this happening — a crash, or the
// in-app updater's os.Exit(0) (issue #184), skips it — which is why every
// record carries the owning PID and Resolve rejects records whose process is
// gone. Removing them anyway keeps the common case tidy and closes the window
// in which a reader would dial a port nobody owns.
func (a *AppService) releaseDiscoveryRecords() {
	for _, svc := range []discovery.Service{discovery.ServiceFocus, discovery.ServiceMCP} {
		if err := discovery.Remove(svc); err != nil {
			log.Printf("[discovery] could not remove %s record: %v", svc, err)
		}
	}
}
