//go:build windows

package windows

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/y3owk1n/neru/internal/adapter/systray/icon"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Win32 notification-area (system tray) icon + popup menu via pure syscall.
// Does not implement macOS template-icon theming or per-item icons.
//
// winTrayIconPNG is the startup/fallback tray tile. The shared component
// normally selects the platform-appropriate asset and passes it via SetIcon;
// this covers the window before that first call.
var winTrayIconPNG = icon.Brand

// Win32 message and shell constants.
const (
	wmDestroy     = 0x0002
	wmQuit        = 0x0012
	wmApp         = 0x8000
	wmTrayicon    = wmApp + 1
	wmNull        = 0x0000
	wmRButtonUp   = 0x0205
	wmLButtonUp   = 0x0202
	wmContextMenu = 0x007B

	nimAdd    = 0x0000
	nimModify = 0x0001
	nimDelete = 0x0002

	nifMessage = 0x0001
	nifIcon    = 0x0002
	nifTip     = 0x0004
	nifInfo    = 0x0010

	// niifNone: no stock glyph on the balloon; Windows 10 and 11 render the
	// tip as a toast that already carries the tray icon.
	niifNone = 0x0000

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	mfString    = 0x0000
	mfSeparator = 0x0800
	mfPopup     = 0x0010
	mfGrayed    = 0x0001
	mfChecked   = 0x0008

	idiApplication = 32512
	idcArrow       = 32512

	diRgbColors = 0
	biRGB       = 0
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procRegisterWindowMsgW  = user32.NewProc("RegisterWindowMessageW")
	procCreateIconIndirect  = user32.NewProc("CreateIconIndirect")
	procDestroyIcon         = user32.NewProc("DestroyIcon")

	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")

	procCreateDIBSection = gdi32.NewProc("CreateDIBSection")
	procCreateBitmap     = gdi32.NewProc("CreateBitmap")
	procDeleteObject     = gdi32.NewProc("DeleteObject")

	procGetModuleHandleW   = kernel32.NewProc("GetModuleHandleW")
	procGetCurrentThreadID = kernel32.NewProc("GetCurrentThreadId")
)

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type point struct {
	x int32
	y int32
}

type winMsg struct {
	hwnd     uintptr
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       point
	lPrivate uint32
}

// notifyIconData mirrors the full NOTIFYICONDATAW so cbSize selects the
// Vista+ layout, which is what makes NIF_INFO balloon tips available.
type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     uintptr
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

// menuNode is one entry (item or separator) in an ordered menu level.
type menuNode struct {
	item      *MenuItem
	separator bool
}

// MenuItem is a menu item in the system tray.
type MenuItem struct {
	ClickedCh chan struct{}
	id        int
	mu        sync.RWMutex
	title     string
	disabled  bool
	checked   bool
	hidden    bool
	children  []*menuNode
}

var (
	menuItems     = make(map[int]*MenuItem)
	menuItemsLock sync.RWMutex
	nextID        = 1
	topNodes      []*menuNode

	trayMu          sync.Mutex
	trayHWND        uintptr
	trayThreadID    uint32
	trayHeadless    bool
	trayQuit        bool
	trayIconHandle  uintptr
	trayNID         notifyIconData
	taskbarCreated  uint32
	wndProcCallback = syscall.NewCallback(trayWndProc)
)

// Title returns the menu item title.
func (m *MenuItem) Title() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.title
}

// Disabled returns whether the menu item is disabled.
func (m *MenuItem) Disabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.disabled
}

// Checked returns whether the menu item is checked.
func (m *MenuItem) Checked() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.checked
}

// Hidden returns whether the menu item is hidden.
func (m *MenuItem) Hidden() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.hidden
}

// SetTitle sets the menu item title.
func (m *MenuItem) SetTitle(title string) {
	m.mu.Lock()
	m.title = title
	m.mu.Unlock()
}

// SetTooltip sets the menu item tooltip (unused on Windows popup menus).
func (m *MenuItem) SetTooltip(tooltip string) {}

// SetIcon sets the menu item icon (per-item icons unsupported on Windows).
func (m *MenuItem) SetIcon(icon []byte) {}

// Enable enables the menu item.
func (m *MenuItem) Enable() {
	m.mu.Lock()
	m.disabled = false
	m.mu.Unlock()
}

