//go:build windows

package backend

import (
	"log"
	"sync"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

// The overlay goes through ITaskbarList3::SetOverlayIcon (wails' w32 wrapper).
// It is called on the UI thread (InvokeSync), which WebView2 has already
// initialised as STA.

var (
	modUser32           = windows.NewLazySystemDLL("user32.dll")
	procCreateIconIndir = modUser32.NewProc("CreateIconIndirect")
	procDestroyIcon     = modUser32.NewProc("DestroyIcon")
	modGdi32            = windows.NewLazySystemDLL("gdi32.dll")
	procCreateBitmap    = modGdi32.NewProc("CreateBitmap")
	procDeleteObject    = modGdi32.NewProc("DeleteObject")
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
	list *w32.ITaskbarList3 // nil when unavailable
	icon uintptr            // the HICON currently shown
	last int
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
			list, err := w32.NewTaskbarList3()
			if err != nil {
				log.Printf("[taskbar] overlay unavailable: %v", err)
				return
			}
			badge.list = list
		})
		hwnd := uintptr(w.NativeWindow())
		if badge.list == nil || hwnd == 0 || (n == badge.last && badge.icon != 0) {
			return
		}
		var icon uintptr
		if n > 0 {
			if icon = badgeIcon(n); icon == 0 {
				log.Printf("[taskbar] could not create the overlay icon for %d", n)
				return
			}
		}
		desc, _ := windows.UTF16PtrFromString(badgeText(n))
		if err := badge.list.SetOverlayIcon(hwnd, icon, desc); err != nil {
			log.Printf("[taskbar] SetOverlayIcon failed: %v", err)
			if icon != 0 {
				procDestroyIcon.Call(icon)
			}
			return
		}
		if badge.icon != 0 {
			procDestroyIcon.Call(badge.icon)
		}
		badge.icon, badge.last = icon, n
	})
}
