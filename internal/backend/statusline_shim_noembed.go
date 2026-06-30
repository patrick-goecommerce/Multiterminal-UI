//go:build !production

package backend

// shimBin is empty in non-production builds (dev / CI `go build ./...`). The
// statusline-forward shim is then located as a sibling binary instead of being
// extracted. The production build (statusline_shim_embed.go) embeds the real exe.
var shimBin []byte

// statuslineBin is empty in non-production builds. The mtui-statusline renderer
// is located as a sibling binary next to the running exe instead.
var statuslineBin []byte

// hookBin is empty in non-production builds. The mtui-hook binary is located as
// a sibling binary next to the running exe instead.
var hookBin []byte
