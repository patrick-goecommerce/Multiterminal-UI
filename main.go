// Multiterminal UI (mtui) – A GUI terminal multiplexer optimised for Claude Code.
//
// Stack: Go · Wails · Svelte · xterm.js · go-pty
package main

import (
	"embed"
	"log"

	"github.com/patrick-goecommerce/multiterminal/internal/backend"
	"github.com/patrick-goecommerce/multiterminal/internal/config"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	log.Println("Starting Multiterminal UI...")

	cfg := config.Load()
	log.Println("Config loaded, theme:", cfg.Theme)

	// Enable file logging if configured (persistent or auto-enabled after crashes)
	backend.InitLoggingFromConfig(cfg)

	app := backend.NewApp(cfg)
	log.Println("App created, starting Wails...")

	err := wails.Run(&options.App{
		Title:            "Multiterminal UI",
		Width:            1400,
		Height:           900,
		MinWidth:         800,
		MinHeight:        600,
		WindowStartState: options.Maximised,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.Startup,
		OnShutdown: app.Shutdown,
		Bind: []interface{}{
			app,
		},
		LogLevel: logger.DEBUG,
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		log.Println("Wails error:", err)
		println("Error:", err.Error())
	}
	log.Println("Multiterminal UI exited")
}
