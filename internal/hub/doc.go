// Package hub owns terminal sessions independently of who is looking at them.
//
// Today MTUI's sessions live in AppService: the map, the PTYs and the screen
// buffers belong to the Wails process, and ServiceShutdown closes all of them.
// Closing the window therefore kills every agent, and "restore" re-launches
// panes from SavedPane rather than re-attaching to anything.
//
// This package is the seam that separates session ownership from the GUI. A
// Host owns sessions; a client attaches to them. Two implementations are
// planned:
//
//   - Embedded runs the sessions in the caller's own process. It is what the
//     GUI uses today and what cmd/mtuid uses inside the daemon.
//   - Remote (phase 1c) talks to a running daemon over loopback, so the GUI
//     can come and go while the sessions keep running.
//
// Because both sit behind the same interface, the daemon is a mode rather than
// a fork: the logic exists once, in Embedded, and Remote is pure transport.
//
// Session IDs are local to one Host. The hub identity (see Embedded.HubID) is
// added at the wire boundary, so a client that later talks to several hubs can
// tell two session 3s apart without a second protocol.
//
// Design: docs/superpowers/specs/2026-09-15-mtuid-daemon-architecture-design.md
package hub
