//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	projectruntime "comic_downloader_go_playwright_stealth/runtime"
	"comic_downloader_go_playwright_stealth/tasks"
	"comic_downloader_go_playwright_stealth/ui"
)

const (
	windowClassName = "ComicDownloaderWin32Frontend"
	windowTitle     = "\u6f2b\u753b\u4e0b\u8f7d\u5668"

	menuIDSetExecutable   = 1001
	menuIDSetChromium     = 1002
	menuIDSetMother       = 1003
	menuIDRefreshProfile  = 1004
	menuIDStartAll        = 1005
	menuIDSetDownloadDir  = 1006
	menuIDSetConcurrency  = 1007
	menuIDRefreshAdblock  = 1008
	menuIDClearCompleted  = 1009
	menuIDInstallBrowsers = 1010

	controlIDURLEdit        = 2001
	controlIDAddTask        = 2002
	controlIDTaskTitle      = 2003
	controlIDListBox        = 2004
	controlIDInfoTitle      = 2005
	controlIDInfoEdit       = 2006
	controlIDStatusText     = 2007
	controlIDStartTasks     = 2008
	controlIDRefreshBtn     = 2009
	controlIDDownloadDir    = 2010
	controlIDConcurrency    = 2011
	controlIDAdblockRules   = 2012
	controlIDClearDone      = 2013
	controlIDActionProgress = 2014

	msgRefreshTasks  = 0x8001
	msgRefreshInfo   = 0x8002
	msgRefreshStatus = 0x8003
	msgRefreshBoard  = 0x8004
	msgRefreshAction = 0x8005
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	uxtheme  = syscall.NewLazyDLL("uxtheme.dll")

	procAppendMenuW            = user32.NewProc("AppendMenuW")
	procCreateMenu             = user32.NewProc("CreateMenu")
	procCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	procCreateWindowExW        = user32.NewProc("CreateWindowExW")
	procBeginPaint             = user32.NewProc("BeginPaint")
	procDefWindowProcW         = user32.NewProc("DefWindowProcW")
	procDispatchMessageW       = user32.NewProc("DispatchMessageW")
	procEndPaint               = user32.NewProc("EndPaint")
	procGetClientRect          = user32.NewProc("GetClientRect")
	procGetMessageW            = user32.NewProc("GetMessageW")
	procGetModuleHandleW       = kernel32.NewProc("GetModuleHandleW")
	procGetWindowTextLen       = user32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW         = user32.NewProc("GetWindowTextW")
	procGetOpenFileNameW       = comdlg32.NewProc("GetOpenFileNameW")
	procLoadCursorW            = user32.NewProc("LoadCursorW")
	procMessageBoxW            = user32.NewProc("MessageBoxW")
	procMoveWindow             = user32.NewProc("MoveWindow")
	procPostMessageW           = user32.NewProc("PostMessageW")
	procPostQuitMessage        = user32.NewProc("PostQuitMessage")
	procRegisterClassExW       = user32.NewProc("RegisterClassExW")
	procSendMessageW           = user32.NewProc("SendMessageW")
	procSetMenu                = user32.NewProc("SetMenu")
	procSetWindowTextW         = user32.NewProc("SetWindowTextW")
	procShowWindow             = user32.NewProc("ShowWindow")
	procTranslateMessage       = user32.NewProc("TranslateMessage")
	procUpdateWindow           = user32.NewProc("UpdateWindow")
	procInvalidateRect         = user32.NewProc("InvalidateRect")
	procSetScrollRange         = user32.NewProc("SetScrollRange")
	procSetScrollPos           = user32.NewProc("SetScrollPos")
	procGetScrollPos           = user32.NewProc("GetScrollPos")
	procSHBrowseForFolderW     = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDListW   = shell32.NewProc("SHGetPathFromIDListW")
	procCoTaskMemFree          = ole32.NewProc("CoTaskMemFree")
	gdi32                      = syscall.NewLazyDLL("gdi32.dll")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procFillRect               = user32.NewProc("FillRect")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procSetBkColor             = gdi32.NewProc("SetBkColor")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procTextOutW               = gdi32.NewProc("TextOutW")
	procCreateFontW            = gdi32.NewProc("CreateFontW")
	procGetStockObject         = gdi32.NewProc("GetStockObject")
	procDwmSetWindowAttribute  = dwmapi.NewProc("DwmSetWindowAttribute")
	procSetPreferredAppMode    = uxtheme.NewProc("SetPreferredAppMode")
	procFlushMenuThemes        = uxtheme.NewProc("FlushMenuThemes")
	procAllowDarkModeForWindow = uxtheme.NewProc("AllowDarkModeForWindow")
)

var (
	themeOnce      sync.Once
	themeInitErr   error
	darkBgBrush    HBRUSH
	darkPanelBrush HBRUSH
	darkEditBrush  HBRUSH
	uiFont         HGDIOBJ
	rowFont        HGDIOBJ
	titleFont      HGDIOBJ
)

