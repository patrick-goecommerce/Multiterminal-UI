# tmux-Shim Phase 1: Logging Shim Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a logging tmux shim that captures all tmux commands Claude Code sends, enabling us to discover the exact API surface needed for Phase 2.

**Architecture:** A Go binary (`tmux.exe`) intercepts tmux CLI calls, logs them to file and forwards them via HTTP to a small API server embedded in Multiterminal. The Multiterminal backend starts an HTTP listener on a random port and injects `MTUI_PORT` + the shim directory into every PTY session's environment.

**Tech Stack:** Go (shim binary + HTTP server), net/http, os/exec

---

### Task 1: HTTP API Server in Backend

**Files:**
- Create: `internal/backend/app_tmux_api.go`
- Modify: `internal/backend/app.go:38` (add `tmuxAPIPort` field)
- Modify: `internal/backend/app.go:78-109` (start server in `ServiceStartup`)

**Step 1: Create `app_tmux_api.go`**

```go
// internal/backend/app_tmux_api.go
package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
)

// TmuxLogEntry represents a logged tmux command from the shim.
type TmuxLogEntry struct {
	Args []string `json:"args"`
	Dir  string   `json:"dir"`
	Env  string   `json:"env"`
}

// startTmuxAPI starts a localhost HTTP server for the tmux shim.
// Returns the port number.
func (a *AppService) startTmuxAPI() (int, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tmux/log", a.handleTmuxLog)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("tmux API listen: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	log.Printf("[tmux-api] listening on port %d", port)

	go http.Serve(listener, mux)
	return port, nil
}

func (a *AppService) handleTmuxLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var entry TmuxLogEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("[tmux-shim] args=%v dir=%q", entry.Args, entry.Dir)

	// Emit event so frontend can see it too
	if a.app != nil {
		a.app.Event.Emit("tmux:command", entry)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "logged"})
}
```

**Step 2: Add `tmuxAPIPort` field to AppService**

In `internal/backend/app.go`, add to the struct (around line 44):
```go
tmuxAPIPort int // port for the tmux shim HTTP API
```

**Step 3: Start the API in ServiceStartup**

In `internal/backend/app.go` `ServiceStartup()`, add after `registerProtocol()` (around line 108):
```go
// Start tmux shim API server
if port, err := a.startTmuxAPI(); err != nil {
	log.Printf("[tmux-api] failed to start: %v", err)
} else {
	a.tmuxAPIPort = port
}
```

**Step 4: Expose port to frontend (for GetTmuxAPIPort binding)**

Add to `app_tmux_api.go`:
```go
// GetTmuxAPIPort returns the tmux shim API port (0 if not started).
func (a *AppService) GetTmuxAPIPort() int {
	return a.tmuxAPIPort
}
```

**Step 5: Build and verify**

Run: `go build -o build/bin/multiterminal.exe -tags desktop .`
Expected: Compiles without errors.

**Step 6: Commit**

```bash
git add internal/backend/app_tmux_api.go internal/backend/app.go
git commit -m "feat: add tmux shim HTTP API server (Phase 1 logging)"
```

---

### Task 2: PATH + Env Injection in PTY Sessions

**Files:**
- Modify: `internal/terminal/session.go:104-117` (env block)
- Modify: `internal/backend/app.go:184-188` (pass MTUI_PORT in env)

**Step 1: Pass MTUI_PORT from CreateSession**

In `internal/backend/app.go` `CreateSession()`, after the mode check (line 188):
```go
// Inject tmux shim API port so the shim can phone home
if a.tmuxAPIPort > 0 {
	env = append(env, fmt.Sprintf("MTUI_PORT=%d", a.tmuxAPIPort))
}
```

**Step 2: Prepend shim directory to PATH**

In `internal/terminal/session.go` `Start()`, after building `fullEnv` (before line 119),
add logic to prepend the executable's directory to PATH:
```go
// Prepend executable directory to PATH so the tmux shim is found first
if exePath, err := os.Executable(); err == nil {
	exeDir := filepath.Dir(exePath)
	for i, e := range fullEnv {
		if strings.HasPrefix(e, "PATH=") {
			fullEnv[i] = "PATH=" + exeDir + string(os.PathListSeparator) + e[5:]
			break
		}
	}
}
```

Note: Add `"path/filepath"` to imports in session.go.

**Step 3: Build and verify**

