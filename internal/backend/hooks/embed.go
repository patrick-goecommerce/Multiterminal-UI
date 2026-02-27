package hooks

import _ "embed"

//go:embed hook_handler.ps1
var HookHandlerScript string
