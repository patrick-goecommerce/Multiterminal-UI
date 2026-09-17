package hub

import (
	"fmt"
	"net/http"
)

// The queue over the wire.
//
// Nothing here decides anything: the daemon owns the queue and advances it on
// its own schedule, and these only ask. That is the point of the split, and it
// is why a window that is closed does not stop a queued task.

// QueueAdd implements Host.
func (r *Remote) QueueAdd(id int, prompt string) (QueueItem, error) {
	var item QueueItem
	err := r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/queue", id),
		map[string]string{"prompt": prompt}, &item)
	return item, err
}

// QueueList implements Host. A failure reads as an empty queue, which is what
// a caller has to render anyway, and the alternative is a nil slice that a
// JSON consumer turns into null.
func (r *Remote) QueueList(id int) []QueueItem {
	var out struct {
		Items []QueueItem `json:"items"`
	}
	if err := r.call(http.MethodGet, fmt.Sprintf("/v1/sessions/%d/queue", id), nil, &out); err != nil {
		return []QueueItem{}
	}
	if out.Items == nil {
		return []QueueItem{}
	}
	return out.Items
}

// QueueRemove implements Host.
func (r *Remote) QueueRemove(id, itemID int, force bool) (bool, error) {
	var out struct {
		Removed bool `json:"removed"`
	}
	path := fmt.Sprintf("/v1/sessions/%d/queue/%d", id, itemID)
	if force {
		path += "?force=1"
	}
	err := r.call(http.MethodDelete, path, nil, &out)
	return out.Removed, err
}

// QueueClear implements Host.
func (r *Remote) QueueClear(id int, doneOnly bool) error {
	path := fmt.Sprintf("/v1/sessions/%d/queue", id)
	if doneOnly {
		path += "?done=1"
	}
	return r.call(http.MethodDelete, path, nil, nil)
}

// QueueAdvance implements Host. Errors are dropped: the caller is nudging a
// queue the daemon already advances by itself, so a failed nudge changes
// nothing that the next transition will not fix.
func (r *Remote) QueueAdvance(id int) {
	_ = r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/queue/advance", id), nil, nil)
}