const (
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	WS_BORDER           = 0x00800000
	WS_TABSTOP          = 0x00010000
	WS_VSCROLL          = 0x00200000
	WS_EX_CLIENTEDGE    = 0x00000200

	BS_PUSHBUTTON        = 0x00000000
	BS_AUTOCHECKBOX      = 0x00000003
	BS_FLAT              = 0x00008000
	LBS_NOTIFY           = 0x0001
	LBS_NOINTEGRALHEIGHT = 0x0100
	ES_AUTOHSCROLL       = 0x0080
	ES_READONLY          = 0x0800
	ES_MULTILINE         = 0x0004
	ES_AUTOVSCROLL       = 0x0040
	ES_WANTRETURN        = 0x1000

	WM_CREATE          = 0x0001
	WM_DESTROY         = 0x0002
	WM_SIZE            = 0x0005
	WM_GETMINMAXINFO   = 0x0024
	WM_COMMAND         = 0x0111
	WM_PAINT           = 0x000F
	WM_VSCROLL         = 0x0115
	WM_ERASEBKGND      = 0x0014
	WM_SETFONT         = 0x0030
	WM_CTLCOLORMSGBOX  = 0x0132
	WM_CTLCOLOREDIT    = 0x0133
	WM_CTLCOLORLISTBOX = 0x0134
	WM_CTLCOLORBTN     = 0x0135
	WM_CTLCOLORDLG     = 0x0136
	WM_CTLCOLORSTATIC  = 0x0138
	EM_SETRECTNP       = 0x00B4

	SW_SHOW = 5

	CW_USEDEFAULT = 0x80000000

	COLOR_WINDOW = 5
	IDC_ARROW    = 32512

	MF_STRING = 0x00000000
	MF_POPUP  = 0x00000010

	LB_RESETCONTENT = 0x0184
	LB_ADDSTRING    = 0x0180

	SB_LINEUP        = 0
	SB_LINEDOWN      = 1
	SB_PAGEUP        = 2
	SB_PAGEDOWN      = 3
	SB_THUMBPOSITION = 4
	SB_THUMBTRACK    = 5
	SB_TOP           = 6
	SB_BOTTOM        = 7
	SB_ENDSCROLL     = 8

	MB_OK = 0x00000000

	DWMWA_USE_IMMERSIVE_DARK_MODE = 20
	DWMWA_CAPTION_COLOR           = 35
	DWMWA_TEXT_COLOR              = 36

	APPMODE_DEFAULT    = 0
	APPMODE_ALLOWDARK  = 1
	APPMODE_FORCEDARK  = 2
	APPMODE_FORCELIGHT = 3
	APPMODE_MAX        = 4

	OFN_EXPLORER      = 0x00080000
	OFN_FILEMUSTEXIST = 0x00001000
	OFN_PATHMUSTEXIST = 0x00000800

	BIF_RETURNONLYFSDIRS = 0x0001
	BIF_USENEWUI         = 0x0040
)

type HWND uintptr
type HINSTANCE uintptr
type HCURSOR uintptr
type HBRUSH uintptr
type HDC uintptr
type HGDIOBJ uintptr

type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type POINT struct {
	X int32
	Y int32
}

type MINMAXINFO struct {
	PtReserved     POINT
	PtMaxSize      POINT
	PtMaxPosition  POINT
	PtMinTrackSize POINT
	PtMaxTrackSize POINT
}

type MSG struct {
	Hwnd    HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     HINSTANCE
	HIcon         uintptr
	HCursor       HCURSOR
	HbrBackground HBRUSH
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type PAINTSTRUCT struct {
	Hdc         HDC
	Erase       int32
	RcPaint     RECT
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}

type OPENFILENAMEW struct {
	LStructSize       uint32
	HwndOwner         HWND
	HInstance         HINSTANCE
	LpstrFilter       *uint16
	LpstrCustomFilter *uint16
	NMaxCustFilter    uint32
	NFilterIndex      uint32
	LpstrFile         *uint16
	NMaxFile          uint32
	LpstrFileTitle    *uint16
	NMaxFileTitle     uint32
	LpstrInitialDir   *uint16
	LpstrTitle        *uint16
	Flags             uint32
	NFileOffset       uint16
	NFileExtension    uint16
	LpstrDefExt       *uint16
	LCustData         uintptr
	LpfnHook          uintptr
	LpTemplateName    *uint16
	PvReserved        uintptr
	DwReserved        uint32
	FlagsEx           uint32
}

type BROWSEINFOW struct {
	HwndOwner      HWND
	PidlRoot       uintptr
	PszDisplayName *uint16
	LpszTitle      *uint16
	UlFlags        uint32
	Lpfn           uintptr
	LParam         uintptr
	IImage         int32
}

type frontendApp struct {
	workspaceRoot string
	paths         projectruntime.Paths
	menuState     ui.BrowserMenuState
	profileMW     ui.BrowserProfileMiddleware
	installMW     ui.BrowserInstallMiddleware
	todo          *ui.TodoList
	downloadDir   string
	concurrency   int
	adblockInfo   string

	hwnd HWND

	urlEdit        HWND
	downloadDirBtn HWND
	concurrencyBtn HWND
	adblockBtn     HWND
	clearDoneBtn   HWND
	addTaskBtn     HWND
	startTasksBtn  HWND
	refreshBtn     HWND
	actionProgress HWND
	taskTitle      HWND
	taskBoard      HWND
	infoTitle      HWND
	infoEdit       HWND
	statusText     HWND

	taskScrollY  int
	taskContentH int

	actionProgressValue float64
	actionProgressText  string

	mu     sync.RWMutex
	status string
}

var app *frontendApp

var wndProc = syscall.NewCallback(windowProc)

func main() {
	stdruntime.LockOSThread()
	workspaceRoot, err := executableWorkspaceRoot()
	if err != nil {
		log.Fatalf("resolve workspace root: %v", err)
	}
	app = newFrontendApp(workspaceRoot)
	app.todo.SetNotifier(func() {
		app.post(msgRefreshTasks)
	})
	if err := app.run(); err != nil {
		log.Fatal(err)
	}
}

func newFrontendApp(workspaceRoot string) *frontendApp {
	paths := projectruntime.NewPaths(workspaceRoot)
	_ = paths.Ensure()
	menu := ui.DefaultBrowserMenuState().
		WithFirefoxExecutablePath(projectruntime.DefaultFirefoxExecutablePath(paths.Root)).
		WithChromiumInstallRoot(projectruntime.DefaultChromiumInstallDir(paths.Root)).
		WithFirefoxInstallRoot(projectruntime.DefaultFirefoxInstallDir(paths.Root)).
		WithFirefoxMotherProfileDir(projectruntime.DefaultFirefoxProfileSourceDir()).
		WithFirefoxWorkingProfileDir(paths.BrowserBaseline)
	return &frontendApp{
		workspaceRoot: workspaceRoot,
		paths:         paths,
		menuState:     menu,
		profileMW:     ui.NewBrowserProfileMiddleware(workspaceRoot),
		installMW:     ui.NewBrowserInstallMiddleware(workspaceRoot),
		todo:          ui.NewTodoList(),
		downloadDir:   paths.OutputRoot,
		concurrency:   1,
		adblockInfo:   "rules not loaded",
		status:        "ready",
	}
}

func (a *frontendApp) run() error {
	if err := ensureThemeResources(); err != nil {
		return err
	}
	enableDarkAppMode()
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	if err := registerTaskBoardClass(HINSTANCE(hInstance)); err != nil {
		return err
	}
	if err := registerActionProgressClass(HINSTANCE(hInstance)); err != nil {
		return err
	}
	className, err := utf16Ptr(windowClassName)
	if err != nil {
		return err
	}
	title, err := utf16Ptr(windowTitle)
	if err != nil {
		return err
	}
	cursor, _, _ := procLoadCursorW.Call(0, uintptr(IDC_ARROW))
	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc:   wndProc,
		HInstance:     HINSTANCE(hInstance),
		HCursor:       HCURSOR(cursor),
		HbrBackground: darkBgBrush,
		LpszClassName: className,
	}
	atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		return fmt.Errorf("RegisterClassExW: %w", err)
	}
	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		WS_OVERLAPPEDWINDOW|WS_VISIBLE,
		CW_USEDEFAULT,
		CW_USEDEFAULT,
		1300,
		900,
		0,
		0,
		hInstance,
		0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW: %w", err)
	}
	a.hwnd = HWND(hwnd)
	applyDarkWindowChrome(a.hwnd)
	procShowWindow.Call(hwnd, SW_SHOW)
	procUpdateWindow.Call(hwnd)

	var msg MSG
	for {
		r, _, err := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		switch int32(r) {
		case -1:
			return fmt.Errorf("GetMessageW: %w", err)
		case 0:
			return nil
		default:
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}
}

