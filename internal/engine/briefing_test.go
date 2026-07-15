package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupBriefingRepo creates a temp git repo with a committed Go file.
func setupBriefingRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.name", "test")
	gitRun(t, dir, "config", "user.email", "test@test.com")

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0o644)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "init")
	return dir
}

func TestCleanBriefing(t *testing.T) {
	dir := setupBriefingRepo(t)
	// Make a small harmless change.
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {\n\t// hello\n}\n"), 0o644)

	b := BuildBriefing(context.Background(), dir, "card-1", []string{"main.go"}, nil, nil)

	if b.Recommendation != "proceed_to_qa" {
		t.Errorf("expected proceed_to_qa, got %s", b.Recommendation)
	}
	if len(b.SecretsFound) != 0 {
		t.Errorf("expected no secrets, got %d", len(b.SecretsFound))
	}
	if b.ConflictRisk != "low" {
		t.Errorf("expected conflict risk low, got %s", b.ConflictRisk)
	}
}

func TestSecretDetectedAWSKey(t *testing.T) {
	dir := setupBriefingRepo(t)
	secret := "package main\n\nvar key = \"AKIAIOSFODNN7EXAMPLE\"\n"
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(secret), 0o644)

	b := BuildBriefing(context.Background(), dir, "card-2", []string{"main.go"}, nil, nil)

	if b.Recommendation != "needs_human_review" {
		t.Errorf("expected needs_human_review, got %s", b.Recommendation)
	}
	if len(b.SecretsFound) == 0 {
		t.Fatal("expected at least one secret finding")
	}
	if b.SecretsFound[0].Type != "AWS_KEY" {
		t.Errorf("expected type AWS_KEY, got %s", b.SecretsFound[0].Type)
	}
	if !containsReason(b.Reasons, "secret_detected") {
		t.Errorf("expected reason secret_detected, got %v", b.Reasons)
	}
}

func TestSecretDetectedGitHubToken(t *testing.T) {
	dir := setupBriefingRepo(t)
	// ghp_ token is 36 chars.
	token := "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"
	content := "package main\n\nvar token = \"" + token + "\"\n"
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0o644)

	b := BuildBriefing(context.Background(), dir, "card-3", []string{"main.go"}, nil, nil)

	if len(b.SecretsFound) == 0 {
		t.Fatal("expected GitHub token finding")
	}
	f := b.SecretsFound[0]
	if f.Type != "GITHUB_TOKEN" {
		t.Errorf("expected GITHUB_TOKEN, got %s", f.Type)
	}
	// Verify the preview is redacted (max 10 chars of match + ****)
	if !strings.Contains(f.Preview, "****") {
		t.Errorf("expected redacted preview, got %q", f.Preview)
	}
	// Full token must not appear in preview.
	if strings.Contains(f.Preview, token) {
		t.Errorf("full token should not appear in preview: %q", f.Preview)
	}
}

func TestScopeExceeded(t *testing.T) {
	dir := setupBriefingRepo(t)
	// Write a file with many lines to exceed scope.
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, "// line")
	}
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)

	limit := &ScopeLimit{MaxFiles: 1, MaxLines: 10, MaxDeletes: 5}
	b := BuildBriefing(context.Background(), dir, "card-scope", []string{"main.go"}, nil, limit)

	if b.ScopeStatus != "exceeded" {
		t.Errorf("expected scope exceeded, got %s", b.ScopeStatus)
	}
	if b.Recommendation != "needs_human_review" {
		t.Errorf("expected needs_human_review, got %s", b.Recommendation)
	}
	if !containsReason(b.Reasons, "scope_exceeded") {
		t.Errorf("expected reason scope_exceeded, got %v", b.Reasons)
	}
}

func TestMassDeletes(t *testing.T) {
	dir := setupBriefingRepo(t)
	// Start with a large file, then delete most of it.
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "// line "+strings.Repeat("x", 10))
	}
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "add big file")

	// Now delete most lines.
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)

	b := BuildBriefing(context.Background(), dir, "card-del", []string{"main.go"}, nil, nil)

	if !b.MassDeletes {
		t.Errorf("expected mass_deletes=true, got false (added=%d, deleted=%d)", b.LinesAdded, b.LinesDeleted)
	}
	if b.Recommendation != "needs_human_review" {
		t.Errorf("expected needs_human_review, got %s", b.Recommendation)
	}
}

func TestDependencyRiskFromManifest(t *testing.T) {
	dir := setupBriefingRepo(t)
	// Update go.mod to add a new dependency -- stage it so git diff HEAD sees it.
	gomod := "module example.com/test\n\ngo 1.21\n\nrequire (\n\tgithub.com/new/dep v1.0.0\n)\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644)
	gitRun(t, dir, "add", "go.mod")

	b := BuildBriefing(context.Background(), dir, "card-dep", []string{"go.mod"}, nil, nil)

	if b.DependencyRisk == DependencyRiskNone {
		t.Errorf("expected non-none dependency risk, got %s", b.DependencyRisk)
	}
	if len(b.ManifestChanges) == 0 {
		t.Error("expected manifest changes")
	}
}

func TestMultipleReasons(t *testing.T) {
	dir := setupBriefingRepo(t)
	// Write a file with a secret.
	content := "var key = \"AKIAIOSFODNN7EXAMPLE\"\n"
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0o644)

	// Use a tight scope limit to trigger scope_exceeded.
	limit := &ScopeLimit{MaxFiles: 1, MaxLines: 1, MaxDeletes: 1}
	b := BuildBriefing(context.Background(), dir, "card-multi", []string{"main.go"}, nil, limit)

	if !containsReason(b.Reasons, "secret_detected") {
		t.Errorf("expected secret_detected in reasons: %v", b.Reasons)
	}
	if !containsReason(b.Reasons, "scope_exceeded") {
		t.Errorf("expected scope_exceeded in reasons: %v", b.Reasons)
	}
	if b.Recommendation != "needs_human_review" {
		t.Errorf("expected needs_human_review, got %s", b.Recommendation)
	}
}
