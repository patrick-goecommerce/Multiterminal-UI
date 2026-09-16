package tsmodels

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

// GoField is one struct field as the frontend would see it.
type GoField struct {
	// Name is the json name, which is what models.ts indexes source by.
	Name string
	// GoName is the field in Go, for an error message somebody can act on.
	GoName string
}

// GoStructs maps a struct name to its json fields, for one package.
type GoStructs map[string][]GoField

// ParseGoPackage reads the struct definitions in one directory.
//
// Test files are skipped: a struct declared in a _test.go file is not part of
// the API the frontend talks to, and a test fixture named like a real type
// would otherwise shadow it.
func ParseGoPackage(dir string) (GoStructs, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		return nil, err
	}

	out := GoStructs{}
	for name, pkg := range pkgs {
		if strings.HasSuffix(name, "_test") {
			continue
		}
		for _, file := range pkg.Files {
			collectStructs(file, out)
		}
	}
	return out, nil
}

// PackageName returns the name a directory's package appears under in
// models.ts, which is the last path element.
func PackageName(dir string) string { return filepath.Base(dir) }

func collectStructs(file *ast.File, out GoStructs) {
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		st, ok := spec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		out[spec.Name.Name] = jsonFields(st)
		return true
	})
}

// jsonFields returns the fields a JSON encoder would emit for the struct.
func jsonFields(st *ast.StructType) []GoField {
	var fields []GoField
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			// An embedded struct is inlined by encoding/json, and models.ts
			// spells its fields out. Following it would need type resolution
			// across packages; skipping it can only make this check miss a
			// field, never invent one, which is the safe direction.
			continue
		}
		name, ok := jsonName(f)
		if !ok {
			continue
		}
		for _, id := range f.Names {
			if !id.IsExported() {
				continue
			}
			n := name
			if n == "" {
				n = id.Name
			}
			fields = append(fields, GoField{Name: n, GoName: id.Name})
			// Only the first name of a grouped declaration can carry the tag's
			// name; a grouped field with an explicit json name is a bug in the
			// struct, not something to reproduce here.
			name = ""
		}
	}
	return fields
}

// jsonName reads the json tag. The second result is false when the field is
// excluded from JSON entirely.
func jsonName(f *ast.Field) (string, bool) {
	if f.Tag == nil {
		return "", true // no tag: encoding/json uses the Go name
	}
	raw, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return "", true
	}
	tag, ok := reflect.StructTag(raw).Lookup("json")
	if !ok {
		return "", true
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "-" {
		return "", false
	}
	return name, true
}
