//go:build windows

package main

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"comic_downloader_go_playwright_stealth/ui"
)

const taskBoardClassName = "TaskBoardControl"

var taskBoardWndProc = syscall.NewCallback(taskBoardWindowProc)

func registerTaskBoardClass(hInstance HINSTANCE) error {
	className, err := utf16Ptr(taskBoardClassName)
	if err != nil {
		return err
	}
	cursor, _, _ := procLoadCursorW.Call(0, uintptr(IDC_ARROW))
	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc:   taskBoardWndProc,
		HInstance:     hInstance,
		HCursor:       HCURSOR(cursor),
		HbrBackground: darkBgBrush,
		LpszClassName: className,
	}
	atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		return fmt.Errorf("RegisterClassExW task board: %w", err)
	}
	return nil
}

func taskBoardRefresh(hwnd HWND) {
	if app == nil {
		return
	}
	items := app.todo.Items()
	contentHeight := 0
	if len(items) > 0 {
		contentHeight = taskCardTopMargin + len(items)*(taskCardHeight+taskCardGap)
	}
	visibleHeight := taskBoardVisibleHeight(hwnd)
	app.mu.Lock()
	app.taskContentH = contentHeight
	maxScroll := contentHeight - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if app.taskScrollY > maxScroll {
		app.taskScrollY = maxScroll
	}
	scrollY := app.taskScrollY
	app.mu.Unlock()
	procSetScrollRange.Call(uintptr(hwnd), 1, 0, uintptr(maxScroll), 1)
	procSetScrollPos.Call(uintptr(hwnd), 1, uintptr(scrollY), 1)
	procInvalidateRect.Call(uintptr(hwnd), 0, 1)
}

func taskBoardVisibleHeight(hwnd HWND) int {
	var rc RECT
	if r, _, _ := procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc))); r == 0 {
		return 0
	}
	return int(rc.Bottom - rc.Top)
}

func taskBoardWindowProc(hwnd HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		taskBoardRefresh(hwnd)
		return 0
	case WM_VSCROLL:
		if app == nil {
			return 0
		}
		code := int(wParam & 0xFFFF)
		app.mu.Lock()
		scrollY := app.taskScrollY
		maxScroll := app.taskContentH - taskBoardVisibleHeight(hwnd)
		if maxScroll < 0 {
			maxScroll = 0
		}
		page := taskBoardVisibleHeight(hwnd) / 2
		if page < 40 {
			page = 40
		}
		switch code {
		case SB_LINEUP:
			scrollY -= 32
		case SB_LINEDOWN:
			scrollY += 32
		case SB_PAGEUP:
			scrollY -= page
		case SB_PAGEDOWN:
			scrollY += page
		case SB_THUMBPOSITION, SB_THUMBTRACK:
			scrollY = int(int32(lParam))
		case SB_TOP:
			scrollY = 0
		case SB_BOTTOM:
			scrollY = maxScroll
		}
		if scrollY < 0 {
			scrollY = 0
		}
		if scrollY > maxScroll {
			scrollY = maxScroll
		}
		app.taskScrollY = scrollY
		app.mu.Unlock()
		procSetScrollPos.Call(uintptr(hwnd), 1, uintptr(scrollY), 1)
		procInvalidateRect.Call(uintptr(hwnd), 0, 1)
		return 0
	case WM_PAINT:
		taskBoardPaint(hwnd)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func taskBoardPaint(hwnd HWND) {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

	brushBg, _, _ := procCreateSolidBrush.Call(uintptr(rgb(18, 22, 28)))
	defer procDeleteObject.Call(brushBg)
	brushCard, _, _ := procCreateSolidBrush.Call(uintptr(rgb(28, 34, 42)))
	defer procDeleteObject.Call(brushCard)
	brushThumb, _, _ := procCreateSolidBrush.Call(uintptr(rgb(48, 56, 68)))
	defer procDeleteObject.Call(brushThumb)
	brushBorder, _, _ := procCreateSolidBrush.Call(uintptr(rgb(100, 150, 110)))
	defer procDeleteObject.Call(brushBorder)
	brushBarBg, _, _ := procCreateSolidBrush.Call(uintptr(rgb(52, 62, 76)))
	defer procDeleteObject.Call(brushBarBg)
	brushPending, _, _ := procCreateSolidBrush.Call(uintptr(rgb(96, 118, 140)))
	defer procDeleteObject.Call(brushPending)
	brushDone, _, _ := procCreateSolidBrush.Call(uintptr(rgb(84, 138, 92)))
	defer procDeleteObject.Call(brushDone)
	brushFail, _, _ := procCreateSolidBrush.Call(uintptr(rgb(182, 82, 82)))
	defer procDeleteObject.Call(brushFail)

	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&ps.RcPaint)), brushBg)
	procSetBkMode.Call(hdc, 1)

	var rc RECT
	if r, _, _ := procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc))); r == 0 {
		return
	}
	width := int(rc.Right - rc.Left)
	scrollY := app.taskScrollSnapshot()

	items := app.todo.Items()
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})

	for idx, item := range items {
		top := taskCardTopMargin + idx*(taskCardHeight+taskCardGap) - scrollY
		bottom := top + taskCardHeight
		if bottom < 0 || top > int(rc.Bottom) {
			continue
		}
		drawTaskCard(HDC(hdc), width, top, item, brushCard, brushThumb, brushBorder, brushBarBg, brushPending, brushDone, brushFail)
	}
}

