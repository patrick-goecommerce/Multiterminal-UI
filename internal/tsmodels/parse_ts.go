package tsmodels

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// TSClass is one class in models.ts.
type TSClass struct {
	// Declared are the property names on the class body.
	Declared map[string]bool
	// Assigned are the names the constructor actually reads out of source.
	// The distinction is the whole point: a property declared but never
	// assigned type-checks, and is undefined at runtime for every instance.
	Assigned map[string]bool
	// Line is where the class starts, so a failure can be clicked.
	Line int
}

// TSFile is models.ts, by namespace and class.
type TSFile map[string]map[string]*TSClass

var (
	reNamespace = regexp.MustCompile(`^export namespace (\w+) \{`)
	reClass     = regexp.MustCompile(`^\s*export class (\w+) \{`)
	reCtor      = regexp.MustCompile(`^\s*constructor\(`)
	reProperty  = regexp.MustCompile(`^\s{4,}(\w+)[?]?:\s`)
	// Both shapes Wails emits: a primitive read straight out of source, and a
	// nested value routed through convertValues.
	reAssignPlain   = regexp.MustCompile(`this\.(\w+)\s*=\s*source\[`)
	reAssignConvert = regexp.MustCompile(`this\.(\w+)\s*=\s*this\.convertValues\(source\[`)
)

// ParseTS reads models.ts.
//
// It is a line scanner rather than a TypeScript parser because the file is
// machine-shaped: Wails emits one property per line and one assignment per
// line, and anything that does not match those shapes is not something this
// check should have an opinion about.
func ParseTS(path string) (TSFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := TSFile{}
	var namespace string
	var class *TSClass
	inCtor := false
	depth := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()

		if m := reNamespace.FindStringSubmatch(text); m != nil {
			namespace = m[1]
			if out[namespace] == nil {
				out[namespace] = map[string]*TSClass{}
			}
			continue
		}
		if m := reClass.FindStringSubmatch(text); m != nil && namespace != "" {
			class = &TSClass{Declared: map[string]bool{}, Assigned: map[string]bool{}, Line: line}
			out[namespace][m[1]] = class
			inCtor = false
			depth = 0
			continue
		}
		if class == nil {
			continue
		}
		if reCtor.MatchString(text) {
			inCtor = true
			depth = 0
		}
		if inCtor {
			depth += strings.Count(text, "{") - strings.Count(text, "}")
			if m := reAssignConvert.FindStringSubmatch(text); m != nil {
				class.Assigned[m[1]] = true
			} else if m := reAssignPlain.FindStringSubmatch(text); m != nil {
				class.Assigned[m[1]] = true
			}
			if depth <= 0 && strings.Contains(text, "}") {
				inCtor = false
				class = nil
			}
			continue
		}
		if m := reProperty.FindStringSubmatch(text); m != nil {
			class.Declared[m[1]] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return out, nil
}
