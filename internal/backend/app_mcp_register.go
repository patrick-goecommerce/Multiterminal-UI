package backend

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
)

// ensureMCPRegisteredWithClaude registers mtui's local MCP server with the
// Claude CLI at user scope (~/.claude.json, all projects) so the user never
// has to run `claude mcp add` by hand, and never has to repeat it per
// project — the CLI's own default scope for `claude mcp add` is "local"
// (that project directory only), which is exactly the per-project friction
// this sidesteps. Best-effort and silent: run in the background after the
// MCP server itself is already listening; any failure (claude CLI missing,
// unexpected output) is logged, never surfaced to the user.
//
// The URL is read back from the per-user discovery record rather than passed
// in, so the registration and every other consumer resolve the port through
// exactly one source of truth. That matters because ~/.claude.json is written
// at user scope: registering a port that is not this user's own listener is
// how an agent would end up driving another account's MTUI (issue #183).
func (a *AppService) ensureMCPRegisteredWithClaude() {
	if a.cfg.ClaudeEnabled != nil && !*a.cfg.ClaudeEnabled {
		return
	}

	rec, err := discovery.Resolve(discovery.ServiceMCP)
	if err != nil {
		log.Printf("[mcp-register] no usable MCP port record, skipping registration: %v", err)
		return
	}
	url := mcpURL(rec.Port)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	getCmd := wrapClaudeCmdContext(ctx, a.resolvedClaudePath, []string{"mcp", "get", "mtui"})
	hideConsole(getCmd)
	out, err := getCmd.CombinedOutput()
	if err == nil && strings.Contains(string(out), url) {
		return // already registered with the current URL — nothing to do
	}

	if err == nil {
		// Registered, but under a stale URL (the port changed since the last
		// run, e.g. a config edit) — replace it rather than leaving two.
		removeCmd := wrapClaudeCmdContext(ctx, a.resolvedClaudePath, []string{"mcp", "remove", "mtui", "-s", "user"})
		hideConsole(removeCmd)
		if out, err := removeCmd.CombinedOutput(); err != nil {
			log.Printf("[mcp-register] failed to remove stale registration: %v (%s)", err, out)
			return
		}
	}

	addCmd := wrapClaudeCmdContext(ctx, a.resolvedClaudePath, []string{
		"mcp", "add", "--transport", "http", "--scope", "user", "mtui", url,
	})
	hideConsole(addCmd)
	if out, err := addCmd.CombinedOutput(); err != nil {
		log.Printf("[mcp-register] failed to register with claude CLI: %v (%s)", err, out)
		return
	}
	log.Printf("[mcp-register] registered mtui MCP server with claude CLI (user scope): %s", url)
}
