package hub

import (
	"encoding/json"
	"testing"
)

func TestRef_StringRoundTrip(t *testing.T) {
	for _, r := range []Ref{
		{Hub: "a1b2c3", ID: 7},
		{Hub: "host:with:colons", ID: 12},
		{ID: 3}, // legacy
		{},
	} {
		got, err := ParseRef(r.String())
		if err != nil || got != r {
			t.Errorf("ParseRef(%q) = (%+v, %v), want %+v", r.String(), got, err, r)
		}
	}
}

func TestParseRef_RejectsGarbage(t *testing.T) {
	for _, s := range []string{":3", "hub:", "hub:x", "hub:0", "hub:-1", "abc"} {
		if r, err := ParseRef(s); err == nil {
			t.Errorf("ParseRef(%q) = %+v, want an error", s, r)
		}
	}
}

// The frontend holds a Ref as a plain string, and Wails decodes a binding
// argument with json.Unmarshal. Both directions have to agree on that form.
func TestRef_JSONIsAString(t *testing.T) {
	b, err := json.Marshal(struct {
		S Ref         `json:"s"`
		Z Ref         `json:"z"`
		M map[Ref]int `json:"m"`
	}{S: Local("h", 4), M: map[Ref]int{Local("h", 5): 1}})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"s":"h:4","z":"","m":{"h:5":1}}`; string(b) != want {
		t.Errorf("json = %s, want %s", b, want)
	}

	var r Ref
	if err := json.Unmarshal([]byte(`"h:4"`), &r); err != nil || r != Local("h", 4) {
		t.Errorf("unmarshal string = (%+v, %v)", r, err)
	}
}

// Session files, kanban.json and saved panes were written with bare numbers
// before the switch. They must still load, as refs without a hub.
func TestRef_UnmarshalAcceptsLegacyNumbers(t *testing.T) {
	cases := map[string]Ref{`3`: {ID: 3}, `0`: {}, `null`: {}, `-2`: {}}
	for in, want := range cases {
		var r Ref
		if err := json.Unmarshal([]byte(in), &r); err != nil || r != want {
			t.Errorf("unmarshal %s = (%+v, %v), want %+v", in, r, err, want)
		}
	}
	var r Ref
	if json.Unmarshal([]byte(`{"hub":"h"}`), &r) == nil {
		t.Error("an object must not decode as a ref")
	}
	if !(Ref{ID: 3}).Legacy() || Local("h", 3).Legacy() || (Ref{}).Legacy() {
		t.Error("Legacy is true exactly for a ref with an id and no hub")
	}
}
