//go:build windows

package windows

import (
	"errors"
	"fmt"
	"image"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	overlayClassName = "NeruOverlayWindow"

	wsPopup                 = 0x80000000
	wsExLayered             = 0x00080000
	wsExTransparent         = 0x00000020
	wsExTopmost             = 0x00000008
	wsExToolWindow          = 0x00000080
	wsExNoActivate          = 0x08000000
	wsExNoRedirectionBitmap = 0x00200000
	swHide                  = 0
	swShowNoActivate        = 4
	hwndTopMost             = ^uintptr(0)
	swpNoActivate           = 0x0010
	swpShowWindow           = 0x0040
	swpNoMove               = 0x0002
	swpNoSize               = 0x0001

	wmNCHitTest   = 0x0084
	htTransparent = ^uintptr(0) // HTTRANSPARENT, LRESULT -1

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

	// ARGB compositing constants.
	alphaMax = 255

	// GDI text rendering defaults.
	defaultFontSize = 14
	gdiWhiteText    = 0x00FFFFFF

	// LCS_WINDOWS_COLOR_SPACE, 'Win ' as a little-endian FOURCC.
	colorSpaceWinRGB = 0x206E6957

	// pixelHalf is the offset from a pixel's corner to its center.
	pixelHalf = 0.5

	// Triangle rasterization. A filled triangle has no signed-distance form as
	// cheap as sdRoundedBox, so coverage is sampled on a grid inside each pixel
	// instead: triangleSamples x triangleSamples points, counted against the
	// three half-planes. The shape this draws is a hint connector arrow a few
	// pixels on a side, so the sample count costs nothing and the edges land as
	// smooth as the badge they hang off.
	triangleVertices = 3
	triangleSamples  = 4
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

	procRegisterClassExW            = user32.NewProc("RegisterClassExW")
	procCreateWindowExW             = user32.NewProc("CreateWindowExW")
	procDestroyWindow               = user32.NewProc("DestroyWindow")
	procShowWindow                  = user32.NewProc("ShowWindow")
	procSetWindowPos                = user32.NewProc("SetWindowPos")
	procDefWindowProcW              = user32.NewProc("DefWindowProcW")
	procIsWindow                    = user32.NewProc("IsWindow")
	procUpdateLayeredWindowIndirect = user32.NewProc("UpdateLayeredWindowIndirect")
	procDrawTextW                   = user32.NewProc("DrawTextW")

	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

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

// drawKind says which primitive a drawCmd is.
type drawKind uint8

const (
	drawFill drawKind = iota
	drawRoundedFill
	drawStroke
	drawRoundedStroke
	drawTriangle
	drawText
)

// drawCmd is one queued primitive. Commands are kept in the order they were
// queued and painted in that order, so a later shape always lands over an
// earlier one — the ordering every other backend has, and the one a hint
// badge drawn over its neighbor relies on.
type drawCmd struct {
	kind     drawKind
	rect     image.Rectangle
	color    uint32
	radius   float64
	width    int
	vertices [triangleVertices]image.Point
	text     string
	font     string
	fontSize float64
}

// bounds is the rectangle a command can touch, before clipping to the surface.
func (c drawCmd) bounds() image.Rectangle {
	if c.kind != drawTriangle {
		// A text command paints one pixel of anti-aliasing padding around
		// its rectangle (gdiTextRenderer.paint); the same inset serves every
		// rectangular command.
		return c.rect.Inset(-1)
	}

	rect := image.Rectangle{Min: c.vertices[0], Max: c.vertices[0]}
	for _, vertex := range c.vertices {
		rect.Min.X = min(rect.Min.X, vertex.X)
		rect.Min.Y = min(rect.Min.Y, vertex.Y)
		rect.Max.X = max(rect.Max.X, vertex.X+1)
		rect.Max.Y = max(rect.Max.Y, vertex.Y+1)
	}

	return rect
}

// frame is what Flush hands the UI thread: the commands queued since the last
// Flush, and whether the surface is to be cleared before they are painted.
// Frames coalesce — a Flush that finds one still waiting appends to it, and a
// Clear folds every waiting command away — so a burst of keystrokes presents
// once, with the newest state.
type frame struct {
	clear bool
	cmds  []drawCmd
}

// FrameStats describes one presented frame: what it cost and where. Counts,
// durations and rectangles only; nothing about what was drawn.
type FrameStats struct {
	// Backend names the surface that presented: "direct2d" or "gdi".
	Backend string
	// Commands is how many primitives the frame painted.
	Commands int
	// Dirty is the surface rectangle the frame changed.
	Dirty image.Rectangle
	// Raster is how long painting the commands took.
	Raster time.Duration
	// Present is how long handing the pixels to the compositor took.
	Present time.Duration
	// Err is why the frame was not presented, when it was not: the surface
	// failed and could not be rebuilt on GDI. Flush returns before the paint,
	// so this is the only place that failure surfaces.
	Err error
}

// overlaySurface is what actually holds pixels for a window. The window owns
// the command queue and the HWND; the surface owns whatever the pixels live in
// — a DIB the layered window is updated from, or a DirectComposition swapchain
// — and paints one frame at a time. Every method runs on the overlay UI thread.
type overlaySurface interface {
	// name is the value FrameStats.Backend carries.
	name() string
	// render paints a frame and presents it. It returns an error only when
	// the surface itself is broken (a lost device), never for a bad command.
	render(f *frame) (FrameStats, error)
	// resize replaces the pixel store for a new window size.
	resize(width, height int) error
	// destroy releases the native resources.
	destroy()
}

// OverlayWindow is a fullscreen click-through HWND with per-pixel alpha.
//
// Drawing is a queue of commands. Flush hands the queue to the overlay UI
// thread and returns at once; the thread paints and presents when it gets to
// it, and coalesces frames that pile up. Nothing on the caller's thread — the
// keyboard hook's, on a keystroke — waits for pixels.
type OverlayWindow struct {
	mu      sync.Mutex
	hwnd    windows.HWND
	bounds  image.Rectangle
	width   int
	height  int
	visible bool
	dirty   bool

	cmds         []drawCmd
	clearPending bool
	pending      *frame
	renderQueued bool

	// surface is touched only on the overlay UI thread.
	surface overlaySurface
	// noDComp is set once DirectComposition failed for this window, so a
	// rebuild does not try it again.
	noDComp bool

	observer func(FrameStats)
}

func overlayWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	// Click-through. WS_EX_TRANSPARENT makes a layered window pass the mouse
	// on; a DirectComposition window is not layered, so answer the hit test
	// directly and both kinds behave the same.
	if msg == wmNCHitTest {
		return htTransparent
	}

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

// NewOverlayWindow creates an overlay sized to the active monitor.
func NewOverlayWindow() (*OverlayWindow, error) {
	err := registerOverlayWindowClass()
	if err != nil {
		return nil, err
	}

	bounds, err := activeScreenBounds()
	if err != nil {
		return nil, err
	}

	return newOverlayWindowWithBounds(bounds)
}

// NewOverlayWindowAt creates an overlay at the given screen position and
// size. Used for small transient windows like the mode indicator badge.
func NewOverlayWindowAt(posX, posY, width, height int) (*OverlayWindow, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("%w: %dx%d", errInvalidOverlayBounds, width, height)
	}

	err := registerOverlayWindowClass()
	if err != nil {
		return nil, err
	}

	return newOverlayWindowWithBounds(image.Rect(posX, posY, posX+width, posY+height))
}

