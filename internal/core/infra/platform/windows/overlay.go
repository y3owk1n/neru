//go:build windows

package windows

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	overlayClassName = "NeruOverlayWindow"

	wsPopup          = 0x80000000
	wsExLayered      = 0x00080000
	wsExTransparent  = 0x00000020
	wsExTopmost      = 0x00000008
	wsExToolWindow   = 0x00000080
	wsExNoActivate   = 0x08000000
	swHide           = 0
	swShowNoActivate = 4
	hwndTopMost      = ^uintptr(0)
	swpNoActivate    = 0x0010
	swpShowWindow    = 0x0040

	defaultOverlayFont = "Segoe UI"
	fwBold             = 700
	dtCenter           = 0x00000001
	dtVCenter          = 0x00000004
	dtSingleLine       = 0x00000020
	transparentBk      = 1

	bmpV4Size     = 108
	biBitfields   = 3
	ulwAlpha      = 2
	bytesPerPixel = 4
	acSrcOver     = 0
	acSrcAlpha    = 1

	// DIB section bit depth for ARGB overlay rendering.
	dibBitCount = 32

	// Channel masks for BITMAPV4HEADER in BGRA pixel layout.
	maskRed   = 0x00FF0000
	maskGreen = 0x0000FF00
	maskBlue  = 0x000000FF
	maskAlpha = 0xFF000000

	// Windows sRGB color space identifier ('Win ').
	colorSpaceWinRGB = 0x206E6957

	// Half-pixel offset for SDF sample points to match the pixel-as-area model.
	pixelHalf = 0.5

	// ARGB compositing constants.
	alphaMax = 255

	// GDI text rendering defaults.
	defaultFontSize = 14
	gdiWhiteText    = 0x00FFFFFF
)

var (
	errInvalidOverlayBounds = errors.New("invalid overlay bounds")
	errOverlayNil           = errors.New("overlay is nil")
	errOverlayNotInit       = errors.New("overlay window is not initialized")
)

var (
	gdi32 = windows.NewLazySystemDLL("gdi32.dll")

	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procSetBkMode          = gdi32.NewProc("SetBkMode")
	procSetTextColor       = gdi32.NewProc("SetTextColor")
	procCreateFontW        = gdi32.NewProc("CreateFontW")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procIsWindow            = user32.NewProc("IsWindow")
	procUpdateLayeredWindow = user32.NewProc("UpdateLayeredWindow")

	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procRtlMoveMemory    = kernel32.NewProc("RtlMoveMemory")

	overlayClassOnce  sync.Once
	errOverlayClass   error
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

type rectFill struct {
	rect   image.Rectangle
	color  uint32
	radius float64
}

type rectStroke struct {
	rect   image.Rectangle
	color  uint32
	width  int
	radius float64
}

type textDraw struct {
	text       string
	rect       image.Rectangle
	fontFamily string
	fontSize   float64
	color      uint32
}

type bitmapV4Header struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
	RedMask       uint32
	GreenMask     uint32
	BlueMask      uint32
	AlphaMask     uint32
	CSType        uint32
	Endpoints     [9]uint32
	GammaRed      uint32
	GammaGreen    uint32
	GammaBlue     uint32
}

type blendFunction struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

type point struct {
	X, Y int32
}

type size struct {
	CX, CY int32
}

// OverlayWindow is a fullscreen click-through layered HWND with per-pixel alpha.
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

	pixels []byte
}

func overlayWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)

	return ret
}

func registerOverlayWindowClass() error {
	overlayClassOnce.Do(func() {
		className, err := windows.UTF16PtrFromString(overlayClassName)
		if err != nil {
			errOverlayClass = err

			return
		}

		overlayWndProcPtr = syscall.NewCallback(overlayWndProc)
		instance, _, _ := procGetModuleHandleW.Call(0)

		class := wndClassEx{
			cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
			style:         0,
			lpfnWndProc:   overlayWndProcPtr,
			hInstance:     windows.Handle(instance),
			lpszClassName: className,
		}

		atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
		if atom == 0 {
			errOverlayClass = fmt.Errorf("RegisterClassExW: %w", err)
		}
	})

	return errOverlayClass
}

