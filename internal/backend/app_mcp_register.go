package backend

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// ensureMCPRegisteredWithClaude registers mtui's local MCP server with the
// Claude CLI at user scope (~/.claude.json, all projects) so the user never
// has to run `claude mcp add` by hand, and never has to repeat it per
// project — the CLI's own default scope for `claude mcp add` is "local"
// (that project directory only), which is exactly the per-project friction
// this sidesteps. Best-effort and silent: run in the background after the
// MCP server itself is already listening; any failure (claude CLI missing,
// unexpected output) is logged, never surfaced to the user.
func (a *AppService) ensureMCPRegisteredWithClaude(port int) {
	if a.cfg.ClaudeEnabled != nil && !*a.cfg.ClaudeEnabled {
		return
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)

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
