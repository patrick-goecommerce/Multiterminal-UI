package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectStack_GoMod(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)

	got := DetectStack(dir)
	if len(got) != 1 || got[0] != "go.mod" {
		t.Fatalf("expected [go.mod], got %v", got)
	}
}

func TestDetectStack_Multiple(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)

	got := DetectStack(dir)
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %v", got)
	}
	// Sorted alphabetically.
	if got[0] != "go.mod" || got[1] != "package.json" {
		t.Fatalf("expected [go.mod, package.json], got %v", got)
	}
}

func TestDetectStack_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	got := DetectStack(dir)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func writeSkillJSON(t *testing.T, dir string, name string, skill Skill) {
	t.Helper()
	data, err := json.MarshalIndent(skill, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, name), data, 0644)
}

func TestLoadSkills_ParsesCorrectly(t *testing.T) {
	dir := t.TempDir()
	writeSkillJSON(t, dir, "go.json", Skill{
		Name:     "go-backend",
		Detect:   []string{"go.mod"},
		Priority: 10,
		Policies: SkillPolicy{PreferredModel: "sonnet"},
	})

	skills, err := LoadSkills(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "go-backend" {
		t.Fatalf("expected go-backend, got %s", skills[0].Name)
	}
}

func TestLoadSkills_SortedByPriority(t *testing.T) {
	dir := t.TempDir()
	writeSkillJSON(t, dir, "a.json", Skill{Name: "low", Priority: 5})
	writeSkillJSON(t, dir, "b.json", Skill{Name: "high", Priority: 100})
	writeSkillJSON(t, dir, "c.json", Skill{Name: "mid", Priority: 50})

	skills, err := LoadSkills(dir)
	if err != nil {
		t.Fatal(err)
	}
	if skills[0].Name != "high" || skills[1].Name != "mid" || skills[2].Name != "low" {
		t.Fatalf("wrong order: %s, %s, %s", skills[0].Name, skills[1].Name, skills[2].Name)
	}
}

func TestMatchSkills_Filters(t *testing.T) {
	skills := []Skill{
		{Name: "go-backend", Detect: []string{"go.mod"}},
		{Name: "svelte", Detect: []string{"svelte.config.js"}},
		{Name: "universal", Detect: []string{}},
	}
	detected := []string{"go.mod"}

	matched := MatchSkills(detected, skills)
	if len(matched) != 2 {
		t.Fatalf("expected 2 matches (go-backend + universal), got %d", len(matched))
	}
	names := map[string]bool{}
	for _, m := range matched {
		names[m.Name] = true
	}
	if !names["go-backend"] || !names["universal"] {
		t.Fatalf("expected go-backend and universal, got %v", matched)
	}
}

func TestMatchSkills_NoMatches(t *testing.T) {
	skills := []Skill{
		{Name: "go-backend", Detect: []string{"go.mod"}},
	}
	matched := MatchSkills([]string{"package.json"}, skills)
	if len(matched) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matched))
	}
}

func TestMergePolicies_ModelConflict(t *testing.T) {
	skills := []Skill{
		{Policies: SkillPolicy{PreferredModel: "haiku"}},
		{Policies: SkillPolicy{PreferredModel: "sonnet"}},
	}
	mp := MergePolicies(skills, t.TempDir())
	if mp.PreferredModel != "sonnet" {
		t.Fatalf("expected sonnet, got %s", mp.PreferredModel)
	}
}

func TestMergePolicies_ModelOpusWins(t *testing.T) {
	skills := []Skill{
		{Policies: SkillPolicy{PreferredModel: "sonnet"}},
		{Policies: SkillPolicy{PreferredModel: "opus"}},
		{Policies: SkillPolicy{PreferredModel: "haiku"}},
	}
	mp := MergePolicies(skills, t.TempDir())
	if mp.PreferredModel != "opus" {
		t.Fatalf("expected opus, got %s", mp.PreferredModel)
	}
}

func TestMergePolicies_VerifyDeduplicated(t *testing.T) {
	skills := []Skill{
		{Policies: SkillPolicy{Verify: []VerifyStep{
			{Command: "go build ./...", Description: "Build"},
			{Command: "go test ./...", Description: "Test"},
		}}},
		{Policies: SkillPolicy{Verify: []VerifyStep{
			{Command: "go build ./...", Description: "Build again"},
			{Command: "go vet ./...", Description: "Vet"},
		}}},
	}
	mp := MergePolicies(skills, t.TempDir())
	if len(mp.Verify) != 3 {
		t.Fatalf("expected 3 unique verify steps, got %d", len(mp.Verify))
	}
}

func TestMergePolicies_ScopeLimitsStrictest(t *testing.T) {
	skills := []Skill{
		{Policies: SkillPolicy{ScopeLimits: map[string]ScopeLimit{
			"bugfix": {MaxFiles: 10, MaxLines: 200, MaxDeletes: 50},
		}}},
		{Policies: SkillPolicy{ScopeLimits: map[string]ScopeLimit{
			"bugfix": {MaxFiles: 5, MaxLines: 300, MaxDeletes: 30},
		}}},
	}
	mp := MergePolicies(skills, t.TempDir())
	lim := mp.ScopeLimits["bugfix"]
	if lim.MaxFiles != 5 {
		t.Fatalf("expected MaxFiles=5, got %d", lim.MaxFiles)
	}
	if lim.MaxLines != 200 {
		t.Fatalf("expected MaxLines=200, got %d", lim.MaxLines)
	}
	if lim.MaxDeletes != 30 {
		t.Fatalf("expected MaxDeletes=30, got %d", lim.MaxDeletes)
	}
}

func TestMergePolicies_SingleSkill(t *testing.T) {
	skills := []Skill{
		{Policies: SkillPolicy{
			PreferredModel: "sonnet",
			Verify:         []VerifyStep{{Command: "go test ./...", Description: "Test"}},
			QARules:        []string{"no-global-state"},
		}},
	}
	mp := MergePolicies(skills, t.TempDir())
	if mp.PreferredModel != "sonnet" {
		t.Fatalf("expected sonnet, got %s", mp.PreferredModel)
	}
	if len(mp.Verify) != 1 {
		t.Fatalf("expected 1 verify, got %d", len(mp.Verify))
	}
	if len(mp.QARules) != 1 || mp.QARules[0] != "no-global-state" {
		t.Fatalf("expected [no-global-state], got %v", mp.QARules)
	}
}

func TestMergePolicies_LoadsPromptFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# Skill A"), 0644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("# Skill B"), 0644)

	skills := []Skill{
		{Priority: 100, PromptFile: "a.md"},
		{Priority: 10, PromptFile: "b.md"},
	}
	mp := MergePolicies(skills, dir)
	if len(mp.SkillPrompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(mp.SkillPrompts))
	}
	if mp.SkillPrompts[0] != "# Skill A" {
		t.Fatalf("expected '# Skill A', got %q", mp.SkillPrompts[0])
	}
}

func TestMergePolicies_QARulesDeduplicated(t *testing.T) {
	skills := []Skill{
		{Policies: SkillPolicy{QARules: []string{"no-secrets", "error-wrapping"}}},
		{Policies: SkillPolicy{QARules: []string{"no-secrets", "no-sql-injection"}}},
	}
	mp := MergePolicies(skills, t.TempDir())
	if len(mp.QARules) != 3 {
		t.Fatalf("expected 3 unique QA rules, got %d: %v", len(mp.QARules), mp.QARules)
	}
}
