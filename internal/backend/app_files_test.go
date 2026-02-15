package backend

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// ListDirectory
// ---------------------------------------------------------------------------

func TestListDirectory_BasicEntries(t *testing.T) {
	dir := t.TempDir()

	// Create dirs and files
	os.Mkdir(filepath.Join(dir, "src"), 0755)
	os.Mkdir(filepath.Join(dir, "docs"), 0755)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Readme"), 0644)

	a := newTestApp()
	entries := a.ListDirectory(dir)

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	// Directories should come first
	if !entries[0].IsDir || !entries[1].IsDir {
		t.Fatal("first two entries should be directories")
	}
	if entries[2].IsDir || entries[3].IsDir {
		t.Fatal("last two entries should be files")
	}
}

func TestListDirectory_SkipsHiddenFiles(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("secret"), 0644)
	os.Mkdir(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("hello"), 0644)

	a := newTestApp()
	entries := a.ListDirectory(dir)

	if len(entries) != 1 {
		t.Fatalf("expected 1 visible entry, got %d", len(entries))
	}
	if entries[0].Name != "visible.txt" {
		t.Fatalf("expected 'visible.txt', got %q", entries[0].Name)
	}
}

func TestListDirectory_SkipsNodeModules(t *testing.T) {
	dir := t.TempDir()

	os.Mkdir(filepath.Join(dir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(dir, "index.js"), []byte(""), 0644)

	a := newTestApp()
	entries := a.ListDirectory(dir)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (node_modules skipped), got %d", len(entries))
	}
}

func TestListDirectory_DirsFirstAlphabetical(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "zebra.txt"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte(""), 0644)
	os.Mkdir(filepath.Join(dir, "zdir"), 0755)
	os.Mkdir(filepath.Join(dir, "adir"), 0755)

	a := newTestApp()
	entries := a.ListDirectory(dir)

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	// Dirs first, alphabetical
	if entries[0].Name != "adir" {
		t.Errorf("expected first entry 'adir', got %q", entries[0].Name)
	}
	if entries[1].Name != "zdir" {
		t.Errorf("expected second entry 'zdir', got %q", entries[1].Name)
	}
	// Files second, alphabetical
	if entries[2].Name != "alpha.txt" {
		t.Errorf("expected third entry 'alpha.txt', got %q", entries[2].Name)
	}
	if entries[3].Name != "zebra.txt" {
		t.Errorf("expected fourth entry 'zebra.txt', got %q", entries[3].Name)
	}
}

func TestListDirectory_NonExistentDir(t *testing.T) {
	a := newTestApp()
	entries := a.ListDirectory("/this/does/not/exist/at/all")
	if entries != nil {
		t.Fatalf("expected nil for non-existent dir, got %v", entries)
	}
}

func TestListDirectory_PathCorrectness(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte(""), 0644)

	a := newTestApp()
	entries := a.ListDirectory(dir)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	expected := filepath.Join(dir, "test.txt")
	if entries[0].Path != expected {
		t.Fatalf("expected path %q, got %q", expected, entries[0].Path)
	}
}

// ---------------------------------------------------------------------------
// CreateDirectory
// ---------------------------------------------------------------------------

func TestCreateDirectory_Success(t *testing.T) {
	dir := t.TempDir()
	newDir := filepath.Join(dir, "a", "b", "c")

	a := newTestApp()
	errStr := a.CreateDirectory(newDir)
	if errStr != "" {
		t.Fatalf("expected success, got error: %s", errStr)
	}

	info, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("should be a directory")
	}
}

func TestCreateDirectory_AlreadyExists(t *testing.T) {
	dir := t.TempDir()

	a := newTestApp()
	errStr := a.CreateDirectory(dir)
	if errStr != "" {
		t.Fatalf("creating existing dir should succeed, got error: %s", errStr)
	}
}

// ---------------------------------------------------------------------------
// SearchFiles
// ---------------------------------------------------------------------------

func TestSearchFiles_FindsByName(t *testing.T) {
	dir := t.TempDir()

	os.Mkdir(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "src", "main_test.go"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte(""), 0644)

	a := newTestApp()
	results := a.SearchFiles(dir, "main")

	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'main', got %d", len(results))
	}
}

func TestSearchFiles_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "MyFile.TXT"), []byte(""), 0644)

	a := newTestApp()
	results := a.SearchFiles(dir, "myfile")

	if len(results) != 1 {
		t.Fatalf("expected 1 case-insensitive match, got %d", len(results))
	}
}

func TestSearchFiles_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()

	os.Mkdir(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(""), 0644)

	a := newTestApp()
	results := a.SearchFiles(dir, "config")

	if len(results) != 1 {
		t.Fatalf("expected 1 result (hidden dir skipped), got %d", len(results))
	}
	if results[0].Name != "config.yaml" {
		t.Fatalf("expected 'config.yaml', got %q", results[0].Name)
	}
}

func TestSearchFiles_SkipsNodeModules(t *testing.T) {
	dir := t.TempDir()

	os.Mkdir(filepath.Join(dir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(dir, "node_modules", "lodash.js"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "app.js"), []byte(""), 0644)

	a := newTestApp()
	results := a.SearchFiles(dir, "js")

	if len(results) != 1 {
		t.Fatalf("expected 1 result (node_modules skipped), got %d", len(results))
	}
}

func TestSearchFiles_EmptyQueryReturnsNil(t *testing.T) {
	a := newTestApp()
	if a.SearchFiles("/tmp", "") != nil {
		t.Fatal("empty query should return nil")
	}
}

func TestSearchFiles_EmptyDirReturnsNil(t *testing.T) {
	a := newTestApp()
	if a.SearchFiles("", "query") != nil {
		t.Fatal("empty dir should return nil")
	}
}

func TestSearchFiles_Limit100Results(t *testing.T) {
	dir := t.TempDir()

	// Create 120 files matching query
	for i := 0; i < 120; i++ {
		name := filepath.Join(dir, filepath.Base(t.Name())+"_match_"+filepath.Base(filepath.Join("x", string(rune('a'+i%26))))+".txt")
		_ = os.WriteFile(name, []byte(""), 0644)
	}

	a := newTestApp()
	results := a.SearchFiles(dir, "match")

	if len(results) > 100 {
		t.Fatalf("expected at most 100 results, got %d", len(results))
	}
}

func TestSearchFiles_IncludesDirectories(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "components"), 0755)

	a := newTestApp()
	results := a.SearchFiles(dir, "comp")

	if len(results) != 1 {
		t.Fatalf("expected 1 directory match, got %d", len(results))
	}
	if !results[0].IsDir {
		t.Fatal("match should be a directory")
	}
}
