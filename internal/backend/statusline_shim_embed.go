//go:build production

package backend

import _ "embed"

// statuslineBin is the GUI-subsystem statusline renderer binary, embedded for
// production builds. The release pipeline MUST build ./cmd/mtui-statusline into
// internal/backend/mtui-statusline.exe before the production build.
//
//go:embed mtui-statusline.exe
var statuslineBin []byte

// hookBin is the GUI-subsystem hook handler binary, embedded for production
// builds. The release pipeline MUST build ./cmd/mtui-hook into
// internal/backend/mtui-hook.exe before the production build.
//
//go:embed mtui-hook.exe
var hookBin []byte
