package config

import (
	"os"
	"path/filepath"
	"testing"
)

// useTempHome points configPath() at a throwaway home directory so Load() never
// reads or rewrites the developer's real ~/.multiterminal.yaml.
func useTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)        // unix
	t.Setenv("USERPROFILE", home) // windows
	return home
}

func writeConfigYAML(t *testing.T, home, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(home, ".multiterminal.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// The default must be 0 ("ask the OS for a free port"), not the old
// machine-wide 51533 that two users on one RDP host would fight over (#183).
func TestDefaultConfigMCPPortIsOSAssigned(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MCPServer.Port != 0 {
		t.Fatalf("default MCPServer.Port = %d, want 0", cfg.MCPServer.Port)
	}
	if !cfg.ShouldRunMCPServer() {
		t.Fatal("MCP server should still be enabled by default")
	}
}

func TestLoadKeepsExplicitlyConfiguredMCPPort(t *testing.T) {
	home := useTempHome(t)
	writeConfigYAML(t, home, "mcp_server:\n  enabled: true\n  port: 51533\n")

	cfg := Load()
	if cfg.MCPServer.Port != 51533 {
		t.Fatalf("MCPServer.Port = %d, want the explicitly configured 51533", cfg.MCPServer.Port)
	}
}

func TestLoadDefaultsMissingMCPPortToZero(t *testing.T) {
	home := useTempHome(t)
	writeConfigYAML(t, home, "theme: dracula\n")

	cfg := Load()
	if cfg.MCPServer.Port != 0 {
		t.Fatalf("MCPServer.Port = %d, want 0 when unset", cfg.MCPServer.Port)
	}
}

func TestLoadResetsOutOfRangeMCPPort(t *testing.T) {
	for _, body := range []string{
		"mcp_server:\n  port: -1\n",
		"mcp_server:\n  port: 70000\n",
	} {
		home := useTempHome(t)
		writeConfigYAML(t, home, body)

		cfg := Load()
		if cfg.MCPServer.Port != 0 {
			t.Fatalf("config %q gave MCPServer.Port = %d, want 0", body, cfg.MCPServer.Port)
		}
	}
}

// Port 0 must survive a save/load round trip instead of being "healed" back to
// a fixed port on the next start.
func TestMCPPortZeroSurvivesRoundTrip(t *testing.T) {
	useTempHome(t)

	cfg := DefaultConfig()
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded := Load()
	if loaded.MCPServer.Port != 0 {
		t.Fatalf("MCPServer.Port after round trip = %d, want 0", loaded.MCPServer.Port)
	}
}