// Disable disables the menu item.
func (m *MenuItem) Disable() {
	m.mu.Lock()
	m.disabled = true
	m.mu.Unlock()
}

// Check checks the menu item.
func (m *MenuItem) Check() {
	m.mu.Lock()
	m.checked = true
	m.mu.Unlock()
}

// Uncheck unchecks the menu item.
func (m *MenuItem) Uncheck() {
	m.mu.Lock()
	m.checked = false
	m.mu.Unlock()
}

// Show shows the menu item.
func (m *MenuItem) Show() {
	m.mu.Lock()
	m.hidden = false
	m.mu.Unlock()
}

// Hide hides the menu item.
func (m *MenuItem) Hide() {
	m.mu.Lock()
	m.hidden = true
	m.mu.Unlock()
}

// AddSubMenuItem adds a sub menu item to the menu item.
func (m *MenuItem) AddSubMenuItem(title string) *MenuItem {
	item := newMenuItem(title)

	m.mu.Lock()
	m.children = append(m.children, &menuNode{item: item})
	m.mu.Unlock()

	return item
}

// AddSeparator adds a separator to the menu item's submenu.
func (m *MenuItem) AddSeparator() {
	m.mu.Lock()
	m.children = append(m.children, &menuNode{separator: true})
	m.mu.Unlock()
}

// Run starts the system tray loop with a status icon.
func Run(onReadyFunc, onExitFunc func()) {
	runTray(true, onReadyFunc, onExitFunc)
}

// RunHeadless starts the system tray loop without a status icon.
func RunHeadless(onReadyFunc, onExitFunc func()) {
	runTray(false, onReadyFunc, onExitFunc)
}

func runTray(withIcon bool, onReadyFunc, onExitFunc func()) {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	trayMu.Lock()
	if trayQuit {
		trayMu.Unlock()

		if onExitFunc != nil {
			onExitFunc()
		}

		return
	}

	trayThreadID = currentThreadID()
	trayHeadless = !withIcon
	trayMu.Unlock()

	if withIcon {
		err := createTrayWindow()
		if err != nil {
			if onExitFunc != nil {
				onExitFunc()
			}

			return
		}

		addTrayIcon()
	}

	if onReadyFunc != nil {
		onReadyFunc()
	}

	pumpMessages()

	if withIcon {
		removeTrayIcon()
		destroyTrayWindow()
	}

	if onExitFunc != nil {
		onExitFunc()
	}
}

// Quit stops the tray message loop.
func Quit() {
	trayMu.Lock()
	trayQuit = true
	tid := trayThreadID
	trayMu.Unlock()

	if tid != 0 {
		// WM_QUIT delivered via the thread queue unblocks GetMessageW even
		// when no window has focus.
		discardCall(procPostThreadMessageW.Call(uintptr(tid), wmQuit, 0, 0))
	}
}

// SetTitle is a no-op: Windows tray icons have no inline title text.
func SetTitle(title string) {}

// SetTooltip sets the tooltip shown when hovering the tray icon.
func SetTooltip(tooltip string) {
	trayMu.Lock()
	defer trayMu.Unlock()

	if trayHWND == 0 {
		return
	}

	copyTip(&trayNID, tooltip)
	trayNID.uFlags = nifMessage | nifIcon | nifTip
	shellNotify(nimModify, &trayNID)
}

// SetIcon replaces the tray icon with one built from the passed PNG bytes.
// The shared component selects a host-appropriate asset (the colored brand
// tile on Windows — a macOS template glyph would be invisible in the
// notification area). Empty or undecodable bytes leave the current icon.
func SetIcon(iconBytes []byte) {
	if len(iconBytes) == 0 {
		return
	}

	newIcon := iconFromPNG(iconBytes)
	if newIcon == 0 {
		return
	}

	trayMu.Lock()
	defer trayMu.Unlock()

	if trayHWND == 0 {
		// Tray not created yet; addTrayIcon will pick this handle up.
		if trayIconHandle != 0 {
			discardCall(procDestroyIcon.Call(trayIconHandle))
		}

		trayIconHandle = newIcon

		return
	}

	oldIcon := trayIconHandle
	trayIconHandle = newIcon
	trayNID.hIcon = newIcon
	trayNID.uFlags = nifMessage | nifIcon | nifTip
	shellNotify(nimModify, &trayNID)

	if oldIcon != 0 {
		discardCall(procDestroyIcon.Call(oldIcon))
	}
}

