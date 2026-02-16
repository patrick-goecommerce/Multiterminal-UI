//go:build windows

package backend

import "os"

// claudeSearchPaths returns common Claude CLI installation locations on Windows.
func claudeSearchPaths() []string {
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

	// Standalone installer
	add(home + `\.local\bin\claude.exe`)

	// npm global
	add(expandEnv("APPDATA", `npm\claude.cmd`))

	// pnpm
	add(expandEnv("LOCALAPPDATA", `pnpm\claude.cmd`))

	// Yarn global
	add(expandEnv("LOCALAPPDATA", `Yarn\bin\claude.cmd`))

	// Volta
	add(expandEnv("LOCALAPPDATA", `Volta\bin\claude.cmd`))
	add(expandEnv("VOLTA_HOME", `bin\claude.cmd`))

	// nvm-windows (NVM_SYMLINK points to active Node)
	add(expandEnv("NVM_SYMLINK", `claude.cmd`))

	// Scoop
	add(home + `\scoop\shims\claude.cmd`)
	add(expandEnv("SCOOP", `shims\claude.cmd`))

	// Chocolatey
	add(`C:\ProgramData\chocolatey\bin\claude.cmd`)

	// Bun
	add(home + `\.bun\bin\claude.exe`)

	return paths
}
