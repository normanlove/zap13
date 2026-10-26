package main

import (
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGetSystemPowerStatus = kernel32.NewProc("GetSystemPowerStatus")

	user32               = windows.NewLazySystemDLL("user32.dll")
	procShowWindow       = user32.NewProc("ShowWindow")
	procSetForeground    = user32.NewProc("SetForegroundWindow")
	procFindWindow       = user32.NewProc("FindWindowW")
	procGetDpiForWindow  = user32.NewProc("GetDpiForWindow")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procSetWindowPos     = user32.NewProc("SetWindowPos")
	procGetWindowLongPtr = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtr = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProc   = user32.NewProc("CallWindowProcW")
	procReleaseCapture   = user32.NewProc("ReleaseCapture")
	procPostMessage      = user32.NewProc("PostMessageW")

	dwmapi            = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWinAttr = dwmapi.NewProc("DwmSetWindowAttribute")
)

const (
	wmClose         = 0x0010
	wmNCCalcSize    = 0x0083
	wmNCLButtonDown = 0x00A1
	htCaption       = 2

	swRestore  = 9
	swMinimize = 6
)

var origWndProc uintptr

// Returning 0 from WM_NCCALCSIZE makes the whole window client area, so the
// title bar disappears but we keep the shadow, snap, taskbar minimise etc.
func wndProc(hwnd, msg, wp, lp uintptr) uintptr {
	if msg == wmNCCalcSize && wp != 0 {
		return 0
	}
	r, _, _ := procCallWindowProc.Call(origWndProc, hwnd, msg, wp, lp)
	return r
}

func removeTitleBar(w webview2.WebView, width, height int) {
	hwnd := uintptr(w.Window())

	const (
		gwlStyle   = ^uintptr(15) // -16
		gwlWndProc = ^uintptr(3)  // -4
		wsCaption  = 0x00C00000
		wsSysMenu  = 0x00080000
		wsMinBox   = 0x00020000
		wsMaxBox   = 0x00010000
		wsThick    = 0x00040000
	)
	origWndProc, _, _ = procSetWindowLongPtr.Call(hwnd, gwlWndProc, windows.NewCallback(wndProc))
	style, _, _ := procGetWindowLongPtr.Call(hwnd, gwlStyle)
	style = style&^(wsThick|wsMaxBox) | wsCaption | wsSysMenu | wsMinBox
	procSetWindowLongPtr.Call(hwnd, gwlStyle, style)

	// win11 only, ignored on win10
	corners := uint32(2) // DWMWCP_ROUND
	procDwmSetWinAttr.Call(hwnd, 33, uintptr(unsafe.Pointer(&corners)), 4)
	border := uint32(0x00362B27) // matches --line in the css
	procDwmSetWinAttr.Call(hwnd, 34, uintptr(unsafe.Pointer(&border)), 4)

	dpi := uintptr(96)
	if procGetDpiForWindow.Find() == nil {
		if d, _, _ := procGetDpiForWindow.Call(hwnd); d != 0 {
			dpi = d
		}
	}
	pw := int32(width * int(dpi) / 96)
	ph := int32(height * int(dpi) / 96)
	sw, _, _ := procGetSystemMetrics.Call(0)
	sh, _, _ := procGetSystemMetrics.Call(1)
	x, y := (int32(sw)-pw)/2, max((int32(sh)-ph)/2, 0)

	const swpFrameChanged, swpNoZOrder, swpNoActivate = 0x0020, 0x0004, 0x0010
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(pw), uintptr(ph),
		swpFrameChanged|swpNoZOrder|swpNoActivate)
}

func startDrag(hwnd uintptr) {
	procReleaseCapture.Call()
	procPostMessage.Call(hwnd, wmNCLButtonDown, htCaption, 0)
}

func messageBox(text string, icon uint32) {
	t, _ := windows.UTF16PtrFromString(text)
	c, _ := windows.UTF16PtrFromString("zap13")
	windows.MessageBox(0, t, c, icon)
}

// only one copy should be touching the power limits at a time
func alreadyRunning() bool {
	name, _ := windows.UTF16PtrFromString(`Local\zap13`)
	_, err := windows.CreateMutex(nil, false, name)
	if err != windows.ERROR_ALREADY_EXISTS {
		return false
	}
	cls, _ := windows.UTF16PtrFromString("webview")
	if h, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(cls)), 0); h != 0 {
		procShowWindow.Call(h, swRestore)
		procSetForeground.Call(h)
	}
	return true
}
