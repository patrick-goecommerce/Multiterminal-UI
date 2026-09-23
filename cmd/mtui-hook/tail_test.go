package main

import "testing"

func TestTail_KeepsTheEndAndWholeRunes(t *testing.T) {
	if got := tail("kurz", 10); got != "kurz" {
		t.Errorf("short input changed: %q", got)
	}
	if got := tail("öffnen? Ja", 4); got != "? Ja" {
		t.Errorf("tail = %q, want %q", got, "? Ja")
	}
	if got := tail("äöü", 2); got != "öü" {
		t.Errorf("tail split a rune: %q", got)
	}
}
