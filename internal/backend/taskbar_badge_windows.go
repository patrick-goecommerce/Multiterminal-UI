//go:build windows

package backend

import (
	"sync"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

// The overlay goes through ITaskbarList3::SetOverlayIcon. COM is called on the
// UI thread (InvokeSync), which WebView2 has already initialised as STA.

var (
	modOle32             = windows.NewLazySystemDLL("ole32.dll")
	procCoCreateInstance = modOle32.NewProc("CoCreateInstance")
	modUser32            = windows.NewLazySystemDLL("user32.dll")
	procCreateIconIndir  = modUser32.NewProc("CreateIconIndirect")
	procDestroyIcon      = modUser32.NewProc("DestroyIcon")
	modGdi32             = windows.NewLazySystemDLL("gdi32.dll")
	procCreateBitmap     = modGdi32.NewProc("CreateBitmap")
	procDeleteObject     = modGdi32.NewProc("DeleteObject")

	clsidTaskbarList = windows.GUID{Data1: 0x56FDF344, Data2: 0xFD6D, Data3: 0x11d0, Data4: [8]byte{0x95, 0x8A, 0x00, 0x60, 0x97, 0xC9, 0xA0, 0x90}}
	iidTaskbarList3  = windows.GUID{Data1: 0xea1afb91, Data2: 0x9e28, Data3: 0x4b86, Data4: [8]byte{0x90, 0xe9, 0x9e, 0x9f, 0x8a, 0x5e, 0xef, 0xaf}}
)

const (
	clsctxInprocServer = 0x1
	// vtable slots: IUnknown 0-2, ITaskbarList 3-7, ITaskbarList2 8,
	// ITaskbarList3 9-20.
	slotHrInit         = 3
	slotSetOverlayIcon = 18
)

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

var badge struct {
	once sync.Once
	list uintptr // ITaskbarList3*, 0 when unavailable
	icon uintptr // the HICON currently shown
	last int
}

func comCall(obj uintptr, slot int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(slot)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return r
}

func badgeIcon(n int) uintptr {
	pix := badgePixels(n)
	mask := make([]byte, 16*2) // 1 bpp, 16 rows of 2 bytes, all zero: alpha decides
	color, _, _ := procCreateBitmap.Call(16, 16, 1, 32, uintptr(unsafe.Pointer(&pix[0])))
	maskBm, _, _ := procCreateBitmap.Call(16, 16, 1, 1, uintptr(unsafe.Pointer(&mask[0])))
	defer procDeleteObject.Call(color)
	defer procDeleteObject.Call(maskBm)
	if color == 0 || maskBm == 0 {
		return 0
	}
	info := iconInfo{fIcon: 1, hbmMask: maskBm, hbmColor: color}
	icon, _, _ := procCreateIconIndir.Call(uintptr(unsafe.Pointer(&info)))
	return icon
}

func setTaskbarBadge(w *application.WebviewWindow, n int) {
	application.InvokeSync(func() {
		badge.once.Do(func() {
			var p uintptr
			hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidTaskbarList)), 0, clsctxInprocServer,
				uintptr(unsafe.Pointer(&iidTaskbarList3)), uintptr(unsafe.Pointer(&p)))
			if hr == 0 && p != 0 && comCall(p, slotHrInit) == 0 {
				badge.list = p
			}
		})
		hwnd := uintptr(w.NativeWindow())
		if badge.list == 0 || hwnd == 0 || (n == badge.last && badge.icon != 0) {
			return
		}
		var icon uintptr
		if n > 0 {
			icon = badgeIcon(n)
		}
		desc, _ := windows.UTF16PtrFromString(badgeText(n))
		comCall(badge.list, slotSetOverlayIcon, hwnd, icon, uintptr(unsafe.Pointer(desc)))
		if badge.icon != 0 {
			procDestroyIcon.Call(badge.icon)
		}
		badge.icon, badge.last = icon, n
	})
}
