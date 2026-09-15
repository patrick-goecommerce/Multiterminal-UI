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
	a.mu.Lock()
	defer a.mu.Unlock()

	result := make([]QueueOverviewItem, 0)
	for id, q := range a.queues {
		if q == nil || len(q.items) == 0 {
			continue
		}
		summary, err := a.host.Get(id)
		if err != nil {
			continue
		}

		items := make([]QueueItem, len(q.items))
		copy(items, q.items)

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
