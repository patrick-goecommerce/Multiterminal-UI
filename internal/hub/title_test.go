package hub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

func TestCleanTitle(t *testing.T) {
	cases := map[string]string{
		"✳ Datenbankverbindung live anzeige": "Datenbankverbindung live anzeige",
		"⠂ Datenbankverbindung":              "Datenbankverbindung",
		"⠐⠂  Tests":                          "Tests",
		"● läuft":                            "läuft",
		"~/src/mtui":                         "~/src/mtui",
		"/home/user":                         "/home/user",
		"C:\\Users\\x":                       "C:\\Users\\x",
		"":                                   "",
		"✳":                                  "",
	}
	for in, want := range cases {
		if got := cleanTitle(in); got != want {
			t.Errorf("cleanTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

// Two frames of Claude Code's spinner must read as the same title, or every
// frame is a change and an event.
func TestScanOne_TitleIgnoresTheSpinner(t *testing.T) {
	a := terminal.NewSession(1, 24, 80)
	a.Screen.Write([]byte("\x1b]0;✳ Aufgabe\x07"))
	b := terminal.NewSession(2, 24, 80)
	b.Screen.Write([]byte("\x1b]0;⠂ Aufgabe\x07"))
	if ta, tb := scanOne(1, a).Title, scanOne(2, b).Title; ta != tb || ta != "Aufgabe" {
		t.Errorf("titles %q and %q, want both %q", ta, tb, "Aufgabe")
	}
}

// Claude Code's own session name arrives with the status line and is kept.
func TestShimStatusline_KeepsTheSessionName(t *testing.T) {
	h, sess, _ := hookTestHost(t, 5)
	post := func(body string) {
		req := httptest.NewRequest(http.MethodPost, "/api/statusline", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.handleShimStatusline(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
	}
	post(`{"sessionId":5,"payload":{"session_name":"Datenbankverbindung live","cost":{"total_cost_usd":0.5}}}`)
	if got := sess.AgentName(); got != "Datenbankverbindung live" {
		t.Fatalf("AgentName = %q", got)
	}
	// A later status line without the field does not forget it.
	post(`{"sessionId":5,"payload":{"cost":{"total_cost_usd":0.6}}}`)
	if got := sess.AgentName(); got != "Datenbankverbindung live" {
		t.Errorf("name forgotten: %q", got)
	}
	if got := scanOne(5, sess).Name; got != "Datenbankverbindung live" {
		t.Errorf("scan result Name = %q", got)
	}
	s, err := h.Get(5)
	if err != nil || s.AgentName != "Datenbankverbindung live" || s.Name != "Datenbankverbindung live" {
		t.Errorf("summary = %+v, %v", s, err)
	}
}
