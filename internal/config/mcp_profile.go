// Package config – MCP profile definitions.
//
// Every Claude pane spawns its own stdio child process per MCP server, so a
// user with a handful of globally registered servers pays that cost N times
// over for N panes (issue #179). A profile lets a pane load only the servers
// it actually needs.
package config

import "strings"

// MCP profile sentinels. Every Claude pane carries one profile name:
//
//	MCPProfileGlobal ("") — legacy behaviour: no MCP flags at all, so the
//	    claude CLI loads every globally registered MCP server.
//	MCPProfileNone ("none") — only --strict-mcp-config, which loads ZERO
//	    servers and therefore needs no config file on disk at all.
//	any other name — must match an MCPProfile; resolves to a .mcp.json passed
//	    via --mcp-config (alongside --strict-mcp-config).
const (
	MCPProfileGlobal = ""
	MCPProfileNone   = "none"
)

// MCPProfile is a named subset of MCP servers a Claude pane may load.
//
// Exactly one source is used, ConfigPath winning when both are set:
//   - ConfigPath: a ready-made .mcp.json handed to `claude --mcp-config`.
//   - Servers: names looked up in the user's own claude MCP registration
//     (~/.claude.json / the project's .mcp.json) and materialised into a
//     generated .mcp.json (see backend.ResolveMCPProfile).
type MCPProfile struct {
	Name       string   `yaml:"name" json:"name"`
	Servers    []string `yaml:"servers,omitempty" json:"servers,omitempty"`
	ConfigPath string   `yaml:"config_path,omitempty" json:"config_path,omitempty"`
}

// FindMCPProfile returns the profile with the given name. The sentinels
// MCPProfileGlobal/MCPProfileNone are not profiles and never match.
func (c Config) FindMCPProfile(name string) (MCPProfile, bool) {
	if name == MCPProfileGlobal || name == MCPProfileNone {
		return MCPProfile{}, false
	}
	for _, p := range c.MCPProfiles {
		if p.Name == name {
			return p, true
		}
	}
	return MCPProfile{}, false
}

// normalizeMCPProfiles drops unusable profile entries and clears a default
// that points at a profile which no longer exists — a stale default would
// otherwise make every launch fail to resolve its --mcp-config.
//
// Note: unlike claude_models there is deliberately no "empty list heals back
// to defaults" rule here. An empty list is a legitimate user choice (delete
// all profiles), and re-adding the built-in one on every Load would make that
// choice impossible to persist.
func normalizeMCPProfiles(c *Config) {
	kept := c.MCPProfiles[:0]
	seen := map[string]bool{}
	for _, p := range c.MCPProfiles {
		p.Name = strings.TrimSpace(p.Name)
		if p.Name == "" || p.Name == MCPProfileNone || seen[p.Name] {
			continue
		}
		if len(p.Servers) == 0 && strings.TrimSpace(p.ConfigPath) == "" {
			continue // neither source set: would resolve to nothing
		}
		seen[p.Name] = true
		kept = append(kept, p)
	}
	c.MCPProfiles = kept

	c.DefaultMCPProfile = strings.TrimSpace(c.DefaultMCPProfile)
	if c.DefaultMCPProfile == MCPProfileGlobal || c.DefaultMCPProfile == MCPProfileNone {
		return
	}
	if _, ok := c.FindMCPProfile(c.DefaultMCPProfile); !ok {
		c.DefaultMCPProfile = MCPProfileGlobal
	}
}
