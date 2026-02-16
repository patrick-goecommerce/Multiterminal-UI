//go:build !windows

package backend

// claudeSearchPaths returns common Claude CLI installation locations on
// Unix-like systems (Linux, macOS).
func claudeSearchPaths() []string {
	var paths []string
	add := func(p string) {
		if p != "" {
			paths = append(paths, p)
		}
	}

	add(expandHome("~/.local/bin/claude"))
	add("/usr/local/bin/claude")
	add(expandHome("~/.npm-global/bin/claude"))
	add(expandEnv("VOLTA_HOME", "bin/claude"))
	add(expandHome("~/.volta/bin/claude"))
	add(expandHome("~/.bun/bin/claude"))
	add(expandHome("~/.yarn/bin/claude"))
	add(expandHome("~/.pnpm-global/bin/claude"))

	return paths
}