Run: `go build -o build/bin/multiterminal.exe -tags desktop .`
Expected: Compiles without errors.

**Step 4: Commit**

```bash
git add internal/terminal/session.go internal/backend/app.go
git commit -m "feat: inject MTUI_PORT and shim PATH into PTY sessions"
```

---

### Task 3: tmux.exe Shim Binary

**Files:**
- Create: `cmd/tmux-shim/main.go`

**Step 1: Create the shim**

```go
// cmd/tmux-shim/main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type logEntry struct {
	Args []string `json:"args"`
	Dir  string   `json:"dir"`
	Env  string   `json:"env"`
}

func main() {
	args := os.Args[1:]
	dir, _ := os.Getwd()

	// Collect relevant env vars for logging
	envInfo := fmt.Sprintf("MTUI_PORT=%s TMUX=%s",
		os.Getenv("MTUI_PORT"), os.Getenv("TMUX"))

	// Always log to file
	logToFile(args, dir, envInfo)

	// Send to Multiterminal API if available
	port := os.Getenv("MTUI_PORT")
	if port != "" {
		sendToAPI(port, args, dir, envInfo)
	}

	// Phase 1: return fake success for commands Claude Code might check
	if len(args) > 0 {
		switch args[0] {
		case "has-session", "has":
			// Pretend we have a session
			os.Exit(0)
		case "list-panes":
			// Return empty pane list
			fmt.Println("")
			os.Exit(0)
		case "display-message":
			// Print nothing
			os.Exit(0)
		case "split-window":
			// Return a fake pane ID so Claude Code thinks it worked
			fmt.Println("%1")
			os.Exit(0)
		}
	}

	os.Exit(0)
}

func logToFile(args []string, dir, envInfo string) {
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".multiterminal")
	os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, "tmux-shim.log")

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	ts := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Fprintf(f, "[%s] dir=%q env=%q args=%s\n", ts, dir, envInfo, strings.Join(args, " "))
}

func sendToAPI(port string, args []string, dir, envInfo string) {
	entry := logEntry{Args: args, Dir: dir, Env: envInfo}
	body, err := json.Marshal(entry)
	if err != nil {
		return
	}
	url := fmt.Sprintf("http://127.0.0.1:%s/api/tmux/log", port)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[tmux-shim] API error: %v", err)
		return
	}
	resp.Body.Close()
}
```

**Step 2: Build the shim**

Run: `go build -o build/bin/tmux.exe ./cmd/tmux-shim/`
Expected: Produces `build/bin/tmux.exe` next to `multiterminal.exe`.

**Step 3: Verify shim works standalone**

Run: `MTUI_PORT=9999 build/bin/tmux.exe split-window -h -- claude --resume abc`
Expected: Prints `%1`, exits 0, and writes a line to `~/.multiterminal/tmux-shim.log`.

**Step 4: Commit**

```bash
git add cmd/tmux-shim/main.go
git commit -m "feat: add tmux.exe logging shim for Agent Teams integration"
```

---

### Task 4: Integration Test — End-to-End Logging

**Step 1: Build everything**

```bash
cd frontend && npm run build && cd ..
go build -o build/bin/multiterminal.exe -tags desktop .
go build -o build/bin/tmux.exe ./cmd/tmux-shim/
```

**Step 2: Start Multiterminal**

Launch `build/bin/multiterminal.exe`. Check logs for:
```
[tmux-api] listening on port XXXXX
```

**Step 3: Test from a Multiterminal pane**

Open a shell pane inside Multiterminal and run:
```bash
echo $MTUI_PORT        # should show a port number
which tmux             # should point to build/bin/tmux.exe
tmux split-window -h -- echo hello
```

Expected:
- `tmux` resolves to our shim
- Shim logs to `~/.multiterminal/tmux-shim.log`
- Multiterminal backend logs `[tmux-shim] args=[split-window -h -- echo hello]`

**Step 4: Test with Claude Code Agent Teams**

In the Multiterminal pane, ensure `~/.claude/settings.json` has:
```json
{
  "env": { "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1" },
  "teammateMode": "tmux"
}
```

Start claude and ask it to create a team. Watch `~/.multiterminal/tmux-shim.log` for all tmux commands it sends.

**Step 5: Commit any fixes**

```bash
git add -A
git commit -m "feat: tmux shim Phase 1 complete — logging all tmux commands"
```
