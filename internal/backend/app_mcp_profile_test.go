package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

// fakeHome points os.UserHomeDir at a temp dir so profile resolution neither
// reads the developer's real ~/.claude.json nor writes into their home.
func fakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir on Windows
	return home
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveMCPProfile_SentinelsNeedNoFile(t *testing.T) {
	fakeHome(t)
	app := &AppService{cfg: config.DefaultConfig()}

	for _, name := range []string{config.MCPProfileGlobal, config.MCPProfileNone, "  "} {
		path, err := app.ResolveMCPProfile("", name)
		if err != nil {
			t.Fatalf("ResolveMCPProfile(%q) errored: %v", name, err)
		}
		if path != "" {
			t.Errorf("ResolveMCPProfile(%q) = %q, want no file", name, path)
		}
	}
}

func TestResolveMCPProfile_UnknownProfileErrors(t *testing.T) {
	fakeHome(t)
	app := &AppService{cfg: config.Config{}}

	if _, err := app.ResolveMCPProfile("", "ghost"); err == nil {
		t.Fatal("expected an error for an unknown profile")
	}
}

func TestResolveMCPProfile_ConfigPath(t *testing.T) {
	home := fakeHome(t)
	existing := filepath.Join(home, "web.mcp.json")
	writeJSON(t, existing, map[string]any{"mcpServers": map[string]any{}})

	app := &AppService{cfg: config.Config{MCPProfiles: []config.MCPProfile{
		{Name: "Web", ConfigPath: "~/web.mcp.json"},
		{Name: "Gone", ConfigPath: filepath.Join(home, "missing.json")},
		// ConfigPath wins over Servers when both are set.
		{Name: "Both", ConfigPath: existing, Servers: []string{"whatever"}},
	}}}

	got, err := app.ResolveMCPProfile("", "Web")
	if err != nil {
		t.Fatalf("Web: %v", err)
	}
	if got != existing {
		t.Errorf("Web = %q, want %q (~ must be expanded)", got, existing)
	}

	if _, err := app.ResolveMCPProfile("", "Gone"); err == nil {
		t.Error("a config_path that does not exist must error, not launch silently")
	}

	if got, err := app.ResolveMCPProfile("", "Both"); err != nil || got != existing {
		t.Errorf("Both = %q, %v; want the config_path to win", got, err)
	}
}

func TestResolveMCPProfile_ServersFilterTheRegistry(t *testing.T) {
	home := fakeHome(t)
	projectDir := t.TempDir()

	// User scope (~/.claude.json) + project scope (<dir>/.mcp.json).
	writeJSON(t, filepath.Join(home, ".claude.json"), map[string]any{
		"mcpServers": map[string]any{
			"mtui":       map[string]any{"type": "http", "url": "http://127.0.0.1:51533/mcp"},
			"playwright": map[string]any{"command": "npx", "args": []string{"@playwright/mcp"}},
		},
	})
	writeJSON(t, filepath.Join(projectDir, ".mcp.json"), map[string]any{
		"mcpServers": map[string]any{
			"projectonly": map[string]any{"command": "node", "args": []string{"server.js"}},
		},
	})

	app := &AppService{cfg: config.Config{MCPProfiles: []config.MCPProfile{
		{Name: "Nur MTUI", Servers: []string{"mtui", "projectonly", "not-registered"}},
	}}}

	path, err := app.ResolveMCPProfile(projectDir, "Nur MTUI")
	if err != nil {
		t.Fatalf("ResolveMCPProfile: %v", err)
	}
	if !strings.HasPrefix(path, filepath.Join(home, ".multiterminal", "mcp-profiles")) {
		t.Errorf("generated file landed outside the profile cache: %q", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("generated file unreadable: %v", err)
	}
	var out struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("generated file is not valid .mcp.json: %v (%s)", err, data)
	}
	if len(out.MCPServers) != 2 {
		t.Fatalf("expected exactly the 2 known servers, got %v", keysOf(out.MCPServers))
	}
	if _, ok := out.MCPServers["mtui"]; !ok {
		t.Error("user-scope server missing")
	}
	if _, ok := out.MCPServers["projectonly"]; !ok {
		t.Error("project-scope server missing")
	}
	if _, ok := out.MCPServers["playwright"]; ok {
		t.Error("a server outside the profile leaked in — that is the whole point of #179")
	}
	// The definition must be copied verbatim so transports/env survive.
	if !strings.Contains(string(out.MCPServers["mtui"]), "127.0.0.1:51533") {
		t.Errorf("server definition was not copied verbatim: %s", out.MCPServers["mtui"])
	}
}

func TestResolveMCPProfile_LocalScopeOverridesUserScope(t *testing.T) {
	home := fakeHome(t)
	projectDir := t.TempDir()

	writeJSON(t, filepath.Join(home, ".claude.json"), map[string]any{
		"mcpServers": map[string]any{"srv": map[string]any{"scope": "user"}},
		"projects": map[string]any{
			projectDir: map[string]any{
				"mcpServers": map[string]any{"srv": map[string]any{"scope": "local"}},
			},
		},
	})

	app := &AppService{cfg: config.Config{MCPProfiles: []config.MCPProfile{
		{Name: "P", Servers: []string{"srv"}},
	}}}

	path, err := app.ResolveMCPProfile(projectDir, "P")
	if err != nil {
		t.Fatalf("ResolveMCPProfile: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"scope": "local"`) {
		t.Errorf("local scope must win over user scope, got %s", data)
	}
}

func TestMCPProfileFileNameIsSafeAndStable(t *testing.T) {
	a := mcpProfileFileName("Nur MTUI")
	if a != mcpProfileFileName("Nur MTUI") {
		t.Fatal("file name must be stable across calls")
	}
	if strings.ContainsAny(a, `/\: *?"<>|`) {
		t.Errorf("unsafe file name: %q", a)
	}
	// Names that sanitise identically must not collide.
	if mcpProfileFileName("a b") == mcpProfileFileName("a_b") {
		t.Error("distinct profile names collided")
	}
	if !strings.HasSuffix(mcpProfileFileName("###"), ".json") {
		t.Error("expected a .json file name even for an all-punctuation profile")
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
