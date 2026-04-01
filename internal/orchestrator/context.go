package orchestrator

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ContextPaths contains the file paths an agent should read.
type ContextPaths struct {
	// Direct files from the step's files_modify and files_create
	DirectFiles []string `json:"direct_files"`
	// Neighbor files in the same packages as direct files
	NeighborFiles []string `json:"neighbor_files"`
	// Test files for the affected packages
	TestFiles []string `json:"test_files"`
}

// BuildContext determines the minimal set of file paths an agent needs.
// dir is the project root. step contains files_modify and files_create.
// Returns only PATHS — the agent reads the files itself with fresh context.
func BuildContext(dir string, step PlanStep) (ContextPaths, error) {
	ctx := ContextPaths{}

	// 1. Direct files: files_modify + files_create (these are given, just verify they make sense)
	for _, f := range step.FilesModify {
		ctx.DirectFiles = append(ctx.DirectFiles, f)
	}
	for _, f := range step.FilesCreate {
		ctx.DirectFiles = append(ctx.DirectFiles, f)
	}
	sort.Strings(ctx.DirectFiles)

	// 2. Neighbor files: other .go/.ts/.svelte files in the same directory/package
	//    This gives the agent context about existing patterns and interfaces
	//    LIMIT: max 10 neighbor files per directory to prevent bloat
	neighborDirs := uniqueDirs(ctx.DirectFiles)
	for _, d := range neighborDirs {
		neighbors, err := findNeighborFiles(dir, d, ctx.DirectFiles)
		if err != nil {
			continue // non-fatal: skip if dir doesn't exist yet
		}
		ctx.NeighborFiles = append(ctx.NeighborFiles, neighbors...)
	}
	sort.Strings(ctx.NeighborFiles)

	// 3. Test files: *_test.go, *.test.ts, *.spec.ts in the same directories
	for _, d := range neighborDirs {
		tests, err := findTestFiles(dir, d)
		if err != nil {
			continue
		}
		ctx.TestFiles = append(ctx.TestFiles, tests...)
	}
	sort.Strings(ctx.TestFiles)

	return ctx, nil
}

// uniqueDirs extracts unique directory paths from file paths.
func uniqueDirs(files []string) []string {
	seen := map[string]bool{}
	var dirs []string
	for _, f := range files {
		d := filepath.Dir(f)
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	sort.Strings(dirs)
	return dirs
}

// findNeighborFiles finds source files in the same directory, excluding direct files.
// Max 10 files per directory to prevent context bloat.
func findNeighborFiles(projectDir, relDir string, exclude []string) ([]string, error) {
	absDir := filepath.Join(projectDir, relDir)
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}

	excludeSet := map[string]bool{}
	for _, f := range exclude {
		excludeSet[filepath.ToSlash(f)] = true
	}

	var neighbors []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		relPath := filepath.Join(relDir, name)
		relPathSlash := filepath.ToSlash(relPath)

		// Skip if it's a direct file
		if excludeSet[relPathSlash] {
			continue
		}

		// Skip test files (handled separately)
		if isTestFile(name) {
			continue
		}

		// Only include source files
		if isSourceFile(name) {
			neighbors = append(neighbors, relPath)
		}

		// Max 10 per directory
		if len(neighbors) >= 10 {
			break
		}
	}

	return neighbors, nil
}

// findTestFiles finds test files in a directory.
// Max 5 test files per directory.
func findTestFiles(projectDir, relDir string) ([]string, error) {
	absDir := filepath.Join(projectDir, relDir)
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}

	var tests []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if isTestFile(e.Name()) {
			tests = append(tests, filepath.Join(relDir, e.Name()))
		}
		if len(tests) >= 5 {
			break
		}
	}

	return tests, nil
}

func isSourceFile(name string) bool {
	exts := []string{".go", ".ts", ".svelte", ".js", ".tsx", ".jsx", ".py", ".rs"}
	for _, ext := range exts {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func isTestFile(name string) bool {
	return strings.HasSuffix(name, "_test.go") ||
		strings.HasSuffix(name, ".test.ts") ||
		strings.HasSuffix(name, ".test.js") ||
		strings.HasSuffix(name, ".spec.ts") ||
		strings.HasSuffix(name, ".spec.js")
}
