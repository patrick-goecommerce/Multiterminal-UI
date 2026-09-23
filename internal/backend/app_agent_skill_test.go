package backend

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/skills"
)

// useTempHome points the home directory at a temp dir, so nothing a test
// starts can write into the developer's real ~/.claude.
func useTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func delegateSkillFile(home string) string {
	return filepath.Join(home, ".claude", "skills", skills.DelegateSkillName, "SKILL.md")
}

// The skill comes with the agent control: whoever gets the MCP tools also
// gets the instructions for them.
func TestStartLocalListeners_InstallsTheDelegateSkill(t *testing.T) {
	useTempDiscoveryDir(t)
	home := useTempHome(t)

	a := newTestApp()
	a.cfg = config.DefaultConfig()
	a.cfg.MCPServer.Port = 0
	a.startLocalListeners()
	t.Cleanup(a.releaseDiscoveryRecords)

	if _, err := os.Stat(delegateSkillFile(home)); err != nil {
		t.Errorf("delegate skill was not installed: %v", err)
	}
}

// Turning agent control off takes the skill with it: it would only point an
// agent at tools that are not there.
func TestStartLocalListeners_RemovesTheSkillWhenAgentControlIsOff(t *testing.T) {
	useTempDiscoveryDir(t)
	home := useTempHome(t)
	if _, err := skills.InstallDelegateSkill(filepath.Join(home, ".claude")); err != nil {
		t.Fatal(err)
	}

	a := newTestApp()
	a.cfg = config.DefaultConfig()
	disabled := false
	a.cfg.MCPServer.Enabled = &disabled
	a.startLocalListeners()
	t.Cleanup(a.releaseDiscoveryRecords)

	if _, err := os.Stat(delegateSkillFile(home)); !os.IsNotExist(err) {
		t.Errorf("delegate skill survived with agent control off (stat err %v)", err)
	}
}

// With Claude switched off in the settings there is nobody to read it.
func TestInstallDelegateSkill_SkipsWhenClaudeIsDisabled(t *testing.T) {
	home := useTempHome(t)
	a := newTestApp()
	a.cfg = config.DefaultConfig()
	off := false
	a.cfg.ClaudeEnabled = &off
	a.installDelegateSkill()

	if _, err := os.Stat(delegateSkillFile(home)); !os.IsNotExist(err) {
		t.Errorf("skill installed although Claude is disabled (stat err %v)", err)
	}
}
