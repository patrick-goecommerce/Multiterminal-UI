package backend

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/go-toast/toast"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
)

// focusHandshakeTimeout bounds how long the listener waits for a client to
// present its token before dropping the connection.
const focusHandshakeTimeout = 2 * time.Second

// focusTokenMaxLen caps how much a client may send during the handshake, so a
// stray connection cannot make us buffer without bound.
const focusTokenMaxLen = 128

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

// startFocusListener starts a TCP listener that brings the window to the
// foreground when a notification click signals it.
//
// The listener binds port 0 rather than the old fixed 41987: that port was
// machine-wide, so on a multi-user / RDP machine the second instance could not
// bind it at all and a notification click by one user raised another user's
// window (issue #183). The OS-assigned port is published in the per-user
// discovery directory, and the caller must echo the record's token before
// anything is focused — a bare TCP connect from a recycled-port neighbour is
// not enough.
func (a *AppService) startFocusListener() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("focus listener listen: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	rec, err := discovery.Publish(discovery.ServiceFocus, port)
	if err != nil {
		ln.Close()
		return fmt.Errorf("focus listener publish: %w", err)
	}
	a.focusToken = rec.Token
	log.Printf("[focusListener] listening on 127.0.0.1:%d", port)

	go a.acceptFocusSignals(ln, rec.Token)
	return nil
}

// acceptFocusSignals serves the focus listener until it is closed.
func (a *AppService) acceptFocusSignals(ln net.Listener, token string) {
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go a.handleFocusSignal(conn, token)
	}
}

// handleFocusSignal validates one focus request and raises the main window.
func (a *AppService) handleFocusSignal(conn net.Conn, token string) {
	defer conn.Close()
	if !readFocusToken(conn, token) {
		log.Printf("[focusListener] rejected focus request from %s: bad token", conn.RemoteAddr())
		return
	}
	a.focusMainWindow()
}

// readFocusToken reads the client's token line and reports whether it matches.
func readFocusToken(conn net.Conn, want string) bool {
	_ = conn.SetReadDeadline(time.Now().Add(focusHandshakeTimeout))
	line, err := bufio.NewReaderSize(conn, focusTokenMaxLen).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	got := strings.TrimSpace(line)
	if got == "" || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// focusMainWindow raises and un-minimises the main window.
func (a *AppService) focusMainWindow() {
	if a.mainWindow == nil {
		return
	}
	if a.mainWindow.IsMinimised() {
		a.mainWindow.UnMinimise()
	}
	a.mainWindow.Show()
	a.mainWindow.SetAlwaysOnTop(true)
	a.mainWindow.SetAlwaysOnTop(false)
}
