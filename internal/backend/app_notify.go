package backend

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// focusHandshakeTimeout bounds how long the listener waits for a client to
// present its token before dropping the connection.
const focusHandshakeTimeout = 2 * time.Second

// focusTokenMaxLen caps how much a client may send during the handshake, so a
// stray connection cannot make us buffer without bound.
const focusTokenMaxLen = 128

// SendNotification shows a desktop notification. Clicking it brings the window
// to the foreground via the multiterminal: custom protocol. The implementation
// is per platform (notify_toast_windows.go), because the toast library is
// Windows-only and importing it unconditionally kept this whole package from
// building anywhere else.
func (a *AppService) SendNotification(title string, body string) {
	sendDesktopNotification(title, body, focusActivation)
}

// focusActivation is what a toast click runs: mtui.exe with this argument,
// which signals the running instance (main.go, signalFocus).
const focusActivation = "multiterminal:focus"

// SendPaneNotification is SendNotification for a toast about one pane: a
// click raises the window and jumps to that pane.
func (a *AppService) SendPaneNotification(title string, body string, r hub.Ref) {
	sendDesktopNotification(title, body, focusActivation+"/"+r.String())
}

// FocusSessionEvent asks the frontend to show one pane (a toast was clicked).
type FocusSessionEvent struct {
	ID hub.Ref `json:"id" yaml:"id"`
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
	ok, ref := readFocusRequest(conn, token)
	if !ok {
		log.Printf("[focusListener] rejected focus request from %s: bad token", conn.RemoteAddr())
		return
	}
	a.focusMainWindow()
	if !ref.IsZero() && a.app != nil {
		a.app.Event.Emit("mtui:focus-session", FocusSessionEvent{ID: ref})
	}
}

// focusRefPattern is what a focus request may name: a hub.Ref, "hubid:number".
var focusRefPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}:[0-9]{1,9}$`)

// readFocusRequest reads the token line and an optional pane line. ok is false
// when the token does not match; ref is zero when no valid pane was named. The
// token is checked first: nothing after it is looked at for a wrong caller.
func readFocusRequest(conn net.Conn, want string) (ok bool, ref hub.Ref) {
	_ = conn.SetReadDeadline(time.Now().Add(focusHandshakeTimeout))
	rd := bufio.NewReaderSize(io.LimitReader(conn, 2*focusTokenMaxLen), focusTokenMaxLen)
	line, err := rd.ReadString('\n')
	if err != nil && line == "" {
		return false, hub.Ref{}
	}
	got := strings.TrimSpace(line)
	if got == "" || want == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return false, hub.Ref{}
	}
	next, _ := rd.ReadString('\n')
	r := strings.TrimSpace(next)
	if !focusRefPattern.MatchString(r) {
		return true, hub.Ref{}
	}
	parsed, err := hub.ParseRef(r)
	if err != nil || parsed.IsZero() {
		return true, hub.Ref{}
	}
	return true, parsed
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
