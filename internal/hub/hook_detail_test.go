package hub

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
)

func TestStopDetail(t *testing.T) {
	cases := []struct{ name, msg, want string }{
		{"question mid-paragraph (screenshot case)",
			"Zur Einordnung: Der Spesen-Test ist zweimal gescheitert.\n\nSoll ich den Stack als zwei PRs submitten? Danach starte ich den Review je Layer.",
			"Soll ich den Stack als zwei PRs submitten?"},
		{"question after a statement", "Tests sind grün.\n\nAlles erledigt. Soll ich pushen?", "Soll ich pushen?"},
		{"no question", "Erledigt.\n\nAlle 41 Tests grün.", "Alle 41 Tests grün."},
		{"bold question", "**Soll ich mergen?**", "Soll ich mergen?"},
		{"question introduces a list", "Wie weiter?\n\n1. Mergen\n2. Warten", "Wie weiter?"},
		{"url is not a question", "Siehe https://x.example/?tab=1 dort.", "Siehe https://x.example/?tab=1 dort."},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := stopDetail(c.msg); got != c.want {
			t.Errorf("%s: stopDetail = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestHookDetail(t *testing.T) {
	cases := []struct {
		ev   hooks.Event
		want string
	}{
		{hooks.Event{Event: "PermissionRequest", Tool: "Bash", Detail: "Bash: npm install"}, "Bash: npm install"},
		{hooks.Event{Event: "Notification", Message: "Claude needs your permission"}, "Claude needs your permission"},
		{hooks.Event{Event: "Stop", Message: "Fertig.\n\nSoll ich pushen?"}, "Soll ich pushen?"},
		{hooks.Event{Event: "PostToolUse", Tool: "Bash"}, ""},
	}
	for _, c := range cases {
		if got := hookDetail(c.ev); got != c.want {
			t.Errorf("%s: hookDetail = %q, want %q", c.ev.Event, got, c.want)
		}
	}
}
