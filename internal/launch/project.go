package launch

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// ProjectDir is the per-project directory MTUI keeps its settings in.
const ProjectDir = ".mtui"

// ProjectConfig holds project-specific MTUI settings stored in
// .mtui/config.json.
//
// NOTE: this file is NOT gitignored (only subdirectories like .mtui/chat/
// are), so anything stored here is committed and shared with the whole team.
type ProjectConfig struct {
	Initialized bool   `json:"initialized" yaml:"initialized"`
	ProjectName string `json:"project_name" yaml:"project_name"`
	// ForceWorktrees overrides the global config.ForceWorktrees for this
	// project. Tri-state: nil inherits the global setting, true forces
	// worktree isolation, false exempts this project even when the global
	// setting is on.
	ForceWorktrees *bool `json:"force_worktrees,omitempty" yaml:"force_worktrees,omitempty"`
}

// LoadProjectConfig reads .mtui/config.json from a project root.
//
// A missing or unparseable file is not an error: it yields the zero value,
// which means "nothing overridden here". Refusing to launch because somebody
// hand-edited a JSON file would be the wrong trade every time.
func LoadProjectConfig(dir string) ProjectConfig {
	var cfg ProjectConfig
	if dir == "" {
		return cfg
	}
	data, err := os.ReadFile(filepath.Join(dir, ProjectDir, "config.json"))
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("[launch] %s: %v, treating the project config as empty", dir, err)
		return ProjectConfig{}
	}
	return cfg
}

// SaveProjectConfig writes .mtui/config.json, creating the directory if needed.
func SaveProjectConfig(dir string, cfg ProjectConfig) error {
	if err := os.MkdirAll(filepath.Join(dir, ProjectDir), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ProjectDir, "config.json"), data, 0644)
}