func drawTaskCard(hdc HDC, width, top int, item ui.TodoItem, brushCard, brushThumb, brushBorder, brushBarBg, brushPending, brushDone, brushFail uintptr) {
	cardLeft := 10
	cardTop := top
	cardWidth := width - 20
	cardHeight := taskCardHeight
	if cardWidth < 100 {
		cardWidth = 100
	}
	cardRect := RECT{Left: int32(cardLeft), Top: int32(cardTop), Right: int32(cardLeft + cardWidth), Bottom: int32(cardTop + cardHeight)}
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&cardRect)), brushCard)

	borderRect := RECT{Left: int32(cardLeft), Top: int32(cardTop), Right: int32(cardLeft + cardWidth), Bottom: int32(cardTop + 4)}
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&borderRect)), brushBorder)

	thumbRect := RECT{Left: int32(cardLeft + 8), Top: int32(cardTop + 10), Right: int32(cardLeft + 92), Bottom: int32(cardTop + 94)}
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&thumbRect)), brushThumb)

	title := item.Request.URL
	if title == "" {
		title = item.ID
	}
	if u, err := url.Parse(item.Request.URL); err == nil && u.Host != "" {
		title = fmt.Sprintf("%s [%s]", hostTitle(item.Request.URL), item.Status)
	}
	sub := fmt.Sprintf("%s | %s", item.ID, strings.ToLower(string(item.Status)))
	siteLine := item.Request.URL
	if u, err := url.Parse(item.Request.URL); err == nil {
		if u.Host != "" {
			siteLine = u.Host
		}
	}
	if item.Request.Headless {
		title += " [headless]"
	}

	drawTextLineWithFont(hdc, titleFont, cardLeft+104, cardTop+10, title, rgb(244, 247, 250))
	drawTextLineWithFont(hdc, uiFont, cardLeft+104, cardTop+36, siteLine, rgb(184, 196, 208))
	drawTextLineWithFont(hdc, uiFont, cardLeft+104, cardTop+56, sub, rgb(156, 168, 182))

	barY := cardTop + 76
	barRectBg := RECT{Left: int32(cardLeft + 104), Top: int32(barY), Right: int32(cardLeft + cardWidth - 14), Bottom: int32(barY + 22)}
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&barRectBg)), brushBarBg)
	barRectFill := barRectBg
	progress := taskProgress(item)
	barRectFill.Right = barRectBg.Left + int32(float64(barRectBg.Right-barRectBg.Left)*progress)
	if barRectFill.Right < barRectFill.Left+2 {
		barRectFill.Right = barRectFill.Left + 2
	}
	switch item.Status {
	case ui.TodoStatusCompleted:
		procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&barRectFill)), brushDone)
	case ui.TodoStatusFailed:
		procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&barRectFill)), brushFail)
	default:
		procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&barRectFill)), brushPending)
	}
	percent := fmt.Sprintf("%d%%", int(progress*100))
	drawTextLineWithFont(hdc, uiFont, cardLeft+104+(cardWidth-120)/2, cardTop+78, percent, rgb(252, 252, 252))
	statusText := taskProgressLabel(item)
	if statusText != "" {
		drawTextLineWithFont(hdc, uiFont, cardLeft+104, cardTop+96, statusText, rgb(184, 196, 208))
	}
}

