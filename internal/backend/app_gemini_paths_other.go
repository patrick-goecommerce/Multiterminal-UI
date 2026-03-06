//go:build !windows

package backend

// geminiSearchPaths returns common Gemini CLI installation locations on
// Unix-like systems (Linux, macOS).
func geminiSearchPaths() []string {
	var paths []string
	add := func(p string) {
		if p != "" {
			paths = append(paths, p)
		}
	}

	add(expandHome("~/.local/bin/gemini"))
	add("/usr/local/bin/gemini")
	add(expandHome("~/.npm-global/bin/gemini"))
	add(expandEnv("VOLTA_HOME", "bin/gemini"))
	add(expandHome("~/.volta/bin/gemini"))
	add(expandHome("~/.bun/bin/gemini"))
	add(expandHome("~/.yarn/bin/gemini"))
	add(expandHome("~/.pnpm-global/bin/gemini"))

	return paths
}
