package tsmodels

import (
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// App.js is hand-written like models.ts: Wails v3 calls a binding by the
// FNV-1a hash of its fully qualified name, and nothing recomputes that hash.
// A typo in a new binding's ID fails only at runtime, as "method not found"
// on the first click. This test recomputes every ID and checks App.d.ts
// declares the same functions.

const bindingPrefix = "github.com/patrick-goecommerce/Multiterminal-UI/internal/backend.AppService."

var (
	jsBinding = regexp.MustCompile(`export function (\w+)\([^)]*\) \{\s*return \$Call\.ByID\((\d+)`)
	dtsFunc   = regexp.MustCompile(`export function (\w+)\(`)
)

func bindingID(name string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(bindingPrefix + name))
	return h.Sum32()
}

func TestBindingIDsMatchTheirNames(t *testing.T) {
	root := repoRoot(t)
	js, err := os.ReadFile(filepath.Join(root, "frontend/wailsjs/go/backend/App.js"))
	if err != nil {
		t.Fatal(err)
	}
	dts, err := os.ReadFile(filepath.Join(root, "frontend/wailsjs/go/backend/App.d.ts"))
	if err != nil {
		t.Fatal(err)
	}

	declared := map[string]bool{}
	for _, m := range dtsFunc.FindAllStringSubmatch(string(dts), -1) {
		declared[m[1]] = true
	}
	found := 0
	for _, m := range jsBinding.FindAllStringSubmatch(string(js), -1) {
		found++
		name := m[1]
		id, _ := strconv.ParseUint(m[2], 10, 32)
		if want := bindingID(name); uint32(id) != want {
			t.Errorf("App.js %s calls ID %d, want %d", name, id, want)
		}
		if !declared[name] {
			t.Errorf("App.js has %s but App.d.ts does not declare it", name)
		}
	}
	if found < 50 {
		t.Fatalf("found only %d bindings in App.js; the pattern no longer matches", found)
	}
}
