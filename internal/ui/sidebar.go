package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileEntry represents a file or directory in the sidebar tree.
type FileEntry struct {
	Name     string
	Path     string
	IsDir    bool
	Children []FileEntry
	Expanded bool
	Depth    int
}

// Sidebar holds the state of the file browser sidebar.
type Sidebar struct {
	Visible  bool
	Focused  bool   // when true, arrow keys navigate the sidebar instead of panes
	RootDir  string
	Entries  []FileEntry // flat list of currently visible entries
	Selected int         // index into Entries
	Search   string      // current filter text
	Editing  bool        // true when the search input is focused
	Width    int
}

// NewSidebar creates a sidebar rooted at dir.
func NewSidebar(dir string, width int) Sidebar {
	sb := Sidebar{
		RootDir: dir,
		Width:   width,
	}
	sb.Refresh()
	return sb
}

// Refresh re-reads the root directory and rebuilds the entry list.
func (sb *Sidebar) Refresh() {
	root := readDir(sb.RootDir, 0, 2) // read 2 levels deep initially
	sb.Entries = flattenEntries(root, sb.Search)
	if sb.Selected >= len(sb.Entries) {
		sb.Selected = len(sb.Entries) - 1
	}
	if sb.Selected < 0 {
		sb.Selected = 0
	}
}

// Toggle expands or collapses the currently selected directory entry.
func (sb *Sidebar) Toggle() {
	if sb.Selected < 0 || sb.Selected >= len(sb.Entries) {
		return
	}
	entry := &sb.Entries[sb.Selected]
	if !entry.IsDir {
		return
	}
	entry.Expanded = !entry.Expanded
	if entry.Expanded && len(entry.Children) == 0 {
		entry.Children = readDir(entry.Path, entry.Depth+1, 1)
	}
	sb.Entries = flattenFromRoot(sb.Entries, sb.Search)
}

// MoveUp moves the selection cursor up.
func (sb *Sidebar) MoveUp() {
	if sb.Selected > 0 {
		sb.Selected--
	}
}

// MoveDown moves the selection cursor down.
func (sb *Sidebar) MoveDown() {
	if sb.Selected < len(sb.Entries)-1 {
		sb.Selected++
	}
}

// SelectedPath returns the full path of the currently selected entry.
func (sb *Sidebar) SelectedPath() string {
	if sb.Selected < 0 || sb.Selected >= len(sb.Entries) {
		return ""
	}
	return sb.Entries[sb.Selected].Path
}

// SelectedEntry returns the currently selected FileEntry, or nil.
func (sb *Sidebar) SelectedEntry() *FileEntry {
	if sb.Selected < 0 || sb.Selected >= len(sb.Entries) {
		return nil
	}
	return &sb.Entries[sb.Selected]
}

// IsSelectedDir reports whether the currently selected entry is a directory.
func (sb *Sidebar) IsSelectedDir() bool {
	e := sb.SelectedEntry()
	return e != nil && e.IsDir
}

// Render draws the sidebar as a string.
func (sb *Sidebar) Render(height int) string {
	if !sb.Visible {
		return ""
	}

	var b strings.Builder
	maxW := sb.Width - 3 // account for border + padding

	// Title (shows focus indicator)
	titleText := "Files"
	if sb.Focused {
		titleText = "Files [ACTIVE]"
	}
	title := SidebarTitle.Render(titleText)
	b.WriteString(title)
	b.WriteByte('\n')

	// Search bar
	if sb.Search != "" || sb.Editing {
		searchLabel := SidebarSearch.Render("/ " + sb.Search + "█")
		b.WriteString(searchLabel)
		b.WriteByte('\n')
		height -= 2
	}
	height -= 2 // title + bottom padding

	// Scroll offset
	offset := 0
	if sb.Selected >= height {
		offset = sb.Selected - height + 1
	}

	for i := offset; i < len(sb.Entries) && i-offset < height; i++ {
		entry := sb.Entries[i]
		indent := strings.Repeat("  ", entry.Depth)

		var icon string
		if entry.IsDir {
			if entry.Expanded {
				icon = "▾ "
			} else {
				icon = "▸ "
			}
		} else {
			icon = fileIcon(entry.Name) + " "
		}

		name := entry.Name
		if len(indent)+len(icon)+len(name) > maxW {
			avail := maxW - len(indent) - len(icon) - 1
			if avail > 0 {
				name = name[:avail] + "…"
			}
		}

		line := indent + icon + name
		if i == sb.Selected {
			line = SidebarSelected.Render(line)
		} else if entry.IsDir {
			line = SidebarDir.Render(line)
		} else {
			line = SidebarFile.Render(line)
		}

		b.WriteString(line)
		if i-offset < height-1 {
			b.WriteByte('\n')
		}
	}

	return SidebarStyle.Width(sb.Width).Height(height + 4).Render(b.String())
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// readDir reads a directory and returns FileEntry children up to maxDepth
// additional levels.
func readDir(dir string, depth, maxDepth int) []FileEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var result []FileEntry
	// Sort: directories first, then alphabetical
	sort.Slice(entries, func(i, j int) bool {
		di, dj := entries[i].IsDir(), entries[j].IsDir()
		if di != dj {
			return di
		}
		return entries[i].Name() < entries[j].Name()
	})

	for _, e := range entries {
		// Skip hidden files and common noise
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__" || name == "vendor" {
			continue
		}

		fe := FileEntry{
			Name:  name,
			Path:  filepath.Join(dir, name),
			IsDir: e.IsDir(),
			Depth: depth,
		}

		if e.IsDir() && maxDepth > 0 {
			fe.Children = readDir(fe.Path, depth+1, maxDepth-1)
			fe.Expanded = true
		}

		result = append(result, fe)
	}
	return result
}

// flattenEntries flattens a tree of FileEntry into a linear list,
// only including expanded directories' children.
// If filter is non-empty, only entries whose name contains filter are shown.
func flattenEntries(entries []FileEntry, filter string) []FileEntry {
	var flat []FileEntry
	filter = strings.ToLower(filter)
	for _, e := range entries {
		if filter != "" && !strings.Contains(strings.ToLower(e.Name), filter) && !e.IsDir {
			continue
		}
		flat = append(flat, e)
		if e.IsDir && e.Expanded {
			flat = append(flat, flattenEntries(e.Children, filter)...)
		}
	}
	return flat
}

// flattenFromRoot re-flattens the existing entry list preserving expand state.
func flattenFromRoot(entries []FileEntry, filter string) []FileEntry {
	return flattenEntries(entries, filter)
}

// IsImage reports whether the filename has an image extension.
func IsImage(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".svg", ".webp", ".ico", ".tiff":
		return true
	}
	return false
}

// fileIcon returns a unicode icon for common file types.
func fileIcon(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	// Images
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".svg", ".webp", ".ico", ".tiff":
		return "\U0001F5BC" // framed picture
	// Code
	case ".go", ".rs", ".py", ".js", ".ts", ".jsx", ".tsx", ".c", ".cpp", ".h", ".java", ".rb":
		return "\u2630" // trigram (code-ish)
	// Config / data
	case ".json", ".yaml", ".yml", ".toml", ".xml", ".env", ".ini", ".conf":
		return "\u2699" // gear
	// Documents
	case ".md", ".txt", ".rst", ".doc", ".pdf":
		return "\u2637" // document-ish
	// Shell / scripts
	case ".sh", ".bash", ".zsh", ".fish", ".bat", ".ps1":
		return "\u25B6" // play/run
	// Lock / build
	case ".lock", ".sum":
		return "\u26BF" // lock
	default:
		return "\u2022" // bullet
	}
}
