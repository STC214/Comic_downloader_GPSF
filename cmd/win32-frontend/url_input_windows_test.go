//go:build windows

package main

import (
	"testing"
	"unicode/utf16"
	"unsafe"
)

var procDestroyWindowForTest = user32.NewProc("DestroyWindow")

func TestSetWindowLongPtrProcNameMatchesArchitecture(t *testing.T) {
	want := "SetWindowLongW"
	if unsafe.Sizeof(uintptr(0)) == 8 {
		want = "SetWindowLongPtrW"
	}
	if got := setWindowLongPtrProcName(); got != want {
		t.Fatalf("setWindowLongPtrProcName() = %q, want %q", got, want)
	}
}

func TestShouldSelectAllShortcut(t *testing.T) {
	tests := []struct {
		name    string
		key     uintptr
		control bool
		alt     bool
		want    bool
	}{
		{name: "control A", key: vkA, control: true, want: true},
		{name: "plain A", key: vkA, want: false},
		{name: "control alt A", key: vkA, control: true, alt: true, want: false},
		{name: "control other key", key: 'B', control: true, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldSelectAllShortcut(test.key, test.control, test.alt); got != test.want {
				t.Fatalf("shouldSelectAllShortcut() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestURLInputClickTrackerAcceptsThirdClickAfterDoubleClick(t *testing.T) {
	var tracker urlInputClickTracker
	if tracker.consume(1000, 20, 10, 500, 4, 4) {
		t.Fatal("unarmed tracker accepted a click")
	}
	tracker.arm(1200, 22, 11)
	if !tracker.consume(1400, 21, 12, 500, 4, 4) {
		t.Fatal("tracker rejected a valid third click")
	}
	if tracker.consume(1500, 21, 12, 500, 4, 4) {
		t.Fatal("tracker accepted another click without a new double-click")
	}
}

func TestURLInputClickTrackerResetsAtBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		secondTime uint32
		secondX    int
		secondY    int
	}{
		{name: "too slow", secondTime: 1501, secondX: 20, secondY: 10},
		{name: "too far horizontally", secondTime: 1200, secondX: 25, secondY: 10},
		{name: "too far vertically", secondTime: 1200, secondX: 20, secondY: 15},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var tracker urlInputClickTracker
			tracker.arm(1000, 20, 10)
			if tracker.consume(test.secondTime, test.secondX, test.secondY, 500, 4, 4) {
				t.Fatal("tracker accepted an out-of-range third click")
			}
			if tracker.awaitingThird {
				t.Fatal("tracker remained armed after an out-of-range click")
			}
		})
	}
}

func TestURLInputClickTrackerHandlesMessageTimeWraparound(t *testing.T) {
	var tracker urlInputClickTracker
	tracker.arm(^uint32(0)-100, 20, 10)
	if !tracker.consume(50, 20, 10, 500, 4, 4) {
		t.Fatal("tracker rejected a valid wrapped third click")
	}
}

func TestURLInputClickTrackerCancelsOnInterveningInput(t *testing.T) {
	tracker := urlInputClickTracker{suppressMouseUp: true}
	tracker.arm(1000, 20, 10)
	tracker.cancelPendingTriple()
	if tracker.consume(1200, 20, 10, 500, 4, 4) {
		t.Fatal("tracker accepted a click after cancellation")
	}
	if !tracker.suppressMouseUp {
		t.Fatal("cancelling a pending third click cleared an active mouse-up suppression")
	}
}

func TestURLInputClickTrackerClearsPointerGesture(t *testing.T) {
	tracker := urlInputClickTracker{awaitingThird: true, suppressMouseUp: true}
	tracker.cancelPointerGesture()
	if tracker.awaitingThird || tracker.suppressMouseUp {
		t.Fatal("pointer gesture state was not fully cleared")
	}
}

