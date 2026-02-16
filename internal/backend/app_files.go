package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// FileEntry represents a file or directory in the sidebar.
type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

// ListDirectory returns the contents of a directory, sorted dirs-first.
// If dir is empty, it defaults to the current working directory.
func (a *App) ListDirectory(dir string) []FileEntry {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	result := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		// Skip hidden files and common noise
		if strings.HasPrefix(name, ".") || name == "node_modules" {
			continue
		}
		result = append(result, FileEntry{
			Name:  name,
			Path:  filepath.Join(dir, name),
			IsDir: e.IsDir(),
		})
	}

	// Sort: directories first, then alphabetically
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result
}

// CreateDirectory creates a new directory (including parents) and returns
// an error string (empty on success).
func (a *App) CreateDirectory(path string) string {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err.Error()
	}
	return ""
}

// SearchFiles searches for files matching a query string in the given directory.
func (a *App) SearchFiles(dir string, query string) []FileEntry {
	if query == "" || dir == "" {
		return nil
	}
	query = strings.ToLower(query)

	var results []FileEntry
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		name := info.Name()
		// Skip hidden dirs
		if info.IsDir() && strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}
		if name == "node_modules" && info.IsDir() {
			return filepath.SkipDir
		}
		if strings.Contains(strings.ToLower(name), query) {
			results = append(results, FileEntry{
				Name:  name,
				Path:  path,
				IsDir: info.IsDir(),
			})
		}
		if len(results) >= 100 {
			return filepath.SkipAll
		}
		return nil
	})

	return results
}

// maxPreviewSize is the maximum file size (1 MB) for preview.
const maxPreviewSize = 1 << 20

// FileContent holds the result of reading a file for preview.
type FileContent struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
	Error   string `json:"error"`
	Binary  bool   `json:"binary"`
}

// ReadFile reads a file and returns its content for preview.
// Files larger than 1 MB or detected as binary are rejected.
func (a *App) ReadFile(path string) FileContent {
	info, err := os.Stat(path)
	if err != nil {
		return FileContent{Path: path, Name: filepath.Base(path), Error: err.Error()}
	}
	if info.IsDir() {
		return FileContent{Path: path, Name: info.Name(), Error: "Verzeichnis kann nicht angezeigt werden"}
	}
	size := info.Size()
	if size > maxPreviewSize {
		mb := float64(size) / (1 << 20)
		return FileContent{
			Path:  path,
			Name:  info.Name(),
			Size:  size,
			Error: fmt.Sprintf("Datei zu groß (%.1f MB, max 1 MB)", mb),
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return FileContent{Path: path, Name: info.Name(), Size: size, Error: err.Error()}
	}

	// Detect binary: check first 512 bytes for NUL
	probe := data
	if len(probe) > 512 {
		probe = probe[:512]
	}
	for _, b := range probe {
		if b == 0 {
			return FileContent{Path: path, Name: info.Name(), Size: size, Binary: true}
		}
	}

	return FileContent{
		Path:    path,
		Name:    info.Name(),
		Content: string(data),
		Size:    size,
	}
}

// OpenFileInEditor opens the file in the system default editor.
func (a *App) OpenFileInEditor(path string) string {
	cmd := exec.Command("cmd", "/c", "start", "", path)
	if err := cmd.Start(); err != nil {
		return err.Error()
	}
	return ""
}
