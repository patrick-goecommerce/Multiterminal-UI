package tsmodels

import (
	"os"
	"path/filepath"
	"testing"
)

// repoRoot walks up to the directory holding go.mod, so the test does not care
// where it is run from.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the working directory")
		}
		dir = parent
	}
}

// modelsPath is the file Wails does not regenerate.
const modelsPath = "frontend/wailsjs/go/models.ts"

// mirrored maps a models.ts namespace to the Go package it mirrors.
var mirrored = map[string]string{
	"backend": "internal/backend",
	"config":  "internal/config",
	"board":   "internal/board",
}

// The recurring bug, as a test.
//
// Wails v3 does not regenerate models.ts, so a field added to a Go struct has
// to be added there by hand, in the class body AND in the constructor. Miss
// the constructor and Wails drops the value on deserialization: no error, no
// warning, the frontend just always reads undefined. It shipped that way once
// with status_line, and CLAUDE.md has carried a warning about it ever since.
//
// A warning in a memory file is a hope. This is the check.
func TestModelsTSHasEveryGoField(t *testing.T) {
	root := repoRoot(t)
	ts, err := ParseTS(filepath.Join(root, modelsPath))
	if err != nil {
		t.Fatalf("parsing %s: %v", modelsPath, err)
	}

	var total int
	for namespace, pkgDir := range mirrored {
		classes, ok := ts[namespace]
		if !ok {
			t.Errorf("%s has no namespace %q; did the bindings move?", modelsPath, namespace)
			continue
		}
		structs, err := ParseGoPackage(filepath.Join(root, pkgDir))
		if err != nil {
			t.Fatalf("parsing %s: %v", pkgDir, err)
		}
		for _, d := range Compare(namespace, classes, structs) {
			t.Errorf("%v", d)
			total++
		}
	}
	if total > 0 {
		t.Logf("%d field(s) out of sync. This is the status_line bug: the "+
			"frontend reads undefined and nothing tells it otherwise.", total)
	}
}

// The parser has to see the file at all. A regex that stops matching after a
// Wails upgrade would make the check above pass by finding nothing, which is
// the one way a check like this fails silently.
func TestParseTS_FindsTheClassesItIsSupposedTo(t *testing.T) {
	ts, err := ParseTS(filepath.Join(repoRoot(t), modelsPath))
	if err != nil {
		t.Fatalf("ParseTS: %v", err)
	}
	for namespace := range mirrored {
		if len(ts[namespace]) == 0 {
			t.Errorf("namespace %q parsed as empty", namespace)
		}
	}
	// Config is the class the recurring bug keeps landing in, and it is large,
	// so it is the useful canary for a parser that quietly stopped matching.
	cfg, ok := ts["config"]["Config"]
	if !ok {
		t.Fatal("config.Config was not found")
	}
	if len(cfg.Declared) < 20 {
		t.Errorf("config.Config parsed with only %d properties", len(cfg.Declared))
	}
	if len(cfg.Assigned) < 20 {
		t.Errorf("config.Config parsed with only %d constructor assignments", len(cfg.Assigned))
	}
}

func TestParseGoPackage_ReadsJSONNames(t *testing.T) {
	structs, err := ParseGoPackage(filepath.Join(repoRoot(t), "internal/config"))
	if err != nil {
		t.Fatalf("ParseGoPackage: %v", err)
	}
	fields, ok := structs["Config"]
	if !ok {
		t.Fatal("config.Config was not found")
	}
	byName := map[string]string{}
	for _, f := range fields {
		byName[f.Name] = f.GoName
	}
	// The field whose absence from models.ts was the bug CLAUDE.md documents.
	if byName["session_host"] != "SessionHost" {
		t.Errorf("session_host maps to %q, want SessionHost", byName["session_host"])
	}
	if _, snakeMissing := byName["theme"]; !snakeMissing {
		t.Error("theme was not read from the json tag")
	}
}
