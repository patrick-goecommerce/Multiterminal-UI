package backend

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

// MCP profiles (issue #179): every Claude pane starts its own stdio child
// process per MCP server, so N panes × M servers child processes. A pane's
// profile narrows that set down; this file turns a profile name into the
// concrete .mcp.json path handed to `claude --mcp-config`.

// mcpServerSet is the shape shared by ~/.claude.json's per-project entries and
// any .mcp.json: a map of server name → opaque server definition. The
// definitions are copied verbatim (json.RawMessage), so MTUI never has to know
// about transports, env vars or auth blocks.
type mcpServerSet struct {
	MCPServers map[string]json.RawMessage `json:"mcpServers"`
}

// claudeUserConfig is the part of ~/.claude.json we care about: user-scope
// servers plus the per-project ("local" scope) ones.
type claudeUserConfig struct {
	MCPServers map[string]json.RawMessage `json:"mcpServers"`
	Projects   map[string]mcpServerSet    `json:"projects"`
}

// mcpProfileDir is where generated per-profile .mcp.json files are written.
// They are regenerated on every launch, so the directory is a cache, not
// user-editable state.
func mcpProfileDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".multiterminal", "mcp-profiles")
}

// mcpProfileFileName maps a profile name to a stable, filesystem-safe file
// name. The hash suffix keeps names that sanitise identically ("a b" vs "a_b")
// apart.
func mcpProfileFileName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	slug := strings.Trim(b.String(), "_")
	if slug == "" {
		slug = "profile"
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return fmt.Sprintf("%s-%08x.json", slug, h.Sum32())
}

// expandHomePath resolves a leading ~ so users can write config paths the way
// they do everywhere else in the YAML.
func expandHomePath(p string) string {
	p = strings.TrimSpace(p)
	if p != "~" && !strings.HasPrefix(p, "~/") && !strings.HasPrefix(p, `~\`) {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimLeft(p[1:], `/\`))
}

// readMCPServerSet reads the "mcpServers" object out of a .mcp.json-shaped
// file. A missing or malformed file yields nil — MCP config is best-effort
// input, never a hard failure of the launch path.
func readMCPServerSet(path string) map[string]json.RawMessage {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var set mcpServerSet
	if err := json.Unmarshal(data, &set); err != nil {
		return nil
	}
	return set.MCPServers
}

// mcpRegistry collects every MCP server the claude CLI would know about for
// dir, merged in the CLI's own precedence order (user < project < local).
func mcpRegistry(dir string) map[string]json.RawMessage {
	merged := map[string]json.RawMessage{}

	var local map[string]json.RawMessage
	if home, err := os.UserHomeDir(); err == nil {
		if data, err := os.ReadFile(filepath.Join(home, ".claude.json")); err == nil {
			var uc claudeUserConfig
			if err := json.Unmarshal(data, &uc); err == nil {
				for name, def := range uc.MCPServers {
					merged[name] = def
				}
				local = projectScopeServers(uc, dir)
			}
		}
	}

	if dir != "" {
		for name, def := range readMCPServerSet(filepath.Join(dir, ".mcp.json")) {
			merged[name] = def
		}
	}
	for name, def := range local {
		merged[name] = def
	}
	return merged
}

// projectScopeServers looks up the "local" scope entry for dir in
// ~/.claude.json's projects map. Keys are absolute paths written by the CLI,
// so they are compared cleaned and case-insensitively (Windows).
func projectScopeServers(uc claudeUserConfig, dir string) map[string]json.RawMessage {
	if dir == "" || len(uc.Projects) == 0 {
		return nil
	}
	want := strings.ToLower(filepath.Clean(dir))
	for key, set := range uc.Projects {
		if strings.ToLower(filepath.Clean(key)) == want {
			return set.MCPServers
		}
	}
	return nil
}

// ResolveMCPProfile turns a pane's MCP profile name into the path of the
// .mcp.json that should be passed to `claude --mcp-config`.
//
// Returns "" (and no error) for the two sentinels that need no file:
// config.MCPProfileGlobal ("", inherit everything — the caller adds no MCP
// flags at all) and config.MCPProfileNone ("none", --strict-mcp-config alone
// already loads zero servers). Any other name must exist in the config.
func (a *AppService) ResolveMCPProfile(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == config.MCPProfileGlobal || name == config.MCPProfileNone {
		return "", nil
	}

	profile, ok := a.cfg.FindMCPProfile(name)
	if !ok {
		return "", fmt.Errorf("unknown MCP profile %q", name)
	}

	if p := expandHomePath(profile.ConfigPath); p != "" {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", fmt.Errorf("MCP profile %q: %w", name, err)
		}
		if _, err := os.Stat(abs); err != nil {
			return "", fmt.Errorf("MCP profile %q: config file not readable: %w", name, err)
		}
		return abs, nil
	}

	return writeMCPProfileFile(name, profile.Servers, mcpRegistry(dir))
}

// writeMCPProfileFile materialises the named subset of registry into a
// generated .mcp.json and returns its path. Servers the user listed but that
// are not registered anywhere are skipped (logged, not fatal): the pane then
// simply runs with fewer servers instead of failing to launch.
func writeMCPProfileFile(name string, servers []string, registry map[string]json.RawMessage) (string, error) {
	picked := map[string]json.RawMessage{}
	var missing []string
	for _, s := range servers {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if def, ok := registry[s]; ok {
			picked[s] = def
		} else {
			missing = append(missing, s)
		}
	}
	if len(missing) > 0 {
		log.Printf("[mcp-profile] %q: unknown MCP server(s) %s — skipped", name, strings.Join(missing, ", "))
	}

	dir := mcpProfileDir()
	if dir == "" {
		return "", fmt.Errorf("MCP profile %q: cannot resolve home directory", name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("MCP profile %q: %w", name, err)
	}

	data, err := json.MarshalIndent(mcpServerSet{MCPServers: picked}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("MCP profile %q: %w", name, err)
	}
	path := filepath.Join(dir, mcpProfileFileName(name))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("MCP profile %q: %w", name, err)
	}
	return path, nil
}
