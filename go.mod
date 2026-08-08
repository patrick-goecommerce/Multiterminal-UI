module github.com/patrick-goecommerce/Multiterminal-UI

go 1.25.5

// Pins the build toolchain, not the language level: go 1.26.0 ships 15 known
// stdlib vulnerabilities that 1.26.5 closes (crypto/x509, crypto/tls, net/http,
// archive/tar). CI resolves go-version "1.26" to the latest patch anyway, so
// this only matters for local builds — which is where the release binary comes
// from. Raise it when a newer patch release lands.
toolchain go1.26.5

require (
	github.com/aymanbagabas/go-pty v0.2.3
	github.com/go-toast/toast v0.0.0-20190211030409-01e6764cf0a4
	github.com/mark3labs/mcp-go v0.57.0
	github.com/wailsapp/wails/v3 v3.0.0-alpha2.117
	golang.org/x/sys v0.47.0
	golang.org/x/text v0.40.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/creack/pty v1.1.24 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jchv/go-winloader v0.0.0-20250406163304-c1995be93bd1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/u-root/u-root v0.16.0 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/crypto v0.54.0 // indirect
)
