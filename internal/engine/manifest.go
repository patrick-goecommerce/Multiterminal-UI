package engine

import (
	"context"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ManifestChange represents a single dependency change.
type ManifestChange struct {
	File    string `json:"file"`    // "go.mod" or "package.json"
	Kind    string `json:"kind"`    // "add" | "update" | "remove"
	Package string `json:"package"` // "github.com/foo/bar" or "@types/node"
	From    string `json:"from"`    // previous version (empty for add)
	To      string `json:"to"`      // new version (empty for remove)
}

// DependencyRisk classifies the overall risk of manifest changes.
type DependencyRisk string

const (
	DependencyRiskNone DependencyRisk = "none"
	DependencyRiskLow  DependencyRisk = "low"  // only additions
	DependencyRiskHigh DependencyRisk = "high" // updates or removals
)

var (
	goModLineRe    = regexp.MustCompile(`^\s*([\w./\-@]+)\s+(v[\d.].*)$`)
	pkgJSONLineRe  = regexp.MustCompile(`"([@\w/.\-]+)"\s*:\s*"([^"]+)"`)
)

// ParseManifestChanges analyzes git diff for dependency manifest changes.
// workDir should be the git repo root or worktree.
// files is the list of changed files (from git diff --name-only).
func ParseManifestChanges(ctx context.Context, workDir string, files []string) ([]ManifestChange, DependencyRisk) {
	var changes []ManifestChange

	for _, f := range files {
		if !isManifestFile(f) {
			continue
		}
		// go.sum is too noisy for individual entries — detect but skip parsing.
		base := filepath.Base(f)
		if base == "go.sum" || base == "package-lock.json" {
			continue
		}
		diff := getFileDiff(ctx, workDir, f)
		if diff == "" {
			continue
		}
		fileChanges := parseFileDiff(f, diff)
		changes = append(changes, fileChanges...)
	}

	risk := classifyRisk(changes)
	return changes, risk
}

func isManifestFile(path string) bool {
	base := filepath.Base(path)
	switch base {
	case "go.mod", "go.sum",
		"package.json", "package-lock.json",
		"Cargo.toml", "requirements.txt",
		"pyproject.toml":
		return true
	}
	return false
}

func getFileDiff(ctx context.Context, workDir, file string) string {
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD", "--", file)
	cmd.Dir = workDir
	output, _ := cmd.Output()
	return string(output)
}

func classifyRisk(changes []ManifestChange) DependencyRisk {
	risk := DependencyRiskNone
	for _, c := range changes {
		switch c.Kind {
		case "update", "remove":
			return DependencyRiskHigh
		case "add":
			risk = DependencyRiskLow
		}
	}
	return risk
}

func parseFileDiff(file, diff string) []ManifestChange {
	base := filepath.Base(file)
	switch base {
	case "go.mod":
		return parseGoModDiff(file, diff)
	case "package.json":
		return parsePkgJSONDiff(file, diff)
	}
	return nil
}

// parseGoModDiff pairs added/removed lines by package name.
func parseGoModDiff(file, diff string) []ManifestChange {
	type verPair struct {
		added   string // version from + line
		removed string // version from - line
	}
	pairs := make(map[string]*verPair)

	for _, line := range strings.Split(diff, "\n") {
		if len(line) == 0 {
			continue
		}
		prefix := line[0]
		if prefix != '+' && prefix != '-' {
			continue
		}
		// Skip diff headers (+++, ---)
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		rest := line[1:]
		m := goModLineRe.FindStringSubmatch(rest)
		if m == nil {
			continue
		}
		pkg, ver := m[1], strings.TrimSpace(m[2])
		p, ok := pairs[pkg]
		if !ok {
			p = &verPair{}
			pairs[pkg] = p
		}
		if prefix == '+' {
			p.added = ver
		} else {
			p.removed = ver
		}
	}

	var changes []ManifestChange
	for pkg, p := range pairs {
		c := ManifestChange{File: file, Package: pkg}
		switch {
		case p.added != "" && p.removed != "":
			if p.added == p.removed {
				continue // same version, just formatting change
			}
			c.Kind = "update"
			c.From = p.removed
			c.To = p.added
		case p.added != "":
			c.Kind = "add"
			c.To = p.added
		default:
			c.Kind = "remove"
			c.From = p.removed
		}
		changes = append(changes, c)
	}
	return changes
}

// parsePkgJSONDiff pairs added/removed lines by package name.
func parsePkgJSONDiff(file, diff string) []ManifestChange {
	type verPair struct {
		added   string
		removed string
	}
	pairs := make(map[string]*verPair)

	for _, line := range strings.Split(diff, "\n") {
		if len(line) == 0 {
			continue
		}
		prefix := line[0]
		if prefix != '+' && prefix != '-' {
			continue
		}
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		rest := line[1:]
		m := pkgJSONLineRe.FindStringSubmatch(rest)
		if m == nil {
			continue
		}
		pkg, ver := m[1], m[2]
		p, ok := pairs[pkg]
		if !ok {
			p = &verPair{}
			pairs[pkg] = p
		}
		if prefix == '+' {
			p.added = ver
		} else {
			p.removed = ver
		}
	}

	var changes []ManifestChange
	for pkg, p := range pairs {
		c := ManifestChange{File: file, Package: pkg}
		switch {
		case p.added != "" && p.removed != "":
			if p.added == p.removed {
				continue // same version, just formatting/comma change
			}
			c.Kind = "update"
			c.From = p.removed
			c.To = p.added
		case p.added != "":
			c.Kind = "add"
			c.To = p.added
		default:
			c.Kind = "remove"
			c.From = p.removed
		}
		changes = append(changes, c)
	}
	return changes
}
