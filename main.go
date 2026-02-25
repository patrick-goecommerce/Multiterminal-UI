// Multiterminal UI (mtui) – A GUI terminal multiplexer optimised for Claude Code.
//
// Stack: Go · Wails · Svelte · xterm.js · go-pty
package main

import (
	"embed"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/backend"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
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

	log.Println("Starting Multiterminal UI...")

	cfg := config.Load()
	log.Println("Config loaded, theme:", cfg.Theme)

	// Enable file logging if configured (persistent or auto-enabled after crashes)
	backend.InitLoggingFromConfig(cfg)

	app := application.New(application.Options{
		Name: "Multiterminal",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	svc := backend.NewAppService(app, cfg)
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

// signalFocus connects to the running instance's focus listener
// to bring the window to the foreground.
func signalFocus() {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:41987", 2*time.Second)
	if err != nil {
		return
	}
	conn.Close()
}
