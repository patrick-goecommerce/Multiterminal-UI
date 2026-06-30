package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

func TestStatuslineRenderFlags(t *testing.T) {
	cfg := config.StatusLineSettings{Template: "standard", ShowModel: true, ShowContext: true, ShowGitBranch: true}
	got := statuslineRenderFlags(cfg)
	want := "--template standard --model --context --git"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStatuslineRenderFlagsAllFlags(t *testing.T) {
	cfg := config.StatusLineSettings{
		Template:      "extended",
		ShowModel:     true,
		ShowContext:   true,
		ShowCost:      true,
		ShowGitBranch: true,
		ShowDuration:  true,
	}
	got := statuslineRenderFlags(cfg)
	want := "--template extended --model --context --cost --git --duration"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStatuslineRenderFlagsNoFlags(t *testing.T) {
	cfg := config.StatusLineSettings{Template: "minimal"}
	got := statuslineRenderFlags(cfg)
	want := "--template minimal"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
