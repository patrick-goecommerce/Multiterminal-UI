//go:build !windows

package backend

import "log"

// sendDesktopNotification has no implementation outside Windows yet. It is not
// silently dropped: a missing notification is the kind of thing a user reports
// as "it never told me", and the log is where that gets answered.
func sendDesktopNotification(title, body string) {
	log.Printf("[SendNotification] no desktop notifier on this platform: %s — %s", title, body)
}
