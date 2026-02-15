//go:build windows

package backend

import (
	"log"
	"os"

	"golang.org/x/sys/windows/registry"
)

// registerProtocol registers the multiterminal: custom URI protocol
// in the current user's registry so notification clicks launch our exe.
func registerProtocol() {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("[registerProtocol] could not get exe path: %v", err)
		return
	}

	// HKCU\Software\Classes\multiterminal
	key, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Classes\multiterminal`,
		registry.ALL_ACCESS,
	)
	if err != nil {
		log.Printf("[registerProtocol] create key failed: %v", err)
		return
	}
	key.SetStringValue("", "URL:Multiterminal Protocol")
	key.SetStringValue("URL Protocol", "")
	key.Close()

	// HKCU\Software\Classes\multiterminal\shell\open\command
	cmdKey, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Classes\multiterminal\shell\open\command`,
		registry.ALL_ACCESS,
	)
	if err != nil {
		log.Printf("[registerProtocol] create command key failed: %v", err)
		return
	}
	cmdKey.SetStringValue("", `"`+exe+`" "%1"`)
	cmdKey.Close()

	log.Printf("[registerProtocol] registered multiterminal: protocol -> %s", exe)
}