func TestURLInputSubclassWithRealEditControl(t *testing.T) {
	parentClass, _ := utf16Ptr("Static")
	empty, _ := utf16Ptr("")
	parent, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(parentClass)),
		uintptr(unsafe.Pointer(empty)),
		WS_OVERLAPPEDWINDOW,
		0, 0, 320, 80,
		0, 0, 0, 0,
	)
	if parent == 0 {
		t.Fatalf("create test parent window: %v", err)
	}
	defer procDestroyWindowForTest.Call(parent)

	const value = "https://example.com/comic/测试"
	editClass, _ := utf16Ptr("Edit")
	text, _ := utf16Ptr(value)
	edit, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(editClass)),
		uintptr(unsafe.Pointer(text)),
		WS_CHILD|ES_AUTOHSCROLL,
		0, 0, 300, 30,
		parent, 0, 0, 0,
	)
	if edit == 0 {
		t.Fatalf("create test Edit control: %v", err)
	}
	editHWND := HWND(edit)
	if !installURLInputSelection(editHWND) {
		t.Fatal("install URL input subclass")
	}
	if urlInputOriginalWndProc == 0 {
		t.Fatal("URL input subclass was not installed")
	}
	originalWndProc := urlInputOriginalWndProc
	if !installURLInputSelection(editHWND) {
		t.Fatal("reinstalling the same URL input should be idempotent")
	}
	if urlInputOriginalWndProc != originalWndProc {
		t.Fatal("idempotent reinstall replaced the original WndProc")
	}

	secondEdit, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(editClass)),
		uintptr(unsafe.Pointer(text)),
		WS_CHILD|ES_AUTOHSCROLL,
		0, 35, 300, 30,
		parent, 0, 0, 0,
	)
	if secondEdit == 0 {
		t.Fatalf("create second test Edit control: %v", err)
	}
	if installURLInputSelection(HWND(secondEdit)) {
		t.Fatal("installing the subclass on a second live control should fail")
	}
	if urlInputOriginalWndProc != originalWndProc || urlInputHWND != editHWND {
		t.Fatal("second-control attempt changed active subclass ownership")
	}
	procDestroyWindowForTest.Call(secondEdit)

	// A translated Ctrl+A control character must not alter the URL.
	procSendMessageW.Call(edit, WM_CHAR, 1, 0)
	if got := getControlText(editHWND); got != value {
		t.Fatalf("text after Ctrl+A WM_CHAR = %q, want %q", got, value)
	}

	// Exercise the real Edit WndProc sequence: click, double-click, third click.
	procSendMessageW.Call(edit, EM_SETSEL, 1, 1)
	clickPoint := uintptr(5 | 5<<16)
	procSendMessageW.Call(edit, WM_LBUTTONDOWN, 0, clickPoint)
	procSendMessageW.Call(edit, WM_LBUTTONUP, 0, clickPoint)
	procSendMessageW.Call(edit, WM_LBUTTONDBLCLK, 0, clickPoint)
	procSendMessageW.Call(edit, WM_LBUTTONUP, 0, clickPoint)
	procSendMessageW.Call(edit, WM_LBUTTONDOWN, 0, clickPoint)
	procSendMessageW.Call(edit, WM_LBUTTONUP, 0, clickPoint)

	selection, _, _ := procSendMessageW.Call(edit, EM_GETSEL, 0, 0)
	start := int(selection & 0xFFFF)
	end := int((selection >> 16) & 0xFFFF)
	wantEnd := len(utf16.Encode([]rune(value)))
	if start != 0 || end != wantEnd {
		t.Fatalf("selection after triple-click = (%d, %d), want (0, %d)", start, end, wantEnd)
	}

	// Any intervening wheel input cancels the pending third click.
	procSendMessageW.Call(edit, WM_LBUTTONDBLCLK, 0, clickPoint)
	procSendMessageW.Call(edit, WM_LBUTTONUP, 0, clickPoint)
	procSendMessageW.Call(edit, WM_MOUSEWHEEL, 0, clickPoint)
	procSendMessageW.Call(edit, EM_SETSEL, 1, 1)
	procSendMessageW.Call(edit, WM_LBUTTONDOWN, 0, clickPoint)
	procSendMessageW.Call(edit, WM_LBUTTONUP, 0, clickPoint)
	selection, _, _ = procSendMessageW.Call(edit, EM_GETSEL, 0, 0)
	start = int(selection & 0xFFFF)
	end = int((selection >> 16) & 0xFFFF)
	if start == 0 && end == wantEnd {
		t.Fatal("wheel input did not cancel the pending third click")
	}

	procDestroyWindowForTest.Call(edit)
	if urlInputOriginalWndProc != 0 || urlInputHWND != 0 {
		t.Fatal("URL input subclass ownership was not released on destroy")
	}

	replacementEdit, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(editClass)),
		uintptr(unsafe.Pointer(text)),
		WS_CHILD|ES_AUTOHSCROLL,
		0, 0, 300, 30,
		parent, 0, 0, 0,
	)
	if replacementEdit == 0 {
		t.Fatalf("create replacement test Edit control: %v", err)
	}
	if !installURLInputSelection(HWND(replacementEdit)) {
		t.Fatal("installing the subclass after prior control destruction should succeed")
	}
	procDestroyWindowForTest.Call(replacementEdit)
	if urlInputOriginalWndProc != 0 || urlInputHWND != 0 {
		t.Fatal("replacement URL input subclass ownership was not released")
	}
}
