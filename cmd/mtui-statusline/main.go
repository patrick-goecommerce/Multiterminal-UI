package main

import (
	"encoding/json"
	"flag"
	"io"
	"math"
	"os"
)

func main() {
	defer func() { _ = recover() }()
	cfg := RenderConfig{}
	flag.StringVar(&cfg.Template, "template", "standard", "")
	flag.BoolVar(&cfg.ShowModel, "model", false, "")
	flag.BoolVar(&cfg.ShowContext, "context", false, "")
	flag.BoolVar(&cfg.ShowCost, "cost", false, "")
	flag.BoolVar(&cfg.ShowGitBranch, "git", false, "")
	flag.BoolVar(&cfg.ShowDuration, "duration", false, "")
	flag.Parse()

	raw, _ := io.ReadAll(os.Stdin)
	s := parseStatus(raw)
	branch := ""
	if cfg.ShowGitBranch {
		branch = gitBranch(s.CurrentDir)
	}
	// Display FIRST (never blocked by the POST).
	os.Stdout.WriteString(Render(cfg, s, branch))
	postCapture(raw, os.Getenv("MTUI_PORT"), os.Getenv("MULTITERMINAL_SESSION_ID"))
}

func parseStatus(raw []byte) Status {
	var d struct {
		Model struct {
			DisplayName string `json:"display_name"`
		} `json:"model"`
		Context struct {
			UsedPercentage *float64 `json:"used_percentage"`
		} `json:"context_window"`
		Cost struct {
			TotalCostUSD  *float64 `json:"total_cost_usd"`
			TotalDuration *int     `json:"total_duration_ms"`
		} `json:"cost"`
		Workspace struct {
			CurrentDir string `json:"current_dir"`
		} `json:"workspace"`
	}
	_ = json.Unmarshal(raw, &d)
	s := Status{
		Model:      d.Model.DisplayName,
		CostUSD:    d.Cost.TotalCostUSD,
		DurationMs: d.Cost.TotalDuration,
		CurrentDir: d.Workspace.CurrentDir,
	}
	if d.Context.UsedPercentage != nil {
		v := int(math.RoundToEven(*d.Context.UsedPercentage))
		s.ContextPct = &v
	}
	return s
}
