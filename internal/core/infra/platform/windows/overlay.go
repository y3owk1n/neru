//go:build windows

// internal/core/infra/platform/windows/overlay.go
// Layered Win32 overlay using color-key transparency and WM_PAINT GDI drawing.
// Does not implement grid logic or ports; ui/overlay consumes this surface.

package windows

import (
	"fmt"
	"image"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	overlayClassName = "NeruOverlayWindow"

	csHRedraw = 0x0002
	csVRedraw = 0x0001

	wsPopup          = 0x80000000
	wsExLayered      = 0x00080000
	wsExTransparent  = 0x00000020
	wsExTopmost      = 0x00000008
	wsExToolWindow   = 0x00000080
	wsExNoActivate   = 0x08000000
	swHide           = 0
	swShowNoActivate = 4
	srccopy          = 0x00CC0020
	transparentBk    = 1
	dtCenter         = 0x00000001
	dtVCenter        = 0x00000004
	dtSingleLine     = 0x00000020
	hwndTopMost      = ^uintptr(0)
	swpNoActivate    = 0x0010
	swpShowWindow    = 0x0040
	lwaColorKey      = 0x00000001
	ulwColorKey      = 0x00000001
	wmPaint          = 0x000F

	defaultOverlayFont = "Segoe UI"
	fwBold             = 700
)

var (
	gdi32 = windows.NewLazySystemDLL("gdi32.dll")

	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procCreateFontW            = gdi32.NewProc("CreateFontW")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procSetTextColor     = gdi32.NewProc("SetTextColor")
	procRegisterClassExW       = user32.NewProc("RegisterClassExW")
	procCreateWindowExW        = user32.NewProc("CreateWindowExW")
	procDestroyWindow          = user32.NewProc("DestroyWindow")
	procShowWindow             = user32.NewProc("ShowWindow")
	procSetWindowPos           = user32.NewProc("SetWindowPos")
	procDefWindowProcW         = user32.NewProc("DefWindowProcW")
	procUnregisterClassW       = user32.NewProc("UnregisterClassW")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procInvalidateRect         = user32.NewProc("InvalidateRect")
	procUpdateWindow           = user32.NewProc("UpdateWindow")
	procIsWindow               = user32.NewProc("IsWindow")
	procGetDC                  = user32.NewProc("GetDC")
	procReleaseDC              = user32.NewProc("ReleaseDC")
	procUpdateLayeredWindow    = user32.NewProc("UpdateLayeredWindow")
	procBeginPaint             = user32.NewProc("BeginPaint")
	procEndPaint               = user32.NewProc("EndPaint")
	procFillRect               = user32.NewProc("FillRect")
	procDrawTextW              = user32.NewProc("DrawTextW")
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	overlayClassOnce sync.Once
	overlayClassErr  error
	overlayWndProcPtr uintptr

	overlayRegistry sync.Map
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         windows.Handle
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       windows.Handle
}

type paintStruct struct {
	hdc         windows.Handle
	fErase      int32
	rcPaint     windows.Rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type rectFill struct {
	rect  image.Rectangle
	color uint32
}

type rectStroke struct {
	rect  image.Rectangle
	color uint32
	width int
}

type textDraw struct {
	text       string
	rect       image.Rectangle
	fontFamily string
	fontSize   float64
	color      uint32
}

// OverlayWindow is a fullscreen click-through layered HWND painted via WM_PAINT.
type OverlayWindow struct {
	mu      sync.Mutex
	hwnd    windows.HWND
	bounds  image.Rectangle
	width   int
	height  int
	visible bool
	dirty   bool

	fills   []rectFill
	strokes []rectStroke
	texts   []textDraw

	colorBlendRGB uint32
}

func overlayWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 {
			if raw, ok := overlayRegistry.Load(windows.HWND(hwnd)); ok {
				if overlay, ok := raw.(*OverlayWindow); ok {
					overlay.paintLocked(windows.Handle(hdc))
				}
			}

			procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		}

		return 0
	default:
		ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)

		return ret
	}
}

