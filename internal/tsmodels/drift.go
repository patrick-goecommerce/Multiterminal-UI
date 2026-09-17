package tsmodels

import (
	"fmt"
	"sort"
)

// Drift is one field a class in models.ts is missing.
type Drift struct {
	Namespace string
	Class     string
	Field     string
	GoName    string
	// Declared says the property exists on the class but the constructor
	// never assigns it. That is the worse of the two: it type-checks, so
	// nothing in the frontend complains, and the value is undefined for every
	// instance at runtime.
	Declared bool
	Line     int
}

func (d Drift) String() string {
	where := fmt.Sprintf("%s.%s (models.ts:%d)", d.Namespace, d.Class, d.Line)
	if d.Declared {
		return fmt.Sprintf(
			"%s declares %q but never assigns it. Add to the constructor:\n"+
				"        this.%s = source[%q];",
			where, d.Field, d.Field, d.Field)
	}
	return fmt.Sprintf(
		"%s is missing %q (Go field %s). Add both:\n"+
			"        %s: <typ>;\n"+
			"        this.%s = source[%q];",
		where, d.Field, d.GoName, d.Field, d.Field, d.Field)
}

// Compare reports the json fields a namespace's classes are missing.
//
// Only classes that models.ts already has are checked. A Go struct the
// frontend never receives has no business in that file, and demanding one
// would turn this check into a nuisance that gets switched off.
func Compare(namespace string, ts map[string]*TSClass, structs GoStructs) []Drift {
	var drifts []Drift
	classes := make([]string, 0, len(ts))
	for name := range ts {
		classes = append(classes, name)
	}
	sort.Strings(classes)

	for _, name := range classes {
		fields, ok := structs[name]
		if !ok {
			// A class with no Go struct of that name: an enum wrapper, or a
			// type that moved. Not this check's business either way.
			continue
		}
		cls := ts[name]
		for _, f := range fields {
			switch {
			case cls.Assigned[f.Name]:
				// Assigned is what actually matters at runtime. A field
				// assigned but not declared is a TypeScript lint problem, not
				// a silently dropped value, so it is not reported here.
			case cls.Declared[f.Name]:
				drifts = append(drifts, Drift{namespace, name, f.Name, f.GoName, true, cls.Line})
			default:
				drifts = append(drifts, Drift{namespace, name, f.Name, f.GoName, false, cls.Line})
			}
		}
	}
	return drifts
}