func newOverlayWindowWithBounds(bounds image.Rectangle) (*OverlayWindow, error) {
	overlay := &OverlayWindow{bounds: bounds}

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
	return o.handle()
}

// Healthy reports whether the overlay window is initialized and still valid.
func (o *OverlayWindow) Healthy() bool {
	hwnd := o.handle()
	if hwnd == 0 {
		return false
	}

	ret, _, _ := procIsWindow.Call(uintptr(hwnd))

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

// Backend names the surface this window presents through: "direct2d" when
// DirectComposition came up, "gdi" otherwise, and "" before the window exists.
func (o *OverlayWindow) Backend() string {
	if o == nil {
		return ""
	}

	var name string

	runOnOverlayUI(func() {
		if o.surface != nil {
			name = o.surface.name()
		}
	})

	return name
}

// SetFrameObserver registers a callback that receives the statistics of every
// presented frame. It is called on the overlay UI thread, so it must not block
// and must not call back into this window.
func (o *OverlayWindow) SetFrameObserver(observer func(FrameStats)) {
	if o == nil {
		return
	}

	o.mu.Lock()
	o.observer = observer
	o.mu.Unlock()
}

// SetColorBlendRGB is a no-op since the overlay uses per-pixel alpha.
func (o *OverlayWindow) SetColorBlendRGB(uint32) {}

// Show displays the overlay without taking focus.
//
// A frame still waiting is presented first, so the window comes up showing
// what was last drawn rather than the frame before it.
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

		o.renderPending()

		discardCall(procShowWindow.Call(uintptr(o.hwnd), swShowNoActivate))
		discardCall(procSetWindowPos.Call(
			uintptr(o.hwnd),
			hwndTopMost,
			0,
			0,
			0,
			0,
			swpNoActivate|swpShowWindow|swpNoMove|swpNoSize,
		))

		o.mu.Lock()
		o.visible = true
		o.mu.Unlock()
	})
}