func registerOverlayWindowClass() error {
	overlayClassOnce.Do(func() {
		className, err := windows.UTF16PtrFromString(overlayClassName)
		if err != nil {
			overlayClassErr = err

			return
		}

		overlayWndProcPtr = syscall.NewCallback(overlayWndProc)
		instance, _, _ := procGetModuleHandleW.Call(0)

		bgBrush, _, _ := procCreateSolidBrush.Call(overlayColorKey)
		if bgBrush == 0 {
			overlayClassErr = fmt.Errorf("CreateSolidBrush failed")

			return
		}

		class := wndClassEx{
			cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
			style:         csHRedraw | csVRedraw,
			lpfnWndProc:   overlayWndProcPtr,
			hInstance:     windows.Handle(instance),
			hbrBackground: windows.Handle(bgBrush),
			lpszClassName: className,
		}

		atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
		if atom == 0 {
			overlayClassErr = fmt.Errorf("RegisterClassExW: %w", err)
		}
	})

	return overlayClassErr
}

// NewOverlayWindow creates a layered overlay sized to the active monitor.
func NewOverlayWindow() (*OverlayWindow, error) {
	if err := registerOverlayWindowClass(); err != nil {
		return nil, err
	}

	bounds, err := activeScreenBounds()
	if err != nil {
		return nil, err
	}

	overlay := &OverlayWindow{
		bounds:        bounds,
		colorBlendRGB: ThemeSurfaceRGB(),
	}
	var createErr error

	runOnOverlayUI(func() {
		createErr = overlay.createHWNDLocked()
	})
	if createErr != nil {
		return nil, createErr
	}

	return overlay, nil
}

func (o *OverlayWindow) createHWNDLocked() error {
	className, err := windows.UTF16PtrFromString(overlayClassName)
	if err != nil {
		return err
	}

	width := o.bounds.Dx()
	height := o.bounds.Dy()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid overlay bounds %v", o.bounds)
	}

	hwnd, _, err := procCreateWindowExW.Call(
		wsExLayered|wsExTransparent|wsExTopmost|wsExToolWindow|wsExNoActivate,
		uintptr(unsafe.Pointer(className)),
		0,
		wsPopup,
		uintptr(o.bounds.Min.X),
		uintptr(o.bounds.Min.Y),
		uintptr(width),
		uintptr(height),
		0,
		0,
		moduleHandle(),
		0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW: %w", err)
	}

	o.hwnd = windows.HWND(hwnd)
	o.width = width
	o.height = height
	overlayRegistry.Store(o.hwnd, o)

	ret, _, err := procSetLayeredWindowAttributes.Call(
		hwnd,
		overlayColorKey,
		0,
		lwaColorKey,
	)
	if ret == 0 {
		return fmt.Errorf("SetLayeredWindowAttributes: %w", err)
	}

	const swpNomove = 0x0002
	const swpNosize = 0x0001
	procSetWindowPos.Call(
		hwnd,
		hwndTopMost,
		0,
		0,
		0,
		0,
		swpNoActivate|swpNomove|swpNosize,
	)
	procShowWindow.Call(hwnd, swHide)
	o.visible = false

	return nil
}

func (o *OverlayWindow) destroyHWNDLocked() {
	if o.hwnd != 0 {
		overlayRegistry.Delete(o.hwnd)
		procDestroyWindow.Call(uintptr(o.hwnd))
		o.hwnd = 0
	}
}

// HWND returns the native window handle.
func (o *OverlayWindow) HWND() windows.HWND {
	return o.hwnd
}

// Healthy reports whether the overlay window is initialized and still valid.
func (o *OverlayWindow) Healthy() bool {
	if o == nil || o.hwnd == 0 {
		return false
	}

	ret, _, _ := procIsWindow.Call(uintptr(o.hwnd))

	return ret != 0
}

// Visible reports whether the overlay HWND is shown.
func (o *OverlayWindow) Visible() bool {
	if o == nil {
		return false
	}

	o.mu.Lock()
	visible := o.visible
	o.mu.Unlock()

	return visible
}

// Bounds returns the overlay rectangle in screen coordinates.
func (o *OverlayWindow) Bounds() image.Rectangle {
	return o.bounds
}

// SetColorBlendRGB sets the opaque RGB backdrop used to approximate semi-transparent
// theme colors on the GDI overlay (see argbToGDIColorRef).
func (o *OverlayWindow) SetColorBlendRGB(rgb uint32) {
	if o == nil {
		return
	}

	o.mu.Lock()
	o.colorBlendRGB = rgb & 0xFFFFFF
	o.mu.Unlock()
}

