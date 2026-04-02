package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCriticalFilesDetected(t *testing.T) {
	files := []string{"go.mod", "main.go", "internal/auth.go"}
	critical := findCriticalFiles(files, DefaultCriticalFilePatterns)

	found := make(map[string]bool)
	for _, c := range critical {
		found[c] = true
	}
	if !found["go.mod"] {
		t.Error("expected go.mod in critical files")
	}
	if !found["internal/auth.go"] {
		t.Error("expected internal/auth.go in critical files")
	}
	if found["main.go"] {
		t.Error("main.go should not be critical")
	}
}

func TestConflictRiskHigh(t *testing.T) {
	changedFiles := []string{"internal/backend/app.go", "main.go"}
	activeCards := map[string][]string{
		"other-card": {"internal/backend/app.go", "README.md"},
	}

	risk, surfaces := assessConflictRisk(changedFiles, activeCards)
	if risk != "high" {
		t.Errorf("expected high conflict risk, got %s", risk)
	}
	if len(surfaces) == 0 {
		t.Error("expected shared surfaces")
	}
}

func TestConflictRiskMedium(t *testing.T) {
	changedFiles := []string{"internal/backend/app.go"}
	activeCards := map[string][]string{
		"other-card": {"internal/backend/app_scan.go"},
	}

	risk, surfaces := assessConflictRisk(changedFiles, activeCards)
	if risk != "medium" {
		t.Errorf("expected medium conflict risk, got %s", risk)
	}
	if len(surfaces) == 0 {
		t.Error("expected shared surfaces for directory overlap")
	}
}

func TestConflictRiskLow(t *testing.T) {
	changedFiles := []string{"internal/backend/app.go"}
	activeCards := map[string][]string{
		"other-card": {"frontend/src/App.svelte"},
	}

	risk, _ := assessConflictRisk(changedFiles, activeCards)
	if risk != "low" {
		t.Errorf("expected low conflict risk, got %s", risk)
	}
}

func TestRedactionWorks(t *testing.T) {
	// Verify that the full secret value never appears in redacted output.
	line := `var key = "AKIAIOSFODNN7EXAMPLEFULL"`
	match := "AKIAIOSFODNN7EXAMPLEFULL"

	redacted := redactValue(line, match)
	if strings.Contains(redacted, match) {
		t.Errorf("full secret value appears in redacted output: %q", redacted)
	}
	if !strings.Contains(redacted, "****") {
		t.Errorf("expected **** in redacted output, got %q", redacted)
	}
	// Should show at most 10 chars of the match.
	if !strings.Contains(redacted, match[:10]) {
		t.Errorf("expected first 10 chars of match in redacted output, got %q", redacted)
	}
}

func TestSecretDetectedPrivateKey(t *testing.T) {
	dir := t.TempDir()
	content := "-----BEGIN RSA PRIVATE KEY-----\nMIIE...\n-----END RSA PRIVATE KEY-----\n"
	os.WriteFile(filepath.Join(dir, "key.pem"), []byte(content), 0o644)

	findings := scanSecrets(dir, []string{"key.pem"})
	if len(findings) == 0 {
		t.Fatal("expected private key finding")
	}
	if findings[0].Type != "PRIVATE_KEY" {
		t.Errorf("expected PRIVATE_KEY, got %s", findings[0].Type)
	}
}

func TestSecretDetectedDBCredentials(t *testing.T) {
	dir := t.TempDir()
	content := "DSN=postgres://admin:s3cret@localhost:5432/mydb\n"
	os.WriteFile(filepath.Join(dir, "config.env"), []byte(content), 0o644)

	findings := scanSecrets(dir, []string{"config.env"})
	if len(findings) == 0 {
		t.Fatal("expected DB credentials finding")
	}
	f := findings[0]
	if f.Type != "DB_CREDENTIALS" {
		t.Errorf("expected DB_CREDENTIALS, got %s", f.Type)
	}
	// Verify redaction — full password must not appear.
	if strings.Contains(f.Preview, "s3cret") {
		t.Errorf("password should be redacted in preview: %q", f.Preview)
	}
}

func TestNoFalsePositiveOnNormalCode(t *testing.T) {
	dir := t.TempDir()
	// Common variable names that should NOT trigger.
	content := `package main

var apiKey string
var secretKey string
func getAccessToken() string { return "" }
const maxRetries = 5
var awsRegion = "us-east-1"
`
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0o644)

	findings := scanSecrets(dir, []string{"main.go"})
	if len(findings) != 0 {
		t.Errorf("expected no false positives, got %d findings: %+v", len(findings), findings)
	}
}

func TestParseDiffStatSummary(t *testing.T) {
	tests := []struct {
		input                       string
		wantFiles, wantAdd, wantDel int
	}{
		{" 3 files changed, 45 insertions(+), 12 deletions(-)", 3, 45, 12},
		{" 1 file changed, 10 insertions(+)", 1, 10, 0},
		{" 1 file changed, 5 deletions(-)", 1, 0, 5},
		{"", 0, 0, 0},
	}
	for _, tt := range tests {
		f, a, d := parseDiffStatSummary(tt.input)
		if f != tt.wantFiles || a != tt.wantAdd || d != tt.wantDel {
			t.Errorf("parseDiffStatSummary(%q) = (%d,%d,%d), want (%d,%d,%d)",
				tt.input, f, a, d, tt.wantFiles, tt.wantAdd, tt.wantDel)
		}
	}
}

func TestCheckScopeBriefing(t *testing.T) {
	limit := ScopeLimit{MaxFiles: 10, MaxLines: 100, MaxDeletes: 50}

	if s := checkScope(5, 50, 10, limit); s != "within_limits" {
		t.Errorf("expected within_limits, got %s", s)
	}
	if s := checkScope(9, 90, 10, limit); s != "warning" {
		t.Errorf("expected warning at 90%%, got %s", s)
	}
	if s := checkScope(15, 200, 60, limit); s != "exceeded" {
		t.Errorf("expected exceeded, got %s", s)
	}
}

func TestMatchCriticalPattern(t *testing.T) {
	tests := []struct {
		file, pattern string
		want          bool
	}{
		{"go.mod", "go.mod", true},
		{"internal/router.go", "**/router.go", true},
		{"deep/nested/router.go", "**/router.go", true},
		{"router.go", "**/router.go", true},
		{"main.go", "**/router.go", false},
		{".env.local", ".env*", true},
		{".env", ".env*", true},
		{"Dockerfile", "Dockerfile", true},
		{"internal/config.go", "**/config.go", true},
	}
	for _, tt := range tests {
		got := matchCriticalPattern(tt.file, tt.pattern)
		if got != tt.want {
			t.Errorf("matchCriticalPattern(%q, %q) = %v, want %v", tt.file, tt.pattern, got, tt.want)
		}
	}
}

// --- test helpers ---

func containsReason(reasons []string, target string) bool {
	for _, r := range reasons {
		if r == target {
			return true
		}
	}
	return false
}

// ensureGitAvailable skips tests if git is not available.
func ensureGitAvailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}
