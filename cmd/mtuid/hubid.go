package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
)

// hubIDFile holds the daemon's identity between runs, next to the port
// records. It is not a secret: it only has to stay the same, so that a client
// that reconnects after a daemon restart can tell it is a different hub rather
// than the same one having lost every session.
const hubIDFile = "hub.id"

// hubIDMaxLen bounds what is accepted from the file. Anything longer is
// treated as corruption rather than as an identity.
const hubIDMaxLen = 64

func hubIDPath() (string, error) {
	dir, err := discovery.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, hubIDFile), nil
}

// loadHubID returns the persisted identity, or an empty string when there is
// none yet. A missing file is the normal first run, not an error.
func loadHubID() (string, error) {
	path, err := hubIDPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	id := strings.TrimSpace(string(data))
	if id == "" || len(id) > hubIDMaxLen {
		return "", nil
	}
	return id, nil
}

// saveHubID stores the identity generated on first run.
func saveHubID(id string) error {
	path, err := hubIDPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(id+"\n"), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
