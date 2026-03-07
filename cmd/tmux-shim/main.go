// tmux-shim intercepts tmux CLI calls from Claude Code Agent Teams
// and forwards them to the Multiterminal backend via HTTP.
//
// Phase 1: Logs all commands to file + HTTP API.
// Phase 2: Will translate tmux commands into real pane operations.
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

	envInfo := fmt.Sprintf("MTUI_PORT=%s TMUX=%s",
		os.Getenv("MTUI_PORT"), os.Getenv("TMUX"))

	logToFile(args, dir, envInfo)

	port := os.Getenv("MTUI_PORT")
	if port != "" {
		sendToAPI(port, args, dir, envInfo)
	}

	handleCommand(args)
}

func handleCommand(args []string) {
	if len(args) == 0 {
		os.Exit(0)
	}

	// Handle flags before subcommands
	if args[0] == "-V" {
		fmt.Println("tmux 3.4")
		os.Exit(0)
	}

	// Parse: tmux [-S socket] [-L name] subcommand [args...]
	// Skip option flags to find the actual subcommand
	i := 0
	for i < len(args) {
		if args[i] == "-S" || args[i] == "-L" || args[i] == "-f" {
			i += 2 // skip flag + value
			continue
		}
		if strings.HasPrefix(args[i], "-") && len(args[i]) == 2 {
			i++
			continue
		}
		break
	}
	if i >= len(args) {
		os.Exit(0)
	}

	cmd := args[i]
	// subArgs := args[i+1:]

	// Extract -F format and -t target from remaining args
	format := ""
	for j := i + 1; j < len(args); j++ {
		if args[j] == "-F" && j+1 < len(args) {
			format = args[j+1]
		}
	}

	switch cmd {
	case "has-session", "has":
		os.Exit(0) // session exists

	case "display-message":
		// Claude Code uses this to query session/pane info
		if format != "" {
			result := expandFormat(format)
			fmt.Println(result)
		}
		os.Exit(0)

	case "list-sessions":
		// Return a fake session
		fmt.Println("mtui: 1 windows (created Fri Mar  7 22:00:00 2026)")
		os.Exit(0)

	case "list-windows":
		fmt.Println("0: main* (1 panes) [200x50]")
		os.Exit(0)

	case "list-panes":
		if format != "" {
			result := expandFormat(format)
			fmt.Println(result)
		} else {
			fmt.Println("0: [200x50] [history 0/10000, 0 bytes] %0 (active)")
		}
		os.Exit(0)

	case "split-window":
		fmt.Println("%1")
		os.Exit(0)

	case "new-window":
		fmt.Println("@1")
		os.Exit(0)

	case "send-keys":
		os.Exit(0)

	case "kill-pane", "kill-window":
		os.Exit(0)

	case "select-pane", "select-window":
		os.Exit(0)

	case "new-session":
		os.Exit(0)

	case "resize-pane":
		os.Exit(0)

	case "set-option", "set":
		os.Exit(0)

	case "show-option", "show":
		os.Exit(0)

	case "respawn-pane":
		os.Exit(0)

	case "pipe-pane":
		os.Exit(0)

	case "capture-pane":
		os.Exit(0)

	case "save-buffer":
		os.Exit(0)

	default:
		// Unknown command — succeed silently
		os.Exit(0)
	}
}

// expandFormat handles tmux format strings like "#{session_name}" etc.
func expandFormat(format string) string {
	r := strings.NewReplacer(
		"#{session_name}", "mtui",
		"#{session_id}", "$0",
		"#{window_id}", "@0",
		"#{window_index}", "0",
		"#{window_name}", "main",
		"#{pane_id}", "%0",
		"#{pane_index}", "0",
		"#{pane_pid}", fmt.Sprintf("%d", os.Getpid()),
		"#{pane_current_path}", mustGetwd(),
		"#{pane_width}", "200",
		"#{pane_height}", "50",
		"#{pane_active}", "1",
		"#{session_attached}", "1",
		"#{session_windows}", "1",
		"#{window_panes}", "1",
		"#S", "mtui",
		"#I", "0",
		"#P", "0",
		"#W", "main",
		"#D", "%0",
	)
	return r.Replace(format)
}

func mustGetwd() string {
	d, _ := os.Getwd()
	return d
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
