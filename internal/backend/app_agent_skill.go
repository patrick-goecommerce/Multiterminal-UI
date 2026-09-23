package backend

import (
	"log"
	"os"
	"path/filepath"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/skills"
)

// The delegation skill goes wherever the agent control goes. It explains the
// mtui MCP tools and `mt` to an agent in a pane, so it is installed with the
// MCP registration and removed when agent control is turned off: a skill
// pointing at tools that are not there sends the agent down a dead end.

// claudeUserDir is ~/.claude, where Claude Code looks for user skills.
func claudeUserDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// installDelegateSkill puts the skill in place, or brings it up to date.
func (a *AppService) installDelegateSkill() {
	if a.cfg.ClaudeEnabled != nil && !*a.cfg.ClaudeEnabled {
		return
	}
	dir, err := claudeUserDir()
	if err != nil {
		log.Printf("[agent-skill] no home directory: %v", err)
		return
	}
	changed, err := skills.InstallDelegateSkill(dir)
	if err != nil {
		log.Printf("[agent-skill] install failed: %v", err)
		return
	}
	if changed {
		log.Printf("[agent-skill] installed %s in %s", skills.DelegateSkillName, filepath.Join(dir, "skills"))
	}
}

// removeDelegateSkill takes the skill away again, unless the user owns it.
func (a *AppService) removeDelegateSkill() {
	dir, err := claudeUserDir()
	if err != nil {
		return
	}
	removed, err := skills.RemoveDelegateSkill(dir)
	if err != nil {
		log.Printf("[agent-skill] remove failed: %v", err)
		return
	}
	if removed {
		log.Printf("[agent-skill] removed %s: agent control is off", skills.DelegateSkillName)
	}
}