func windowProc(hwnd HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		app.hwnd = hwnd
		app.attachMenu(hwnd)
		app.createControls(hwnd)
		app.refreshAllUI()
		return 0
	case WM_ERASEBKGND:
		if darkBgBrush != 0 {
			var rc RECT
			if r, _, _ := procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc))); r != 0 {
				procFillRect.Call(wParam, uintptr(unsafe.Pointer(&rc)), uintptr(darkBgBrush))
			}
		}
		return 1
	case WM_CTLCOLORSTATIC, WM_CTLCOLORBTN:
		procSetTextColor.Call(wParam, uintptr(rgb(238, 242, 247)))
		procSetBkMode.Call(wParam, 1)
		procSetBkColor.Call(wParam, uintptr(rgb(18, 22, 28)))
		if msg == WM_CTLCOLORBTN {
			return uintptr(darkPanelBrush)
		}
		return uintptr(darkBgBrush)
	case WM_CTLCOLOREDIT:
		procSetTextColor.Call(wParam, uintptr(rgb(245, 247, 250)))
		procSetBkMode.Call(wParam, 2)
		procSetBkColor.Call(wParam, uintptr(rgb(36, 42, 52)))
		return uintptr(darkEditBrush)
	case WM_SIZE:
		app.layout()
		return 0
	case WM_GETMINMAXINFO:
		if lParam != 0 {
			mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
			mmi.PtMinTrackSize.X = 420
			mmi.PtMinTrackSize.Y = 620
		}
		return 0
	case WM_COMMAND:
		app.handleCommand(uint16(wParam & 0xFFFF))
		return 0
	case msgRefreshTasks:
		app.refreshTaskList()
		return 0
	case msgRefreshInfo:
		app.refreshInfo()
		return 0
	case msgRefreshStatus:
		app.refreshStatus()
		return 0
	case msgRefreshAction:
		app.refreshActionProgress()
		return 0
	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func (a *frontendApp) attachMenu(hwnd HWND) {
	mainMenu, _, _ := procCreateMenu.Call()
	browserMenu, _, _ := procCreatePopupMenu.Call()
	settingsMenu, _, _ := procCreatePopupMenu.Call()
	taskMenu, _, _ := procCreatePopupMenu.Call()

	addMenuItem(browserMenu, menuIDSetExecutable, "\u8bbe\u7f6e Firefox \u53ef\u6267\u884c\u6587\u4ef6...")
	addMenuItem(browserMenu, menuIDSetChromium, "\u8bbe\u7f6e Chromium \u53ef\u6267\u884c\u6587\u4ef6...")
	addMenuItem(browserMenu, menuIDInstallBrowsers, "\u5b89\u88c5\u6d4f\u89c8\u5668...")
	addMenuItem(browserMenu, menuIDSetMother, "\u8bbe\u7f6e Firefox \u6bcd\u914d\u7f6e\u76ee\u5f55...")
	addMenuItem(browserMenu, menuIDRefreshProfile, "\u5173\u95ed\u6d4f\u89c8\u5668\u5e76\u5237\u65b0\u914d\u7f6e")
	addMenuItem(settingsMenu, menuIDSetDownloadDir, "\u8bbe\u7f6e\u4e0b\u8f7d\u76ee\u5f55...")
	addMenuItem(settingsMenu, menuIDSetConcurrency, "\u8bbe\u7f6e\u5e76\u53d1\u6570...")
	addMenuItem(settingsMenu, menuIDRefreshAdblock, "\u66f4\u65b0\u5e7f\u544a\u62e6\u622a\u89c4\u5219")
	addMenuItem(taskMenu, menuIDStartAll, "\u5f00\u59cb\u6240\u6709\u672a\u5b8c\u6210\u4efb\u52a1")
	addMenuItem(taskMenu, menuIDClearCompleted, "\u6e05\u7406\u5df2\u5b8c\u6210\u4efb\u52a1")

	browserLabel, _ := utf16Ptr("\u6d4f\u89c8\u5668")
	settingsLabel, _ := utf16Ptr("\u8bbe\u7f6e")
	taskLabel, _ := utf16Ptr("\u4efb\u52a1")
	procAppendMenuW.Call(mainMenu, MF_POPUP, browserMenu, uintptr(unsafe.Pointer(browserLabel)))
	procAppendMenuW.Call(mainMenu, MF_POPUP, settingsMenu, uintptr(unsafe.Pointer(settingsLabel)))
	procAppendMenuW.Call(mainMenu, MF_POPUP, taskMenu, uintptr(unsafe.Pointer(taskLabel)))
	procSetMenu.Call(uintptr(hwnd), mainMenu)
}

