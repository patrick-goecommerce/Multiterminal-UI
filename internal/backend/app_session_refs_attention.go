package backend

import "github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"

// The attention bindings, translated at the window boundary like those in
// app_session_refs.go. The logic is in app_attention.go.

// MarkSessionSeen tells the host the person looked at a pane: a finished or
// failed pane leaves the waiting list.
func (a *AppService) MarkSessionSeen(r hub.Ref) { a.markSessionSeen(a.local(r)) }

// MarkSessionUnseen puts a finished or failed pane back on the waiting list.
func (a *AppService) MarkSessionUnseen(r hub.Ref) { a.markSessionUnseen(a.local(r)) }
