package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// setupTestProject creates a temp directory with a realistic project structure.
func setupTestProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// internal/backend/ — Go files + test
	mkDir(t, dir, "internal/backend")
	touch(t, dir, "internal/backend/app.go")
	touch(t, dir, "internal/backend/app_board.go")
	touch(t, dir, "internal/backend/app_board_test.go")
	touch(t, dir, "internal/backend/app_stream.go")

	// frontend/src/components/ — Svelte + TS files + test
	mkDir(t, dir, "frontend/src/components")
	touch(t, dir, "frontend/src/components/KanbanBoard.svelte")
	touch(t, dir, "frontend/src/components/KanbanCard.svelte")
	touch(t, dir, "frontend/src/components/KanbanBoard.test.ts")

	return dir
}

func mkDir(t *testing.T, base, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(base, rel), 0o755); err != nil {
		t.Fatal(err)
	}
}

func touch(t *testing.T, base, rel string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(base, rel), []byte("// stub"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildContext_DirectFiles(t *testing.T) {
	dir := setupTestProject(t)
	step := PlanStep{
		FilesModify: []string{"internal/backend/app.go"},
		FilesCreate: []string{"internal/backend/app_new.go"},
	}

	ctx, err := BuildContext(dir, step)
	if err != nil {
		t.Fatal(err)
	}

	// Both files_modify and files_create should appear in DirectFiles
	want := []string{"internal/backend/app.go", "internal/backend/app_new.go"}
	assertStringSlice(t, "DirectFiles", want, ctx.DirectFiles)
}

func TestBuildContext_NeighborFiles(t *testing.T) {
	dir := setupTestProject(t)
	step := PlanStep{
		FilesModify: []string{"internal/backend/app.go"},
	}

	ctx, err := BuildContext(dir, step)
	if err != nil {
		t.Fatal(err)
	}

	// Neighbors should include other .go source files, excluding direct and test files
	assertContains(t, "NeighborFiles", ctx.NeighborFiles, "internal/backend/app_board.go")
	assertContains(t, "NeighborFiles", ctx.NeighborFiles, "internal/backend/app_stream.go")
	assertNotContains(t, "NeighborFiles", ctx.NeighborFiles, "internal/backend/app.go")
}

func TestBuildContext_NeighborFilesExcludeTests(t *testing.T) {
	dir := setupTestProject(t)
	step := PlanStep{
		FilesModify: []string{"internal/backend/app.go"},
	}

	ctx, err := BuildContext(dir, step)
	if err != nil {
		t.Fatal(err)
	}

	assertNotContains(t, "NeighborFiles", ctx.NeighborFiles, "internal/backend/app_board_test.go")
}

func TestBuildContext_NeighborFilesMax10(t *testing.T) {
	dir := t.TempDir()
	mkDir(t, dir, "pkg")
	// Create 20 .go files
	for i := 0; i < 20; i++ {
		touch(t, dir, fmt.Sprintf("pkg/file_%02d.go", i))
	}

	step := PlanStep{
		FilesModify: []string{"pkg/file_00.go"},
	}

	ctx, err := BuildContext(dir, step)
	if err != nil {
		t.Fatal(err)
	}

	if len(ctx.NeighborFiles) > 10 {
		t.Errorf("NeighborFiles should be max 10, got %d", len(ctx.NeighborFiles))
	}
}

func TestBuildContext_TestFiles(t *testing.T) {
	dir := setupTestProject(t)
	step := PlanStep{
		FilesModify: []string{"internal/backend/app.go"},
	}

	ctx, err := BuildContext(dir, step)
	if err != nil {
		t.Fatal(err)
	}

	assertContains(t, "TestFiles", ctx.TestFiles, "internal/backend/app_board_test.go")
}

func TestBuildContext_TestFilesMax5(t *testing.T) {
	dir := t.TempDir()
	mkDir(t, dir, "pkg")
	touch(t, dir, "pkg/main.go")
	// Create 10 test files
	for i := 0; i < 10; i++ {
		touch(t, dir, fmt.Sprintf("pkg/thing_%02d_test.go", i))
	}

	step := PlanStep{
		FilesModify: []string{"pkg/main.go"},
	}

	ctx, err := BuildContext(dir, step)
	if err != nil {
		t.Fatal(err)
	}

	if len(ctx.TestFiles) > 5 {
		t.Errorf("TestFiles should be max 5, got %d", len(ctx.TestFiles))
	}
}

func TestBuildContext_NonExistentDirectory(t *testing.T) {
	dir := t.TempDir()
	step := PlanStep{
		FilesCreate: []string{"new/pkg/handler.go"},
	}

	ctx, err := BuildContext(dir, step)
	if err != nil {
		t.Fatal("should not error on non-existent directory")
	}

	// DirectFiles still listed, but no neighbors or tests
	assertContains(t, "DirectFiles", ctx.DirectFiles, "new/pkg/handler.go")
	if len(ctx.NeighborFiles) != 0 {
		t.Errorf("expected empty NeighborFiles, got %v", ctx.NeighborFiles)
	}
	if len(ctx.TestFiles) != 0 {
		t.Errorf("expected empty TestFiles, got %v", ctx.TestFiles)
	}
}

func TestBuildContext_MultipleDirectories(t *testing.T) {
	dir := setupTestProject(t)
	step := PlanStep{
		FilesModify: []string{
			"internal/backend/app.go",
			"frontend/src/components/KanbanBoard.svelte",
		},
	}

	ctx, err := BuildContext(dir, step)
	if err != nil {
		t.Fatal(err)
	}

	// Neighbors from backend
	assertContains(t, "NeighborFiles", ctx.NeighborFiles, "internal/backend/app_board.go")
	// Neighbors from frontend
	assertContains(t, "NeighborFiles", ctx.NeighborFiles, "frontend/src/components/KanbanCard.svelte")
	// Test from frontend
	assertContains(t, "TestFiles", ctx.TestFiles, "frontend/src/components/KanbanBoard.test.ts")
}

func TestBuildContext_Deterministic(t *testing.T) {
	dir := setupTestProject(t)
	step := PlanStep{
		FilesModify: []string{
			"internal/backend/app.go",
			"frontend/src/components/KanbanBoard.svelte",
		},
	}

	ctx1, _ := BuildContext(dir, step)
	ctx2, _ := BuildContext(dir, step)

	assertStringSlice(t, "DirectFiles", ctx1.DirectFiles, ctx2.DirectFiles)
	assertStringSlice(t, "NeighborFiles", ctx1.NeighborFiles, ctx2.NeighborFiles)
	assertStringSlice(t, "TestFiles", ctx1.TestFiles, ctx2.TestFiles)
}

func TestBuildContext_FrontendFiles(t *testing.T) {
	dir := setupTestProject(t)
	step := PlanStep{
		FilesModify: []string{"frontend/src/components/KanbanBoard.svelte"},
	}

	ctx, err := BuildContext(dir, step)
	if err != nil {
		t.Fatal(err)
	}

	// .svelte recognized as source
	assertContains(t, "NeighborFiles", ctx.NeighborFiles, "frontend/src/components/KanbanCard.svelte")
	// .test.ts recognized as test
	assertContains(t, "TestFiles", ctx.TestFiles, "frontend/src/components/KanbanBoard.test.ts")
}

// --- helpers ---

func assertStringSlice(t *testing.T, label string, want, got []string) {
	t.Helper()
	if len(want) != len(got) {
		t.Errorf("%s: length mismatch: want %d, got %d\n  want: %v\n  got:  %v",
			label, len(want), len(got), want, got)
		return
	}
	for i := range want {
		// Normalize to forward slashes for comparison
		w := filepath.ToSlash(want[i])
		g := filepath.ToSlash(got[i])
		if w != g {
			t.Errorf("%s[%d]: want %q, got %q", label, i, w, g)
		}
	}
}

func assertContains(t *testing.T, label string, slice []string, want string) {
	t.Helper()
	for _, s := range slice {
		if filepath.ToSlash(s) == want {
			return
		}
	}
	t.Errorf("%s should contain %q, got %v", label, want, slice)
}

func assertNotContains(t *testing.T, label string, slice []string, bad string) {
	t.Helper()
	for _, s := range slice {
		if filepath.ToSlash(s) == bad {
			t.Errorf("%s should NOT contain %q", label, bad)
			return
		}
	}
}
