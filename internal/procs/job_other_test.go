//go:build !windows

package procs

import "testing"

func TestNilJobIsSafe(t *testing.T) {
	j := NewJob()
	j.Assign(123)
	j.Release()
}