// SetTemplateIcon sets the tray icon from the passed PNG bytes. Windows has
// no template rendering, so the template flag is ignored and the bytes are
// rendered literally.
func SetTemplateIcon(iconBytes []byte, template bool) {
	SetIcon(iconBytes)
}

// Headless reports whether this process started its tray loop without an
// icon (systray.enabled = false). It is false before the loop starts, so a
// process that never runs a tray, such as the CLI, keeps the static capability
// rather than reporting a tray it was never going to show.
func Headless() bool {
	trayMu.Lock()
	defer trayMu.Unlock()

	return trayHeadless
}

// ShowBalloon shows a balloon tip (a toast on Windows 10 and 11) anchored to
// the tray icon. It is the Windows notification path: the shell renders
// NIF_INFO tips for any icon in the notification area, with no AppUserModelID
// or WinRT involved. Without a tray icon there is nothing to anchor to, so it
// reports CodeNotSupported naming the reason rather than dropping the message.
func ShowBalloon(title, message string) error {
	trayMu.Lock()
	defer trayMu.Unlock()

	if trayHWND == 0 {
		return derrors.New(
			derrors.CodeNotSupported,
			"notifications are tray balloon tips on Windows and the tray icon is "+
				"not shown; enable systray.enabled to see them",
		)
	}

	nid := notifyIconData{
		cbSize:      uint32(unsafe.Sizeof(notifyIconData{})),
		hWnd:        trayHWND,
		uID:         trayNID.uID,
		uFlags:      nifInfo,
		dwInfoFlags: niifNone,
	}
	copyUTF16(nid.szInfoTitle[:], title)
	// An empty szInfo removes the current balloon instead of showing one.
	copyUTF16(nid.szInfo[:], message)

	ret, _, err := procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
	if ret == 0 {
		return derrors.Wrap(
			err,
			derrors.CodeActionFailed,
			"Shell_NotifyIcon NIM_MODIFY balloon failed",
		)
	}

	return nil
}

// AddMenuItem adds a top-level menu item to the tray menu.
func AddMenuItem(title string) *MenuItem {
	item := newMenuItem(title)

	menuItemsLock.Lock()

	topNodes = append(topNodes, &menuNode{item: item})
	menuItemsLock.Unlock()

	return item
}

// AddSeparator adds a separator to the top-level tray menu.
func AddSeparator() {
	menuItemsLock.Lock()

	topNodes = append(topNodes, &menuNode{separator: true})
	menuItemsLock.Unlock()
}

func newMenuItem(title string) *MenuItem {
	item := &MenuItem{
		ClickedCh: make(chan struct{}, 1),
		title:     title,
	}
	item.id = registerMenuItem(item)

	return item
}

func registerMenuItem(item *MenuItem) int {
	menuItemsLock.Lock()
	defer menuItemsLock.Unlock()

	id := nextID
	nextID++
	menuItems[id] = item

	return id
}

// ResetForTesting resets all global state.
func ResetForTesting() {
	menuItemsLock.Lock()
	defer menuItemsLock.Unlock()

	menuItems = make(map[int]*MenuItem)
	nextID = 1
	topNodes = nil
}

const (
	loWordMask    = 0xFFFF
	bitsPerPixel  = 32
	bytesPerPixel = 4
)

// discardCall consumes a fire-and-forget user32/shell32/gdi32 syscall result.
// These tray calls have no actionable failure path here; the sink keeps
// errcheck satisfied without a `_, _, _ =` assignment (which trips dogsled).
func discardCall(uintptr, uintptr, error) {}

