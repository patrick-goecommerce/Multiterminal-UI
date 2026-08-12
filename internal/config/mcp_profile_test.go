package config

import "testing"

func TestFindMCPProfile(t *testing.T) {
	cfg := Config{MCPProfiles: []MCPProfile{
		{Name: "Nur MTUI", Servers: []string{"mtui"}},
		{Name: "Web", ConfigPath: "~/web.mcp.json"},
	}}

	if p, ok := cfg.FindMCPProfile("Web"); !ok || p.ConfigPath != "~/web.mcp.json" {
		t.Fatalf("FindMCPProfile(Web) = %+v, %v", p, ok)
	}
	if _, ok := cfg.FindMCPProfile("nope"); ok {
		t.Fatal("unknown profile must not match")
	}
	// The sentinels are not profiles, even if a user names one after them.
	if _, ok := cfg.FindMCPProfile(MCPProfileGlobal); ok {
		t.Fatal("MCPProfileGlobal must never match a profile")
	}
	if _, ok := cfg.FindMCPProfile(MCPProfileNone); ok {
		t.Fatal("MCPProfileNone must never match a profile")
	}
}

func TestNormalizeMCPProfilesDropsUnusableEntries(t *testing.T) {
	cfg := Config{MCPProfiles: []MCPProfile{
		{Name: "  Padded  ", Servers: []string{"mtui"}},
		{Name: "", Servers: []string{"mtui"}},         // no name
		{Name: "Empty"},                               // neither servers nor path
		{Name: "none", Servers: []string{"mtui"}},     // collides with the sentinel
		{Name: "Padded", ConfigPath: "/other.json"},   // duplicate of the first
		{Name: "Path", ConfigPath: "/some/.mcp.json"}, // path-only is valid
	}}

	normalizeMCPProfiles(&cfg)

	if len(cfg.MCPProfiles) != 2 {
		t.Fatalf("expected 2 surviving profiles, got %+v", cfg.MCPProfiles)
	}
	if cfg.MCPProfiles[0].Name != "Padded" {
		t.Errorf("name not trimmed: %q", cfg.MCPProfiles[0].Name)
	}
	if cfg.MCPProfiles[0].Servers[0] != "mtui" {
		t.Errorf("first (not the duplicate) entry must win: %+v", cfg.MCPProfiles[0])
	}
	if cfg.MCPProfiles[1].Name != "Path" {
		t.Errorf("path-only profile dropped: %+v", cfg.MCPProfiles)
	}
}

func TestNormalizeMCPProfilesDefaultSelection(t *testing.T) {
	base := []MCPProfile{{Name: "Nur MTUI", Servers: []string{"mtui"}}}

	cfg := Config{MCPProfiles: base, DefaultMCPProfile: " Nur MTUI "}
	normalizeMCPProfiles(&cfg)
	if cfg.DefaultMCPProfile != "Nur MTUI" {
		t.Errorf("valid default was not kept/trimmed: %q", cfg.DefaultMCPProfile)
	}

	cfg = Config{MCPProfiles: base, DefaultMCPProfile: "deleted-profile"}
	normalizeMCPProfiles(&cfg)
	if cfg.DefaultMCPProfile != MCPProfileGlobal {
		t.Errorf("stale default must fall back to global, got %q", cfg.DefaultMCPProfile)
	}

	cfg = Config{MCPProfiles: base, DefaultMCPProfile: MCPProfileNone}
	normalizeMCPProfiles(&cfg)
	if cfg.DefaultMCPProfile != MCPProfileNone {
		t.Errorf("the none sentinel must survive, got %q", cfg.DefaultMCPProfile)
	}
}

func TestNormalizeMCPProfilesKeepsEmptyList(t *testing.T) {
	// Deleting every profile is a legitimate choice — unlike claude_models
	// there is no heal-back-to-defaults rule here.
	cfg := Config{MCPProfiles: []MCPProfile{}}
	normalizeMCPProfiles(&cfg)
	if len(cfg.MCPProfiles) != 0 {
		t.Fatalf("empty profile list must stay empty, got %+v", cfg.MCPProfiles)
	}
}

func TestDefaultConfigMCPProfiles(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DefaultMCPProfile != MCPProfileGlobal {
		t.Errorf("default must preserve pre-#179 behaviour, got %q", cfg.DefaultMCPProfile)
	}
	if _, ok := cfg.FindMCPProfile("Nur MTUI"); !ok {
		t.Errorf("expected the built-in starter profile, got %+v", cfg.MCPProfiles)
	}
}
