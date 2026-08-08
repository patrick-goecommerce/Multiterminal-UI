package backend

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

func policyService(global bool) *AppService {
	return &AppService{cfg: config.Config{ForceWorktrees: boolPtr(global)}}
}

func TestEffectiveForceWorktrees_FallsBackToGlobal(t *testing.T) {
	repo := initPaneTestRepo(t)

	if policyService(false).EffectiveForceWorktrees(repo) {
		t.Error("no project override: global false must win")
	}
	if !policyService(true).EffectiveForceWorktrees(repo) {
		t.Error("no project override: global true must win")
	}
}

func TestEffectiveForceWorktrees_ProjectOverrideWins(t *testing.T) {
	repo := initPaneTestRepo(t)

	if err := saveProjectConfig(repo, ProjectConfig{ForceWorktrees: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}
	if !policyService(false).EffectiveForceWorktrees(repo) {
		t.Error("project override 'on' must beat global 'off'")
	}

	if err := saveProjectConfig(repo, ProjectConfig{ForceWorktrees: boolPtr(false)}); err != nil {
		t.Fatal(err)
	}
	if policyService(true).EffectiveForceWorktrees(repo) {
		t.Error("project override 'off' must beat global 'on'")
	}
}

// The override is keyed on the main repo root, so a session running in a
// subdirectory resolves to the same project.
func TestEffectiveForceWorktrees_ResolvesFromSubdirectory(t *testing.T) {
	repo := initPaneTestRepo(t)
	sub := filepath.Join(repo, "internal", "backend")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := saveProjectConfig(repo, ProjectConfig{ForceWorktrees: boolPtr(false)}); err != nil {
		t.Fatal(err)
	}

	if policyService(true).EffectiveForceWorktrees(sub) {
		t.Error("override must apply to subdirectories of the project root")
	}
}

func TestEffectiveForceWorktrees_NonGitDirUsesGlobal(t *testing.T) {
	if !policyService(true).EffectiveForceWorktrees(t.TempDir()) {
		t.Error("outside a git repo the global setting still applies")
	}
}

func TestForceWorktreeRoot(t *testing.T) {
	repo := initPaneTestRepo(t)

	if got := policyService(false).forceWorktreeRoot(repo); got != "" {
		t.Errorf("policy off must yield no root, got %q", got)
	}
	if got := policyService(true).forceWorktreeRoot(repo); got != repo {
		t.Errorf("policy on must yield the repo root, got %q want %q", got, repo)
	}
	// Not a git repo: nothing to protect, so no env var is handed to the hook.
	if got := policyService(true).forceWorktreeRoot(t.TempDir()); got != "" {
		t.Errorf("non-git dir must yield no root, got %q", got)
	}
	if got := policyService(true).forceWorktreeRoot(""); got != "" {
		t.Errorf("empty dir must yield no root, got %q", got)
	}
}

func TestGetSetProjectForceWorktrees_RoundTrip(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := policyService(false)

	if got := a.GetProjectForceWorktrees(repo); got != forceWorktreesInherit {
		t.Errorf("fresh project should inherit, got %q", got)
	}

	for _, mode := range []string{forceWorktreesOn, forceWorktreesOff, forceWorktreesInherit} {
		if err := a.SetProjectForceWorktrees(repo, mode); err != nil {
			t.Fatalf("set %q: %v", mode, err)
		}
		if got := a.GetProjectForceWorktrees(repo); got != mode {
			t.Errorf("after setting %q, got %q", mode, got)
		}
	}

	if err := a.SetProjectForceWorktrees(repo, "vielleicht"); err == nil {
		t.Error("an invalid mode must be rejected")
	}
	if err := a.SetProjectForceWorktrees("", forceWorktreesOn); err == nil {
		t.Error("an empty dir must be rejected")
	}
}

// A re-init must not silently drop a per-project override the user set — the
// old InitProject wrote a fresh struct over the whole file.
func TestInitProject_PreservesForceWorktreesOverride(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := policyService(false)

	if err := a.SetProjectForceWorktrees(repo, forceWorktreesOff); err != nil {
		t.Fatal(err)
	}
	if res := a.InitProject(repo, nil); !res.Success {
		t.Fatalf("InitProject failed: %s", res.Error)
	}

	if got := a.GetProjectForceWorktrees(repo); got != forceWorktreesOff {
		t.Errorf("InitProject wiped the override: got %q", got)
	}
	if cfg := loadProjectConfig(repo); !cfg.Initialized || cfg.ProjectName == "" {
		t.Errorf("InitProject did not set its own fields: %+v", cfg)
	}
}

func TestLoadProjectConfig_MissingAndBroken(t *testing.T) {
	if got := loadProjectConfig(t.TempDir()); got.ForceWorktrees != nil || got.Initialized {
		t.Errorf("missing file must yield the zero value, got %+v", got)
	}

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, mtuiDir), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, mtuiDir, "config.json"), []byte("{nope"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := loadProjectConfig(dir); got.ForceWorktrees != nil {
		t.Errorf("unparseable file must yield the zero value, got %+v", got)
	}
}
