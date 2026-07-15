package engine

import (
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/orchestrator"
)

func failResult(output string) []orchestrator.VerifyResult {
	return []orchestrator.VerifyResult{{
		Command: "go build ./...", ExitCode: 1, Output: output, Passed: false,
	}}
}

func passResult() []orchestrator.VerifyResult {
	return []orchestrator.VerifyResult{{
		Command: "go build ./...", ExitCode: 0, Output: "ok", Passed: true,
	}}
}

func TestSameErrorDetected(t *testing.T) {
	d := NewStepLoopDetector()
	err := "cannot find package \"foo\"\n"
	d.Record(failResult(err), 10)
	d.Record(failResult(err), 15)
	signals := d.Detect()
	if !hasSignal(signals, "same_error") {
		t.Fatalf("expected same_error signal, got %v", signals)
	}
}

func TestSameErrorNotTriggered(t *testing.T) {
	d := NewStepLoopDetector()
	d.Record(failResult("cannot find package \"foo\""), 10)
	d.Record(failResult("undefined: bar"), 15)
	signals := d.Detect()
	if hasSignal(signals, "same_error") {
		t.Fatalf("did not expect same_error signal, got %v", signals)
	}
}

func TestGrowingDiffDetected(t *testing.T) {
	d := NewStepLoopDetector()
	err := "cannot find package \"foo\""
	d.Record(failResult(err), 100)
	d.Record(failResult(err), 200)
	d.Record(failResult(err), 300)
	signals := d.Detect()
	if !hasSignal(signals, "growing_diff") {
		t.Fatalf("expected growing_diff signal, got %v", signals)
	}
}

func TestErrorPendulumDetected(t *testing.T) {
	d := NewStepLoopDetector()
	d.Record(failResult("cannot find package \"foo\""), 10)
	d.Record(failResult("undefined: bar"), 10)
	d.Record(failResult("cannot find package \"foo\""), 10)
	signals := d.Detect()
	if !hasSignal(signals, "error_pendulum") {
		t.Fatalf("expected error_pendulum signal, got %v", signals)
	}
}

func TestNoTestProgressDetected(t *testing.T) {
	d := NewStepLoopDetector()
	output := "--- FAIL: TestA\n--- FAIL: TestB\n--- FAIL: TestC\n--- FAIL: TestD\n--- FAIL: TestE\n"
	d.Record(failResult(output), 10)
	d.Record(failResult(output), 20)
	d.Record(failResult(output), 30)
	signals := d.Detect()
	if !hasSignal(signals, "no_test_progress") {
		t.Fatalf("expected no_test_progress signal, got %v", signals)
	}
}

func TestNormalizeErrorStripsTimestamps(t *testing.T) {
	input := "error at 2026-04-01T22:06:39.123Z: something failed"
	result := NormalizeError(input)
	if strings.Contains(result, "2026-04-01") {
		t.Fatalf("expected timestamp stripped, got: %s", result)
	}
	if !strings.Contains(result, "TIMESTAMP") {
		t.Fatalf("expected TIMESTAMP placeholder, got: %s", result)
	}
}

func TestNormalizeErrorStripsTmpPaths(t *testing.T) {
	input := "error in /tmp/go-build123456/main.go: compile failed"
	result := NormalizeError(input)
	if strings.Contains(result, "go-build123456") {
		t.Fatalf("expected tmp path stripped, got: %s", result)
	}
}

func TestNormalizeErrorStripsHexAddresses(t *testing.T) {
	input := "panic at 0x7ff77af5f165 in runtime"
	result := NormalizeError(input)
	if strings.Contains(result, "0x7ff77af5f165") {
		t.Fatalf("expected hex address stripped, got: %s", result)
	}
	if !strings.Contains(result, "0xADDR") {
		t.Fatalf("expected 0xADDR placeholder, got: %s", result)
	}
}

func TestNormalizeErrorStripsLineNumbers(t *testing.T) {
	input := "app.go:142: undefined: foo"
	result := NormalizeError(input)
	if strings.Contains(result, ":142") {
		t.Fatalf("expected line number stripped, got: %s", result)
	}
	if !strings.Contains(result, "app.go:LINE") {
		t.Fatalf("expected app.go:LINE, got: %s", result)
	}
}

func TestErrorSignatureExtractsClass(t *testing.T) {
	sig := ErrorSignature("cannot find package \"fmt\"")
	if !strings.HasPrefix(sig, "build:") {
		t.Fatalf("expected build class, got: %s", sig)
	}
}

func TestHashErrorDeterministic(t *testing.T) {
	input := "some error output"
	h1 := HashError(input)
	h2 := HashError(input)
	if h1 != h2 {
		t.Fatalf("hash not deterministic: %s != %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Fatalf("expected 64 char hex hash, got %d chars", len(h1))
	}
}

func TestNoSignalsOnFirstAttempt(t *testing.T) {
	d := NewStepLoopDetector()
	d.Record(failResult("some error"), 10)
	signals := d.Detect()
	if len(signals) != 0 {
		t.Fatalf("expected no signals on first attempt, got %v", signals)
	}
}

func TestRealGoCompilerOutput(t *testing.T) {
	output := `# github.com/example/project/internal/engine
internal/engine/loop_step.go:42:15: cannot use x (variable of type string) as int value in argument to fmt.Println
internal/engine/loop_step.go:55:2: undefined: nonExistentFunc
FAIL	github.com/example/project/internal/engine [build failed]`

	d := NewStepLoopDetector()
	d.Record(failResult(output), 10)
	d.Record(failResult(output), 20)
	signals := d.Detect()
	if !hasSignal(signals, "same_error") {
		t.Fatalf("expected same_error for real compiler output, got %v", signals)
	}

	// Verify normalization is stable
	norm := NormalizeError(output)
	if strings.Contains(norm, ":42:") || strings.Contains(norm, ":55:") {
		t.Fatalf("expected line numbers stripped from real output, got: %s", norm)
	}
}

func hasSignal(signals []orchestrator.LoopSignal, typ string) bool {
	for _, s := range signals {
		if s.Type == typ {
			return true
		}
	}
	return false
}
