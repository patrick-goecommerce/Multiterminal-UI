//go:build !windows

package backend

// codexSearchPaths returns common Codex CLI installation locations on
// Unix-like systems (Linux, macOS).
func codexSearchPaths() []string {
	var paths []string
	add := func(p string) {
		if p != "" {
			paths = append(paths, p)
		}
	}

	add(expandHome("~/.local/bin/codex"))
	add("/usr/local/bin/codex")
	add(expandHome("~/.npm-global/bin/codex"))
	add(expandEnv("VOLTA_HOME", "bin/codex"))
	add(expandHome("~/.volta/bin/codex"))
	add(expandHome("~/.bun/bin/codex"))
	add(expandHome("~/.yarn/bin/codex"))
	add(expandHome("~/.pnpm-global/bin/codex"))

	return paths
}