func addMenuItem(menu uintptr, id uintptr, title string) {
	text, _ := utf16Ptr(title)
	procAppendMenuW.Call(menu, MF_STRING, id, uintptr(unsafe.Pointer(text)))
}

func (a *frontendApp) createControls(hwnd HWND) {
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	def := []struct {
		class   string
		text    string
		style   uintptr
		exStyle uintptr
		x, y    int32
		w, h    int32
		id      int
	}{
		{class: "ActionProgressControl", text: "", style: WS_CHILD | WS_VISIBLE, x: 20, y: 8, w: 1240, h: 22, id: controlIDActionProgress},
		{class: "Edit", text: "请输入漫画 URL", style: WS_CHILD | WS_VISIBLE | WS_BORDER | ES_AUTOHSCROLL | ES_MULTILINE | ES_AUTOVSCROLL | ES_WANTRETURN, exStyle: WS_EX_CLIENTEDGE, x: 20, y: 44, w: 980, h: 38, id: controlIDURLEdit},
		{class: "Button", text: "添加任务", style: WS_CHILD | WS_VISIBLE | BS_PUSHBUTTON | WS_TABSTOP, x: 900, y: 44, w: 120, h: 38, id: controlIDAddTask},
		{class: "Static", text: "任务列表", style: WS_CHILD | WS_VISIBLE, x: 20, y: 106, w: 120, h: 20, id: controlIDTaskTitle},
		{class: "TaskBoardControl", text: "", style: WS_CHILD | WS_VISIBLE | WS_BORDER | WS_VSCROLL, exStyle: WS_EX_CLIENTEDGE, x: 20, y: 124, w: 1240, h: 272, id: controlIDListBox},
		{class: "Static", text: "浏览器与配置状态", style: WS_CHILD | WS_VISIBLE, x: 20, y: 408, w: 180, h: 20, id: controlIDInfoTitle},
		{class: "Edit", text: "", style: WS_CHILD | WS_VISIBLE | WS_BORDER | ES_MULTILINE | ES_AUTOVSCROLL | ES_READONLY | WS_VSCROLL, exStyle: WS_EX_CLIENTEDGE, x: 20, y: 436, w: 1240, h: 344, id: controlIDInfoEdit},
		{class: "Static", text: "ready", style: WS_CHILD | WS_VISIBLE, x: 20, y: 862, w: 1240, h: 24, id: controlIDStatusText},
	}

	for _, c := range def {
		className, _ := utf16Ptr(c.class)
		text, _ := utf16Ptr(c.text)
		hwndChild, _, err := procCreateWindowExW.Call(
			c.exStyle,
			uintptr(unsafe.Pointer(className)),
			uintptr(unsafe.Pointer(text)),
			c.style,
			uintptr(c.x),
			uintptr(c.y),
			uintptr(c.w),
			uintptr(c.h),
			uintptr(hwnd),
			uintptr(c.id),
			hInstance,
			0,
		)
		if hwndChild == 0 {
			log.Printf("create %s failed: %v", c.class, err)
			continue
		}
		switch c.id {
		case controlIDActionProgress:
			a.actionProgress = HWND(hwndChild)
		case controlIDURLEdit:
			a.urlEdit = HWND(hwndChild)
		case controlIDAddTask:
			a.addTaskBtn = HWND(hwndChild)
		case controlIDTaskTitle:
			a.taskTitle = HWND(hwndChild)
		case controlIDListBox:
			a.taskBoard = HWND(hwndChild)
		case controlIDInfoTitle:
			a.infoTitle = HWND(hwndChild)
		case controlIDInfoEdit:
			a.infoEdit = HWND(hwndChild)
		case controlIDStatusText:
			a.statusText = HWND(hwndChild)
		}
		if c.id == controlIDURLEdit || c.id == controlIDAddTask || c.id == controlIDActionProgress {
			setControlFont(HWND(hwndChild), rowFont)
		} else {
			setControlFont(HWND(hwndChild), uiFont)
		}
	}
	a.setActionProgress(0, "")
	setControlFont(a.taskTitle, uiFont)
	setControlFont(a.taskBoard, uiFont)
	setControlFont(a.infoTitle, uiFont)
	setControlFont(a.infoEdit, uiFont)
	setControlFont(a.statusText, uiFont)
}

