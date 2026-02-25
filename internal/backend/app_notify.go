package backend

import (
	"log"
	"net"

	"github.com/go-toast/toast"
)

const focusAddr = "127.0.0.1:41987"

// SendNotification shows a native Windows toast notification with
// "Multiterminal" as the application name. Clicking it brings the
// window to the foreground via the multiterminal: custom protocol.
func (a *AppService) SendNotification(title string, body string) {
	n := toast.Notification{
		AppID:               "Multiterminal",
		Title:               title,
		Message:             body,
		ActivationType:      "protocol",
		ActivationArguments: "multiterminal:focus",
	}
	if err := n.Push(); err != nil {
		log.Printf("[SendNotification] failed: %v", err)
	}
}

// startFocusListener starts a TCP listener that brings the window to
// the foreground when a signal is received (triggered by notification click).
func (a *AppService) startFocusListener() {
	ln, err := net.Listen("tcp", focusAddr)
	if err != nil {
		log.Printf("[focusListener] could not listen on %s: %v", focusAddr, err)
		return
	}
	go func() {
		defer ln.Close()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
			if a.mainWindow != nil {
				if a.mainWindow.IsMinimised() {
					a.mainWindow.UnMinimise()
				}
				a.mainWindow.Show()
				a.mainWindow.SetAlwaysOnTop(true)
				a.mainWindow.SetAlwaysOnTop(false)
			}
		}
	}()
}
