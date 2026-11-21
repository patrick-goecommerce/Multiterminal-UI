package hub

import (
	"regexp"
	"strings"
)

// Check results. CheckUnsure is never promoted to OK: it means the exit code
// may not be the check's own (a pipe, a segment after ; or ||, a failure
// after && with a non-check behind the check) and the output carried no
// failure mark.
const (
	CheckOK     = "ok"
	CheckFail   = "fail"
	CheckUnsure = "unsure"
)

// CheckResult is the last known result of one check command in an agent turn.
type CheckResult struct {
	Label  string `json:"label" yaml:"label"`
	Result string `json:"result" yaml:"result"`
}

// checkPrefixes maps a command prefix to its label; an empty label means the
// prefix itself. Longer prefixes come before the ones they contain.
var checkPrefixes = []struct{ prefix, label string }{
	{"go test", ""}, {"go vet", ""}, {"go build", ""},
	{"npm run test", "test"}, {"npm test", "test"}, {"npx vitest", "test"},
	{"vitest", "test"}, {"pnpm test", "test"}, {"yarn test", "test"},
	{"npm run build", "build"}, {"pnpm build", "build"}, {"yarn build", "build"},
	{"wails3 build", "build"}, {"wails build", "build"},
	{"npm run lint", "lint"}, {"npx eslint", "lint"}, {"eslint", "lint"},
	{"npx tsc", "tsc"}, {"tsc", "tsc"},
	{"python -m pytest", "pytest"}, {"pytest", "pytest"},
	{"cargo test", ""}, {"cargo build", ""}, {"cargo clippy", ""},
	{"dotnet build", ""}, {"dotnet test", ""},
}

// failureMarks are line-anchored and measured on real output (go test, tsc,
// vitest). Nothing else counts as evidence of a failure behind a pipe.
var failureMarks = regexp.MustCompile(
	`(?m)^--- FAIL:|^FAIL\r?$|^FAIL\t|error TS\d+:|^\s*Tests\s+\d+ failed|^\s*Test Files\s+\d+ failed`)

var publishPrefixes = []string{"git push", "gh pr create", "gh pr merge"}

// classifyCheck reports whether command is (or contains) a check and its
// result. failed is true for a PostToolUseFailure event. Several checks in
// one line give one result with the labels joined by " + ".
func classifyCheck(command string, failed bool, outputTail string) (CheckResult, bool) {
	var labels []string
	piped, trailing := false, false
	for _, seg := range splitSegments(command) {
		label, ok := checkLabel(seg.text)
		if !ok {
			// A non-check after a check (`&& git commit`, `&& ./x`) may be
			// what failed the line; a success still proves the check.
			trailing = trailing || len(labels) > 0
			continue
		}
		if !containsLabel(labels, label) {
			labels = append(labels, label)
		}
		// A pipe, or a segment after ; or ||, hides the check's own exit code.
		if hasPipe(seg.text) || seg.masked {
			piped = true
		}
	}
	if len(labels) == 0 {
		return CheckResult{}, false
	}
	res := CheckOK
	switch {
	case piped || (failed && trailing):
		res = CheckUnsure
		if failureMarks.MatchString(outputTail) {
			res = CheckFail
		}
	case failed:
		res = CheckFail
	}
	return CheckResult{Label: strings.Join(labels, " + "), Result: res}, true
}

// publishCommand reports whether any segment of command publishes work:
// git push, gh pr create or gh pr merge.
func publishCommand(command string) bool {
	for _, seg := range splitSegments(command) {
		text := stripLeading(seg.text)
		for _, p := range publishPrefixes {
			if hasWordPrefix(text, p) {
				return true
			}
		}
	}
	return false
}

func checkLabel(seg string) (string, bool) {
	seg = stripLeading(seg)
	for _, p := range checkPrefixes {
		if hasWordPrefix(seg, p.prefix) {
			if p.label == "" {
				return p.prefix, true
			}
			return p.label, true
		}
	}
	return "", false
}

func containsLabel(labels []string, l string) bool {
	for _, x := range labels {
		if x == l {
			return true
		}
	}
	return false
}

// hasWordPrefix is a prefix match that must end at a word boundary, so
// "gotest" does not match "go test" and "tsc" does not match "tscx".
func hasWordPrefix(seg, prefix string) bool {
	if !strings.HasPrefix(seg, prefix) {
		return false
	}
	rest := seg[len(prefix):]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

// segment is one part of a command line. masked is set when another segment
// follows after ; or ||: the line's exit code then is not this segment's.
type segment struct {
	text   string
	masked bool
}

// splitSegments cuts at &&, ; and || outside quotes. A single | stays in the
// segment, where hasPipe finds it.
func splitSegments(command string) []segment {
	var segs []segment
	var cur strings.Builder
	var quote rune
	rs := []rune(command)
	prevMasks := false // the separator that ended the previous segment was ; or ||
	flush := func(masks bool) {
		if s := strings.TrimSpace(cur.String()); s != "" {
			if n := len(segs); n > 0 {
				segs[n-1].masked = prevMasks
			}
			segs = append(segs, segment{text: s})
			prevMasks = masks
		}
		cur.Reset()
	}
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
			cur.WriteRune(r)
		case r == '"' || r == '\'':
			quote = r
			cur.WriteRune(r)
		case r == ';':
			flush(true)
		case (r == '&' || r == '|') && i+1 < len(rs) && rs[i+1] == r:
			flush(r == '|')
			i++
		default:
			cur.WriteRune(r)
		}
	}
	flush(false)
	return segs
}

// hasPipe reports a | outside quotes (|| was already split off).
func hasPipe(seg string) bool {
	var quote rune
	for _, r := range seg {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '|':
			return true
		}
	}
	return false
}

// stripLeading drops `cd …`, `NAME=value` and `timeout N` in front of the
// real command, repeatedly.
func stripLeading(seg string) string {
	for {
		f := strings.Fields(seg)
		switch {
		case len(f) >= 2 && f[0] == "cd":
			f = f[2:]
		case len(f) >= 1 && isEnvAssign(f[0]):
			f = f[1:]
		case len(f) >= 2 && f[0] == "timeout":
			f = f[2:]
		default:
			return strings.Join(f, " ")
		}
		seg = strings.Join(f, " ")
	}
}

var envAssign = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

func isEnvAssign(tok string) bool { return envAssign.MatchString(tok) }
