package backend

// resendActivity emits the session's current state as terminal:activity:
// "sleeping" for a pane that is asleep, otherwise the host's confirmed state
// with when it began. Nothing is sent before a state was ever confirmed.
func (a *AppService) resendActivity(id int) {
	if id == 0 || a.host == nil || a.app == nil {
		return
	}
	summary, err := a.host.Get(id)
	if err != nil {
		return
	}
	state, since := a.host.ConfirmedActivity(id)
	activity := string(state)
	if summary.Asleep() {
		activity = "sleeping"
	}
	if activity == "" {
		return
	}
	a.app.Event.Emit("terminal:activity", ActivityInfo{
		ID:            a.ref(id),
		Activity:      activity,
		ActivitySince: unixOrZero(since),
	})
}
