package engine

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/orchestrator"
)

// StepLoopDetector tracks fix attempts within a single step execution
// and detects pathological patterns before they waste tokens.
type StepLoopDetector struct {
	history []attemptRecord
}

type attemptRecord struct {
	ErrorHash    string
	ErrorSig     string // error_class:first_line:symbol
	DiffSize     int    // lines changed
	FailingTests int
}

// NewStepLoopDetector creates a new in-memory loop detector.
func NewStepLoopDetector() *StepLoopDetector {
	return &StepLoopDetector{}
}

// Record adds a new attempt's results to the history.
// verifyResults are the verify command outputs from this attempt.
// diffLines is the total lines changed (added + deleted).
func (d *StepLoopDetector) Record(verifyResults []orchestrator.VerifyResult, diffLines int) {
	combined := combineFailedOutput(verifyResults)
	normalized := NormalizeError(combined)
	rec := attemptRecord{
		ErrorHash:    HashError(normalized),
		ErrorSig:     ErrorSignature(combined),
		DiffSize:     diffLines,
		FailingTests: countFailingTests(combined),
	}
	d.history = append(d.history, rec)
}

// Detect checks the history for loop signals.
// Returns any detected signals (may be empty).
func (d *StepLoopDetector) Detect() []orchestrator.LoopSignal {
	var signals []orchestrator.LoopSignal
	n := len(d.history)
	if n < 2 {
		return signals
	}

	// same_error: identical ErrorHash across 2+ consecutive attempts
	if s := d.detectSameError(); s != nil {
		signals = append(signals, *s)
	}

	// growing_diff: DiffSize increases but ErrorHash stays the same
	if s := d.detectGrowingDiff(); s != nil {
		signals = append(signals, *s)
	}

	// error_pendulum: ErrorHash alternates A->B->A
	if s := d.detectPendulum(); s != nil {
		signals = append(signals, *s)
	}

	// no_test_progress: FailingTests unchanged across 2+ attempts
	if s := d.detectNoTestProgress(); s != nil {
		signals = append(signals, *s)
	}

	return signals
}

func (d *StepLoopDetector) detectSameError() *orchestrator.LoopSignal {
	n := len(d.history)
	if n < 2 {
		return nil
	}
	curr := d.history[n-1]
	prev := d.history[n-2]
	if curr.ErrorHash == prev.ErrorHash && curr.ErrorHash != "" {
		return &orchestrator.LoopSignal{
			Type:   "same_error",
			Detail: fmt.Sprintf("identical error hash across attempts %d and %d", n-1, n),
			Source: "step",
		}
	}
	return nil
}

func (d *StepLoopDetector) detectGrowingDiff() *orchestrator.LoopSignal {
	n := len(d.history)
	if n < 3 {
		return nil
	}
	a, b, c := d.history[n-3], d.history[n-2], d.history[n-1]
	if a.ErrorHash == b.ErrorHash && b.ErrorHash == c.ErrorHash && c.ErrorHash != "" {
		if a.DiffSize < b.DiffSize && b.DiffSize < c.DiffSize {
			return &orchestrator.LoopSignal{
				Type:   "growing_diff",
				Detail: fmt.Sprintf("diff growing (%d->%d->%d) but same error", a.DiffSize, b.DiffSize, c.DiffSize),
				Source: "step",
			}
		}
	}
	return nil
}

func (d *StepLoopDetector) detectPendulum() *orchestrator.LoopSignal {
	n := len(d.history)
	if n < 3 {
		return nil
	}
	a, b, c := d.history[n-3], d.history[n-2], d.history[n-1]
	if a.ErrorHash == c.ErrorHash && a.ErrorHash != b.ErrorHash && a.ErrorHash != "" && b.ErrorHash != "" {
		return &orchestrator.LoopSignal{
			Type:   "error_pendulum",
			Detail: fmt.Sprintf("error alternates between %s... and %s...", a.ErrorHash[:8], b.ErrorHash[:8]),
			Source: "step",
		}
	}
	return nil
}

func (d *StepLoopDetector) detectNoTestProgress() *orchestrator.LoopSignal {
	n := len(d.history)
	if n < 2 {
		return nil
	}
	curr := d.history[n-1]
	prev := d.history[n-2]
	if curr.FailingTests > 0 && curr.FailingTests == prev.FailingTests {
		return &orchestrator.LoopSignal{
			Type:   "no_test_progress",
			Detail: fmt.Sprintf("failing tests unchanged at %d across attempts %d and %d", curr.FailingTests, n-1, n),
			Source: "step",
		}
	}
	return nil
}