func createTrayWindow() error {
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	className := utf16Ptr("NeruTrayWindow")
	cursor, _, _ := procLoadCursorW.Call(0, uintptr(idcArrow))

	windowClass := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		lpfnWndProc:   wndProcCallback,
		hInstance:     hInstance,
		hCursor:       cursor,
		lpszClassName: className,
	}

	// Ignore RegisterClassExW failure: a re-run reuses the existing class.
	discardCall(procRegisterClassExW.Call(uintptr(unsafe.Pointer(&windowClass))))

	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16Ptr("Neru"))),
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		hInstance,
		0,
	)
	if hwnd == 0 {
		return err
	}

	if taskbarCreated == 0 {
		ret, _, _ := procRegisterWindowMsgW.Call(
			uintptr(unsafe.Pointer(utf16Ptr("TaskbarCreated"))),
		)
		taskbarCreated = uint32(ret)
	}

	trayMu.Lock()
	trayHWND = hwnd
	trayMu.Unlock()

	return nil
}

func destroyTrayWindow() {
	trayMu.Lock()
	hwnd := trayHWND
	trayHWND = 0
	icon := trayIconHandle
	trayIconHandle = 0
	trayMu.Unlock()

	if hwnd != 0 {
		discardCall(procDestroyWindow.Call(hwnd))
	}

	if icon != 0 {
		discardCall(procDestroyIcon.Call(icon))
	}
}

func addTrayIcon() {
	trayMu.Lock()
	defer trayMu.Unlock()

	if trayHWND == 0 {
		return
	}

	icon := trayIconHandle
	if icon == 0 {
		icon = loadBrandIcon()
		trayIconHandle = icon
	}

	trayNID = notifyIconData{
		cbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		hWnd:             trayHWND,
		uID:              1,
		uFlags:           nifMessage | nifIcon | nifTip,
		uCallbackMessage: wmTrayicon,
		hIcon:            icon,
	}
	copyTip(&trayNID, "Neru")
	shellNotify(nimAdd, &trayNID)
}

func removeTrayIcon() {
	trayMu.Lock()
	defer trayMu.Unlock()

	if trayHWND == 0 {
		return
	}

	shellNotify(nimDelete, &trayNID)
}

// loadBrandIcon builds an HICON from the embedded brand PNG, falling back to
// the stock application icon if decoding fails.
func loadBrandIcon() uintptr {
	icon := iconFromPNG(winTrayIconPNG)
	if icon == 0 {
		icon, _, _ = procLoadIconW.Call(0, uintptr(idiApplication))
	}

	return icon
}

func shellNotify(message uint32, nid *notifyIconData) {
	discardCall(procShellNotifyIconW.Call(uintptr(message), uintptr(unsafe.Pointer(nid))))
}

func pumpMessages() {
	var msg winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		switch int32(ret) {
		case 0: // WM_QUIT
			return
		case -1: // error
			return
		default:
			discardCall(procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg))))
			discardCall(procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg))))
		}
	}
}

func trayWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch {
	case msg == wmTrayicon:
		event := uint32(lParam & loWordMask)
		if event == wmRButtonUp || event == wmLButtonUp || event == wmContextMenu {
			showTrayMenu(hwnd)
		}

		return 0
	case taskbarCreated != 0 && msg == taskbarCreated:
		// Explorer restarted; re-add the icon.
		addTrayIcon()

		return 0
	case msg == wmDestroy:
		discardCall(procPostQuitMessage.Call(0))

		return 0
	default:
		ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)

		return ret
	}
}

func showTrayMenu(hwnd uintptr) {
	menuItemsLock.RLock()

	nodes := append([]*menuNode(nil), topNodes...)

	menuItemsLock.RUnlock()

	hmenu := buildMenu(nodes)
	if hmenu == 0 {
		return
	}

	defer func() { discardCall(procDestroyMenu.Call(hmenu)) }()

	var cursorPos point

	discardCall(procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursorPos))))

	// Required so the menu dismisses when the user clicks elsewhere.
	discardCall(procSetForegroundWindow.Call(hwnd))

	cmd, _, _ := procTrackPopupMenu.Call(
		hmenu,
		tpmRightButton|tpmReturnCmd,
		uintptr(cursorPos.x),
		uintptr(cursorPos.y),
		0,
		hwnd,
		0,
	)

	discardCall(procPostMessageW.Call(hwnd, wmNull, 0, 0))

	if cmd != 0 {
		dispatchClick(int(cmd))
	}
}

