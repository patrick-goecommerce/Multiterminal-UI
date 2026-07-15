//go:build windows

package backend

import "os"

// geminiSearchPaths returns common Gemini CLI installation locations on Windows.
func geminiSearchPaths() []string {
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
	add(expandEnv("APPDATA", `npm\gemini.cmd`))

	// pnpm
	add(expandEnv("LOCALAPPDATA", `pnpm\gemini.cmd`))

	// Yarn global
	add(expandEnv("LOCALAPPDATA", `Yarn\bin\gemini.cmd`))

	// Volta
	add(expandEnv("LOCALAPPDATA", `Volta\bin\gemini.cmd`))
	add(expandEnv("VOLTA_HOME", `bin\gemini.cmd`))

	// nvm-windows
	add(expandEnv("NVM_SYMLINK", `gemini.cmd`))

	// Scoop
	add(home + `\scoop\shims\gemini.cmd`)
	add(expandEnv("SCOOP", `shims\gemini.cmd`))

	// Bun
	add(home + `\.bun\bin\gemini.exe`)

	return paths
}
