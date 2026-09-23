// Package config – idle-suspend settings.
package config

// IdleSuspendSettings controls whether a pane that has been finished and quiet
// for a while releases its process tree until it is used again.
//
// The point is memory: a Claude pane's process tree — claude.exe plus its
// npx/node MCP children — measures around 860 MB, and it holds that whether the
// pane is working or has been idle since this morning. Suspending kills the
// tree and reopens the same conversation with --resume when the pane is touched
// again, so the scrollback and the pane's place in the grid stay put.
//
// Off by default: it kills processes, and that is not something to switch on
// behind a user's back.
type IdleSuspendSettings struct {
	Enabled *bool `yaml:"enabled" json:"enabled"`
	// TimeoutMinutes is how long a pane must sit in a confirmed "done" state
	// before it is suspended. Clamped to MinIdleSuspendMinutes: a value like 1
	// would suspend panes faster than a person switches between them, and the
	// wake-up costs 12-15 seconds.
	TimeoutMinutes int `yaml:"timeout_minutes" json:"timeout_minutes"`
}

// MinIdleSuspendMinutes is the smallest timeout that still leaves the app
// usable. Resuming a pane takes 12-15 s (measured), so a short timeout would
// trade a little memory for a lot of waiting.
const MinIdleSuspendMinutes = 5

// DefaultIdleSuspendMinutes is generous on purpose — a pane that has been quiet
// for half an hour is one the user has moved on from.
const DefaultIdleSuspendMinutes = 30
