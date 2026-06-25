//go:build production

package backend

import _ "embed"

// shimBin is the statusline-forward shim, embedded into the production
// (single-portable-exe) build. It is extracted at runtime next to the statusline
// script (see ensureStatuslineForward). The release pipeline MUST build
// ./cmd/statusline-forward into internal/backend/statusline-forward.exe before
// building the main binary with `-tags production`, or this embed fails to find
// the file.
//
//go:embed statusline-forward.exe
var shimBin []byte