func buildMenu(nodes []*menuNode) uintptr {
	hmenu, _, _ := procCreatePopupMenu.Call()
	if hmenu == 0 {
		return 0
	}

	for _, node := range nodes {
		if node.separator {
			discardCall(procAppendMenuW.Call(hmenu, mfSeparator, 0, 0))

			continue
		}

		item := node.item
		item.mu.RLock()
		hidden := item.hidden
		disabled := item.disabled
		checked := item.checked
		title := item.title
		children := append([]*menuNode(nil), item.children...)
		itemID := item.id
		item.mu.RUnlock()

		if hidden {
			continue
		}

		text := utf16Ptr(title)

		if len(children) > 0 {
			sub := buildMenu(children)

			flags := uintptr(mfString | mfPopup)
			if disabled {
				flags |= mfGrayed
			}

			discardCall(procAppendMenuW.Call(hmenu, flags, sub, uintptr(unsafe.Pointer(text))))

			continue
		}

		flags := uintptr(mfString)
		if disabled {
			flags |= mfGrayed
		}

		if checked {
			flags |= mfChecked
		}

		discardCall(
			procAppendMenuW.Call(hmenu, flags, uintptr(itemID), uintptr(unsafe.Pointer(text))),
		)
	}

	return hmenu
}

func dispatchClick(id int) {
	menuItemsLock.RLock()

	item := menuItems[id]

	menuItemsLock.RUnlock()

	if item == nil {
		return
	}

	select {
	case item.ClickedCh <- struct{}{}:
	default:
	}
}

// iconFromPNG decodes PNG bytes into an HICON, or returns 0 on failure so the
// caller can fall back to a stock icon.
func iconFromPNG(data []byte) uintptr {
	if len(data) == 0 {
		return 0
	}

	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return 0
	}

	bounds := src.Bounds()
	width := bounds.Dx()

	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return 0
	}

	rgba := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(rgba, rgba.Bounds(), src, bounds.Min, draw.Src)

	bmpInfo := bitmapInfoHeader{
		biSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		biWidth:       int32(width),
		biHeight:      -int32(height), // top-down
		biPlanes:      1,
		biBitCount:    bitsPerPixel,
		biCompression: biRGB,
	}

	var bits unsafe.Pointer

	hbmColor, _, _ := procCreateDIBSection.Call(
		0,
		uintptr(unsafe.Pointer(&bmpInfo)),
		diRgbColors,
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if hbmColor == 0 || bits == nil {
		return 0
	}

	pixels := unsafe.Slice((*byte)(bits), width*height*bytesPerPixel)
	for y := range height {
		for x := range width {
			srcIdx := rgba.PixOffset(x, y)
			dstIdx := (y*width + x) * bytesPerPixel
			red := rgba.Pix[srcIdx]
			green := rgba.Pix[srcIdx+1]
			blue := rgba.Pix[srcIdx+2]
			alpha := rgba.Pix[srcIdx+3]
			// DIB is BGRA byte order.
			pixels[dstIdx] = blue
			pixels[dstIdx+1] = green
			pixels[dstIdx+2] = red
			pixels[dstIdx+3] = alpha
		}
	}

	hbmMask, _, _ := procCreateBitmap.Call(uintptr(width), uintptr(height), 1, 1, 0)

	info := iconInfo{
		fIcon:    1,
		hbmMask:  hbmMask,
		hbmColor: hbmColor,
	}

	hicon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))

	// CreateIconIndirect copies the bitmaps; release ours.
	discardCall(procDeleteObject.Call(hbmColor))

	if hbmMask != 0 {
		discardCall(procDeleteObject.Call(hbmMask))
	}

	return hicon
}

func copyTip(nid *notifyIconData, tip string) {
	copyUTF16(nid.szTip[:], tip)
}

// copyUTF16 writes s into a fixed NOTIFYICONDATA text field, truncated to
// leave the terminating NUL.
func copyUTF16(dst []uint16, s string) {
	runes := utf16FromString(s)

	clear(dst)

	limit := min(len(runes), len(dst)-1)

	copy(dst[:limit], runes[:limit])
}

func currentThreadID() uint32 {
	ret, _, _ := procGetCurrentThreadID.Call()

	return uint32(ret)
}

func utf16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}

	return p
}

func utf16FromString(s string) []uint16 {
	u, err := syscall.UTF16FromString(s)
	if err != nil {
		return nil
	}

	return u
}
