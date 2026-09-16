// Command mtuid is MTUI's session daemon: it owns the PTYs so that closing a
// window does not end the agents running in them.
//
// It binds a loopback port, publishes it through internal/discovery, and
// serves the hub protocol. Clients (the Wails GUI today, a CLI later) attach
// and detach; the sessions stay.
//
// Design: docs/superpowers/specs/2026-09-15-mtuid-daemon-architecture-design.md
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/procs"
)

// Version is set at build time by the release workflow.
var Version = "dev"

// lockName is the single-instance guard's name in the runtime directory.
const lockName = "hub"

// exit codes: 0 also covers "another daemon already owns this user's hub",
// because from the caller's point of view a daemon is now running either way.
const (
	exitOK    = 0
	exitError = 1
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("mtuid", flag.ContinueOnError)
	idle := fs.Duration("idle", 10*time.Minute,
		"exit after this long with no sessions and no clients; 0 disables")
	ring := fs.Int("ring", hub.DefaultRingBytes,
		"per-session replay buffer in bytes")
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return exitError
	}
	if *showVersion {
		fmt.Println(Version)
		return exitOK
	}

	closeLog := setupLogging()
	defer closeLog()

	// One daemon per user. The lock decides the race in the kernel; checking
	// the discovery record first would leave a window in which two clients
	// both see no daemon and both start one.
	lock, err := discovery.AcquireLock(lockName)
	if errors.Is(err, discovery.ErrLocked) {
		log.Println("[mtuid] another daemon already owns this user's hub, exiting")
		return exitOK
	}
	if err != nil {
		log.Printf("[mtuid] lock: %v", err)
		return exitError
	}
	defer func() { _ = lock.Release() }()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Printf("[mtuid] listen: %v", err)
		return exitError
	}
	port := ln.Addr().(*net.TCPAddr).Port

	rec, err := discovery.Publish(discovery.ServiceHub, port)
	if err != nil {
		log.Printf("[mtuid] publish: %v", err)
		_ = ln.Close()
		return exitError
	}
	defer func() { _ = discovery.Remove(discovery.ServiceHub) }()

	hubID, err := loadHubID()
	if err != nil {
		log.Printf("[mtuid] hub id: %v (continuing with a fresh one)", err)
	}

	// The host reports into the server's sink, and the server needs the host:
	// the indirection through the variable is what breaks that circle.
	var srv *hub.Server
	host := hub.NewEmbedded(hub.Options{
		HubID:     hubID,
		Version:   Version,
		RingBytes: *ring,
		// The daemon scans on its own: nobody else can. Its whole reason for
		// existing is the stretch of time when no window is open, and an
		// agent's state has to keep being written down through it.
		Scan: true,
		// The hook reader moves with the sessions for the same reason: the
		// agent keeps reporting through it while no window is open.
		HooksDir: config.HooksDir(),
		KillTree: procs.KillProcessTree,
		Sink: hub.SinkFunc(func(name string, payload any) {
			srv.Sink().Emit(name, payload)
		}),
	})
	srv = hub.NewServer(host, rec.Token)

	if hubID == "" {
		if err := saveHubID(host.HubID()); err != nil {
			log.Printf("[mtuid] persisting hub id: %v", err)
		}
	}

	log.Printf("[mtuid] version=%s hub=%s pid=%d port=%d ring=%d idle=%s",
		Version, host.HubID(), os.Getpid(), port, *ring, *idle)

	stop := newStopper()
	srv.SetShutdown(func() {
		log.Println("[mtuid] shutdown requested over the API")
		stop.trigger()
	})

	httpSrv := &http.Server{
		Handler: srv.Handler(),
		// No ReadTimeout or WriteTimeout: the stream socket is long-lived by
		// design, and a timeout here would cut a pane's output every time it
		// went quiet.
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[mtuid] serve: %v", err)
			stop.trigger()
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	idleCtx, cancelIdle := context.WithCancel(context.Background())
	defer cancelIdle()
	go watchIdle(idleCtx, host, srv, *idle, stop.trigger)

	select {
	case sig := <-signals:
		log.Printf("[mtuid] %s received, shutting down", sig)
	case <-stop.done:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)

	// This daemon owns the sessions, so stopping it ends them. That is the one
	// path where the agents do die, and it only runs on an explicit request,
	// a signal, or the idle timeout, which requires there to be no sessions.
	host.Release()
	log.Println("[mtuid] stopped")
	return exitOK
}

// stopper turns several possible shutdown triggers into one channel that is
// closed exactly once.
type stopper struct {
	once sync.Once
	done chan struct{}
}

func newStopper() *stopper {
	return &stopper{done: make(chan struct{})}
}

func (s *stopper) trigger() {
	s.once.Do(func() { close(s.done) })
}
