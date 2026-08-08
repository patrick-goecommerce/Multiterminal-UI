package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

// forcedService returns an AppService whose GLOBAL policy is on/off. No
// per-project override is written, so EffectiveForceWorktrees follows it.
func forcedService(force bool) *AppService {
	return &AppService{cfg: config.Config{ForceWorktrees: boolPtr(force)}}
}

func readMemory(t *testing.T, repo string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, projectWorktreeMemoryFile))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestWorktreeMemory_ForcedVariantIsMandatoryAndForbidsNesting(t *testing.T) {
	repo := initPaneTestRepo(t)
	if err := forcedService(true).EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}
	got := readMemory(t, repo)

	// Assert on phrases that DISTINGUISH the variants — both mention
	// EnterWorktree, so that word proves nothing.
	for _, want := range []string{
		"verpflichtend",
		"BEVOR du Code änderst",
		"keinesfalls ein zweites Worktree", // the nesting guard
		"`docs/`",                          // documentation exemption is stated
	} {
		if !strings.Contains(got, want) {
			t.Errorf("forced memory is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "um deine Änderungen zu isolieren") {
		t.Error("forced memory still carries the advisory wording")
	}
}

func TestWorktreeMemory_AdvisoryVariantWhenPolicyOff(t *testing.T) {
	repo := initPaneTestRepo(t)
	if err := forcedService(false).EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}
	if got := readMemory(t, repo); got != projectWorktreeMemoryContent {
		t.Errorf("policy off must write the advisory variant verbatim:\n%s", got)
	}
}

func TestWorktreeMemory_MigratesBothDirections(t *testing.T) {
	repo := initPaneTestRepo(t)

	if err := forcedService(false).EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}
	if got := readMemory(t, repo); got != projectWorktreeMemoryContent {
		t.Fatalf("setup did not write the advisory variant")
	}

	// off -> on
	if err := forcedService(true).EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}
	if got := readMemory(t, repo); got != projectWorktreeMemoryForcedContent {
		t.Errorf("switching the policy on did not migrate the memory file:\n%s", got)
	}

	// on -> off again
	if err := forcedService(false).EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}
	if got := readMemory(t, repo); got != projectWorktreeMemoryContent {
		t.Errorf("switching the policy off did not migrate back:\n%s", got)
	}
}

// The regression that would silently kill the feature on Windows: git with
// core.autocrlf, or any editor normalizing line endings, turns MTUI's own file
// into CRLF. Byte comparison would then classify it as user-customized and
// freeze the project on whichever variant it happened to hold.
func TestWorktreeMemory_RecognizesCRLFRoundTrip(t *testing.T) {
	repo := initPaneTestRepo(t)
	memPath := filepath.Join(repo, projectWorktreeMemoryFile)
	crlf := strings.ReplaceAll(projectWorktreeMemoryContent, "\n", "\r\n")
	if err := os.WriteFile(memPath, []byte(crlf), 0644); err != nil {
		t.Fatal(err)
	}

	if err := forcedService(true).EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}
	if got := readMemory(t, repo); got != projectWorktreeMemoryForcedContent {
		t.Errorf("a CRLF copy of MTUI's own file must still be recognized and migrated:\n%s", got)
	}
}

// Pre-marker installations must keep migrating, otherwise introducing the
// marker would strand every existing project.
func TestWorktreeMemory_MigratesPreMarkerVersions(t *testing.T) {
	for i, prior := range projectWorktreeMemoryPriorVersions {
		repo := initPaneTestRepo(t)
		memPath := filepath.Join(repo, projectWorktreeMemoryFile)
		if err := os.WriteFile(memPath, []byte(prior), 0644); err != nil {
			t.Fatal(err)
		}
		if err := forcedService(true).EnsureProjectWorktreeSetup(repo); err != nil {
			t.Fatal(err)
		}
		if got := readMemory(t, repo); got != projectWorktreeMemoryForcedContent {
			t.Errorf("prior version %d was not migrated:\n%s", i, got)
		}
	}
}

func TestWorktreeMemory_LeavesUserContentAloneInBothModes(t *testing.T) {
	const custom = "# Meine eigenen Projektnotizen\n\nHier steht etwas Eigenes.\n"

	for _, force := range []bool{true, false} {
		repo := initPaneTestRepo(t)
		memPath := filepath.Join(repo, projectWorktreeMemoryFile)
		if err := os.WriteFile(memPath, []byte(custom), 0644); err != nil {
			t.Fatal(err)
		}
		if err := forcedService(force).EnsureProjectWorktreeSetup(repo); err != nil {
			t.Fatal(err)
		}
		if got := readMemory(t, repo); got != custom {
			t.Errorf("force=%v overwrote user-customized content:\n%s", force, got)
		}
	}
}

func TestWorktreeMemory_ProjectOverrideBeatsGlobal(t *testing.T) {
	repo := initPaneTestRepo(t)

	// Global off, project forces on.
	if err := saveProjectConfig(repo, ProjectConfig{ForceWorktrees: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}
	if err := forcedService(false).EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}
	if got := readMemory(t, repo); got != projectWorktreeMemoryForcedContent {
		t.Error("project override 'on' did not win over global 'off'")
	}

	// Global on, project exempts itself.
	if err := saveProjectConfig(repo, ProjectConfig{ForceWorktrees: boolPtr(false)}); err != nil {
		t.Fatal(err)
	}
	if err := forcedService(true).EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}
	if got := readMemory(t, repo); got != projectWorktreeMemoryContent {
		t.Error("project override 'off' did not win over global 'on'")
	}
}

func TestWorktreeMemory_ExcludedFromGitStatus(t *testing.T) {
	repo := initPaneTestRepo(t)
	if err := forcedService(true).EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}
	out, err := gitCmd(repo, "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), projectWorktreeMemoryFile) {
		t.Errorf("%s must be excluded from git status, got:\n%s", projectWorktreeMemoryFile, out)
	}
}