// Hide hides the overlay window without taking focus.
func (o *OverlayWindow) Hide() {
	if o.handle() == 0 {
		return
	}

	runOnOverlayUI(func() {
		discardCall(procShowWindow.Call(uintptr(o.hwnd), swHide))

		o.mu.Lock()
		o.visible = false
		o.mu.Unlock()
	})
}

// Clear drops every queued command and marks the surface to be emptied when
// the next frame is presented.
func (o *OverlayWindow) Clear() {
	if o == nil {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	o.cmds = o.cmds[:0]
	o.clearPending = true
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

	return o.ResizeTo(bounds.Min.X, bounds.Min.Y, bounds.Dx(), bounds.Dy())
}

// ResizeTo repositions and resizes the overlay window to the given screen
// coordinates and dimensions, replacing the pixel store as needed.
func (o *OverlayWindow) ResizeTo(posX, posY, width, height int) error {
	if o == nil {
		return errOverlayNil
	}

	if width <= 0 || height <= 0 {
		return fmt.Errorf("%w: %dx%d", errInvalidOverlayBounds, width, height)
	}

	o.mu.Lock()
	sameSize := o.width == width && o.height == height
	o.bounds = image.Rect(posX, posY, posX+width, posY+height)
	o.width = width
	o.height = height
	o.dirty = true
	o.mu.Unlock()

	if o.handle() == 0 {
		return nil
	}

	var resizeErr error

	runOnOverlayUI(func() {
		flags := uintptr(swpNoActivate)
		if o.Visible() {
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

		// The indicator badges are moved to the cursor every tick at the size
		// they already have; that is a move, and the pixel store stays.
		if o.surface != nil && !sameSize {
			resizeErr = o.surface.resize(width, height)
		}
	})

	return resizeErr
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

	o.queue(drawCmd{kind: drawFill, rect: rect, color: color})
}

// StrokeRect draws a rectangular border with the given ARGB color and width.
func (o *OverlayWindow) StrokeRect(bounds image.Rectangle, color uint32, lineWidth float64) {
	if o == nil || bounds.Empty() || lineWidth <= 0 {
		return
	}

	o.queue(drawCmd{kind: drawStroke, rect: bounds, color: color, width: max(int(lineWidth), 1)})
}

// FillRoundedRect fills a rounded rectangle with an ARGB color.
//
// A rectangle hanging off the surface is queued whole, unlike the square fill
// above: the shape is derived from the rectangle's own center and
// half-extents, so handing it a clipped rectangle would round the corners of
// the *visible* part instead — a badge at the screen edge would be redrawn as
// a smaller, differently rounded badge, while its stroke and its label (which
// are not clipped here) stayed where the real one was. The surface clips its
// own painting to the buffer, so nothing is drawn out of bounds either way.
func (o *OverlayWindow) FillRoundedRect(bounds image.Rectangle, radius float64, color uint32) {
	if o == nil || bounds.Empty() || radius <= 0 {
		o.FillRect(bounds, color)

		return
	}

	if bounds.Intersect(o.localBounds()).Empty() {
		return
	}

	o.queue(drawCmd{kind: drawRoundedFill, rect: bounds, color: color, radius: radius})
}

// StrokeRoundedRect draws a rounded rectangular border with the given ARGB
// color, width, and corner radius.
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

	o.queue(drawCmd{
		kind:   drawRoundedStroke,
		rect:   bounds,
		color:  color,
		radius: radius,
		width:  max(int(lineWidth), 1),
	})
}

// FillTriangle fills the triangle through the three points with an ARGB color,
// anti-aliased.
//
// It is painted in queue order, so a triangle drawn to tie a badge back to its
// target lands over the badge's own fill; the badge's stroke, queued after it,
// closes the shared edge.
func (o *OverlayWindow) FillTriangle(vertexA, vertexB, vertexC image.Point, color uint32) {
	if o == nil {
		return
	}

	o.queue(drawCmd{
		kind:     drawTriangle,
		vertices: [triangleVertices]image.Point{vertexA, vertexB, vertexC},
		color:    color,
	})
}

// DrawTextCentered renders centered text inside bounds.
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

	o.queue(drawCmd{
		kind:     drawText,
		rect:     bounds,
		color:    color,
		text:     text,
		font:     fontFamily,
		fontSize: fontSize,
	})
}

