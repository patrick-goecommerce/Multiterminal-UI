//go:build windows

package discovery

import "golang.org/x/sys/windows"

// processAlive reports whether pid still identifies a running process.
//
// os.FindProcess is unusable for this on Windows: it calls OpenProcess, which
// keeps succeeding after the process has exited for as long as the kernel holds
// the process object alive — any open handle anywhere does that. A record left
// behind by a crashed instance would therefore look valid.
//
// Waiting on the handle with a zero timeout answers it exactly: a process
// handle becomes signalled the moment the process terminates, so WAIT_TIMEOUT
// means "still running" and WAIT_OBJECT_0 means "already gone". This is also
// why GetExitCodeProcess is not used — a process exiting with code 259
// (STILL_ACTIVE) would be indistinguishable from a running one.
//
// A process we may not synchronise on (another account) reports false, which is
// the safe direction here: the caller then discards the record instead of
// trusting a port it cannot verify.
func processAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)

	ev, err := windows.WaitForSingleObject(h, 0)
	if err != nil {
		return false
	}
	return ev == uint32(windows.WAIT_TIMEOUT)
}