// Show displays the overlay without taking focus.
func (o *OverlayWindow) Show() {
	if o == nil || o.hwnd == 0 {
		return
	}

	runOnOverlayUI(func() {
		if o.hwnd == 0 {
			if err := o.createHWNDLocked(); err != nil {
				return
			}
		}

		o.prepareForDisplayLocked()
		procShowWindow.Call(uintptr(o.hwnd), swShowNoActivate)
		const swpNomove = 0x0002
		const swpNosize = 0x0001
		procSetWindowPos.Call(
			uintptr(o.hwnd),
			hwndTopMost,
			0,
			0,
			0,
			0,
			swpNoActivate|swpShowWindow|swpNomove|swpNosize,
		)
		o.visible = true
		_ = o.presentLayeredLocked()
	})
}

func (o *OverlayWindow) prepareForDisplayLocked() {
	if o == nil || o.hwnd == 0 {
		return
	}

	ret, _, _ := procSetLayeredWindowAttributes.Call(
		uintptr(o.hwnd),
		overlayColorKey,
		0,
		lwaColorKey,
	)
	if ret == 0 && o.visible {
		// Layered attributes can be lost after hide/show cycles on some builds.
		return
	}
}

// Hide hides the overlay window and destroys the HWND so the next show gets a
// fresh layered surface (WM_PAINT does not reliably refresh after hide/show).
func (o *OverlayWindow) Hide() {
	if o == nil {
		return
	}

	runOnOverlayUI(func() {
		if o.hwnd != 0 {
			procShowWindow.Call(uintptr(o.hwnd), swHide)
		}

		o.destroyHWNDLocked()
		o.visible = false
	})
}

