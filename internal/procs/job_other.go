//go:build !windows

package procs

// Job is a Windows job object; elsewhere there is nothing to hold, and every
// method is a no-op. The tree kill in KillProcessTree is all that runs.
type Job struct{}

// NewJob returns nil outside Windows.
func NewJob() *Job { return nil }

// Assign is a no-op outside Windows.
func (j *Job) Assign(pid int) {}

// Release is a no-op outside Windows.
func (j *Job) Release() {}
