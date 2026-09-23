package backend

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/procs"
)

// sessionHostDaemon is the config value that hands the sessions to mtuid.
const sessionHostDaemon = "daemon"

// daemonStartTimeout bounds how long the app waits for a daemon it just
// started to publish its port. Starting a binary and binding a loopback port
// is fast; waiting much longer would only delay the fallback.
const daemonStartTimeout = 5 * time.Second

// daemonPollInterval is how often the record is checked while waiting.
const daemonPollInterval = 50 * time.Millisecond

// newSessionHost builds the host this app runs its sessions on.
//
// The daemon is opt-in and never mandatory: if anything about it fails, the
// app falls back to running the sessions in-process, because a window that
// opens with local sessions is better than a window that opens with none. The
// failure reaches the UI through the bind warnings, which ride along on
// CheckHealth.
func (a *AppService) newSessionHost() hub.Host {
	embedded := func() hub.Host {
		return hub.NewEmbedded(hub.Options{
			Version:  Version,
			Scan:     true,
			Shim:     true,
			HooksDir: config.HooksDir(),
			KillTree: killProcessTree,
			// The same launcher the daemon gets, so a session started or woken
			// through the CLI or MCP behaves identically in both modes.
			Launcher: windowLauncher{app: a},
			// The same keep-alive the daemon runs, so the feature does not
			// depend on which host this window happens to be using.
			KeepAlive: a.keepAlivePolicy,
			Sink:      hub.SinkFunc(a.onHostEvent),
		})
	}

	if a.cfg.SessionHost != sessionHostDaemon {
		return embedded()
	}

	remote, err := a.dialDaemon()
	if err != nil {
		a.recordBindWarning("daemon", fmt.Errorf(
			"Session-Daemon nicht erreichbar (%w) — Sessions laufen in diesem Fenster "+
				"und enden mit ihm", err))
		return embedded()
	}
	log.Printf("[host] using session daemon %s (pid %d)", remote.Info().HubID, remote.Info().PID)
	return remote
}

// dialDaemon connects to the running daemon, starting one if there is none.
func (a *AppService) dialDaemon() (*hub.Remote, error) {
	rec, err := ensureDaemon()
	if err != nil {
		return nil, err
	}
	remote, err := hub.Dial(rec.Addr(), rec.Token, hub.DialOptions{
		Sink: hub.SinkFunc(a.onHostEvent),
	})
	if err != nil {
		// A protocol mismatch is not something to paper over by restarting the
		// daemon: that would end every agent it is holding, which is the one
		// thing this whole arrangement exists to prevent. Say so and stay
		// local.
		if errors.Is(err, hub.ErrProtocol) {
			return nil, fmt.Errorf("%w — mtuid neu starten, sobald die laufenden Agents fertig sind", err)
		}
		return nil, err
	}
	return remote, nil
}

// ensureDaemon returns the running daemon's record, starting mtuid if needed.
func ensureDaemon() (discovery.Record, error) {
	if rec, err := discovery.Resolve(discovery.ServiceHub); err == nil {
		return rec, nil
	}

	exe := siblingBinaryPath("mtuid")
	if exe == "" {
		return discovery.Record{}, errors.New("mtuid nicht gefunden")
	}

	cmd := exec.Command(exe)
	// Detached, so closing this window does not take the daemon (and every
	// agent it holds) with it.
	procs.Detach(cmd)
	if err := cmd.Start(); err != nil {
		return discovery.Record{}, fmt.Errorf("mtuid start: %w", err)
	}
	// Let go of the child: nobody here is going to wait on it, and a daemon
	// that outlives this process must not be left as a zombie either.
	_ = cmd.Process.Release()

	deadline := time.Now().Add(daemonStartTimeout)
	for {
		if rec, err := discovery.Resolve(discovery.ServiceHub); err == nil {
			return rec, nil
		}
		if time.Now().After(deadline) {
			return discovery.Record{}, fmt.Errorf(
				"mtuid meldet sich nicht innerhalb von %s", daemonStartTimeout)
		}
		time.Sleep(daemonPollInterval)
	}
}

// UsesSessionDaemon reports whether the sessions outlive this window. The
// frontend asks so it can say so, and so it knows a restore should re-attach
// rather than launch.
func (a *AppService) UsesSessionDaemon() bool {
	_, remote := a.host.(*hub.Remote)
	return remote
}

// LiveSession is a session the host already holds, in the shape the frontend
// needs to put a pane back in front of it.
type LiveSession struct {
	ID       int    `json:"id" yaml:"id"`
	Name     string `json:"name" yaml:"name"`
	Dir      string `json:"dir" yaml:"dir"`
	Mode     string `json:"mode" yaml:"mode"`
	Activity string `json:"activity" yaml:"activity"`
	Running  bool   `json:"running" yaml:"running"`
}

// ListLiveSessions returns the sessions the host is holding right now.
//
// With the daemon this is how a freshly started window finds the agents that
// kept running while it was closed; with the embedded host it is always empty
// at startup, because the sessions died with the last window.
func (a *AppService) ListLiveSessions() []LiveSession {
	summaries := a.sessionSummaries()
	out := make([]LiveSession, 0, len(summaries))
	for _, s := range summaries {
		out = append(out, LiveSession{
			ID:       s.ID,
			Name:     s.Name,
			Dir:      s.Dir,
			Mode:     s.Mode,
			Activity: string(s.Activity),
			Running:  s.Status == hub.StatusRunning,
		})
	}
	return out
}

// AttachSession puts an existing session back on screen: the pane streams from
// wherever the session is, replayed from the host's ring so it comes back with
// its history rather than blank.
//
// It returns false when the host has no such session, which is how a restore
// tells "this pane is still running" from "this pane has to be launched
// again".
func (a *AppService) AttachSession(id int, rows int, cols int) bool {
	summary, err := a.host.Get(id)
	if err != nil {
		return false
	}

	// The mode map lives in this process and is gone after a restart; the host
	// still knows what the session was started as.
	a.mu.Lock()
	if _, known := a.sessionMode[id]; !known && summary.Mode != "" {
		a.sessionMode[id] = summary.Mode
	}
	a.mu.Unlock()

	if rows >= 5 && cols >= 20 {
		_ = a.host.Resize(id, rows, cols)
	}
	a.streamSession(id)
	log.Printf("[host] re-attached session %d (%s, %s)", id, summary.Mode, summary.Dir)
	return true
}