// Clear resets queued draw commands.
func (o *OverlayWindow) Clear() {
	if o == nil {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	o.fills = o.fills[:0]
	o.strokes = o.strokes[:0]
	o.texts = o.texts[:0]
	o.dirty = true
}

// ResizeToActiveScreen moves and resizes the overlay to the active monitor.
func (o *OverlayWindow) ResizeToActiveScreen() error {
	if o == nil {
		return fmt.Errorf("overlay is nil")
	}

	bounds, err := activeScreenBounds()
	if err != nil {
		return err
	}

	if bounds == o.bounds && o.width == bounds.Dx() && o.height == bounds.Dy() {
		return nil
	}

	o.mu.Lock()
	o.bounds = bounds
	o.width = bounds.Dx()
	o.height = bounds.Dy()
	o.dirty = true
	o.mu.Unlock()

	if o.hwnd == 0 {
		return nil
	}

	runOnOverlayUI(func() {
		flags := uintptr(swpNoActivate)
		if o.visible {
			flags |= swpShowWindow
		}

		procSetWindowPos.Call(
			uintptr(o.hwnd),
			hwndTopMost,
			uintptr(bounds.Min.X),
			uintptr(bounds.Min.Y),
			uintptr(o.width),
			uintptr(o.height),
			flags,
		)

		if o.visible {
			o.requestPaintLocked()
		}
	})

	return nil
}

// Destroy releases native overlay resources.
func (o *OverlayWindow) Destroy() {
	if o == nil {
		return
	}

	runOnOverlayUI(func() {
		o.destroyHWNDLocked()
	})
}

func (o *OverlayWindow) localBounds() image.Rectangle {
	return image.Rect(0, 0, o.width, o.height)
}

// FillRect fills a rectangle with an ARGB color.
// Bounds are window-local coordinates (0,0 at the overlay top-left).
func (o *OverlayWindow) FillRect(bounds image.Rectangle, color uint32) {
	if o == nil || bounds.Empty() {
		return
	}

	rect := bounds.Intersect(o.localBounds())
	if rect.Empty() {
		return
	}

	o.mu.Lock()
	o.fills = append(o.fills, rectFill{rect: rect, color: color})
	o.dirty = true
	o.mu.Unlock()
}

// StrokeRect draws a rectangular border with the given ARGB color and width.
func (o *OverlayWindow) StrokeRect(bounds image.Rectangle, color uint32, lineWidth float64) {
	if o == nil || bounds.Empty() || lineWidth <= 0 {
		return
	}

	width := int(lineWidth)
	if width < 1 {
		width = 1
	}

	o.mu.Lock()
	o.strokes = append(o.strokes, rectStroke{rect: bounds, color: color, width: width})
	o.dirty = true
	o.mu.Unlock()
}

// DrawTextCentered renders centered text inside bounds using GDI.
func (o *OverlayWindow) DrawTextCentered(
	text string,
	bounds image.Rectangle,
	fontFamily string,
	fontSize float64,
	color uint32,
) {
	if o == nil || text == "" || bounds.Empty() {
		return
	}

	if fontFamily == "" {
		fontFamily = defaultOverlayFont
	}

	o.mu.Lock()
	o.texts = append(o.texts, textDraw{
		text:       text,
		rect:       bounds,
		fontFamily: fontFamily,
		fontSize:   fontSize,
		color:      color,
	})
	o.dirty = true
	o.mu.Unlock()
}

// Flush presents queued draw commands through UpdateLayeredWindow.
func (o *OverlayWindow) Flush() error {
	if o == nil {
		return fmt.Errorf("overlay window is nil")
	}

	var flushErr error

	runOnOverlayUI(func() {
		flushErr = o.presentLayeredLocked()
	})

	return flushErr
}

func (o *OverlayWindow) presentLayeredLocked() error {
	if o == nil {
		return fmt.Errorf("overlay window is nil")
	}

	if o.hwnd == 0 {
		if err := o.createHWNDLocked(); err != nil {
			return err
		}
	}

	if o.width <= 0 || o.height <= 0 {
		return fmt.Errorf("overlay has invalid dimensions %dx%d", o.width, o.height)
	}

	o.mu.Lock()
	fills := append([]rectFill(nil), o.fills...)
	strokes := append([]rectStroke(nil), o.strokes...)
	texts := append([]textDraw(nil), o.texts...)
	o.dirty = false
	o.mu.Unlock()

	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return fmt.Errorf("GetDC failed")
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(memDC)

	bitmap, _, _ := procCreateCompatibleBitmap.Call(
		screenDC,
		uintptr(o.width),
		uintptr(o.height),
	)
	if bitmap == 0 {
		return fmt.Errorf("CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(bitmap)

	oldBitmap, _, _ := procSelectObject.Call(memDC, bitmap)
	if oldBitmap == 0 {
		return fmt.Errorf("SelectObject failed")
	}
	defer procSelectObject.Call(memDC, oldBitmap)

	o.renderCommands(windows.Handle(memDC), fills, strokes, texts)

	dstPoint := winPoint{
		x: int32(o.bounds.Min.X),
		y: int32(o.bounds.Min.Y),
	}
	windowSize := winSize{
		cx: int32(o.width),
		cy: int32(o.height),
	}
	srcPoint := winPoint{x: 0, y: 0}

	ret, _, err := procUpdateLayeredWindow.Call(
		uintptr(o.hwnd),
		0,
		uintptr(unsafe.Pointer(&dstPoint)),
		uintptr(unsafe.Pointer(&windowSize)),
		memDC,
		uintptr(unsafe.Pointer(&srcPoint)),
		overlayColorKey,
		0,
		ulwColorKey,
	)
	if ret == 0 {
		return fmt.Errorf("UpdateLayeredWindow: %w", err)
	}

	return nil
}

func (o *OverlayWindow) requestPaintLocked() {
	if o == nil || o.hwnd == 0 {
		return
	}

	_ = o.presentLayeredLocked()
}

func (o *OverlayWindow) paintLocked(hdc windows.Handle) {
	if o == nil {
		return
	}

	o.mu.Lock()
	fills := append([]rectFill(nil), o.fills...)
	strokes := append([]rectStroke(nil), o.strokes...)
	texts := append([]textDraw(nil), o.texts...)
	o.dirty = false
	o.mu.Unlock()

	if o.paintDoubleBuffered(hdc, fills, strokes, texts) {
		return
	}

	o.renderCommands(hdc, fills, strokes, texts)
}

func (o *OverlayWindow) paintDoubleBuffered(
	hdc windows.Handle,
	fills []rectFill,
	strokes []rectStroke,
	texts []textDraw,
) bool {
	if o.width <= 0 || o.height <= 0 {
		return false
	}

	memDC, _, _ := procCreateCompatibleDC.Call(uintptr(hdc))
	if memDC == 0 {
		return false
	}
	defer procDeleteDC.Call(memDC)

	bitmap, _, _ := procCreateCompatibleBitmap.Call(
		uintptr(hdc),
		uintptr(o.width),
		uintptr(o.height),
	)
	if bitmap == 0 {
		return false
	}
	defer procDeleteObject.Call(bitmap)

	oldBitmap, _, _ := procSelectObject.Call(memDC, bitmap)
	if oldBitmap == 0 {
		return false
	}
	defer procSelectObject.Call(memDC, oldBitmap)

	o.renderCommands(windows.Handle(memDC), fills, strokes, texts)

	procBitBlt.Call(
		uintptr(hdc),
		0,
		0,
		uintptr(o.width),
		uintptr(o.height),
		memDC,
		0,
		0,
		srccopy,
	)

	return true
}

func (o *OverlayWindow) renderCommands(
	hdc windows.Handle,
	fills []rectFill,
	strokes []rectStroke,
	texts []textDraw,
) {
	client := windows.Rect{
		Left:   0,
		Top:    0,
		Right:  int32(o.width),
		Bottom: int32(o.height),
	}

	bgBrush, _, _ := procCreateSolidBrush.Call(overlayColorKey)
	if bgBrush != 0 {
		procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&client)), bgBrush)
		procDeleteObject.Call(bgBrush)
	}

	for _, fill := range fills {
		o.fillRectGDI(hdc, fill.rect, fill.color)
	}

	for _, stroke := range strokes {
		o.strokeRectGDI(hdc, stroke.rect, stroke.color, stroke.width)
	}

	for _, text := range texts {
		o.drawTextGDI(hdc, text)
	}
}

