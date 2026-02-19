package backend

import (
	"log"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

// GetFavorites returns the list of favorite paths for the given directory.
func (a *App) GetFavorites(dir string) []string {
	if dir == "" {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	src := a.cfg.Favorites[dir]
	if len(src) == 0 {
		return nil
	}
	result := make([]string, len(src))
	copy(result, src)
	return result
}

// AddFavorite adds a path to the favorites for the given directory
// and persists the config to disk.
func (a *App) AddFavorite(dir string, path string) error {
	if dir == "" || path == "" {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cfg.Favorites == nil {
		a.cfg.Favorites = make(map[string][]string)
	}

	// Check for duplicates
	for _, f := range a.cfg.Favorites[dir] {
		if f == path {
			return nil
		}
	}

	a.cfg.Favorites[dir] = append(a.cfg.Favorites[dir], path)
	log.Printf("[AddFavorite] dir=%q path=%q total=%d", dir, path, len(a.cfg.Favorites[dir]))
	return config.Save(a.cfg)
}

// RemoveFavorite removes a path from the favorites for the given directory
// and persists the config to disk.
func (a *App) RemoveFavorite(dir string, path string) error {
	if dir == "" || path == "" {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cfg.Favorites == nil {
		return nil
	}

	favs := a.cfg.Favorites[dir]
	found := false
	for i, f := range favs {
		if f == path {
			a.cfg.Favorites[dir] = append(favs[:i], favs[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	// Clean up empty entries
	if len(a.cfg.Favorites[dir]) == 0 {
		delete(a.cfg.Favorites, dir)
	}

	log.Printf("[RemoveFavorite] dir=%q path=%q", dir, path)
	return config.Save(a.cfg)
}
