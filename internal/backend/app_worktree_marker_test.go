package backend

import (
	"path/filepath"
	"testing"
)

func TestFinishMarker_Roundtrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "markers.json")
	m := finishMarker{Phase: "merged", Branch: "terminal/x", TargetBranch: "alpha-main"}
	if err := saveFinishMarker(p, `D:\repos\Foo.mt-worktrees\x`, m); err != nil {
		t.Fatal(err)
	}
	got := loadFinishMarkers(p)
	if got[`D:\repos\Foo.mt-worktrees\x`].Phase != "merged" {
		t.Fatalf("roundtrip failed: %+v", got)
	}
	if err := deleteFinishMarker(p, `D:\repos\Foo.mt-worktrees\x`); err != nil {
		t.Fatal(err)
	}
	if len(loadFinishMarkers(p)) != 0 {
		t.Error("marker not deleted")
	}
}

func TestFinishMarker_MissingFileIsEmpty(t *testing.T) {
	if got := loadFinishMarkers(filepath.Join(t.TempDir(), "nope.json")); len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}
