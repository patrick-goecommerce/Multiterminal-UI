package backend

import (
	"os"
	"path/filepath"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/launch"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/skills"
)

const mtuiDir = launch.ProjectDir

// mtuiDir, ProjectConfig, loadProjectConfig and saveProjectConfig moved to
// internal/launch: the daemon has to read the per-project worktree override
// too, and it cannot import this package. See launch_delegate.go.

// SkillInfo is the frontend-facing skill descriptor (without full content).
type SkillInfo struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Category    string `json:"category" yaml:"category"`
}

// ProjectInitResult is returned after initializing an MTUI project.
type ProjectInitResult struct {
	Success bool   `json:"success" yaml:"success"`
	Error   string `json:"error" yaml:"error"`
}

// IsProjectInitialized checks if the directory has a .mtui folder.
func (a *AppService) IsProjectInitialized(dir string) bool {
	return skills.IsMTUIProject(dir)
}

// GetAllSkills returns metadata for all available skills.
func (a *AppService) GetAllSkills() []SkillInfo {
	all := skills.AllSkills()
	result := make([]SkillInfo, len(all))
	for i, s := range all {
		result[i] = SkillInfo{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Category:    s.Category,
		}
	}
	return result
}

// DetectProjectSkills scans a directory and returns IDs of detected skills.
func (a *AppService) DetectProjectSkills(dir string) []string {
	if dir == "" {
		return nil
	}
	return skills.DetectSkills(dir)
}

// GetActiveSkills returns the currently active skill IDs for a project.
// Automatically migrates legacy (pre-consolidation) skill IDs.
func (a *AppService) GetActiveSkills(dir string) []string {
	sel, err := skills.LoadSkillSelection(dir)
	if err != nil {
		return nil
	}
	return skills.MigrateLegacySkills(sel.ActiveSkills)
}

// InitProject initializes the .mtui directory and injects skills into CLAUDE.md.
func (a *AppService) InitProject(dir string, skillIDs []string) ProjectInitResult {
	if dir == "" {
		return ProjectInitResult{Error: "no directory specified"}
	}

	// Create .mtui directory
	if err := skills.InitMTUIDir(dir); err != nil {
		return ProjectInitResult{Error: "creating .mtui: " + err.Error()}
	}

	// Create .mtui/chat directory
	chatDir := filepath.Join(dir, mtuiDir, "chat")
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		return ProjectInitResult{Error: "creating chat dir: " + err.Error()}
	}

	// Save project config. Load first and only set the fields we own — a
	// re-init must not wipe a per-project override (e.g. ForceWorktrees) that
	// the user set earlier.
	cfg := loadProjectConfig(dir)
	cfg.Initialized = true
	cfg.ProjectName = filepath.Base(dir)
	if err := saveProjectConfig(dir, cfg); err != nil {
		return ProjectInitResult{Error: "writing config: " + err.Error()}
	}

	// Save skill selection
	if err := skills.SaveSkillSelection(dir, skillIDs); err != nil {
		return ProjectInitResult{Error: "saving skills: " + err.Error()}
	}

	// Inject skills into CLAUDE.md
	if len(skillIDs) > 0 {
		if err := skills.InjectIntoCLAUDEMD(dir, skillIDs); err != nil {
			return ProjectInitResult{Error: "injecting skills: " + err.Error()}
		}
	}

	return ProjectInitResult{Success: true}
}

// UpdateProjectSkills changes the active skills and re-injects into CLAUDE.md.
func (a *AppService) UpdateProjectSkills(dir string, skillIDs []string) ProjectInitResult {
	if dir == "" {
		return ProjectInitResult{Error: "no directory specified"}
	}

	// Save new selection
	if err := skills.SaveSkillSelection(dir, skillIDs); err != nil {
		return ProjectInitResult{Error: "saving skills: " + err.Error()}
	}

	// Remove old block and inject new
	skills.RemoveFromCLAUDEMD(dir)
	if len(skillIDs) > 0 {
		if err := skills.InjectIntoCLAUDEMD(dir, skillIDs); err != nil {
			return ProjectInitResult{Error: "injecting skills: " + err.Error()}
		}
	}

	return ProjectInitResult{Success: true}
}