// NewOverlayWindow creates a layered overlay sized to the active monitor.
func NewOverlayWindow() (*OverlayWindow, error) {
	err := registerOverlayWindowClass()
	if err != nil {
		return nil, err
	}

	bounds, err := activeScreenBounds()
	if err != nil {
		return nil, err
	}

	overlay := &OverlayWindow{
		bounds: bounds,
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

// NewOverlayWindowAt creates a layered overlay at the given screen position and
// size. Used for small transient windows like the mode indicator badge.
func NewOverlayWindowAt(posX, posY, width, height int) (*OverlayWindow, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("%w: %dx%d", errInvalidOverlayBounds, width, height)
	}

	err := registerOverlayWindowClass()
	if err != nil {
		return nil, err
	}

	overlay := &OverlayWindow{
		bounds: image.Rect(posX, posY, posX+width, posY+height),
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

// SetColorBlendRGB is a no-op since the overlay now uses per-pixel alpha.
func (o *OverlayWindow) SetColorBlendRGB(uint32) {}

// Show displays the overlay without taking focus.
func (o *OverlayWindow) Show() {
	if o == nil {
		return
	}

	runOnOverlayUI(func() {
		if o.hwnd == 0 {
			err := o.createHWNDLocked()
			if err != nil {
				return
			}
		}

		discardCall(procShowWindow.Call(uintptr(o.hwnd), swShowNoActivate))

		const (
			swpNomove = 0x0002
			swpNosize = 0x0001
		)

		discardCall(procSetWindowPos.Call(
			uintptr(o.hwnd),
			hwndTopMost,
			0,
			0,
			0,
			0,
			swpNoActivate|swpShowWindow|swpNomove|swpNosize,
		))
		o.visible = true

		// Force a flush on show so content is visible immediately.
		o.flushPixels()
	})
}

// Hide hides the overlay window without taking focus.
func (o *OverlayWindow) Hide() {
	if o == nil || o.hwnd == 0 {
		return
	}

	runOnOverlayUI(func() {
		discardCall(procShowWindow.Call(uintptr(o.hwnd), swHide))
		o.visible = false
	})
}

// Clear resets queued draw commands and clears the pixel buffer.
func (o *OverlayWindow) Clear() {
	if o == nil {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	o.fills = o.fills[:0]
	o.strokes = o.strokes[:0]
	o.texts = o.texts[:0]
	// Clear pixel buffer to transparent black (all zeros).
	for i := range o.pixels {
		o.pixels[i] = 0
	}

	o.dirty = true
}

// ResizeToActiveScreen moves and resizes the overlay to the active monitor.
func (o *OverlayWindow) ResizeToActiveScreen() error {
	if o == nil {
		return errOverlayNil
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
		o.mu.Lock()
		o.pixels = make([]byte, o.width*o.height*bytesPerPixel)
		o.mu.Unlock()

		flags := uintptr(swpNoActivate)
		if o.visible {
			flags |= swpShowWindow
		}

		discardCall(procSetWindowPos.Call(
			uintptr(o.hwnd),
			hwndTopMost,
			uintptr(bounds.Min.X),
			uintptr(bounds.Min.Y),
			uintptr(o.width),
			uintptr(o.height),
			flags,
		))
	})

	return nil
}

// ResizeTo repositions and resizes the overlay window to the given screen
// coordinates and dimensions, reallocating the pixel buffer as needed.
func (o *OverlayWindow) ResizeTo(posX, posY, width, height int) error {
	if o == nil {
		return errOverlayNil
	}

	if width <= 0 || height <= 0 {
		return fmt.Errorf("%w: %dx%d", errInvalidOverlayBounds, width, height)
	}

	o.mu.Lock()
	o.bounds = image.Rect(posX, posY, posX+width, posY+height)
	o.width = width
	o.height = height
	o.dirty = true
	o.mu.Unlock()

	if o.hwnd == 0 {
		return nil
	}

	runOnOverlayUI(func() {
		o.mu.Lock()
		o.pixels = make([]byte, o.width*o.height*bytesPerPixel)
		o.mu.Unlock()

		flags := uintptr(swpNoActivate)
		if o.visible {
			flags |= swpShowWindow
		}

		discardCall(procSetWindowPos.Call(
			uintptr(o.hwnd),
			hwndTopMost,
			uintptr(posX),
			uintptr(posY),
			uintptr(width),
			uintptr(height),
			flags,
		))
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

// FillRect fills a rectangle with an ARGB color (uses per-pixel alpha).
func (o *OverlayWindow) FillRect(bounds image.Rectangle, color uint32) {
	if o == nil || bounds.Empty() {
		return
	}

	rect := bounds.Intersect(o.localBounds())
	if rect.Empty() {
		return
	}

	o.mu.Lock()
	o.fills = append(o.fills, rectFill{rect: rect, color: color, radius: 0})
	o.dirty = true
	o.mu.Unlock()
}

// StrokeRect draws a rectangular border with the given ARGB color and width.
func (o *OverlayWindow) StrokeRect(bounds image.Rectangle, color uint32, lineWidth float64) {
	if o == nil || bounds.Empty() || lineWidth <= 0 {
		return
	}

	width := max(int(lineWidth), 1)

	o.mu.Lock()
	o.strokes = append(o.strokes, rectStroke{rect: bounds, color: color, width: width, radius: 0})
	o.dirty = true
	o.mu.Unlock()
}

// FillRoundedRect fills a rounded rectangle with an ARGB color using
// signed-distance-function anti-aliasing.
func (o *OverlayWindow) FillRoundedRect(bounds image.Rectangle, radius float64, color uint32) {
	if o == nil || bounds.Empty() || radius <= 0 {
		o.FillRect(bounds, color)

		return
	}

	rect := bounds.Intersect(o.localBounds())
	if rect.Empty() {
		return
	}

	o.mu.Lock()
	o.fills = append(o.fills, rectFill{rect: rect, color: color, radius: radius})
	o.dirty = true
	o.mu.Unlock()
}

// StrokeRoundedRect draws a rounded rectangular border with the given ARGB
// color, width, and corner radius, using signed-distance-function anti-aliasing.
func (o *OverlayWindow) StrokeRoundedRect(
	bounds image.Rectangle,
	radius float64,
	color uint32,
	lineWidth float64,
) {
	if o == nil || bounds.Empty() || lineWidth <= 0 || radius <= 0 {
		o.StrokeRect(bounds, color, lineWidth)

		return
	}

	width := max(int(lineWidth), 1)

	o.mu.Lock()
	o.strokes = append(
		o.strokes,
		rectStroke{rect: bounds, color: color, width: width, radius: radius},
	)
	o.dirty = true
	o.mu.Unlock()
}

// DrawTextCentered renders centered text inside bounds using GDI onto the
// pixel buffer with alpha compositing.
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

// CompositeCurrent composites all queued draw commands into the pixel buffer
// in-place and clears the command queues. Use this when you need to render
// each object as an atomic unit (e.g. per-hint) so that later objects render
// fully on top of earlier ones, avoiding fill/stroke/text batching across
// objects. Call Flush() at the end to present the final buffer.
func (o *OverlayWindow) CompositeCurrent() {
	if o == nil {
		return
	}

	o.mu.Lock()
	fills := append([]rectFill(nil), o.fills...)
	strokes := append([]rectStroke(nil), o.strokes...)
	texts := append([]textDraw(nil), o.texts...)
	o.fills = o.fills[:0]
	o.strokes = o.strokes[:0]
	o.texts = o.texts[:0]
	o.mu.Unlock()

	for _, f := range fills {
		if f.radius > 0 {
			alphaFillRoundedRect(o.pixels, o.width, o.height, f.rect, f.radius, f.color)
		} else {
			alphaFillRect(o.pixels, o.width, o.height, f.rect, f.color)
		}
	}

	for _, s := range strokes {
		if s.radius > 0 {
			alphaStrokeRoundedRect(o.pixels, o.width, o.height, s.rect, s.radius, s.color, s.width)
		} else {
			alphaStrokeRect(o.pixels, o.width, o.height, s.rect, s.color, s.width)
		}
	}

	for _, t := range texts {
		renderTextAlphaInto(o.pixels, o.width, o.height, t)
	}
}

// Flush composites all queued draw commands into the pixel buffer and presents
// them via UpdateLayeredWindow.
func (o *OverlayWindow) Flush() error {
	if o == nil || o.hwnd == 0 {
		return errOverlayNotInit
	}

	o.mu.Lock()

	dirty := o.dirty
	if !dirty {
		o.mu.Unlock()

		return nil
	}

	o.dirty = false

	fills := append([]rectFill(nil), o.fills...)
	strokes := append([]rectStroke(nil), o.strokes...)
	texts := append([]textDraw(nil), o.texts...)

	o.fills = o.fills[:0]
	o.strokes = o.strokes[:0]
	o.texts = o.texts[:0]
	// Copy pixels under the lock so compositing outside the lock is safe.
	pixels := make([]byte, len(o.pixels))
	copy(pixels, o.pixels)
	o.mu.Unlock()

	// Render fills with alpha compositing.
	for _, f := range fills {
		if f.radius > 0 {
			alphaFillRoundedRect(pixels, o.width, o.height, f.rect, f.radius, f.color)
		} else {
			alphaFillRect(pixels, o.width, o.height, f.rect, f.color)
		}
	}

	// Render strokes with alpha compositing.
	for _, s := range strokes {
		if s.radius > 0 {
			alphaStrokeRoundedRect(pixels, o.width, o.height, s.rect, s.radius, s.color, s.width)
		} else {
			alphaStrokeRect(pixels, o.width, o.height, s.rect, s.color, s.width)
		}
	}

	// Render text via GDI onto a temporary bitmap, then composite.
	for _, t := range texts {
		renderTextAlphaInto(pixels, o.width, o.height, t)
	}

	runOnOverlayUI(func() {
		// Swap rendered pixels under the lock.
		o.mu.Lock()
		o.pixels = pixels
		o.mu.Unlock()
		o.flushPixels()
	})

	return nil
}

func (o *OverlayWindow) createHWNDLocked() error {
	className, err := windows.UTF16PtrFromString(overlayClassName)
	if err != nil {
		return err
	}

	width := o.bounds.Dx()

	height := o.bounds.Dy()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("%w: %v", errInvalidOverlayBounds, o.bounds)
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
	o.pixels = make([]byte, width*height*bytesPerPixel)
	overlayRegistry.Store(o.hwnd, o)

	const (
		swpNomove = 0x0002
		swpNosize = 0x0001
	)

	discardCall(procSetWindowPos.Call(
		hwnd,
		hwndTopMost,
		0,
		0,
		0,
		0,
		swpNoActivate|swpNomove|swpNosize,
	))
	discardCall(procShowWindow.Call(hwnd, swHide))

	o.visible = false

	return nil
}

func (o *OverlayWindow) destroyHWNDLocked() {
	if o.hwnd != 0 {
		overlayRegistry.Delete(o.hwnd)
		discardCall(procDestroyWindow.Call(uintptr(o.hwnd)))
		o.hwnd = 0
		o.pixels = nil
	}
}

func (o *OverlayWindow) localBounds() image.Rectangle {
	return image.Rect(0, 0, o.width, o.height)
}

func (o *OverlayWindow) flushPixels() {
	if o == nil || o.hwnd == 0 {
		return
	}

	hdcMem, _, _ := procCreateCompatibleDC.Call(0)

	if hdcMem == 0 {
		return
	}
	defer func() { discardCall(procDeleteDC.Call(hdcMem)) }()

	bih := bitmapV4Header{
		Size:        bmpV4Size,
		Width:       int32(o.width),
		Height:      -int32(o.height), // negative = top-down bitmap
		Planes:      1,
		BitCount:    dibBitCount,
		Compression: biBitfields,
		SizeImage:   uint32(o.width * o.height * bytesPerPixel),
		RedMask:     maskRed,
		GreenMask:   maskGreen,
		BlueMask:    maskBlue,
		AlphaMask:   maskAlpha,
		CSType:      colorSpaceWinRGB,
	}

	var bits unsafe.Pointer

	hDib, _, _ := procCreateDIBSection.Call(
		hdcMem,
		uintptr(unsafe.Pointer(&bih)),
		0, // DIB_RGB_COLORS
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)

	if hDib == 0 {
		return
	}
	defer func() { discardCall(procDeleteObject.Call(hDib)) }()

	// Copy pre-multiplied pixel data into the DIBSection.
	discardCall(procRtlMoveMemory.Call(
		uintptr(bits),
		uintptr(unsafe.Pointer(&o.pixels[0])),
		uintptr(len(o.pixels)),
	))

	prevObj, _, _ := procSelectObject.Call(hdcMem, hDib)

	if prevObj == 0 {
		return
	}
	defer func() { discardCall(procSelectObject.Call(hdcMem, prevObj)) }()

	blend := blendFunction{
		BlendOp:             acSrcOver,
		AlphaFormat:         acSrcAlpha,
		SourceConstantAlpha: alphaMax,
	}

	discardCall(procUpdateLayeredWindow.Call(
		uintptr(o.hwnd),
		hdcMem,
		uintptr(unsafe.Pointer(&point{X: int32(o.bounds.Min.X), Y: int32(o.bounds.Min.Y)})),
		uintptr(unsafe.Pointer(&size{CX: int32(o.width), CY: int32(o.height)})),
		hdcMem,
		uintptr(unsafe.Pointer(&point{})),
		0,
		uintptr(unsafe.Pointer(&blend)),
		ulwAlpha,
	))
}

func renderTextAlphaInto(pixels []byte, bufW, bufH int, textCmd textDraw) {
	// Clamp text rect to overlay bounds.
	textRect := textCmd.rect.Intersect(image.Rect(0, 0, bufW, bufH))
	if textRect.Empty() {
		return
	}

	// Add 1px padding around text rect for anti-aliasing at edges.
	pad := 1
	texW := textRect.Dx() + pad*2 //nolint:mnd
	texH := textRect.Dy() + pad*2 //nolint:mnd
	textX := textRect.Min.X - pad
	textY := textRect.Min.Y - pad

	// Clamp to overlay.
	if textX < 0 {
		texW += textX
		textX = 0
	}

	if textY < 0 {
		texH += textY
		textY = 0
	}

	if textX+texW > bufW {
		texW = bufW - textX
	}

	if textY+texH > bufH {
		texH = bufH - textY
	}

	if texW <= 0 || texH <= 0 {
		return
	}

	hdcMem, _, _ := procCreateCompatibleDC.Call(0)

	if hdcMem == 0 {
		return
	}
	defer func() { discardCall(procDeleteDC.Call(hdcMem)) }()

	bih := bitmapV4Header{
		Size:        bmpV4Size,
		Width:       int32(texW),
		Height:      -int32(texH),
		Planes:      1,
		BitCount:    dibBitCount,
		Compression: biBitfields,
		SizeImage:   uint32(texW * texH * bytesPerPixel),
		RedMask:     maskRed,
		GreenMask:   maskGreen,
		BlueMask:    maskBlue,
		AlphaMask:   maskAlpha,
		CSType:      colorSpaceWinRGB,
	}

	var bits unsafe.Pointer

	hDib, _, _ := procCreateDIBSection.Call(
		hdcMem,
		uintptr(unsafe.Pointer(&bih)),
		0,
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)

	if hDib == 0 {
		return
	}
	defer func() { discardCall(procDeleteObject.Call(hDib)) }()

	prevObj, _, _ := procSelectObject.Call(hdcMem, hDib)

	if prevObj == 0 {
		return
	}
	defer func() { discardCall(procSelectObject.Call(hdcMem, prevObj)) }()

	tmpPixels := (*[1 << 30]byte)(bits)[: texW*texH*bytesPerPixel : texW*texH*bytesPerPixel]
	for i := range tmpPixels {
		tmpPixels[i] = 0
	}

	size := int(-textCmd.fontSize)
	if size == 0 {
		size = -defaultFontSize
	}

	fontName, err := windows.UTF16PtrFromString(textCmd.fontFamily)
	if err != nil {
		return
	}

	hFont, _, _ := procCreateFontW.Call(
		uintptr(size), 0, 0, 0, fwBold, 0, 0, 0, 1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(fontName)),
	)

	if hFont == 0 {
		return
	}
	defer func() { discardCall(procDeleteObject.Call(hFont)) }()

	prevFont, _, _ := procSelectObject.Call(hdcMem, hFont)

	if prevFont == 0 {
		return
	}
	defer func() { discardCall(procSelectObject.Call(hdcMem, prevFont)) }()

	discardCall(procSetBkMode.Call(hdcMem, transparentBk))
	discardCall(procSetTextColor.Call(hdcMem, gdiWhiteText))

	utf16Text, err := windows.UTF16FromString(textCmd.text)
	if err != nil {
		return
	}

	// Draw text centered inside the text bounding box, offset by padding.
	drawRect := windows.Rect{
		Left:   int32(pad),
		Top:    int32(pad),
		Right:  int32(pad + textRect.Dx()),
		Bottom: int32(pad + textRect.Dy()),
	}

	procDrawTextW := user32.NewProc("DrawTextW")
	discardCall(procDrawTextW.Call(
		hdcMem,
		uintptr(unsafe.Pointer(&utf16Text[0])),
		uintptr(^uint32(0)),
		uintptr(unsafe.Pointer(&drawRect)),
		dtCenter|dtVCenter|dtSingleLine,
	))

	// Composite the small text bitmap into the main pixel buffer at the correct offset.
	alphaCompositeTextAt(pixels, bufW, bufH, tmpPixels, texW, texH, textX, textY, textCmd.color)
}

// alphaFillRect composites a semi-transparent ARGB fill over the pixel buffer.
func alphaFillRect(pixels []byte, bufW, bufH int, rect image.Rectangle, color uint32) {
	colA := color >> alphaShift
	if colA == 0 {
		return
	}

	colR := (color >> redShift) & byteMask
	colG := (color >> greenShift) & byteMask
	colB := color & byteMask

	// Pre-multiply the source color by its alpha.
	srcR := colR * colA
	srcG := colG * colA
	srcB := colB * colA

	invA := alphaMax - colA
	startY := clamp(rect.Min.Y, bufH)
	endY := clamp(rect.Max.Y, bufH)
	startX := clamp(rect.Min.X, bufW)
	endX := clamp(rect.Max.X, bufW)

	for y := startY; y < endY; y++ {
		row := y * bufW * bytesPerPixel
		for x := startX; x < endX; x++ {
			idx := row + x*bytesPerPixel
			dstB := uint32(pixels[idx])
			dstG := uint32(pixels[idx+1])
			dstR := uint32(pixels[idx+2])
			dstA := uint32(pixels[idx+3])

			pixels[idx] = byte((srcB + dstB*invA) / alphaMax)
			pixels[idx+1] = byte((srcG + dstG*invA) / alphaMax)
			pixels[idx+2] = byte((srcR + dstR*invA) / alphaMax)
			pixels[idx+3] = byte(colA + (dstA*invA)/alphaMax)
		}
	}
}

// alphaStrokeRect composites a stroked rectangle border over the pixel buffer.
func alphaStrokeRect(
	pixels []byte,
	bufW, bufH int,
	rect image.Rectangle,
	color uint32,
	lineWidth int,
) {
	if lineWidth < 1 {
		return
	}

	for i := range lineWidth {
		inset := rect.Inset(i)
		// Top edge
		alphaFillRect(pixels, bufW, bufH,
			image.Rect(inset.Min.X, inset.Min.Y, inset.Max.X, inset.Min.Y+1), color)
		// Bottom edge
		alphaFillRect(pixels, bufW, bufH,
			image.Rect(inset.Min.X, inset.Max.Y-1, inset.Max.X, inset.Max.Y), color)
		// Left edge
		alphaFillRect(pixels, bufW, bufH,
			image.Rect(inset.Min.X, inset.Min.Y, inset.Min.X+1, inset.Max.Y), color)
		// Right edge
		alphaFillRect(pixels, bufW, bufH,
			image.Rect(inset.Max.X-1, inset.Min.Y, inset.Max.X, inset.Max.Y), color)
	}
}

// sdRoundedBox computes the signed distance from a point (px, py) to a rounded
// rectangle centered at the origin with half-extents (halfW, halfH) and corner
// radius r.  Negative inside, positive outside, zero at the boundary.
func sdRoundedBox(ptX, ptY, halfW, halfH, radius float64) float64 {
	distX := math.Abs(ptX) - halfW + radius
	distY := math.Abs(ptY) - halfH + radius

	insideX := math.Max(distX, 0)
	insideY := math.Max(distY, 0)
	outside := math.Sqrt(insideX*insideX+insideY*insideY) - radius

	inside := math.Min(math.Max(distX, distY), 0)

	return outside + inside
}

// alphaFillRoundedRect composites an anti-aliased rounded rectangle fill
// using signed-distance-function edge smoothing.
func alphaFillRoundedRect(
	pixels []byte,
	bufW, bufH int,
	rect image.Rectangle,
	radius float64,
	color uint32,
) {
	colA := color >> alphaShift
	if colA == 0 {
		return
	}

	colR := (color >> redShift) & byteMask
	colG := (color >> greenShift) & byteMask
	colB := color & byteMask

	halfW := float64(rect.Dx()) / 2.0 //nolint:mnd // simple arithmetic
	halfH := float64(rect.Dy()) / 2.0 //nolint:mnd // simple arithmetic
	centerX := float64(rect.Min.X) + halfW
	centerY := float64(rect.Min.Y) + halfH

	startY := clamp(rect.Min.Y, bufH)
	endY := clamp(rect.Max.Y, bufH)
	startX := clamp(rect.Min.X, bufW)
	endX := clamp(rect.Max.X, bufW)

	// Inner region is fully inside the rounded rect (no SDF needed).
	innerMinX := float64(rect.Min.X) + radius
	innerMaxX := float64(rect.Max.X) - radius
	innerMinY := float64(rect.Min.Y) + radius
	innerMaxY := float64(rect.Max.Y) - radius

	srcR := colR * colA
	srcG := colG * colA
	srcB := colB * colA

	for y := startY; y < endY; y++ {
		row := y * bufW * bytesPerPixel
		floatY := float64(y) + pixelHalf

		for col := startX; col < endX; col++ {
			floatX := float64(col) + pixelHalf

			// Fast path: pixel is well inside the rounded rect.
			if floatX >= innerMinX && floatX <= innerMaxX && floatY >= innerMinY &&
				floatY <= innerMaxY {
				idx := row + col*bytesPerPixel
				dstB := uint32(pixels[idx])
				dstG := uint32(pixels[idx+1])
				dstR := uint32(pixels[idx+2])
				dstA := uint32(pixels[idx+3])

				pixels[idx] = byte((srcB + dstB*(alphaMax-colA)) / alphaMax)
				pixels[idx+1] = byte((srcG + dstG*(alphaMax-colA)) / alphaMax)
				pixels[idx+2] = byte((srcR + dstR*(alphaMax-colA)) / alphaMax)
				pixels[idx+3] = byte(colA + (dstA*(alphaMax-colA))/alphaMax)

				continue
			}

			dist := sdRoundedBox(floatX-centerX, floatY-centerY, halfW, halfH, radius)
			if dist > 1 {
				continue
			}

			pixelAlpha := uint32(math.Max(0, math.Min(1, 1.0-dist)) * float64(colA))
			if pixelAlpha == 0 {
				continue
			}

			invA := alphaMax - pixelAlpha
			idx := row + col*bytesPerPixel
			dstB := uint32(pixels[idx])
			dstG := uint32(pixels[idx+1])
			dstR := uint32(pixels[idx+2])
			dstA := uint32(pixels[idx+3])

			pixels[idx] = byte((colB*pixelAlpha + dstB*invA) / alphaMax)
			pixels[idx+1] = byte((colG*pixelAlpha + dstG*invA) / alphaMax)
			pixels[idx+2] = byte((colR*pixelAlpha + dstR*invA) / alphaMax)
			pixels[idx+3] = byte(pixelAlpha + (dstA*invA)/alphaMax)
		}
	}
}

// alphaStrokeRoundedRect composites an anti-aliased rounded rectangle stroke
// using signed-distance-function edge smoothing at both outer and inner edges.
func alphaStrokeRoundedRect(
	pixels []byte,
	bufW, bufH int,
	rect image.Rectangle,
	radius float64,
	color uint32,
	lineWidth int,
) {
	if lineWidth < 1 {
		return
	}

	colA := color >> alphaShift
	if colA == 0 {
		return
	}

	colR := (color >> redShift) & byteMask
	colG := (color >> greenShift) & byteMask
	colB := color & byteMask

	halfW := float64(rect.Dx()) / 2.0 //nolint:mnd // simple arithmetic
	halfH := float64(rect.Dy()) / 2.0 //nolint:mnd // simple arithmetic
	centerX := float64(rect.Min.X) + halfW
	centerY := float64(rect.Min.Y) + halfH

	strokeW := float64(lineWidth)
	innerRadius := math.Max(radius-strokeW, 0)
	innerHalfW := math.Max(halfW-strokeW, 0)
	innerHalfH := math.Max(halfH-strokeW, 0)

	startY := clamp(rect.Min.Y, bufH)
	endY := clamp(rect.Max.Y, bufH)
	startX := clamp(rect.Min.X, bufW)
	endX := clamp(rect.Max.X, bufW)

	for y := startY; y < endY; y++ {
		row := y * bufW * bytesPerPixel
		for col := startX; col < endX; col++ {
			relX := float64(col) + pixelHalf - centerX
			relY := float64(y) + pixelHalf - centerY

			dOuter := sdRoundedBox(relX, relY, halfW, halfH, radius)
			if dOuter > 1 {
				continue
			}

			dInner := sdRoundedBox(relX, relY, innerHalfW, innerHalfH, innerRadius)
			if dInner < -1 {
				continue // inside inner hole, not part of stroke
			}

			outerAlpha := math.Max(0, math.Min(1, 1.0-dOuter))
			innerAlpha := math.Max(0, math.Min(1, 1.0-dInner))

			pixelAlpha := uint32(outerAlpha * (1.0 - innerAlpha) * float64(colA))
			if pixelAlpha == 0 {
				continue
			}

			invA := alphaMax - pixelAlpha
			srcR := colR * pixelAlpha
			srcG := colG * pixelAlpha
			srcB := colB * pixelAlpha

			idx := row + col*bytesPerPixel
			dstB := uint32(pixels[idx])
			dstG := uint32(pixels[idx+1])
			dstR := uint32(pixels[idx+2])
			dstA := uint32(pixels[idx+3])

			pixels[idx] = byte((srcB + dstB*invA) / alphaMax)
			pixels[idx+1] = byte((srcG + dstG*invA) / alphaMax)
			pixels[idx+2] = byte((srcR + dstR*invA) / alphaMax)
			pixels[idx+3] = byte(pixelAlpha + (dstA*invA)/alphaMax)
		}
	}
}

// alphaCompositeTextAt composites a text bitmap of size (tw x th) at position
// (offX, offY) in the main pixel buffer using the given ARGB color.
func alphaCompositeTextAt(
	pixels []byte,
	bufW, bufH int,
	textPixels []byte,
	texW, texH, offX, offY int,
	color uint32,
) {
	textA := (color >> alphaShift) & byteMask
	if textA == 0 {
		return
	}

	textR := (color >> redShift) & byteMask
	textG := (color >> greenShift) & byteMask
	textB := color & byteMask

	for texY := range texH {
		dstY := offY + texY
		if dstY < 0 || dstY >= bufH {
			continue
		}

		srcRow := texY * texW * bytesPerPixel
		dstRow := dstY * bufW * bytesPerPixel

		for texX := range texW {
			dstX := offX + texX
			if dstX < 0 || dstX >= bufW {
				continue
			}

			srcIdx := srcRow + texX*bytesPerPixel

			coverage := uint32(textPixels[srcIdx+2])
			if coverage == 0 {
				continue
			}

			srcA := coverage * textA / alphaMax
			srcR := textR * srcA
			srcG := textG * srcA
			srcB := textB * srcA
			invA := alphaMax - srcA

			dstIdx := dstRow + dstX*bytesPerPixel
			dstB := uint32(pixels[dstIdx])
			dstG := uint32(pixels[dstIdx+1])
			dstR := uint32(pixels[dstIdx+2])
			dstA := uint32(pixels[dstIdx+3])

			pixels[dstIdx] = byte((srcB + dstB*invA) / alphaMax)
			pixels[dstIdx+1] = byte((srcG + dstG*invA) / alphaMax)
			pixels[dstIdx+2] = byte((srcR + dstR*invA) / alphaMax)
			pixels[dstIdx+3] = byte(srcA + (dstA*invA)/alphaMax)
		}
	}
}

func clamp(val, maxVal int) int {
	if val < 0 {
		return 0
	}

	if val > maxVal {
		return maxVal
	}

	return val
}

func moduleHandle() uintptr {
	handle, _, _ := procGetModuleHandleW.Call(0)

	return handle
}

func discardCall(uintptr, uintptr, error) {}
