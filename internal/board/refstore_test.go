package board

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// setupTestRepo creates a temporary git repository suitable for ref operations.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s failed: %v: %s", strings.Join(args, " "), err, stderr.String())
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-m", "init")
	return dir
}

func TestWriteReadRoundtrip(t *testing.T) {
	dir := setupTestRepo(t)
	rs := NewRefStore(dir)

	content := []byte("hello kanban world")
	ref := "refs/mtui/tasks/abc123/content"

	if err := rs.WriteRef(ref, content); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}

	got, err := rs.ReadRef(ref)
	if err != nil {
		t.Fatalf("ReadRef: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestReadRefNotFound(t *testing.T) {
	dir := setupTestRepo(t)
	rs := NewRefStore(dir)

	_, err := rs.ReadRef("refs/mtui/tasks/nonexistent/content")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrRefNotFound) {
		t.Errorf("expected ErrRefNotFound, got: %v", err)
	}
}

func TestDeleteRefExisting(t *testing.T) {
	dir := setupTestRepo(t)
	rs := NewRefStore(dir)

	ref := "refs/mtui/tasks/del-me/content"
	if err := rs.WriteRef(ref, []byte("data")); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}

	if err := rs.DeleteRef(ref); err != nil {
		t.Fatalf("DeleteRef: %v", err)
	}

	exists, err := rs.RefExists(ref)
	if err != nil {
		t.Fatalf("RefExists: %v", err)
	}
	if exists {
		t.Error("ref still exists after delete")
	}
}

func TestDeleteRefNotFound(t *testing.T) {
	dir := setupTestRepo(t)
	rs := NewRefStore(dir)

	err := rs.DeleteRef("refs/mtui/tasks/ghost/content")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrRefNotFound) {
		t.Errorf("expected ErrRefNotFound, got: %v", err)
	}
}

func TestListRefsMultiple(t *testing.T) {
	dir := setupTestRepo(t)
	rs := NewRefStore(dir)

	refs := []string{
		"refs/mtui/tasks/aaa/content",
		"refs/mtui/tasks/bbb/content",
		"refs/mtui/tasks/ccc/content",
	}
	for _, ref := range refs {
		if err := rs.WriteRef(ref, []byte("data-"+ref)); err != nil {
			t.Fatalf("WriteRef(%s): %v", ref, err)
		}
	}

	got, err := rs.ListRefs("refs/mtui/tasks/")
	if err != nil {
		t.Fatalf("ListRefs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 refs, got %d: %v", len(got), got)
	}

	// Verify all expected refs are present.
	refSet := make(map[string]bool)
	for _, r := range got {
		refSet[r] = true
	}
	for _, r := range refs {
		if !refSet[r] {
			t.Errorf("missing ref %s in result", r)
		}
	}
}

func TestListRefsNoMatches(t *testing.T) {
	dir := setupTestRepo(t)
	rs := NewRefStore(dir)

	got, err := rs.ListRefs("refs/mtui/empty/")
	if err != nil {
		t.Fatalf("ListRefs: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestRefExistsTrue(t *testing.T) {
	dir := setupTestRepo(t)
	rs := NewRefStore(dir)

	ref := "refs/mtui/tasks/exists/content"
	if err := rs.WriteRef(ref, []byte("yes")); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}

	exists, err := rs.RefExists(ref)
	if err != nil {
		t.Fatalf("RefExists: %v", err)
	}
	if !exists {
		t.Error("expected ref to exist")
	}
}

func TestRefExistsFalse(t *testing.T) {
	dir := setupTestRepo(t)
	rs := NewRefStore(dir)

	exists, err := rs.RefExists("refs/mtui/tasks/nope/content")
	if err != nil {
		t.Fatalf("RefExists: %v", err)
	}
	if exists {
		t.Error("expected ref to not exist")
	}
}

func TestInvalidRefName(t *testing.T) {
	dir := setupTestRepo(t)
	rs := NewRefStore(dir)

	badRefs := []string{
		"../../../etc/passwd",
		"refs/heads/main",
		"refs/mtui/bad name/x",
		"refs/mtui/../escape",
		"refs/mtui/semi;colon",
		"refs/mtui/back`tick",
		"refs/mtui/dollar$var",
	}
	for _, ref := range badRefs {
		if err := rs.WriteRef(ref, []byte("evil")); err == nil {
			t.Errorf("expected error for ref %q, got nil", ref)
		}
		if _, err := rs.ReadRef(ref); err == nil {
			t.Errorf("expected error for ReadRef(%q), got nil", ref)
		}
		if err := rs.DeleteRef(ref); err == nil {
			t.Errorf("expected error for DeleteRef(%q), got nil", ref)
		}
		if _, err := rs.RefExists(ref); err == nil {
			t.Errorf("expected error for RefExists(%q), got nil", ref)
		}
	}
}

func TestWriteRefEmptyContent(t *testing.T) {
	dir := setupTestRepo(t)
	rs := NewRefStore(dir)

	ref := "refs/mtui/tasks/empty/content"
	if err := rs.WriteRef(ref, []byte{}); err != nil {
		t.Fatalf("WriteRef with empty content: %v", err)
	}

	got, err := rs.ReadRef(ref)
	if err != nil {
		t.Fatalf("ReadRef: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty content, got %q", got)
	}
}

func TestWriteRefLargeContent(t *testing.T) {
	dir := setupTestRepo(t)
	rs := NewRefStore(dir)

	// ~100KB of content.
	content := bytes.Repeat([]byte("A"), 100*1024)
	ref := "refs/mtui/tasks/large/content"

	if err := rs.WriteRef(ref, content); err != nil {
		t.Fatalf("WriteRef large content: %v", err)
	}

	got, err := rs.ReadRef(ref)
	if err != nil {
		t.Fatalf("ReadRef: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("large content mismatch: got len %d, want %d", len(got), len(content))
	}
}

func TestOverwriteExistingRef(t *testing.T) {
	dir := setupTestRepo(t)
	rs := NewRefStore(dir)

	ref := "refs/mtui/tasks/overwrite/content"

	if err := rs.WriteRef(ref, []byte("version1")); err != nil {
		t.Fatalf("WriteRef v1: %v", err)
	}

	if err := rs.WriteRef(ref, []byte("version2")); err != nil {
		t.Fatalf("WriteRef v2: %v", err)
	}

	got, err := rs.ReadRef(ref)
	if err != nil {
		t.Fatalf("ReadRef: %v", err)
	}
	if string(got) != "version2" {
		t.Errorf("expected version2, got %q", got)
	}
}
