package hub

import "testing"

func TestClassifyCheck(t *testing.T) {
	cases := []struct {
		name    string
		command string
		failed  bool
		tail    string
		label   string
		result  string
		ok      bool
	}{
		{"go test after cd", "cd internal && go test ./...", false, "ok  x 0.1s", "go test", CheckOK, true},
		{"env prefix, failed", "FOO=1 npm run build", true, "", "build", CheckFail, true},
		{"two checks, one label", "go vet ./... && go test ./...", true, "", "go vet + go test", CheckFail, true},
		{"go test piped, failure marks", "go test ./... | tail -20", false,
			"--- FAIL: TestFails (0.00s)\n    fail_test.go:5: boom\nFAIL\nFAIL\tfailprobe\t1.473s\nFAIL", "go test", CheckFail, true},
		{"go test piped, no marks", "go test ./... | tail -20", false, "ok  \tfailprobe\t0.2s", "go test", CheckUnsure, true},
		{"tsc piped, error TS", "npx tsc --noEmit | head", false,
			"bad.ts(1,7): error TS2322: Type 'string' is not assignable to type 'number'.", "tsc", CheckFail, true},
		{"vitest piped, failed counts", "npx vitest run | tail", false,
			" Test Files  1 failed (1)\n      Tests  1 failed | 1 passed (2)", "test", CheckFail, true},
		{"npm build piped, no mark", "npm run build | tail", false,
			"> build\n> node -e \"process.exit(2)\"", "build", CheckUnsure, true},
		{"timeout prefix", "timeout 60 pytest -q", false, "", "pytest", CheckOK, true},
		{"grep is no check", "grep -rn foo .", true, "", "", "", false},
		{"git status is no check", "git status", false, "", "", "", false},
		{"or-separated", "go build ./... || go vet ./...", false, "", "go build + go vet", CheckOK, true},
		{"semicolon", "cd x; cargo test", true, "", "cargo test", CheckFail, true},
		{"prefix needs a word boundary", "gotest ./...", false, "", "", "", false},
		{"npm test", "npm test", false, "", "test", CheckOK, true},
		{"empty", "", false, "", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := classifyCheck(c.command, c.failed, c.tail)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v (got %+v)", ok, c.ok, got)
			}
			if got.Label != c.label || got.Result != c.result {
				t.Fatalf("got %+v, want label %q result %q", got, c.label, c.result)
			}
		})
	}
}

func TestPublishCommand(t *testing.T) {
	cases := map[string]bool{
		"cd x && git push -u origin b": true,
		"gh pr create --fill":          true,
		"gh pr merge 3 --squash":       true,
		"gh pr view 1":                 false,
		"git pull":                     false,
		"git pushall":                  false,
	}
	for cmd, want := range cases {
		if got := publishCommand(cmd); got != want {
			t.Errorf("publishCommand(%q) = %v, want %v", cmd, got, want)
		}
	}
}