func (o *OverlayWindow) fillRectGDI(hdc windows.Handle, rect image.Rectangle, color uint32) {
	if rect.Empty() {
		return
	}

	brush, _, _ := procCreateSolidBrush.Call(uintptr(o.argbToGDI(color)))
	if brush == 0 {
		return
	}
	defer procDeleteObject.Call(brush)

	winRect := windows.Rect{
		Left:   int32(rect.Min.X),
		Top:    int32(rect.Min.Y),
		Right:  int32(rect.Max.X),
		Bottom: int32(rect.Max.Y),
	}
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&winRect)), brush)
}

func (o *OverlayWindow) strokeRectGDI(hdc windows.Handle, bounds image.Rectangle, color uint32, width int) {
	if bounds.Empty() || width < 1 {
		return
	}

	for i := 0; i < width; i++ {
		inset := bounds.Inset(i)
		o.fillRectGDI(hdc, image.Rect(inset.Min.X, inset.Min.Y, inset.Max.X, inset.Min.Y+1), color)
		o.fillRectGDI(hdc, image.Rect(inset.Min.X, inset.Max.Y-1, inset.Max.X, inset.Max.Y), color)
		o.fillRectGDI(hdc, image.Rect(inset.Min.X, inset.Min.Y, inset.Min.X+1, inset.Max.Y), color)
		o.fillRectGDI(hdc, image.Rect(inset.Max.X-1, inset.Min.Y, inset.Max.X, inset.Max.Y), color)
	}
}

func (o *OverlayWindow) drawTextGDI(hdc windows.Handle, text textDraw) {
	size := int(-text.fontSize)
	if size == 0 {
		size = -14
	}

	fontName, err := windows.UTF16PtrFromString(text.fontFamily)
	if err != nil {
		return
	}

	hFont, _, _ := procCreateFontW.Call(
		uintptr(size),
		0,
		0,
		0,
		fwBold,
		0,
		0,
		0,
		1,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(fontName)),
	)
	if hFont == 0 {
		return
	}
	defer procDeleteObject.Call(hFont)

	procSetBkMode.Call(uintptr(hdc), transparentBk)
	procSetTextColor.Call(uintptr(hdc), uintptr(o.argbToGDI(text.color)))

	utf16Text, err := windows.UTF16FromString(text.text)
	if err != nil {
		return
	}

	rect := windows.Rect{
		Left:   int32(text.rect.Min.X),
		Top:    int32(text.rect.Min.Y),
		Right:  int32(text.rect.Max.X),
		Bottom: int32(text.rect.Max.Y),
	}

	procDrawTextW.Call(
		uintptr(hdc),
		uintptr(unsafe.Pointer(&utf16Text[0])),
		uintptr(^uint32(0)),
		uintptr(unsafe.Pointer(&rect)),
		dtCenter|dtVCenter|dtSingleLine,
	)
}

func (o *OverlayWindow) argbToGDI(argb uint32) uint32 {
	blend := themeSurfaceLight
	if o != nil {
		o.mu.Lock()
		blend = o.colorBlendRGB
		o.mu.Unlock()
	}

	return argbToGDIColorRef(argb, blend)
}

func moduleHandle() uintptr {
	handle, _, _ := procGetModuleHandleW.Call(0)

	return handle
}