func (a *frontendApp) layout() {
	var rc RECT
	if r, _, _ := procGetClientRect.Call(uintptr(a.hwnd), uintptr(unsafe.Pointer(&rc))); r == 0 {
		return
	}
	width := int32(rc.Right - rc.Left)
	height := int32(rc.Bottom - rc.Top)

	padding := int32(20)
	addW := int32(120)
	move(a.actionProgress, padding, 8, width-padding*2, 22)
	move(a.urlEdit, 20, 44, max32(260, width-140), 38)
	move(a.addTaskBtn, max32(20, width-140), 44, addW, 38)
	adjustURLInputTextRect(a.urlEdit, 8, 9, 8, 3)
	taskTop := int32(128)
	taskH := max32(160, height-380)
	taskBottom := taskTop + taskH
	infoTitleY := taskBottom + 20
	infoEditY := infoTitleY + 24
	statusY := max32(infoEditY+96+10, height-34)
	infoH := max32(96, statusY-infoEditY-10)
	move(a.taskTitle, padding, 106, 130, 20)
	move(a.taskBoard, padding, taskTop, width-padding*2, taskH)
	move(a.infoTitle, padding, infoTitleY, 240, 20)
	move(a.infoEdit, padding, infoEditY, width-padding*2, infoH)
	move(a.statusText, padding, statusY, width-padding*2, 24)
}

func move(hwnd HWND, x, y, w, h int32) {
	if hwnd == 0 {
		return
	}
	procMoveWindow.Call(uintptr(hwnd), uintptr(x), uintptr(y), uintptr(w), uintptr(h), 1)
}

func adjustURLInputTextRect(hwnd HWND, left, top, right, bottom int32) {
	if hwnd == 0 {
		return
	}
	var rc RECT
	if r, _, _ := procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc))); r == 0 {
		return
	}
	rc.Left = left
	rc.Top = top
	rc.Right = max32(left+20, rc.Right-right)
	rc.Bottom = max32(top+10, rc.Bottom-bottom)
	procSendMessageW.Call(uintptr(hwnd), uintptr(EM_SETRECTNP), 0, uintptr(unsafe.Pointer(&rc)))
}

func (a *frontendApp) handleCommand(id uint16) {
	switch id {
	case menuIDSetExecutable:
		a.pickFirefoxExecutable()
	case menuIDSetChromium:
		a.pickChromiumExecutable()
	case menuIDInstallBrowsers:
		a.installBrowsersAsync()
	case menuIDSetMother:
		a.pickFirefoxMotherProfile()
	case menuIDRefreshProfile:
		a.syncWorkingProfileAsync()
	case menuIDStartAll:
		a.startPendingAsync()
	case menuIDSetDownloadDir:
		a.pickDownloadDir()
	case menuIDSetConcurrency:
		a.cycleConcurrency()
	case menuIDRefreshAdblock:
		a.refreshAdblockRules()
	case menuIDClearCompleted:
		a.clearCompletedTasks()
	case controlIDAddTask:
		a.addPendingTask()
	case controlIDStartTasks:
		a.startPendingAsync()
	case controlIDRefreshBtn:
		a.syncWorkingProfileAsync()
	case controlIDDownloadDir:
		a.pickDownloadDir()
	case controlIDConcurrency:
		a.cycleConcurrency()
	case controlIDAdblockRules:
		a.refreshAdblockRules()
	case controlIDClearDone:
		a.clearCompletedTasks()
	}
}

func (a *frontendApp) showValue(title, value string) {
	msgBox(a.hwnd, title+"\r\n\r\n"+value, title)
}

func (a *frontendApp) pickFirefoxExecutable() {
	path, err := openFileDialog(a.hwnd, "Select Firefox executable", "Executable Files (*.exe)\x00*.exe\x00All Files (*.*)\x00*.*\x00\x00", a.menuState.FirefoxExecutablePath)
	if err != nil {
		a.setStatus("select Firefox executable failed: %v", err)
		return
	}
	if strings.TrimSpace(path) == "" {
		a.setStatus("Firefox executable unchanged")
		return
	}
	a.mu.Lock()
	a.menuState = a.menuState.WithFirefoxExecutablePath(path)
	a.mu.Unlock()
	a.setStatus("Firefox executable set: %s", path)
	a.post(msgRefreshInfo)
}

func (a *frontendApp) pickChromiumExecutable() {
	path, err := openFileDialog(a.hwnd, "Select Chromium executable", "Executable Files (*.exe)\x00*.exe\x00All Files (*.*)\x00*.*\x00\x00", a.menuState.ChromiumExecutablePath)
	if err != nil {
		a.setStatus("select Chromium executable failed: %v", err)
		return
	}
	if strings.TrimSpace(path) == "" {
		a.setStatus("Chromium executable unchanged")
		return
	}
	a.mu.Lock()
	a.menuState = a.menuState.WithChromiumExecutablePath(path)
	a.mu.Unlock()
	a.setStatus("Chromium executable set: %s", path)
	a.post(msgRefreshInfo)
}

func (a *frontendApp) installBrowsersAsync() {
	go func() {
		path, err := browseFolderDialog(a.hwnd, "Select browser install directory", a.defaultBrowserInstallRoot())
		if err != nil {
			a.setStatus("select browser install directory failed: %v", err)
			return
		}
		if strings.TrimSpace(path) == "" {
			a.setStatus("browser install directory unchanged")
			return
		}
		a.setActionProgress(0.05, "选择目录")
		a.setStatus("installing browsers into %s ...", path)
		a.mu.RLock()
		stateSnapshot := a.menuState
		a.mu.RUnlock()
		newState, result, err := a.installMW.InstallAllBrowsersAndApply(stateSnapshot, path, func(update projectruntime.BrowserInstallProgress) {
			phase := strings.TrimSpace(update.Phase)
			message := strings.TrimSpace(update.Message)
			label := "安装浏览器"
			if update.Browser != "" {
				label = browserTypeLabel(update.Browser)
			}
			if phase != "" {
				if message != "" {
					a.setActionProgress(update.Fraction, label+" "+phase)
					a.setStatus("%s: %s", label, message)
					return
				}
				a.setActionProgress(update.Fraction, label+" "+phase)
				a.setStatus("%s: %s", label, phase)
				return
			}
			if message != "" {
				a.setActionProgress(update.Fraction, label)
				a.setStatus("%s: %s", label, message)
				return
			}
			a.setActionProgress(update.Fraction, label)
		})
		if err != nil {
			a.setActionProgress(1, "失败")
			a.setStatus("install browsers failed: %v", err)
			return
		}
		a.mu.Lock()
		a.menuState = newState
		a.mu.Unlock()
		a.setActionProgress(1, "完成")
		a.setStatus("installed browsers into %s", result.TargetRoot)
		msgBox(a.hwnd, "Browsers installed successfully\r\n\r\n"+result.TargetRoot, "Success")
		a.post(msgRefreshInfo)
	}()
}

