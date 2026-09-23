package tsmodels

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// The second mirror.
//
// frontend/src/stores/config.ts declares AppConfig by hand, and that, not
// models.ts, is what the settings code reads. It drifted eleven fields,
// session_host among them, which is why that code was full of `as any`: the
// cast is what a missing field looks like from the outside, and a cast is
// exactly what stops the compiler from noticing the next one.

var (
	reInterface = regexp.MustCompile(`^export interface (\w+) \{`)
	// One property per line: a name, an optional "?", a colon. Comment lines
	// and nested object literals do not match, which is what keeps this from
	// inventing fields.
	reIfaceProp = regexp.MustCompile(`^\s{2,}(\w+)\??:\s`)
)

// ParseInterface returns the property names of one exported TypeScript
// interface.
func ParseInterface(path, name string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	props := map[string]bool{}
	inside := false
	depth := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		text := scanner.Text()
		if !inside {
			if m := reInterface.FindStringSubmatch(text); m != nil && m[1] == name {
				inside = true
				depth = 1
			}
			continue
		}
		depth += strings.Count(text, "{") - strings.Count(text, "}")
		if depth <= 0 {
			break
		}
		// Only the interface's own properties, not the ones inside an inline
		// object literal a property might be typed with.
		if depth == 1 {
			if m := reIfaceProp.FindStringSubmatch(text); m != nil {
				props[m[1]] = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if !inside {
		return nil, fmt.Errorf("%s: no exported interface %q", path, name)
	}
	return props, nil
}

// MissingFromInterface returns the json fields of a Go struct that the
// interface does not declare.
func MissingFromInterface(fields []GoField, props map[string]bool) []GoField {
	var missing []GoField
	for _, f := range fields {
		if !props[f.Name] {
			missing = append(missing, f)
		}
	}
	return missing
}
