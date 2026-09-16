package main

import (
	"io"
	"os"
	"sort"
	"strings"
)

// The key table.
//
// It is deliberately a table of names a person would type rather than a
// pass-through for escape sequences: a caller who knows the bytes can use
// `mtui send --no-enter`, and everybody else wants to write "ctrl-c".
//
// The names match what the MCP tool already accepts, so a prompt that works
// for an agent works in a shell.
var keyTable = map[string][]byte{
	"enter":     []byte("\r"),
	"return":    []byte("\r"),
	"tab":       []byte("\t"),
	"backtab":   []byte("\x1b[Z"),
	"escape":    []byte("\x1b"),
	"esc":       []byte("\x1b"),
	"space":     []byte(" "),
	"backspace": []byte("\x7f"),
	"delete":    []byte("\x1b[3~"),
	"up":        []byte("\x1b[A"),
	"down":      []byte("\x1b[B"),
	"right":     []byte("\x1b[C"),
	"left":      []byte("\x1b[D"),
	"home":      []byte("\x1b[H"),
	"end":       []byte("\x1b[F"),
	"pageup":    []byte("\x1b[5~"),
	"pagedown":  []byte("\x1b[6~"),
	// The two answers a permission prompt wants, spelled as what they mean.
	"yes": []byte("y"),
	"no":  []byte("n"),
}

// keyBytes resolves one key name.
//
// Beyond the table it understands ctrl-<letter>, which is generated rather
// than listed: twenty-six entries that are all the same arithmetic would only
// invite a typo in one of them.
func keyBytes(name string) ([]byte, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if seq, ok := keyTable[name]; ok {
		return seq, true
	}
	for _, prefix := range []string{"ctrl-", "ctrl+", "c-", "^"} {
		rest, found := strings.CutPrefix(name, prefix)
		if !found || len(rest) != 1 {
			continue
		}
		c := rest[0]
		if c >= 'a' && c <= 'z' {
			return []byte{c - 'a' + 1}, true
		}
	}
	return nil, false
}

// knownKeyNames lists the table for help and error messages. ctrl- is
// mentioned as a form rather than expanded, because listing all of it would
// bury the rest.
func knownKeyNames() string {
	names := make([]string, 0, len(keyTable)+1)
	for name := range keyTable {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ") + ", ctrl-<buchstabe>"
}

// readAllStdin exists so a test can read a prompt from a pipe without the
// command reaching for os.Stdin itself.
var readAllStdin = func() ([]byte, error) {
	return io.ReadAll(os.Stdin)
}