func (a *frontendApp) pickFirefoxMotherProfile() {
	path, err := browseFolderDialog(a.hwnd, "Select Firefox mother profile directory", a.menuState.FirefoxMotherProfileDir)
	if err != nil {
		a.setStatus("select Firefox mother profile failed: %v", err)
		return
	}
	if strings.TrimSpace(path) == "" {
		a.setStatus("Firefox mother profile unchanged")
		return
	}
	a.mu.Lock()
	a.menuState = a.menuState.WithFirefoxMotherProfileDir(path)
	a.mu.Unlock()
	a.setStatus("Firefox mother profile set: %s", path)
	a.post(msgRefreshInfo)
}

func (a *frontendApp) pickDownloadDir() {
	path, err := browseFolderDialog(a.hwnd, "Select download directory", a.downloadDir)
	if err != nil {
		a.setStatus("select download directory failed: %v", err)
		return
	}
	if strings.TrimSpace(path) == "" {
		a.setStatus("download directory unchanged")
		return
	}
	a.mu.Lock()
	a.downloadDir = path
	a.mu.Unlock()
	a.setStatus("download directory set: %s", path)
	a.post(msgRefreshInfo)
}

func (a *frontendApp) cycleConcurrency() {
	a.mu.Lock()
	switch a.concurrency {
	case 1:
		a.concurrency = 2
	case 2:
		a.concurrency = 4
	case 4:
		a.concurrency = 8
	default:
		a.concurrency = 1
	}
	concurrency := a.concurrency
	a.mu.Unlock()
	a.updateConcurrencyButton(concurrency)
	a.setStatus("concurrency set to %d", concurrency)
	a.post(msgRefreshInfo)
}

func (a *frontendApp) refreshAdblockRules() {
	rulePath := filepath.Join(a.workspaceRoot, "adblock", "AWAvenue-Ads-Rule.txt")
	data, err := os.ReadFile(rulePath)
	if err != nil {
		a.setStatus("load adblock rules failed: %v", err)
		return
	}
	lines := strings.Count(string(data), "\n")
	a.mu.Lock()
	a.adblockInfo = fmt.Sprintf("loaded %d lines from %s", lines, rulePath)
	a.mu.Unlock()
	a.setStatus("adblock rules updated: %s", rulePath)
	a.post(msgRefreshInfo)
}

func (a *frontendApp) clearCompletedTasks() {
	cleared := a.todo.ClearFinished()
	a.setStatus("cleared %d finished tasks", cleared)
	a.post(msgRefreshTasks)
	a.post(msgRefreshInfo)
}

func (a *frontendApp) updateConcurrencyButton(value int) {
	if a.concurrencyBtn == 0 {
		return
	}
	label := fmt.Sprintf("\u5e76\u53d1 x%d", value)
	text, _ := utf16Ptr(label)
	procSetWindowTextW.Call(uintptr(a.concurrencyBtn), uintptr(unsafe.Pointer(text)))
}

func (a *frontendApp) addPendingTask() {
	url := strings.TrimSpace(getControlText(a.urlEdit))
	if url == "" {
		a.setStatus("URL cannot be empty")
		return
	}
	a.mu.RLock()
	req := tasks.BrowserLaunchRequest{
		URL:          url,
		Headless:     false,
		RuntimeRoot:  a.paths.Root,
		BrowserPath:  a.menuState.FirefoxExecutablePath,
		DriverDir:    a.menuState.PlaywrightDriverDir,
		ProfileDir:   a.menuState.FirefoxWorkingProfileDir,
		UserDataDir:  a.menuState.FirefoxWorkingProfileDir,
		UserAgent:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:149.0) Gecko/20100101 Firefox/149.0",
		DownloadRoot: a.downloadDir,
		OutputDir:    a.downloadDir,
	}
	a.mu.RUnlock()
	a.setStatus("starting task...")
	go func() {
		item, err := a.todo.RunImmediately(req, nil)
		if err != nil {
			a.setStatus("task %s failed: %v", item.ID, err)
		} else {
			a.setStatus("task %s completed", item.ID)
		}
		a.post(msgRefreshTasks)
		a.post(msgRefreshInfo)
	}()
}

func (a *frontendApp) startPendingAsync() {
	go func() {
		if a.todo.BrowserRunning(a.workspaceRoot) {
			a.setStatus("browser is running, please close the current browser window")
			if err := a.todo.WaitForBrowserClose(a.workspaceRoot, 250*time.Millisecond); err != nil {
				a.setStatus("wait for browser close failed: %v", err)
				return
			}
		}
		a.setStatus("starting unfinished tasks...")
		items, err := a.todo.StartAllUnfinished(a.workspaceRoot, nil)
		if err != nil {
			a.setStatus("start tasks failed: %v", err)
			return
		}
		a.setStatus("task run complete, processed %d items", len(items))
		a.post(msgRefreshTasks)
		a.post(msgRefreshInfo)
	}()
}