func taskProgress(item ui.TodoItem) float64 {
	if item.Progress > 0 {
		return clamp01(item.Progress)
	}
	switch item.Status {
	case ui.TodoStatusCompleted:
		return 1
	case ui.TodoStatusRunning:
		return 0.6
	case ui.TodoStatusFailed:
		return 1
	default:
		return 0.0
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func taskProgressLabel(item ui.TodoItem) string {
	switch {
	case item.Status == ui.TodoStatusFailed:
		return "错"
	case item.Status == ui.TodoStatusCompleted && item.Result.DownloadedCount > 0:
		return fmt.Sprintf("%d张", item.Result.DownloadedCount)
	case item.StepTotal > 0 && item.StepCurrent > 0:
		base := fmt.Sprintf("%d/%d", item.StepCurrent, item.StepTotal)
		suffix := taskPhaseShort(item.Phase)
		if suffix == "" {
			return base
		}
		return base + " " + suffix
	case strings.TrimSpace(item.Phase) != "":
		return taskPhaseShort(item.Phase)
	default:
		return ""
	}
}

func taskPhaseShort(phase string) string {
	switch strings.TrimSpace(strings.ToLower(phase)) {
	case "pending":
		return "待"
	case "running":
		return "跑"
	case "completed":
		return "完"
	case "failed":
		return "错"
	case "启动":
		return "启"
	case "解析":
		return "解"
	case "下载中":
		return "下"
	case "完成":
		return "完"
	case "准备":
		return "备"
	case "激活":
		return "激"
	default:
		return phase
	}
}

func drawTextLine(hdc HDC, x, y int, text string, color uint32) {
	if text == "" {
		return
	}
	textPtr, _ := utf16Ptr(text)
	procSetTextColor.Call(uintptr(hdc), uintptr(color))
	procTextOutW.Call(uintptr(hdc), uintptr(x), uintptr(y), uintptr(unsafe.Pointer(textPtr)), uintptr(len(text)))
}

func drawTextLineWithFont(hdc HDC, font HGDIOBJ, x, y int, text string, color uint32) {
	if text == "" {
		return
	}
	textPtr, _ := utf16Ptr(text)
	oldFont, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(font))
	defer procSelectObject.Call(uintptr(hdc), oldFont)
	procSetTextColor.Call(uintptr(hdc), uintptr(color))
	procTextOutW.Call(uintptr(hdc), uintptr(x), uintptr(y), uintptr(unsafe.Pointer(textPtr)), uintptr(len(text)))
}

func hostTitle(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host
}

func taskBoardSnapshotScrollY() int {
	if app == nil {
		return 0
	}
	app.mu.RLock()
	defer app.mu.RUnlock()
	return app.taskScrollY
}

func (a *frontendApp) taskScrollSnapshot() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.taskScrollY
}

func rgb(r, g, b uint8) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

const (
	taskCardTopMargin = 10
	taskCardHeight    = 108
	taskCardGap       = 10
)