// pointerGlyphDefault is the glyph a pointer stand-in draws when the
// configured char is empty.
const pointerGlyphDefault = "●"

// DrawPointerGlyph queues the pointer stand-in a grid mode draws at its
// selection: one glyph centered on center in a box of the given size. Both
// grid and recursive grid draw theirs through here, so the two cannot drift.
//
// The box's half is floored at 1 so a pointer configured to size 0 or 1 still
// has a rectangle to draw its glyph in rather than an empty one DrawTextCentered
// would drop.
func (o *OverlayWindow) DrawPointerGlyph(
	center image.Point,
	size int,
	char string,
	fontFamily string,
	color uint32,
) {
	if char == "" {
		char = pointerGlyphDefault
	}

	halfSize := max(size/2, 1) //nolint:mnd

	o.DrawTextCentered(
		char,
		image.Rect(center.X-halfSize, center.Y-halfSize, center.X+halfSize, center.Y+halfSize),
		fontFamily,
		float64(size),
		color,
	)
}

// Flush hands the queued commands to the overlay UI thread and returns. The
// thread paints and presents them when it gets to them; a Flush that arrives
// while an earlier frame is still waiting joins that frame, so the window is
// never more than one paint behind the newest state and the caller never
// waits for one.
func (o *OverlayWindow) Flush() error {
	if o.handle() == 0 {
		return errOverlayNotInit
	}

	o.mu.Lock()

	if !o.dirty {
		o.mu.Unlock()

		return nil
	}

	o.dirty = false

	if o.pending == nil {
		o.pending = &frame{}
	}

	if o.clearPending {
		o.pending.clear = true
		o.pending.cmds = o.pending.cmds[:0]
		o.clearPending = false
	}

	o.pending.cmds = append(o.pending.cmds, o.cmds...)
	o.cmds = o.cmds[:0]

	enqueue := !o.renderQueued
	o.renderQueued = true
	o.mu.Unlock()

	if enqueue {
		postOnOverlayUI(o.renderPending)
	}

	return nil
}