func (a *frontendApp) syncWorkingProfileAsync() {
	go func() {
		a.setActionProgress(0.08, "\u7b49\u5f85\u6d4f\u89c8\u5668\u5173\u95ed")
		a.setStatus("waiting for browser to close and copying profile...")
		mother := a.currentMotherProfileDir()
		a.setActionProgress(0.35, "\u590d\u5236\u5b50\u914d\u7f6e")
		result, err := a.profileMW.CloseCurrentBrowserAndCopyFirefoxProfileFromSource(mother, 250*time.Millisecond)
		if err != nil {
			a.setActionProgress(1, "\u5931\u8d25")
			a.setStatus("refresh profile failed: %v", err)
			return
		}
		a.mu.Lock()
		a.menuState = a.menuState.WithFirefoxWorkingProfileDir(result.TargetProfileDir)
		a.mu.Unlock()
		a.setActionProgress(1, "\u5b8c\u6210")
		a.setStatus("profile refreshed: %s", result.TargetProfileDir)
		msgBox(a.hwnd, "Profile refreshed successfully\r\n\r\n"+result.TargetProfileDir, "Success")
		a.post(msgRefreshTasks)
		a.post(msgRefreshInfo)
	}()
}

func (a *frontendApp) setActionProgress(value float64, text string) {
	a.mu.Lock()
	a.actionProgressValue = clamp01(value)
	a.actionProgressText = text
	a.mu.Unlock()
	a.post(msgRefreshAction)
}

func (a *frontendApp) refreshActionProgress() {
	if a.actionProgress == 0 {
		return
	}
	procInvalidateRect.Call(uintptr(a.actionProgress), 0, 1)
}

func (a *frontendApp) actionProgressSnapshot() (float64, string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.actionProgressValue, a.actionProgressText
}

func (a *frontendApp) currentMotherProfileDir() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.menuState.FirefoxMotherProfileDir
}

func (a *frontendApp) defaultBrowserInstallRoot() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if strings.TrimSpace(a.menuState.FirefoxInstallRoot) != "" {
		return a.menuState.FirefoxInstallRoot
	}
	if strings.TrimSpace(a.menuState.ChromiumInstallRoot) != "" {
		return a.menuState.ChromiumInstallRoot
	}
	return projectruntime.DefaultFirefoxInstallDir(a.paths.Root)
}

func browserTypeLabel(browserType projectruntime.BrowserType) string {
	switch browserType {
	case projectruntime.BrowserTypeChromium:
		return "Chromium"
	case projectruntime.BrowserTypeFirefox:
		return "Firefox"
	default:
		return string(browserType)
	}
}

func (a *frontendApp) post(msg uint32) {
	if a.hwnd != 0 {
		procPostMessageW.Call(uintptr(a.hwnd), uintptr(msg), 0, 0)
	}
}

func (a *frontendApp) setStatus(format string, args ...any) {
	a.mu.Lock()
	a.status = fmt.Sprintf(format, args...)
	a.mu.Unlock()
	a.post(msgRefreshStatus)
}

func (a *frontendApp) refreshStatus() {
	if a.statusText == 0 {
		return
	}
	procSetWindowTextW.Call(uintptr(a.statusText), uintptr(unsafe.Pointer(utf16PtrMust(a.statusSnapshot()))))
}

func (a *frontendApp) refreshTaskList() {
	if a.taskBoard == 0 {
		return
	}
	taskBoardRefresh(a.taskBoard)
}

func (a *frontendApp) refreshInfo() {
	if a.infoEdit == 0 {
		return
	}
	a.mu.RLock()
	menu := a.menuState
	status := a.status
	a.mu.RUnlock()
	info := strings.Join([]string{
		"\u6d4f\u89c8\u5668\u8bbe\u7f6e",
		"  Chromium \u5b89\u88c5\u76ee\u5f55: " + menu.ChromiumInstallRoot,
		"  Chromium \u53ef\u6267\u884c\u6587\u4ef6: " + menu.ChromiumExecutablePath,
		"  Playwright driver: " + menu.PlaywrightDriverDir,
		"  \u706b\u72d0\u53ef\u6267\u884c\u6587\u4ef6: " + menu.FirefoxExecutablePath,
		"  \u6d4f\u89c8\u5668\u5b89\u88c5\u76ee\u5f55: " + menu.FirefoxInstallRoot,
		"  \u706b\u72d0\u6bcd\u914d\u7f6e\u76ee\u5f55: " + menu.FirefoxMotherProfileDir,
		"  \u706b\u72d0\u5de5\u4f5c\u914d\u7f6e: " + menu.FirefoxWorkingProfileDir,
		"  \u4e0b\u8f7d\u76ee\u5f55: " + a.downloadDir,
		"  \u5e76\u53d1\u91cf: " + fmt.Sprintf("%d", a.concurrency),
		"  \u53bb\u5e7f\u544a: " + a.adblockInfo,
		"",
		"\u8def\u5f84",
		"  workspaceRoot: " + a.workspaceRoot,
		"  runtimeRoot: " + a.paths.Root,
		"  browserProfilesRoot: " + a.paths.BrowserRoot,
		"",
		fmt.Sprintf("\u5f85\u5904\u7406\u4efb\u52a1: %d", len(a.todo.Pending())),
		"\u72b6\u6001: " + status,
	}, "\r\n")
	procSetWindowTextW.Call(uintptr(a.infoEdit), uintptr(unsafe.Pointer(utf16PtrMust(info))))
}

func (a *frontendApp) refreshAllUI() {
	a.refreshStatus()
	a.refreshTaskList()
	a.refreshInfo()
}

func (a *frontendApp) statusSnapshot() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

func msgBox(hwnd HWND, text, caption string) {
	textPtr, _ := utf16Ptr(text)
	captionPtr, _ := utf16Ptr(caption)
	procMessageBoxW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(captionPtr)), MB_OK)
}

func getControlText(hwnd HWND) string {
	if hwnd == 0 {
		return ""
	}
	n, _, _ := procGetWindowTextLen.Call(uintptr(hwnd))
	buf := make([]uint16, int(n)+1)
	procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return strings.TrimRight(string(utf16.Decode(buf)), "\x00")
}

