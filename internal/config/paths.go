package config

import (
	"os"
	"path/filepath"
)

// HooksDir is where mtui-hook appends its lifecycle events, one file per agent
// session.
//
// Both the app and the session daemon need it: the hook binary is registered
// by the app, and the file reader runs on whichever process owns the sessions.
// Returning "" (no APPDATA, i.e. not Windows) means hook integration is off,
// which is the same as it has always been.
func HooksDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "Multiterminal", "hooks")
}
