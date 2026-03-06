//go:build windows

package backend

import "os"

// codexSearchPaths returns common Codex CLI installation locations on Windows.
func codexSearchPaths() []string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}

	var paths []string
	add := func(p string) {
		if p != "" {
			paths = append(paths, p)
		}
	}

	// npm global
	add(expandEnv("APPDATA", `npm\codex.cmd`))

	// pnpm
	add(expandEnv("LOCALAPPDATA", `pnpm\codex.cmd`))

	// Yarn global
	add(expandEnv("LOCALAPPDATA", `Yarn\bin\codex.cmd`))

	// Volta
	add(expandEnv("LOCALAPPDATA", `Volta\bin\codex.cmd`))
	add(expandEnv("VOLTA_HOME", `bin\codex.cmd`))

	// nvm-windows
	add(expandEnv("NVM_SYMLINK", `codex.cmd`))

	// Scoop
	add(home + `\scoop\shims\codex.cmd`)
	add(expandEnv("SCOOP", `shims\codex.cmd`))

	// Bun
	add(home + `\.bun\bin\codex.exe`)

	return paths
}
