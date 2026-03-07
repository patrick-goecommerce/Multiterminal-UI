You are working on a Go project. Follow these principles:

- **Error handling:** always check errors. Return `fmt.Errorf("operation: %w", err)` to add context while preserving the error chain. Never use `panic` for expected errors — reserve it for truly unrecoverable programmer mistakes.
- **Naming:** use short, descriptive names. Receivers are 1-2 letters (`s` for `Server`). Local variables are concise (`buf`, `ctx`, `err`). Exported names don't stutter (`config.Config`, not `config.ConfigStruct`).
- **Interfaces:** define interfaces where they are used, not where they are implemented. Keep interfaces small (1-3 methods). Accept interfaces, return concrete types.
- **Struct design:** use value receivers for small immutable types, pointer receivers for everything else. Group related fields. Add `json` and `yaml` tags when structs cross serialization boundaries.
- **Concurrency:** prefer channels for communication between goroutines, mutexes for protecting shared state. Always use `defer mu.Unlock()` immediately after `mu.Lock()`. Never hold a lock while performing I/O or calling external functions.
- **Goroutines:** every goroutine must have a clear shutdown path. Use `context.Context` for cancellation. Use `errgroup.Group` for fan-out/fan-in patterns with error propagation.
- **Context:** pass `context.Context` as the first parameter to functions that do I/O, make network calls, or may be long-running. Never store contexts in structs.
- **Standard library first:** use `net/http`, `encoding/json`, `os`, `io`, `strings`, `fmt` before reaching for third-party packages. Use `slices`, `maps`, and `cmp` packages (Go 1.21+) for generic operations.
- **Table-driven tests:** structure tests as `[]struct{ name string; input; want; wantErr bool }` slices with `t.Run(tc.name, ...)`. Use `testify/assert` or `cmp.Diff` for readable assertions. Use `t.Helper()` in test helper functions.
- **File organization:** keep files under 300 lines. Group by responsibility: `server.go`, `server_routes.go`, `server_middleware.go`. One package per directory — package name matches the directory name.
- **Dependencies:** use `go mod tidy` to keep `go.mod` clean. Pin major versions. Vendor only when reproducibility in CI is critical.
- **Logging:** use structured logging (`slog` in Go 1.21+ or `zerolog`/`zap`). Log at appropriate levels: `Error` for failures requiring attention, `Info` for significant operations, `Debug` for troubleshooting.
- **Resource cleanup:** use `defer` for closing files, connections, and response bodies immediately after opening. Ensure `io.ReadCloser` bodies are always closed, even on error paths.
- **Zero values are useful.** Design structs so the zero value is valid and meaningful (e.g., `sync.Mutex`, `bytes.Buffer`). Avoid constructors when the zero value works.
