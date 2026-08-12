// Multiterminal UI (mtui) – A GUI terminal multiplexer optimised for Claude Code.
//
// Stack: Go · Wails · Svelte · xterm.js · go-pty
package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/backend"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// If launched via multiterminal: protocol (notification click),
	// signal the running instance to focus and exit immediately.
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "multiterminal:") {
			signalFocus()
			return
		}
	}

	// --- CLI flags ---
	var (
		flagListTabs  = flag.Bool("list-tabs", false, "List saved tab names and exit")
		flagRemoveTab = flag.String("remove-tab", "", "Remove a tab by name and exit")
		flagClean     = flag.Bool("clean", false, "Delete the session file and exit")
		flagSafeMode  = flag.Bool("safe-mode", false, "Start without loading sessions; restore session on close")
		flagDebug     = flag.Bool("debug", false, "Open debug console + enable file logging from startup")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: mtui [options]\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// CLI-only commands — execute and exit without starting the GUI.
	if *flagListTabs {
		runListTabs()
		return
	}
	if *flagRemoveTab != "" {
		runRemoveTab(*flagRemoveTab)
		return
	}
	if *flagClean {
		runClean()
		return
	}

	// --- GUI start ---

	// Debug mode: open console window + file logging BEFORE anything else.
	if *flagDebug {
		backend.InitDebugMode()
	}

	log.Println("Starting Multiterminal UI...")

	cfg := config.Load()
	log.Println("Config loaded, theme:", cfg.Theme)

	// Config-based logging (for non-debug mode, when LoggingEnabled is set in YAML).
	if !*flagDebug {
		backend.InitLoggingFromConfig(cfg)
	}

	app := application.New(application.Options{
		Name: "Multiterminal",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	svc := backend.NewAppService(app, cfg, *flagSafeMode)
	log.Println("AppService created, starting Wails...")

	app.RegisterService(application.NewService(svc))

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            backend.VersionTitle(),
		Width:            1400,
		Height:           900,
		MinWidth:         800,
		MinHeight:        600,
		URL:              "/?windowId=main",
		BackgroundColour: application.NewRGBA(30, 30, 30, 255),
	})
	mainWindow.Center()
	mainWindow.Maximise()

	svc.SetMainWindow(mainWindow)

	if err := app.Run(); err != nil {
		log.Println("Wails error:", err)
	}
	log.Println("Multiterminal UI exited")
}

// runListTabs prints tab names from the saved session, one per line.
func runListTabs() {
	state := config.LoadSession()
	if state == nil {
		fmt.Fprintln(os.Stderr, "No session file found.")
		os.Exit(1)
	}
	for _, tab := range state.Tabs {
		fmt.Println(tab.Name)
	}
}

// runRemoveTab removes a single tab by name from the session file.
func runRemoveTab(name string) {
	found, err := config.RemoveTab(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !found {
		fmt.Fprintf(os.Stderr, "Tab %q not found in session.\n", name)
		os.Exit(1)
	}
	fmt.Printf("Tab %q removed.\n", name)
}

// runClean deletes the session file entirely.
func runClean() {
	config.ClearSession()
	fmt.Println("Session file cleared.")
}

// signalFocus connects to the running instance's focus listener to bring its
// window to the foreground.
//
// The address comes from the per-user discovery record, not from the old fixed
// port 41987: that port was machine-wide, so on an RDP host a notification
// click by one Windows user raised the window of whichever user's instance had
// won the port (issue #183). Resolve only ever sees records written by this
// same user's instances, and rejects records whose process has died. The token
// echoed on the connection proves we reached that instance's listener and not
// an unrelated process that inherited a recycled port.
func signalFocus() {
	rec, err := discovery.Resolve(discovery.ServiceFocus)
	if err != nil {
		return // no running instance of this user, or its listener never started
	}
	conn, err := net.DialTimeout("tcp", rec.Addr(), 2*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, _ = conn.Write([]byte(rec.Token + "\n"))
}
