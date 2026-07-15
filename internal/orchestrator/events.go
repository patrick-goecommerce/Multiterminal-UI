package orchestrator

// ProgressEmitter receives granular orchestration progress events so the
// frontend can react in real time instead of polling. Implementations forward
// the event to the UI (e.g. Wails Event.Emit). A nil emitter disables emission.
type ProgressEmitter func(event string, payload map[string]string)

// Orchestration progress event names (mirrored in the frontend).
const (
	EventTriageDone  = "orchestration:triage-done"
	EventPlanReady   = "orchestration:plan-ready"
	EventWaveStarted = "orchestration:wave-started"
	EventStepDone    = "orchestration:step-done"
	EventQAResult    = "orchestration:qa-result"
)

// SetProgressEmitter installs the emitter used to publish progress events.
func (o *Orchestrator) SetProgressEmitter(e ProgressEmitter) {
	o.emit = e
}

// emitEvent publishes a progress event if an emitter is installed.
func (o *Orchestrator) emitEvent(event string, payload map[string]string) {
	if o.emit == nil {
		return
	}
	o.emit(event, payload)
}
