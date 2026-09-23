package hub

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

func TestShimStatuslineUpdatesSession(t *testing.T) {
	h := newTestHost(t, nil)
	sess := terminal.NewSession(5, 24, 80)
	h.AdoptForTest(5, sess)

	body := `{"sessionId":5,"payload":{"cost":{"total_cost_usd":1.23},` +
		`"context_window":{"used_percentage":40},"model":{"display_name":"Opus 4.8"}}}`
	req := httptest.NewRequest("POST", "/api/statusline", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.handleShimStatusline(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := sess.GetTokens().TotalCost; got != 1.23 {
		t.Fatalf("TotalCost = %v, want 1.23", got)
	}
	pct, model, src := sess.StatuslineInfo()
	if pct != 40 || model != "Opus 4.8" || src != terminal.CostSourceStatusline {
		t.Fatalf("StatuslineInfo = (%d,%q,%d), want (40,Opus 4.8,statusline)", pct, model, src)
	}
}

func TestShimStatuslineUnknownSessionNoCrash(t *testing.T) {
	h := newTestHost(t, nil)
	req := httptest.NewRequest("POST", "/api/statusline", strings.NewReader(`{"sessionId":99,"payload":{}}`))
	rec := httptest.NewRecorder()
	h.handleShimStatusline(rec, req) // must not panic
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestShimStatuslineGarbageBodyIsBadRequest(t *testing.T) {
	h := newTestHost(t, nil)
	req := httptest.NewRequest("POST", "/api/statusline", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()
	h.handleShimStatusline(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestShimStatuslineNonPostReturns405(t *testing.T) {
	h := newTestHost(t, nil)
	req := httptest.NewRequest("GET", "/api/statusline", nil)
	rec := httptest.NewRecorder()
	h.handleShimStatusline(rec, req)
	if rec.Code != 405 {
		t.Fatalf("status = %d, want 405 for GET", rec.Code)
	}
}

func TestShimStatuslineFractionalPercentageTruncates(t *testing.T) {
	// float64 40.9 must truncate to int 40 (not round to 41).
	h := newTestHost(t, nil)
	sess := terminal.NewSession(7, 24, 80)
	h.AdoptForTest(7, sess)

	body := `{"sessionId":7,"payload":{"context_window":{"used_percentage":40.9}}}`
	req := httptest.NewRequest("POST", "/api/statusline", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.handleShimStatusline(rec, req)

	pct, _, _ := sess.StatuslineInfo()
	if pct != 40 {
		t.Fatalf("context pct = %d, want 40 (truncated from 40.9)", pct)
	}
}

// The tmux shim has no tmux to talk to: MTUI answers so an agent that reaches
// for one gets a reply instead of an error, and reports what was asked for.
func TestShimTmuxLog_ReportsTheCommand(t *testing.T) {
	sink := newSink()
	h := NewEmbedded(Options{Sink: sink})
	t.Cleanup(h.Release)

	body := `{"args":["split-window","-h"],"dir":"D:/repos/foo","env":"TMUX="}`
	req := httptest.NewRequest("POST", "/api/tmux/log", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.handleShimTmuxLog(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	sink.await(t, EventTmuxCommand)
}

// The port has to come from the host, because a session's environment names it
// at launch and cannot be told a new one later.
func TestShim_ReportsItsPort(t *testing.T) {
	h := NewEmbedded(Options{Shim: true})
	t.Cleanup(h.Release)

	if h.Info().ShimPort <= 0 {
		t.Errorf("ShimPort = %d, want a bound port", h.Info().ShimPort)
	}
}

// A host without the shim says so rather than reporting a port nobody serves.
func TestShim_OffMeansNoPort(t *testing.T) {
	h := newTestHost(t, nil)
	if got := h.Info().ShimPort; got != 0 {
		t.Errorf("ShimPort = %d, want 0", got)
	}
}
