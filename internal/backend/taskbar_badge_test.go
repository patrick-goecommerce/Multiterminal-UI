package backend

import "testing"

func TestBadgeText(t *testing.T) {
	for n, want := range map[int]string{0: "", -1: "", 1: "1", 9: "9", 10: "9+", 42: "9+"} {
		if got := badgeText(n); got != want {
			t.Errorf("badgeText(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestBadgePixels(t *testing.T) {
	pix := badgePixels(3)
	if len(pix) != 16*16*4 {
		t.Fatalf("len %d", len(pix))
	}
	alpha := func(x, y int) byte { return pix[(y*16+x)*4+3] }
	if alpha(0, 0) != 0 {
		t.Error("corner must be transparent")
	}
	if alpha(1, 1) != 0xff {
		t.Error("background must be opaque")
	}
	ink := 0
	for i := 0; i < len(pix); i += 4 {
		if pix[i+2] == 0x1a { // red channel of the digit colour
			ink++
		}
	}
	if ink == 0 {
		t.Error("no digit drawn")
	}
}
