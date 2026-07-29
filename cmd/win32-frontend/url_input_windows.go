//go:build windows

package main

import (
	"log"
	"syscall"
)

const (
	vkA           = 0x41
	vkMenu        = 0x12
	smCXDoubleClk = 36
	smCYDoubleClk = 37
)

type urlInputClickTracker struct {
	doubleClickTime uint32
	doubleClickX    int
	doubleClickY    int
	awaitingThird   bool
	suppressMouseUp bool
}

var (
	procCallWindowProcW    = user32.NewProc("CallWindowProcW")
	procGetDoubleClickTime = user32.NewProc("GetDoubleClickTime")
	procGetMessageTime     = user32.NewProc("GetMessageTime")
	procGetSystemMetrics   = user32.NewProc("GetSystemMetrics")

	urlInputWndProc         = syscall.NewCallback(urlInputWindowProc)
	urlInputOriginalWndProc uintptr
	urlInputHWND            HWND
	urlInputClicks          urlInputClickTracker
)

func installURLInputSelection(hwnd HWND) bool {
	if urlInputOriginalWndProc != 0 {
		if urlInputHWND == hwnd {
			return true
		}
		log.Printf("refusing to subclass a second URL input while hwnd=%d is active", urlInputHWND)
		return false
	}
	urlInputClicks = urlInputClickTracker{}
	original, _, err := procSetWindowLongPtrW.Call(
		uintptr(hwnd),
		^uintptr(3), // GWLP_WNDPROC (-4), represented at the native pointer width.
		urlInputWndProc,
	)
	if original == 0 {
		log.Printf("subclass URL input failed: %v", err)
		return false
	}
	urlInputOriginalWndProc = original
	urlInputHWND = hwnd
	return true
}

func urlInputWindowProc(hwnd HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_KEYDOWN:
		urlInputClicks.cancelPendingTriple()
		if shouldSelectAllShortcut(wParam, isControlKeyPressed(), isAltKeyPressed()) {
			selectAllURLInput(hwnd)
			return 0
		}
	case WM_CHAR:
		urlInputClicks.cancelPendingTriple()
		// TranslateMessage can enqueue Ctrl+A's U+0001 even when WM_KEYDOWN is
		// handled above. Do not let that control character reach the Edit control.
		if wParam == 1 {
			return 0
		}
	case WM_LBUTTONDBLCLK:
		armURLInputThirdClick(lParam)
	case WM_LBUTTONDOWN:
		if consumeURLInputThirdClick(lParam) {
			selectAllURLInput(hwnd)
			urlInputClicks.suppressMouseUp = true
			return 0
		}
	case WM_LBUTTONUP:
		if urlInputClicks.suppressMouseUp {
			urlInputClicks.suppressMouseUp = false
			return 0
		}
	case WM_KILLFOCUS, WM_CANCELMODE:
		urlInputClicks.cancelPointerGesture()
	case WM_RBUTTONDOWN, WM_MBUTTONDOWN, WM_XBUTTONDOWN, WM_MOUSEWHEEL, WM_MOUSEHWHEEL:
		urlInputClicks.cancelPendingTriple()
	case WM_NCDESTROY:
		return restoreAndCallOriginalURLInputProc(hwnd, msg, wParam, lParam)
	}

	return callOriginalURLInputProc(hwnd, msg, wParam, lParam)
}

func callOriginalURLInputProc(hwnd HWND, msg uint32, wParam, lParam uintptr) uintptr {
	if urlInputOriginalWndProc == 0 {
		result, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
		return result
	}
	result, _, _ := procCallWindowProcW.Call(
		urlInputOriginalWndProc,
		uintptr(hwnd),
		uintptr(msg),
		wParam,
		lParam,
	)
	return result
}

func restoreAndCallOriginalURLInputProc(hwnd HWND, msg uint32, wParam, lParam uintptr) uintptr {
	original := urlInputOriginalWndProc
	if original == 0 {
		result, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
		return result
	}
	procSetWindowLongPtrW.Call(uintptr(hwnd), ^uintptr(3), original)
	urlInputOriginalWndProc = 0
	urlInputHWND = 0
	urlInputClicks = urlInputClickTracker{}
	result, _, _ := procCallWindowProcW.Call(original, uintptr(hwnd), uintptr(msg), wParam, lParam)
	return result
}

func shouldSelectAllShortcut(key uintptr, controlPressed, altPressed bool) bool {
	return key == vkA && controlPressed && !altPressed
}

func isAltKeyPressed() bool {
	r, _, _ := procGetKeyState.Call(vkMenu)
	return r&0x8000 != 0
}

func selectAllURLInput(hwnd HWND) {
	procSendMessageW.Call(uintptr(hwnd), EM_SETSEL, 0, ^uintptr(0))
}

func currentURLInputClick(lParam uintptr) (clickTime uint32, x, y, maxX, maxY int, maxElapsed uint32) {
	now, _, _ := procGetMessageTime.Call()
	doubleClickTime, _, _ := procGetDoubleClickTime.Call()
	doubleClickWidth, _, _ := procGetSystemMetrics.Call(smCXDoubleClk)
	doubleClickHeight, _, _ := procGetSystemMetrics.Call(smCYDoubleClk)
	return uint32(now),
		int(int16(lParam & 0xFFFF)),
		int(int16((lParam >> 16) & 0xFFFF)),
		int(doubleClickWidth) / 2,
		int(doubleClickHeight) / 2,
		uint32(doubleClickTime)
}

func armURLInputThirdClick(lParam uintptr) {
	clickTime, x, y, _, _, _ := currentURLInputClick(lParam)
	urlInputClicks.arm(clickTime, x, y)
}

func consumeURLInputThirdClick(lParam uintptr) bool {
	clickTime, x, y, maxX, maxY, maxElapsed := currentURLInputClick(lParam)
	return urlInputClicks.consume(clickTime, x, y, maxElapsed, maxX, maxY)
}

func (tracker *urlInputClickTracker) arm(clickTime uint32, x, y int) {
	tracker.doubleClickTime = clickTime
	tracker.doubleClickX = x
	tracker.doubleClickY = y
	tracker.awaitingThird = true
}

func (tracker *urlInputClickTracker) consume(clickTime uint32, x, y int, maxElapsed uint32, maxX, maxY int) bool {
	if !tracker.awaitingThird {
		return false
	}
	tracker.awaitingThird = false
	closeInTime := clickTime-tracker.doubleClickTime <= maxElapsed
	closeInSpace := absInt(x-tracker.doubleClickX) <= maxX &&
		absInt(y-tracker.doubleClickY) <= maxY
	return closeInTime && closeInSpace
}

func (tracker *urlInputClickTracker) cancelPendingTriple() {
	tracker.awaitingThird = false
}

func (tracker *urlInputClickTracker) cancelPointerGesture() {
	tracker.awaitingThird = false
	tracker.suppressMouseUp = false
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