// Regexes for error normalization (compiled once).
var (
	reTimestampISO  = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[^\s]*`)
	reTimestampLog  = regexp.MustCompile(`(?i)(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}(:\d{2})?`)
	reTmpUnix       = regexp.MustCompile(`/tmp/[^\s:]+`)
	reTmpWin        = regexp.MustCompile(`(?i)[A-Z]:\\[Uu]sers\\[^\\]+\\AppData\\Local\\Temp\\[^\s:]+`)
	reHexAddr       = regexp.MustCompile(`0x[0-9a-fA-F]{4,}`)
	reAbsPathUnix   = regexp.MustCompile(`/(?:home|usr|var|opt)/[^\s:]+/([^\s:/]+)`)
	reAbsPathWin    = regexp.MustCompile(`(?i)[A-Z]:\\[^\s:]+\\([^\s:\\]+)`)
	reLineNumber    = regexp.MustCompile(`(\w+\.go):(\d+)`)
	reTiming        = regexp.MustCompile(`\(\d+\.\d+s\)`)
	reGoTestFail    = regexp.MustCompile(`(?m)^---\s+FAIL`)
	reGoTestFailAlt = regexp.MustCompile(`(?m)^FAIL\s+`)
)

// NormalizeError strips volatile parts from error output before hashing.
// Removes: timestamps, tmp paths, hex addresses, absolute paths, volatile numbers.
func NormalizeError(output string) string {
	s := output
	s = reTimestampISO.ReplaceAllString(s, "TIMESTAMP")
	s = reTimestampLog.ReplaceAllString(s, "TIMESTAMP")
	s = reTmpUnix.ReplaceAllString(s, "/tmp/TMPDIR")
	s = reTmpWin.ReplaceAllString(s, "TMPDIR")
	s = reHexAddr.ReplaceAllString(s, "0xADDR")
	s = reAbsPathUnix.ReplaceAllStringFunc(s, func(m string) string {
		parts := reAbsPathUnix.FindStringSubmatch(m)
		if len(parts) > 1 {
			return parts[1]
		}
		return m
	})
	s = reAbsPathWin.ReplaceAllStringFunc(s, func(m string) string {
		parts := reAbsPathWin.FindStringSubmatch(m)
		if len(parts) > 1 {
			return parts[1]
		}
		return m
	})
	s = reLineNumber.ReplaceAllString(s, "${1}:LINE")
	s = reTiming.ReplaceAllString(s, "(TIMEs)")
	return s
}

// ErrorSignature extracts a stable signature: "error_class:first_error_line:symbol"
func ErrorSignature(output string) string {
	class := classifyError(output)
	firstLine := extractFirstErrorLine(output)
	symbol := extractSymbol(firstLine)
	return fmt.Sprintf("%s:%s:%s", class, sanitizeSig(firstLine), symbol)
}

// HashError returns a SHA256 hash of the normalized error output.
func HashError(normalized string) string {
	h := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", h)
}

// combineFailedOutput joins the output of all failed verify results.
func combineFailedOutput(results []orchestrator.VerifyResult) string {
	var parts []string
	for _, r := range results {
		if !r.Passed {
			parts = append(parts, r.Output)
		}
	}
	return strings.Join(parts, "\n")
}

// countFailingTests counts lines matching Go test failure patterns.
func countFailingTests(output string) int {
	count := len(reGoTestFail.FindAllString(output, -1))
	count += len(reGoTestFailAlt.FindAllString(output, -1))
	return count
}

var reErrorClassBuild = regexp.MustCompile(`(?i)cannot|undefined|undeclared|import|syntax error|expected`)
var reErrorClassTest = regexp.MustCompile(`(?i)FAIL|--- FAIL|assertion|expected .* got`)
var reErrorClassTimeout = regexp.MustCompile(`(?i)timeout|deadline exceeded|context canceled`)

func classifyError(output string) string {
	switch {
	case reErrorClassTimeout.MatchString(output):
		return "timeout"
	case reErrorClassBuild.MatchString(output):
		return "build"
	case reErrorClassTest.MatchString(output):
		return "test"
	default:
		return "unknown"
	}
}

func extractFirstErrorLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fail") ||
			strings.Contains(lower, "cannot") || strings.Contains(lower, "undefined") {
			if len(trimmed) > 80 {
				trimmed = trimmed[:80]
			}
			return trimmed
		}
	}
	return ""
}

func extractSymbol(line string) string {
	// Try to find a quoted symbol like "foo" or `bar`
	reQuoted := regexp.MustCompile("['\"`]([a-zA-Z_][a-zA-Z0-9_.]*)['\"`]")
	if m := reQuoted.FindStringSubmatch(line); len(m) > 1 {
		return m[1]
	}
	return ""
}

func sanitizeSig(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, s)
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}
