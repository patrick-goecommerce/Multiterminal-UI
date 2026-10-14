//go:build windows

package backend

import (
	"log"

	"github.com/go-toast/toast"
)

// sendDesktopNotification shows a native Windows toast with "Multiterminal" as
// the application name. The activation protocol is what the focus listener
// answers when the user clicks it (see startFocusListener).
func sendDesktopNotification(title, body, activation string) {
	n := toast.Notification{
		AppID:               "Multiterminal",
		Title:               toastSafe(title),
		Message:             toastSafe(body),
		ActivationType:      "protocol",
		ActivationArguments: toastAttr(activation),
	}
	if err := n.Push(); err != nil {
		log.Printf("[SendNotification] failed: %v", err)
	}
}