func openFileDialog(hwnd HWND, title, filter, initialPath string) (string, error) {
	fileBuf := make([]uint16, 32768)
	initialDir := initialPath
	if strings.TrimSpace(initialDir) != "" {
		initialDir = filepath.Dir(initialDir)
	}
	var initialDirPtr *uint16
	if strings.TrimSpace(initialDir) != "" {
		initialDirPtr, _ = utf16Ptr(initialDir)
	}
	titlePtr, _ := utf16Ptr(title)
	filterPtr, _ := utf16Ptr(filter)
	ofn := OPENFILENAMEW{
		LStructSize:     uint32(unsafe.Sizeof(OPENFILENAMEW{})),
		HwndOwner:       hwnd,
		LpstrFilter:     filterPtr,
		LpstrFile:       &fileBuf[0],
		NMaxFile:        uint32(len(fileBuf)),
		LpstrTitle:      titlePtr,
		LpstrInitialDir: initialDirPtr,
		Flags:           OFN_EXPLORER | OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST,
	}
	r, _, err := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return "", err
	}
	return strings.TrimRight(string(utf16.Decode(fileBuf)), "\x00"), nil
}

func browseFolderDialog(hwnd HWND, title, initialPath string) (string, error) {
	displayBuf := make([]uint16, 260)
	titlePtr, _ := utf16Ptr(title)
	var initialPathPtr *uint16
	if strings.TrimSpace(initialPath) != "" {
		initialPathPtr, _ = utf16Ptr(initialPath)
	}
	bi := BROWSEINFOW{
		HwndOwner:      hwnd,
		PidlRoot:       0,
		PszDisplayName: &displayBuf[0],
		LpszTitle:      titlePtr,
		UlFlags:        BIF_RETURNONLYFSDIRS | BIF_USENEWUI,
		LParam:         uintptr(unsafe.Pointer(initialPathPtr)),
	}
	pidl, _, err := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return "", err
	}
	defer procCoTaskMemFree.Call(pidl)

	pathBuf := make([]uint16, 260)
	r, _, err := procSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&pathBuf[0])))
	if r == 0 {
		return "", err
	}
	return strings.TrimRight(string(utf16.Decode(pathBuf)), "\x00"), nil
}

func utf16Ptr(s string) (*uint16, error) {
	return syscall.UTF16PtrFromString(s)
}

func utf16PtrMust(s string) *uint16 {
	p, err := utf16Ptr(s)
	if err != nil {
		return nil
	}
	return p
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func executableWorkspaceRoot() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

func ensureThemeResources() error {
	themeOnce.Do(func() {
		bgHandle, _, _ := procCreateSolidBrush.Call(uintptr(rgb(18, 22, 28)))
		panelHandle, _, _ := procCreateSolidBrush.Call(uintptr(rgb(28, 34, 42)))
		editHandle, _, _ := procCreateSolidBrush.Call(uintptr(rgb(36, 42, 52)))
		darkBgBrush = HBRUSH(bgHandle)
		darkPanelBrush = HBRUSH(panelHandle)
		darkEditBrush = HBRUSH(editHandle)
		uiFont = createUIFont(-14, 400, "Segoe UI")
		rowFont = createUIFont(-12, 400, "Segoe UI")
		titleFont = createUIFont(-14, 700, "Segoe UI")
		if darkBgBrush == 0 || darkPanelBrush == 0 || darkEditBrush == 0 || uiFont == 0 || rowFont == 0 || titleFont == 0 {
			themeInitErr = fmt.Errorf("initialize theme resources failed")
		}
	})
	return themeInitErr
}

func applyDarkWindowChrome(hwnd HWND) {
	if hwnd == 0 {
		return
	}
	if err := procDwmSetWindowAttribute.Find(); err == nil {
		enabled := uint32(1)
		_, _, _ = procDwmSetWindowAttribute.Call(
			uintptr(hwnd),
			uintptr(DWMWA_USE_IMMERSIVE_DARK_MODE),
			uintptr(unsafe.Pointer(&enabled)),
			uintptr(unsafe.Sizeof(enabled)),
		)
		caption := rgb(18, 22, 28)
		text := rgb(230, 236, 242)
		_, _, _ = procDwmSetWindowAttribute.Call(
			uintptr(hwnd),
			uintptr(DWMWA_CAPTION_COLOR),
			uintptr(unsafe.Pointer(&caption)),
			uintptr(unsafe.Sizeof(caption)),
		)
		_, _, _ = procDwmSetWindowAttribute.Call(
			uintptr(hwnd),
			uintptr(DWMWA_TEXT_COLOR),
			uintptr(unsafe.Pointer(&text)),
			uintptr(unsafe.Sizeof(text)),
		)
	}
}

func enableDarkAppMode() {
	if err := procSetPreferredAppMode.Find(); err == nil {
		_, _, _ = procSetPreferredAppMode.Call(uintptr(APPMODE_ALLOWDARK))
	}
	if err := procFlushMenuThemes.Find(); err == nil {
		_, _, _ = procFlushMenuThemes.Call()
	}
}

func createUIFont(height int32, weight int32, face string) HGDIOBJ {
	facePtr, err := utf16Ptr(face)
	if err != nil {
		return 0
	}
	handle, _, _ := procCreateFontW.Call(
		uintptr(height),
		0,
		0,
		0,
		uintptr(weight),
		0,
		0,
		0,
		1,
		0,
		0,
		5,
		0,
		uintptr(unsafe.Pointer(facePtr)),
	)
	return HGDIOBJ(handle)
}

func setControlFont(hwnd HWND, font HGDIOBJ) {
	if hwnd == 0 || font == 0 {
		return
	}
	procSendMessageW.Call(uintptr(hwnd), WM_SETFONT, uintptr(font), 1)
}
