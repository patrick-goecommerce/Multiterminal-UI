// internal/terminal/session_statusline_test.go
package terminal

import "testing"

func TestSetStatuslineDataSetsFieldsAndSource(t *testing.T) {
	s := NewSession(1, 24, 80)
	s.SetStatuslineData(0.42, 35, "claude-opus-4-8")

	if got := s.GetTokens().TotalCost; got != 0.42 {
		t.Fatalf("TotalCost = %v, want 0.42", got)
	}
	pct, model, src := s.StatuslineInfo()
	if pct != 35 || model != "claude-opus-4-8" || src != CostSourceStatusline {
		t.Fatalf("StatuslineInfo() = (%d,%q,%d), want (35,claude-opus-4-8,statusline)", pct, model, src)
	}
}

func TestScanTokensDoesNotOverwriteStatuslineCost(t *testing.T) {
	s := NewSession(1, 24, 80)
	s.SetStatuslineData(0.42, 0, "")
	// Put a different cost on screen; the scrape must NOT win.
	_, _ = s.Screen.Write([]byte("context left: $0.99\r\n"))

	s.ScanTokens()

	if got := s.GetTokens().TotalCost; got != 0.42 {
		t.Fatalf("TotalCost = %v after ScanTokens, want 0.42 (statusline authoritative)", got)
	}
}

func TestScanTokensStillScrapesWhenNoStatusline(t *testing.T) {
	s := NewSession(1, 5, 80) // small screen so row 0 falls within the last-10-row scan window
	_, _ = s.Screen.Write([]byte("cost: $0.99\r\n"))

	s.ScanTokens()

	if got := s.GetTokens().TotalCost; got != 0.99 {
		t.Fatalf("TotalCost = %v, want 0.99 (scrape active)", got)
	}
}
