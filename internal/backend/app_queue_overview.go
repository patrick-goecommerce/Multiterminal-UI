// Package backend provides cross-session queue overview functionality.
package backend

// QueueOverviewItem represents a session's queue with context.
type QueueOverviewItem struct {
	SessionID   int         `json:"session_id" yaml:"session_id"`
	SessionName string      `json:"session_name" yaml:"session_name"`
	Dir         string      `json:"dir" yaml:"dir"`
	Activity    string      `json:"activity" yaml:"activity"`
	Items       []QueueItem `json:"items" yaml:"items"`
}

// GetAllQueues returns queue items for all sessions that have queues.
func (a *AppService) GetAllQueues() []QueueOverviewItem {
	result := make([]QueueOverviewItem, 0)
	// Ask the host which sessions exist rather than which have queues: the
	// queues live there now, and iterating them from here would need a second
	// round trip per session anyway.
	for _, summary := range a.sessionSummaries() {
		id := summary.ID
		items := a.host.QueueList(id)
		if len(items) == 0 {
			continue
		}

		oi := QueueOverviewItem{
			SessionID:   id,
			SessionName: summary.Name,
			Dir:         summary.Dir,
			Activity:    string(summary.Activity),
			Items:       items,
		}
		result = append(result, oi)
	}
	return result
}
