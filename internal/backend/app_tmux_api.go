package backend

// The tmux shim and the statusline forwarder post to the session host
// (internal/hub), not to this process: a session's environment names that port
// at launch and cannot be told a new one, so serving it from a window would
// break every session that outlives the window.

// GetTmuxAPIPort returns the port MTUI's helper binaries post to, 0 if the
// host serves none.
func (a *AppService) GetTmuxAPIPort() int {
	if a.host == nil {
		return 0
	}
	return a.host.Info().ShimPort
}
