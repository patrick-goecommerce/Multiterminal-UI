package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// DetectSkills scans a project directory and returns IDs of matching skills.
func DetectSkills(dir string) []string {
	var detected []string
	for _, skill := range AllSkills() {
		if len(skill.DetectFiles) == 0 {
			continue // skills like "security" have no auto-detect
		}
		for _, pattern := range skill.DetectFiles {
			if matchesProject(dir, pattern) {
				detected = append(detected, skill.ID)
				break
			}
		}
	}
	return detected
}

// matchesProject checks if a detection pattern matches the project.
// Supported formats:
//   - "filename"              → file exists
//   - "dirname/"              → directory exists
//   - "*.ext"                 → any file with extension
//   - "package.json:dep"      → package.json contains dependency
func matchesProject(dir, pattern string) bool {
	// Package dependency check: "package.json:react"
	if parts := strings.SplitN(pattern, ":", 2); len(parts) == 2 {
		return checkPackageDep(dir, parts[0], parts[1])
	}

	// Directory check: "k8s/"
	if strings.HasSuffix(pattern, "/") {
		info, err := os.Stat(filepath.Join(dir, pattern[:len(pattern)-1]))
		return err == nil && info.IsDir()
	}

	// Glob check: "*.tf", "*.csproj"
	if strings.Contains(pattern, "*") {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		return err == nil && len(matches) > 0
	}

	// Simple file existence check
	_, err := os.Stat(filepath.Join(dir, pattern))
	return err == nil
}

// checkPackageDep reads a JSON file and checks if it contains a dependency.
func checkPackageDep(dir, filename, dep string) bool {
	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		return false
	}

	// For package.json, check dependencies and devDependencies
	if filename == "package.json" {
		return packageJSONHasDep(data, dep)
	}

	// Generic: just check if the string appears in the file
	return strings.Contains(string(data), dep)
}

// packageJSONHasDep checks if a package.json has a dependency.
func packageJSONHasDep(data []byte, dep string) bool {
	var pkg struct {
		Deps    map[string]interface{} `json:"dependencies"`
		DevDeps map[string]interface{} `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	if _, ok := pkg.Deps[dep]; ok {
		return true
	}
	if _, ok := pkg.DevDeps[dep]; ok {
		return true
	}
	return false
}
