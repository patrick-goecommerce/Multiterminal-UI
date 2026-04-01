package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupManifestTestRepo creates a temp git repo with an initial go.mod committed.
func setupManifestTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init")
	run("git", "config", "user.name", "test")
	run("git", "config", "user.email", "test@test.com")

	gomod := "module example.com/test\n\ngo 1.21\n\nrequire (\n\tgithub.com/existing/pkg v1.0.0\n)\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644)
	run("git", "add", "-A")
	run("git", "commit", "-m", "init")

	return dir
}

func TestGoModNewDependencyAdded(t *testing.T) {
	dir := setupManifestTestRepo(t)
	gomod := "module example.com/test\n\ngo 1.21\n\nrequire (\n\tgithub.com/existing/pkg v1.0.0\n\tgithub.com/new/dep v2.0.0\n)\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644)

	changes, risk := ParseManifestChanges(context.Background(), dir, []string{"go.mod"})

	if risk != DependencyRiskLow {
		t.Errorf("expected risk=low, got %s", risk)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	c := changes[0]
	if c.Kind != "add" || c.Package != "github.com/new/dep" || c.To != "v2.0.0" {
		t.Errorf("unexpected change: %+v", c)
	}
}

func TestGoModDependencyUpdated(t *testing.T) {
	dir := setupManifestTestRepo(t)
	gomod := "module example.com/test\n\ngo 1.21\n\nrequire (\n\tgithub.com/existing/pkg v1.1.0\n)\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644)

	changes, risk := ParseManifestChanges(context.Background(), dir, []string{"go.mod"})

	if risk != DependencyRiskHigh {
		t.Errorf("expected risk=high, got %s", risk)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	c := changes[0]
	if c.Kind != "update" || c.From != "v1.0.0" || c.To != "v1.1.0" {
		t.Errorf("unexpected change: %+v", c)
	}
}

func TestGoModDependencyRemoved(t *testing.T) {
	dir := setupManifestTestRepo(t)
	gomod := "module example.com/test\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644)

	changes, risk := ParseManifestChanges(context.Background(), dir, []string{"go.mod"})

	if risk != DependencyRiskHigh {
		t.Errorf("expected risk=high, got %s", risk)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Kind != "remove" || changes[0].From != "v1.0.0" {
		t.Errorf("unexpected change: %+v", changes[0])
	}
}

func TestGoModMultipleChanges(t *testing.T) {
	dir := setupManifestTestRepo(t)
	gomod := "module example.com/test\n\ngo 1.21\n\nrequire (\n\tgithub.com/existing/pkg v1.1.0\n\tgithub.com/brand/new v0.1.0\n)\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644)

	changes, risk := ParseManifestChanges(context.Background(), dir, []string{"go.mod"})

	if risk != DependencyRiskHigh {
		t.Errorf("expected risk=high (update present), got %s", risk)
	}
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(changes))
	}
	kinds := map[string]bool{}
	for _, c := range changes {
		kinds[c.Kind] = true
	}
	if !kinds["add"] || !kinds["update"] {
		t.Errorf("expected add + update kinds, got %v", kinds)
	}
}

func TestPackageJSONNewDependency(t *testing.T) {
	dir := setupPkgJSONTestRepo(t)
	pkg := "{\n  \"dependencies\": {\n    \"express\": \"^4.18.0\",\n    \"lodash\": \"^4.17.21\"\n  }\n}\n"
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644)

	changes, risk := ParseManifestChanges(context.Background(), dir, []string{"package.json"})

	if risk != DependencyRiskLow {
		t.Errorf("expected risk=low, got %s", risk)
	}
	found := false
	for _, c := range changes {
		if c.Package == "lodash" && c.Kind == "add" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected add for lodash, got %+v", changes)
	}
}

func TestPackageJSONVersionUpdated(t *testing.T) {
	dir := setupPkgJSONTestRepo(t)
	pkg := "{\n  \"dependencies\": {\n    \"express\": \"^5.0.0\"\n  }\n}\n"
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644)

	changes, risk := ParseManifestChanges(context.Background(), dir, []string{"package.json"})

	if risk != DependencyRiskHigh {
		t.Errorf("expected risk=high, got %s", risk)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	c := changes[0]
	if c.Kind != "update" || c.From != "^4.18.0" || c.To != "^5.0.0" {
		t.Errorf("unexpected change: %+v", c)
	}
}

func TestNoManifestFiles(t *testing.T) {
	dir := setupManifestTestRepo(t)
	changes, risk := ParseManifestChanges(context.Background(), dir, []string{"main.go", "README.md"})

	if risk != DependencyRiskNone {
		t.Errorf("expected risk=none, got %s", risk)
	}
	if len(changes) != 0 {
		t.Errorf("expected no changes, got %d", len(changes))
	}
}

func TestGoSumSkippedForParsing(t *testing.T) {
	dir := setupManifestTestRepo(t)
	// go.sum is detected as manifest but produces no individual changes.
	os.WriteFile(filepath.Join(dir, "go.sum"), []byte("h1:abc123\n"), 0o644)

	changes, risk := ParseManifestChanges(context.Background(), dir, []string{"go.sum"})

	if risk != DependencyRiskNone {
		t.Errorf("expected risk=none for go.sum only, got %s", risk)
	}
	if len(changes) != 0 {
		t.Errorf("expected no parsed changes for go.sum, got %d", len(changes))
	}
}

func TestMixedGoModAndPackageJSON(t *testing.T) {
	dir := setupManifestTestRepo(t)
	// Also add package.json to the same repo.
	pkg := "{\n  \"dependencies\": {\n    \"express\": \"^4.18.0\"\n  }\n}\n"
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "add package.json")

	// Now modify both: add dep to go.mod, update dep in package.json.
	gomod := "module example.com/test\n\ngo 1.21\n\nrequire (\n\tgithub.com/existing/pkg v1.0.0\n\tgithub.com/added/dep v0.5.0\n)\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644)
	pkgNew := "{\n  \"dependencies\": {\n    \"express\": \"^5.0.0\"\n  }\n}\n"
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgNew), 0o644)

	changes, risk := ParseManifestChanges(context.Background(), dir, []string{"go.mod", "package.json"})

	if risk != DependencyRiskHigh {
		t.Errorf("expected risk=high (package.json update), got %s", risk)
	}
	if len(changes) < 2 {
		t.Fatalf("expected at least 2 changes, got %d", len(changes))
	}
}

func TestIsManifestFile(t *testing.T) {
	yes := []string{"go.mod", "go.sum", "package.json", "package-lock.json",
		"Cargo.toml", "requirements.txt", "pyproject.toml",
		"subdir/go.mod", "frontend/package.json"}
	no := []string{"main.go", "README.md", "go.mod.bak", "Makefile", "config.yaml"}

	for _, f := range yes {
		if !isManifestFile(f) {
			t.Errorf("expected %q to be a manifest file", f)
		}
	}
	for _, f := range no {
		if isManifestFile(f) {
			t.Errorf("expected %q NOT to be a manifest file", f)
		}
	}
}

// --- helpers ---

func setupPkgJSONTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.name", "test")
	gitRun(t, dir, "config", "user.email", "test@test.com")

	pkg := "{\n  \"dependencies\": {\n    \"express\": \"^4.18.0\"\n  }\n}\n"
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "init")

	return dir
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"git"}, args...)
	cmd := exec.Command(full[0], full[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
