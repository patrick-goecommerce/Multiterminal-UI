// cmd/mtui-statusline/render_test.go
package main

import "testing"

func p[T any](v T) *T { return &v }

func TestRenderStandardFull(t *testing.T) {
	cfg := RenderConfig{Template: "standard", ShowModel: true, ShowContext: true, ShowCost: true, ShowGitBranch: true}
	s := Status{Model: "Opus 4.8", ContextPct: p(45), CostUSD: p(0.42), CurrentDir: "/x"}
	got := Render(cfg, s, "main")
	want := "[Opus 4.8] | \x1b[32m████░░░░░░\x1b[0m 45% | $0.420 | git:main\n"
	if got != want {
		t.Fatalf("\n got=%q\nwant=%q", got, want)
	}
}

func TestRenderMinimalNoColorNoBar(t *testing.T) {
	cfg := RenderConfig{Template: "minimal", ShowModel: true, ShowContext: true}
	s := Status{Model: "Sonnet", ContextPct: p(72)}
	got := Render(cfg, s, "")
	want := "[Sonnet] | 72%\n"
	if got != want {
		t.Fatalf("\n got=%q\nwant=%q", got, want)
	}
}

func TestRenderContextColorThresholds(t *testing.T) {
	cfg := RenderConfig{Template: "standard", ShowContext: true}
	for pct, color := range map[int]string{50: "\x1b[32m", 75: "\x1b[33m", 95: "\x1b[31m"} {
		got := Render(cfg, Status{ContextPct: p(pct)}, "")
		if got[:len(color)] != color {
			t.Fatalf("pct=%d got prefix %q want %q", pct, got[:len(color)], color)
		}
	}
}

func TestRenderModelFallbackQuestionMark(t *testing.T) {
	got := Render(RenderConfig{Template: "minimal", ShowModel: true}, Status{Model: ""}, "")
	if got != "[?]\n" {
		t.Fatalf("got %q want %q", got, "[?]\n")
	}
}

func TestRenderExtendedTwoLines(t *testing.T) {
	cfg := RenderConfig{Template: "extended", ShowModel: true, ShowContext: true, ShowGitBranch: true}
	s := Status{Model: "Opus", ContextPct: p(20), CurrentDir: "/home/u/proj"}
	got := Render(cfg, s, "dev")
	want := "\x1b[36m[Opus]\x1b[0m | proj | git:dev\n\x1b[32m██░░░░░░░░\x1b[0m 20%\n"
	if got != want {
		t.Fatalf("\n got=%q\nwant=%q", got, want)
	}
}
