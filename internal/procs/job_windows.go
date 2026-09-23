//go:build windows

package procs

import (
	"log"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Job holds every process of one session in a Windows job object.
//
// The job is armed with KILL_ON_JOB_CLOSE, and only this process holds its
// handle. If mtui or mtuid dies without running its shutdown (a crash, "End
// task", a killed daemon), the kernel closes the handle and ends every process
// in the job. Before, nothing did, and every claude tree with its MCP servers
// kept running with no window left to close it.
//
// Children join their parent's job on their own, so membership does not
// depend on parent links the way taskkill /T does: a process whose parent has
// already exited is still in the job.
//
// All methods are safe on a nil *Job, which is what NewJob returns when the
// job cannot be created. The session then behaves as it did before.
type Job struct {
	h windows.Handle
}

// NewJob creates an armed job, or returns nil if that fails.
func NewJob() *Job {
	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		log.Printf("[job] CreateJobObject: %v", err)
		return nil
	}
	j := &Job{h: h}
	if err := j.setKillOnClose(true); err != nil {
		log.Printf("[job] arm: %v", err)
		_ = windows.CloseHandle(h)
		return nil
	}
	return j
}

// Assign adds pid to the job. Children it starts from now on join as well;
// one started in the moment between process start and this call does not.
// taskkill /T still covers that one, so a failure here is logged, not fatal.
func (j *Job) Assign(pid int) {
	if j == nil || pid <= 0 {
		return
	}
	p, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		log.Printf("[job] OpenProcess %d: %v", pid, err)
		return
	}
	defer windows.CloseHandle(p)
	if err := windows.AssignProcessToJobObject(j.h, p); err != nil {
		log.Printf("[job] assign %d: %v", pid, err)
	}
}

// Release ends a session on purpose. Processes still in the job that own no
// visible window are killed: those are the console children taskkill /T
// could not reach because their parent was already gone, typically an MCP
// server or a watcher. Processes with a window, such as an editor or a
// browser started from the pane, are let go, as a closed terminal lets them
// go. Then the job is disarmed and its handle closed.
//
// Call it after the tree kill, not instead of it.
func (j *Job) Release() {
	if j == nil {
		return
	}
	pids := j.pids()
	if len(pids) > 0 {
		windowed := windowedPIDs()
		for _, pid := range pids {
			if windowed[pid] {
				continue
			}
			terminate(pid)
		}
	}
	// Disarm before closing, or the close would take the windowed ones too.
	if err := j.setKillOnClose(false); err != nil {
		log.Printf("[job] disarm: %v", err)
	}
	_ = windows.CloseHandle(j.h)
}

func (j *Job) setKillOnClose(on bool) error {
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	if on {
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	}
	_, err := windows.SetInformationJobObject(j.h, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	return err
}

// maxJobPIDs bounds one process-list query. A session with more live
// processes than this has bigger problems; the rest is left to the kernel
// when mtui exits.
const maxJobPIDs = 1024

// processIDList is JOBOBJECT_BASIC_PROCESS_ID_LIST with a fixed-size array,
// which x/sys/windows does not define.
type processIDList struct {
	assigned uint32
	inList   uint32
	ids      [maxJobPIDs]uintptr
}

func (j *Job) pids() []uint32 {
	var list processIDList
	err := windows.QueryInformationJobObject(j.h, windows.JobObjectBasicProcessIdList,
		uintptr(unsafe.Pointer(&list)), uint32(unsafe.Sizeof(list)), nil)
	if err != nil && list.inList == 0 {
		log.Printf("[job] list processes: %v", err)
		return nil
	}
	n := int(list.inList)
	if n > maxJobPIDs {
		n = maxJobPIDs
	}
	out := make([]uint32, 0, n)
	for _, id := range list.ids[:n] {
		out = append(out, uint32(id))
	}
	return out
}

func terminate(pid uint32) {
	p, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return // already gone
	}
	defer windows.CloseHandle(p)
	_ = windows.TerminateProcess(p, 1)
}

// windowedPIDs returns the processes that own a visible top-level window.
//
// The callback is created once: Windows limits how many callbacks a process
// may create, so one per call would eventually fail. That makes the result
// map shared state, hence the lock around the whole enumeration.
var (
	windowMu       sync.Mutex
	windowResult   map[uint32]bool
	windowCallback = windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if windows.IsWindowVisible(windows.HWND(hwnd)) {
			var pid uint32
			if _, err := windows.GetWindowThreadProcessId(windows.HWND(hwnd), &pid); err == nil && pid != 0 {
				windowResult[pid] = true
			}
		}
		return 1 // keep enumerating
	})
)

func windowedPIDs() map[uint32]bool {
	windowMu.Lock()
	defer windowMu.Unlock()
	windowResult = make(map[uint32]bool)
	if err := windows.EnumWindows(windowCallback, nil); err != nil {
		log.Printf("[job] EnumWindows: %v", err)
	}
	out := windowResult
	windowResult = nil
	return out
}
