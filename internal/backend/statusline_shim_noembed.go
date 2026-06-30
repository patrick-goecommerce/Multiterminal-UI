//go:build !production

package backend

// statuslineBin is empty in non-production builds. The mtui-statusline renderer
// is located as a sibling binary next to the running exe instead.
var statuslineBin []byte

// hookBin is empty in non-production builds. The mtui-hook binary is located as
// a sibling binary next to the running exe instead.
var hookBin []byte
