package backend

import "testing"

func TestGetFavorites_EmptyDir(t *testing.T) {
	a := newTestApp()
	if a.GetFavorites("") != nil {
		t.Fatal("empty dir should return nil")
	}
}

func TestGetFavorites_NoFavorites(t *testing.T) {
	a := newTestApp()
	result := a.GetFavorites("/some/dir")
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestAddFavorite_Basic(t *testing.T) {
	a := newTestApp()
	if err := a.AddFavorite("/project", "/project/main.go"); err != nil {
		t.Fatalf("AddFavorite failed: %v", err)
	}

	favs := a.GetFavorites("/project")
	if len(favs) != 1 {
		t.Fatalf("expected 1 favorite, got %d", len(favs))
	}
	if favs[0] != "/project/main.go" {
		t.Errorf("fav = %q, want '/project/main.go'", favs[0])
	}
}

func TestAddFavorite_NoDuplicates(t *testing.T) {
	a := newTestApp()
	a.AddFavorite("/project", "/project/main.go")
	a.AddFavorite("/project", "/project/main.go")

	favs := a.GetFavorites("/project")
	if len(favs) != 1 {
		t.Fatalf("expected 1 favorite (no duplicates), got %d", len(favs))
	}
}

func TestAddFavorite_MultiplePaths(t *testing.T) {
	a := newTestApp()
	a.AddFavorite("/project", "/project/main.go")
	a.AddFavorite("/project", "/project/src")

	favs := a.GetFavorites("/project")
	if len(favs) != 2 {
		t.Fatalf("expected 2 favorites, got %d", len(favs))
	}
}

func TestAddFavorite_MultipleDirectories(t *testing.T) {
	a := newTestApp()
	a.AddFavorite("/project1", "/project1/main.go")
	a.AddFavorite("/project2", "/project2/app.py")

	if len(a.GetFavorites("/project1")) != 1 {
		t.Fatal("project1 should have 1 favorite")
	}
	if len(a.GetFavorites("/project2")) != 1 {
		t.Fatal("project2 should have 1 favorite")
	}
}

func TestRemoveFavorite_Basic(t *testing.T) {
	a := newTestApp()
	a.AddFavorite("/project", "/project/main.go")
	a.AddFavorite("/project", "/project/src")

	if err := a.RemoveFavorite("/project", "/project/main.go"); err != nil {
		t.Fatalf("RemoveFavorite failed: %v", err)
	}

	favs := a.GetFavorites("/project")
	if len(favs) != 1 {
		t.Fatalf("expected 1 favorite after removal, got %d", len(favs))
	}
	if favs[0] != "/project/src" {
		t.Errorf("remaining fav = %q, want '/project/src'", favs[0])
	}
}

func TestRemoveFavorite_LastOneCleanup(t *testing.T) {
	a := newTestApp()
	a.AddFavorite("/project", "/project/main.go")
	a.RemoveFavorite("/project", "/project/main.go")

	favs := a.GetFavorites("/project")
	if favs != nil {
		t.Fatalf("expected nil after removing last favorite, got %v", favs)
	}
}

func TestRemoveFavorite_NonExistent(t *testing.T) {
	a := newTestApp()
	if err := a.RemoveFavorite("/project", "/project/nope.go"); err != nil {
		t.Fatalf("removing non-existent should not error: %v", err)
	}
}

func TestAddFavorite_EmptyArgs(t *testing.T) {
	a := newTestApp()
	if err := a.AddFavorite("", "/project/main.go"); err != nil {
		t.Fatal("empty dir should return nil, not error")
	}
	if err := a.AddFavorite("/project", ""); err != nil {
		t.Fatal("empty path should return nil, not error")
	}
}
