// internal/backend/app_statusline_api_test.go
package backend

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

func TestHandleStatuslineUpdatesSession(t *testing.T) {
	a := &AppService{sessions: map[int]*terminal.Session{}}
	sess := terminal.NewSession(5, 24, 80)
	a.sessions[5] = sess

	body := `{"sessionId":5,"payload":{"cost":{"total_cost_usd":1.23},` +
		`"context_window":{"used_percentage":40},"model":{"display_name":"Opus 4.8"}}}`
	req := httptest.NewRequest("POST", "/api/statusline", strings.NewReader(body))
	rec := httptest.NewRecorder()

	a.handleStatusline(rec, req)

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

func TestHandleStatuslineUnknownSessionNoCrash(t *testing.T) {
	a := &AppService{sessions: map[int]*terminal.Session{}}
	req := httptest.NewRequest("POST", "/api/statusline", strings.NewReader(`{"sessionId":99,"payload":{}}`))
	rec := httptest.NewRecorder()
	a.handleStatusline(rec, req) // must not panic
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleStatuslineGarbageBodyIsBadRequest(t *testing.T) {
	a := &AppService{sessions: map[int]*terminal.Session{}}
	req := httptest.NewRequest("POST", "/api/statusline", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()
	a.handleStatusline(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
