// Package discovery publishes and resolves the loopback ports that a running
// MTUI instance binds at runtime.
//
// MTUI used to bind fixed ports (41987 for the focus listener, 51533 for the
// agent-control MCP server). Those ports are machine-wide, not per logon
// session: on a multi-user / RDP box the first instance to start wins them and
// every later instance silently fails to bind. Worse, helper processes of user
// B resolve to the listeners of user A, which handed B's agents unauthenticated
// control over A's MTUI sessions (issue #183).
//
// The fix has three parts:
//
//  1. Every listener binds port 0, so the OS hands out a free ephemeral port.
//  2. The resolved port is published here, into a *per-user* directory. A
//     helper process can therefore only ever resolve the ports of instances
//     running under its own Windows account.
//  3. Each record carries a random token. Ports are recycled by the OS, so
//     "something answers on that port" is never proof that it is our listener;
//     protocols that can carry the token (the focus listener) verify it before
//     acting.
package discovery

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Service names one published listener. The value is used as the file name
// stem, so it must stay filesystem-safe.
type Service string

const (
	// ServiceFocus is the listener that raises the main window when the user
	// clicks a notification (multiterminal: protocol handler).
	ServiceFocus Service = "focus"
	// ServiceMCP is the local agent-control MCP server.
	ServiceMCP Service = "mcp"
)

// EnvDirOverride redirects the runtime directory. Set by tests; also a usable
// escape hatch for portable installs that keep all state next to the binary.
const EnvDirOverride = "MTUI_RUNTIME_DIR"

var (
	// ErrNotPublished means no record exists for the service — either no
	// instance is running, or its listener failed to start.
	ErrNotPublished = errors.New("discovery: no port record published")
	// ErrStale means a record exists but the process that wrote it is gone,
	// so the port it names must not be contacted.
	ErrStale = errors.New("discovery: port record is stale")
)

// Record is the published description of one listener.
type Record struct {
	Service   string    `json:"service" yaml:"service"`
	Port      int       `json:"port" yaml:"port"`
	PID       int       `json:"pid" yaml:"pid"`
	Token     string    `json:"token" yaml:"token"`
	StartedAt time.Time `json:"started_at" yaml:"started_at"`
}

// Addr returns the loopback address to dial for this record.
func (r Record) Addr() string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(r.Port))
}

// Dir returns the per-user directory holding the runtime port records.
//
// os.UserCacheDir is deliberately preferred over os.UserConfigDir: on Windows
// the former resolves to %LOCALAPPDATA%, which stays on this machine, while the
// latter resolves to %APPDATA%, which roams. A port record describes a process
// running on this machine right now; roaming it to another machine would hand
// readers there a port number that means nothing (or, worse, something else).
// Both roots are per-user, which is the property that fixes the cross-user
// hijack — the choice between them is only about roaming.
func Dir() (string, error) {
	if custom := strings.TrimSpace(os.Getenv(EnvDirOverride)); custom != "" {
		return custom, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("discovery: locate user cache dir: %w", err)
	}
	return filepath.Join(base, "mtui"), nil
}

// Path returns the record file path for a service.
func Path(svc Service) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, string(svc)+".port.json"), nil
}

// Publish writes the record for a freshly bound listener and returns it. The
// caller passes the port the OS actually assigned (never 0). Any previous
// record for the service — including a stale one left behind by a crash — is
// replaced.
func Publish(svc Service, port int) (Record, error) {
	if port <= 0 || port > 65535 {
		return Record{}, fmt.Errorf("discovery: refusing to publish invalid port %d for %s", port, svc)
	}
	token, err := newToken()
	if err != nil {
		return Record{}, err
	}
	rec := Record{
		Service:   string(svc),
		Port:      port,
		PID:       os.Getpid(),
		Token:     token,
		StartedAt: time.Now().UTC(),
	}
	path, err := Path(svc)
	if err != nil {
		return Record{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return Record{}, fmt.Errorf("discovery: create %s: %w", filepath.Dir(path), err)
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return Record{}, fmt.Errorf("discovery: encode record: %w", err)
	}
	if err := writeAtomic(path, data); err != nil {
		return Record{}, err
	}
	return rec, nil
}

// Read parses the record for a service without judging whether it is still
// current. Use Resolve unless you explicitly want stale records too.
func Read(svc Service) (Record, error) {
	path, err := Path(svc)
	if err != nil {
		return Record{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Record{}, ErrNotPublished
		}
		return Record{}, fmt.Errorf("discovery: read %s: %w", path, err)
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		// A truncated or garbled file is indistinguishable from no file for
		// every caller — treat it as such instead of propagating JSON noise.
		return Record{}, ErrNotPublished
	}
	if rec.Port <= 0 || rec.Port > 65535 {
		return Record{}, ErrNotPublished
	}
	return rec, nil
}

// Resolve reads the record for a service and rejects it if the process that
// published it is gone.
//
// Stale records are the normal case, not the exception: a hard crash skips
// cleanup, and so does the in-app updater, which calls os.Exit(0) and bypasses
// ServiceShutdown entirely (issue #184). Liveness is therefore checked on every
// read rather than trusted from the file's existence.
//
// The PID check is the cheap first stage and is deliberately not the only
// defence — PIDs get recycled, so an unrelated live process can inherit the
// recorded PID while the port belongs to someone else entirely. A plain TCP
// dial does not close that gap either: it proves that *something* listens, not
// that our listener does. The second stage is therefore the token in the
// record, which the caller presents to the listener (see Record.Token).
func Resolve(svc Service) (Record, error) {
	rec, err := Read(svc)
	if err != nil {
		return Record{}, err
	}
	if !ProcessAlive(rec.PID) {
		return Record{}, fmt.Errorf("%w: pid %d of %s is gone", ErrStale, rec.PID, svc)
	}
	return rec, nil
}

// Remove deletes the record for a service. Called on clean shutdown; a missing
// file is not an error.
func Remove(svc Service) error {
	path, err := Path(svc)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("discovery: remove %s: %w", path, err)
	}
	return nil
}

// Probe reports whether anything accepts a connection on the record's address
// within the timeout. It cannot tell our listener from a stranger's, so it is
// only useful as a fast negative check.
func Probe(rec Record, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", rec.Addr(), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// ProcessAlive reports whether a process with the given PID currently exists.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// The actual check is platform-specific (alive_windows.go / alive_other.go):
	// Windows needs the process handle's signal state, Unix the null signal.
	return processAlive(pid)
}

// newToken returns a random 128-bit hex token identifying one listener
// instance.
func newToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("discovery: generate token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// writeAtomic writes data to a temp file in the target directory and renames it
// into place, so a concurrent reader never observes a half-written record.
func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("discovery: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("discovery: write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("discovery: close %s: %w", tmpName, err)
	}
	// 0600: the record is per-user state and its token is a capability.
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("discovery: chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("discovery: rename to %s: %w", path, err)
	}
	return nil
}
