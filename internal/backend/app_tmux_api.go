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
	Args []string `json:"args" yaml:"args"`
	Dir  string   `json:"dir" yaml:"dir"`
	Env  string   `json:"env" yaml:"env"`
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

	if a.app != nil {
		a.app.Event.Emit("tmux:command", entry)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "logged"})
}

// GetTmuxAPIPort returns the tmux shim API port (0 if not started).
func (a *AppService) GetTmuxAPIPort() int {
	return a.tmuxAPIPort
}
