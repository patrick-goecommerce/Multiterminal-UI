package terminal

// Windows argv preparation for ConPTY.
//
// ConPTY can only launch real executables. CLI tools installed via npm
// ("claude", "gemini", …) ship as .cmd/.bat shims on Windows, which is why
// Start used to route *every* command through COMSPEC. That wrapper costs one
// extra cmd.exe process per pane and adds a level to the process tree that the
// kill path has to walk, so it is now applied only when the resolved target
// really needs it.
//
// The functions here are pure/OS-independent on purpose (no build tag) so the
// wrap decision can be unit tested on any platform; the runtime.GOOS guard
// stays in Start.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// defaultComspec is the fallback used when the COMSPEC variable is empty.
const defaultComspec = `C:\Windows\System32\cmd.exe`

// defaultPathExt mirrors the list exec.LookPath falls back to when PATHEXT is
// not present in the environment.
const defaultPathExt = `.com;.exe;.bat;.cmd`

// directExecExtensions are the extensions ConPTY can start on its own.
// Everything else (.cmd/.bat shims, extension-less scripts) needs cmd.exe.
var directExecExtensions = []string{".exe", ".com"}

// canStartDirectly reports whether a resolved executable path can be handed to
// ConPTY without a cmd.exe wrapper. Extension casing is irrelevant on Windows.
func canStartDirectly(resolved string) bool {
	ext := filepath.Ext(resolved)
	for _, e := range directExecExtensions {
		if strings.EqualFold(ext, e) {
			return true
		}
	}
	return false
}

// windowsArgv returns the argv Start hands to ConPTY on Windows.
//
// resolve mimics exec.LookPath against the PATH the session will actually get
// (see sessionLookPath). When it resolves to a .exe/.com the command is
// started directly, with argv[0] replaced by the resolved path — go-pty's
// lookExtensions resolves a bare name against Cmd.Dir rather than PATH, so an
// unresolved "foo.exe" would not be found at all.
//
// Anything else — a .cmd/.bat shim, or a failed lookup — keeps the COMSPEC
// wrapper. Falling back is always safe: cmd.exe does its own PATHEXT
// resolution, which is exactly the previous behaviour.
func windowsArgv(argv []string, comspec string, resolve func(string) (string, error)) []string {
	if len(argv) == 0 {
		return argv
	}
	if resolve != nil {
		if resolved, err := resolve(argv[0]); err == nil && canStartDirectly(resolved) {
			out := make([]string, len(argv))
			copy(out, argv)
			out[0] = resolved
			return out
		}
	}
	if comspec == "" {
		comspec = defaultComspec
	}
	return append([]string{comspec, "/c"}, argv...)
}

// sessionLookPath builds the resolver windowsArgv uses, bound to the exact
// PATH/PATHEXT the session is started with. Start prepends its own executable
// directory (the tmux shim) to the session PATH, so resolving against the
// parent process' PATH could pick a different binary than the session runs.
func sessionLookPath(env []string) func(string) (string, error) {
	dirs := filepath.SplitList(envValue(env, "PATH"))
	exts := pathExtList(envValue(env, "PATHEXT"))
	return func(file string) (string, error) {
		return lookWindowsPath(file, dirs, exts)
	}
}

// lookWindowsPath resolves file the way exec.LookPath does on Windows, but
// against an explicit directory list.
//
// The current directory is deliberately not searched: exec.LookPath reports
// ErrDot for it, and a miss here only means the COMSPEC wrapper is kept.
func lookWindowsPath(file string, dirs []string, exts []string) (string, error) {
	if file == "" {
		return "", fs.ErrNotExist
	}
	// A path (absolute, drive-relative or containing a separator) is not
	// searched in PATH — only completed with an extension.
	if strings.ContainsAny(file, `:\/`) {
		return findExecutable(file, exts)
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if f, err := findExecutable(filepath.Join(dir, file), exts); err == nil {
			return f, nil
		}
	}
	return "", fs.ErrNotExist
}

// findExecutable returns file itself when it already carries an extension and
// exists, otherwise the first existing file+ext candidate.
func findExecutable(file string, exts []string) (string, error) {
	if hasExt(file) && isExistingFile(file) {
		return file, nil
	}
	for _, e := range exts {
		if f := file + e; isExistingFile(f) {
			return f, nil
		}
	}
	return "", fs.ErrNotExist
}

// isExistingFile reports whether path exists and is not a directory.
func isExistingFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// hasExt reports whether file carries a file extension, ignoring dots that
// belong to a directory component. Mirrors the os/exec helper of the same name.
func hasExt(file string) bool {
	i := strings.LastIndex(file, ".")
	if i < 0 {
		return false
	}
	return strings.LastIndexAny(file, `:\/`) < i
}

// pathExtList splits a PATHEXT value into normalised, lower-case extensions.
func pathExtList(pathExt string) []string {
	if strings.TrimSpace(pathExt) == "" {
		pathExt = defaultPathExt
	}
	exts := make([]string, 0, 4)
	for _, e := range strings.Split(strings.ToLower(pathExt), ";") {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if e[0] != '.' {
			e = "." + e
		}
		exts = append(exts, e)
	}
	return exts
}

// envValue returns the value of key in a KEY=VALUE list, matched
// case-insensitively (Windows uses "Path" as often as "PATH"). The LAST match
// wins: go-pty runs dedupEnvCase(true, …) before CreateProcess, which keeps
// the last occurrence, so this is the value the child process really sees.
func envValue(env []string, key string) string {
	prefix := key + "="
	val := ""
	for _, e := range env {
		if len(e) >= len(prefix) && strings.EqualFold(e[:len(prefix)], prefix) {
			val = e[len(prefix):]
		}
	}
	return val
}