// handle reads the window handle under the lock: a rebuild on the UI thread
// (rebuildOnGDI) replaces it mid-life, and every caller off that thread
// reads it through here.
func (o *OverlayWindow) handle() windows.HWND {
	if o == nil {
		return 0
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	return o.hwnd
}

func (o *OverlayWindow) setHandle(hwnd windows.HWND) {
	o.mu.Lock()
	o.hwnd = hwnd
	o.mu.Unlock()
}

func (o *OverlayWindow) queue(cmd drawCmd) {
	o.mu.Lock()
	o.cmds = append(o.cmds, cmd)
	o.dirty = true
	o.mu.Unlock()
}

// renderPending paints and presents the waiting frame, if any. UI thread only.
func (o *OverlayWindow) renderPending() {
	o.mu.Lock()
	pending := o.pending
	o.pending = nil
	o.renderQueued = false
	observer := o.observer
	o.mu.Unlock()

	if pending == nil || o.surface == nil {
		return
	}

	stats, err := o.surface.render(pending)
	if err != nil {
		// The surface is gone (a lost device, most likely). Come back on GDI
		// and paint the frame there; the commands are just rectangles.
		rebuildErr := o.rebuildOnGDI()
		if rebuildErr != nil {
			stats = FrameStats{Err: fmt.Errorf("%w; rebuilding on gdi: %w", err, rebuildErr)}
		} else {
			stats, err = o.surface.render(pending)
			if err != nil {
				stats = FrameStats{Backend: o.surface.name(), Err: err}
			}
		}
	}

	if observer != nil {
		observer(stats)
	}
}

// rebuildOnGDI tears the window down and recreates it on the GDI surface,
// keeping it visible if it was. UI thread only.
func (o *OverlayWindow) rebuildOnGDI() error {
	o.mu.Lock()
	visible := o.visible
	o.mu.Unlock()

	o.destroyHWNDLocked()
	o.noDComp = true

	err := o.createHWNDLocked()
	if err != nil {
		return err
	}

	if visible {
		discardCall(procShowWindow.Call(uintptr(o.hwnd), swShowNoActivate))
		discardCall(procSetWindowPos.Call(
			uintptr(o.hwnd),
			hwndTopMost,
			0,
			0,
			0,
			0,
			swpNoActivate|swpShowWindow|swpNoMove|swpNoSize,
		))
	}

	return nil
}

// createHWNDLocked creates the window and its surface. UI thread only.
//
// DirectComposition is tried first where the build supports it; the window
// it needs is not a layered one, so a surface that fails to come up costs the
// HWND too, and the GDI surface gets a fresh layered window of its own.
func (o *OverlayWindow) createHWNDLocked() error {
	width := o.bounds.Dx()

	height := o.bounds.Dy()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("%w: %v", errInvalidOverlayBounds, o.bounds)
	}

	if !o.noDComp && dcompAvailable() {
		err := o.createWindowWithSurface(width, height, wsExNoRedirectionBitmap, newDCompSurface)
		if err == nil {
			return nil
		}

		o.noDComp = true
	}

	return o.createWindowWithSurface(width, height, wsExLayered, newGDISurface)
}

func (o *OverlayWindow) createWindowWithSurface(
	width, height int,
	exStyle uintptr,
	newSurface func(hwnd windows.HWND, width, height int) (overlaySurface, error),
) error {
	className, err := windows.UTF16PtrFromString(overlayClassName)
	if err != nil {
		return err
	}

	hwnd, _, err := procCreateWindowExW.Call(
		exStyle|wsExTransparent|wsExTopmost|wsExToolWindow|wsExNoActivate,
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

	surface, err := newSurface(windows.HWND(hwnd), width, height)
	if err != nil {
		discardCall(procDestroyWindow.Call(hwnd))

		return err
	}

	o.setHandle(windows.HWND(hwnd))
	o.width = width
	o.height = height
	o.surface = surface
	overlayRegistry.Store(windows.HWND(hwnd), o)

	discardCall(procSetWindowPos.Call(
		hwnd,
		hwndTopMost,
		0,
		0,
		0,
		0,
		swpNoActivate|swpNoMove|swpNoSize,
	))
	discardCall(procShowWindow.Call(hwnd, swHide))

	o.mu.Lock()
	o.visible = false
	o.mu.Unlock()

	return nil
}

func (o *OverlayWindow) destroyHWNDLocked() {
	if o.surface != nil {
		o.surface.destroy()
		o.surface = nil
	}

	if hwnd := o.handle(); hwnd != 0 {
		overlayRegistry.Delete(hwnd)
		discardCall(procDestroyWindow.Call(uintptr(hwnd)))
		o.setHandle(0)
	}
}

func (o *OverlayWindow) localBounds() image.Rectangle {
	return image.Rect(0, 0, o.width, o.height)
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
