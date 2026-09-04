//go:build windows

package windows

import (
	"errors"
	"fmt"
	"image"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The GDI surface: a layered window updated from one persistent DIB section.
//
// It is the fallback for a desktop DirectComposition cannot serve — a session
// with no D3D device, or a build for an architecture the pure-Go Direct2D
// binding cannot call (overlay_dcomp_other.go). The DIB is created once and
// painted in place, so a frame costs its own commands plus one
// UpdateLayeredWindowIndirect of the rectangle it changed, and nothing is
// allocated per frame.

var (
	errGDISurfaceUnavailable = errors.New("gdi overlay surface unavailable")
	errGDITextUnavailable    = errors.New("gdi text renderer unavailable")
)

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

// updateLayeredWindowInfo is UPDATELAYEREDWINDOWINFO: UpdateLayeredWindow's
// arguments plus the dirty rectangle the plain call cannot take.
type updateLayeredWindowInfo struct {
	cbSize   uint32
	hdcDst   uintptr
	pptDst   *point
	psize    *size
	hdcSrc   uintptr
	pptSrc   *point
	crKey    uint32
	pblend   *blendFunction
	dwFlags  uint32
	prcDirty *windows.Rect
}

// dibSection is one top-down 32-bit BGRA DIB selected into its own memory DC,
// with its pixels mapped as a Go slice.
type dibSection struct {
	dc      uintptr
	bitmap  uintptr
	prevObj uintptr
	pixels  []byte
	width   int
	height  int
}

func newDIBSection(width, height int) (*dibSection, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("%w: %dx%d", errInvalidOverlayBounds, width, height)
	}

	hdc, _, _ := procCreateCompatibleDC.Call(0)
	if hdc == 0 {
		return nil, fmt.Errorf("%w: CreateCompatibleDC failed", errGDISurfaceUnavailable)
	}

	header := bitmapV4Header{
		Size:        bmpV4Size,
		Width:       int32(width),
		Height:      -int32(height), // negative = top-down bitmap
		Planes:      1,
		BitCount:    dibBitCount,
		Compression: biBitfields,
		SizeImage:   uint32(width * height * bytesPerPixel),
		RedMask:     maskRed,
		GreenMask:   maskGreen,
		BlueMask:    maskBlue,
		AlphaMask:   maskAlpha,
		CSType:      colorSpaceWinRGB,
	}

	var bits unsafe.Pointer

	bitmap, _, _ := procCreateDIBSection.Call(
		hdc,
		uintptr(unsafe.Pointer(&header)),
		0, // DIB_RGB_COLORS
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if bitmap == 0 || bits == nil {
		discardCall(procDeleteDC.Call(hdc))

		return nil, fmt.Errorf("%w: CreateDIBSection failed", errGDISurfaceUnavailable)
	}

	prevObj, _, _ := procSelectObject.Call(hdc, bitmap)
	if prevObj == 0 {
		discardCall(procDeleteObject.Call(bitmap))
		discardCall(procDeleteDC.Call(hdc))

		return nil, fmt.Errorf("%w: SelectObject failed", errGDISurfaceUnavailable)
	}

	// Start transparent. The first frame erases only what earlier frames
	// painted, and nothing documents the section's bits as zeroed.
	pixels := unsafe.Slice((*byte)(bits), width*height*bytesPerPixel)
	clear(pixels)

	return &dibSection{
		dc:      hdc,
		bitmap:  bitmap,
		prevObj: prevObj,
		pixels:  pixels,
		width:   width,
		height:  height,
	}, nil
}

func (d *dibSection) release() {
	if d == nil {
		return
	}

	if d.dc != 0 {
		if d.prevObj != 0 {
			discardCall(procSelectObject.Call(d.dc, d.prevObj))
		}

		discardCall(procDeleteDC.Call(d.dc))
	}

	if d.bitmap != 0 {
		discardCall(procDeleteObject.Call(d.bitmap))
	}

	d.dc, d.bitmap, d.prevObj, d.pixels = 0, 0, 0, nil
}

// gdiSurface presents a layered window from a persistent DIB.
type gdiSurface struct {
	hwnd    windows.HWND
	dib     *dibSection
	painted image.Rectangle
}

func newGDISurface(hwnd windows.HWND, width, height int) (overlaySurface, error) {
	dib, err := newDIBSection(width, height)
	if err != nil {
		return nil, err
	}

	return &gdiSurface{hwnd: hwnd, dib: dib}, nil
}

func (s *gdiSurface) name() string { return "gdi" }

func (s *gdiSurface) resize(width, height int) error {
	dib, err := newDIBSection(width, height)
	if err != nil {
		return err
	}

	s.dib.release()
	s.dib = dib
	s.painted = image.Rectangle{}

	return nil
}

func (s *gdiSurface) destroy() {
	s.dib.release()
	s.dib = nil
}

// render paints the frame into the DIB and hands the changed rectangle to the
// window. A clear erases only what earlier frames painted, and the rectangle
// presented is the union of that and what this frame painted.
func (s *gdiSurface) render(pending *frame) (FrameStats, error) {
	if s.dib == nil {
		return FrameStats{}, errGDISurfaceUnavailable
	}

	started := time.Now()

	width, height := s.dib.width, s.dib.height
	pixels := s.dib.pixels

	var dirty image.Rectangle

	if pending.clear {
		zeroRect(pixels, width, height, s.painted)
		dirty = s.painted
		s.painted = image.Rectangle{}
	}

	text := gdiText()

	for _, cmd := range pending.cmds {
		touched := paintCommand(pixels, width, height, cmd, text)
		if touched.Empty() {
			continue
		}

		dirty = dirty.Union(touched)
		s.painted = s.painted.Union(touched)
	}

	rastered := time.Now()

	if !dirty.Empty() {
		s.present(dirty)
	}

	return FrameStats{
		Backend:  s.name(),
		Commands: len(pending.cmds),
		Dirty:    dirty,
		Raster:   rastered.Sub(started),
		Present:  time.Since(rastered),
	}, nil
}

// present pushes the DIB to the layered window, telling it which rectangle
// changed so the compositor copies that and not the screen.
func (s *gdiSurface) present(dirty image.Rectangle) {
	blend := blendFunction{
		BlendOp:             acSrcOver,
		AlphaFormat:         acSrcAlpha,
		SourceConstantAlpha: alphaMax,
	}
	rect := windows.Rect{
		Left:   int32(dirty.Min.X),
		Top:    int32(dirty.Min.Y),
		Right:  int32(dirty.Max.X),
		Bottom: int32(dirty.Max.Y),
	}
	dst := size{CX: int32(s.dib.width), CY: int32(s.dib.height)}
	src := point{}

	info := updateLayeredWindowInfo{
		cbSize:   uint32(unsafe.Sizeof(updateLayeredWindowInfo{})),
		psize:    &dst,
		hdcSrc:   s.dib.dc,
		pptSrc:   &src,
		pblend:   &blend,
		dwFlags:  ulwAlpha,
		prcDirty: &rect,
	}

	discardCall(procUpdateLayeredWindowIndirect.Call(
		uintptr(s.hwnd),
		uintptr(unsafe.Pointer(&info)),
	))
}

// gdiTextRenderer rasterizes labels with GDI into a scratch DIB it keeps, with
// the fonts it has realized. It exists once per process and is used only on
// the overlay UI thread, so it needs no lock.
//
// Before this cache a label cost a DC, a DIB and a font created and destroyed
// around one DrawText — and realizing the font is the slow part of that. A
// grid of several hundred cells paid it several hundred times per keystroke.
type gdiTextRenderer struct {
	scratch *dibSection
	fonts   map[fontKey]uintptr
	current uintptr
}

type fontKey struct {
	family string
	size   int
}

var gdiTextCache *gdiTextRenderer

// gdiText returns the process's text renderer, creating it on first use. Its
// scratch DC is created on the first label; a label it cannot create one for
// is skipped.
func gdiText() *gdiTextRenderer {
	if gdiTextCache == nil {
		gdiTextCache = &gdiTextRenderer{fonts: make(map[fontKey]uintptr)}
	}

	return gdiTextCache
}

// ensureScratch makes the scratch DIB at least width x height, growing it to
// the largest label seen so far so it is reallocated a handful of times and
// never per label.
func (r *gdiTextRenderer) ensureScratch(width, height int) error {
	if r.scratch != nil && r.scratch.width >= width && r.scratch.height >= height {
		return nil
	}

	newW, newH := width, height
	if r.scratch != nil {
		newW = max(newW, r.scratch.width)
		newH = max(newH, r.scratch.height)
	}

	scratch, err := newDIBSection(newW, newH)
	if err != nil {
		return err
	}

	r.scratch.release()
	r.scratch = scratch
	r.current = 0

	discardCall(procSetBkMode.Call(scratch.dc, transparentBk))
	discardCall(procSetTextColor.Call(scratch.dc, gdiWhiteText))

	return nil
}

// font returns the realized HFONT for a family and size, creating it once.
func (r *gdiTextRenderer) font(family string, fontSize float64) (uintptr, error) {
	pixelSize := int(fontSize)
	if pixelSize == 0 {
		pixelSize = defaultFontSize
	}

	key := fontKey{family: family, size: pixelSize}
	if hFont, ok := r.fonts[key]; ok {
		return hFont, nil
	}

	fontName, err := windows.UTF16PtrFromString(family)
	if err != nil {
		return 0, err
	}

	// A negative height is the character height, which is what the
	// configured size means; a positive one would include internal leading.
	hFont, _, _ := procCreateFontW.Call(
		uintptr(-pixelSize), 0, 0, 0, fwBold, 0, 0, 0, 1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(fontName)),
	)
	if hFont == 0 {
		return 0, fmt.Errorf("%w: CreateFontW failed", errGDITextUnavailable)
	}

	r.fonts[key] = hFont

	return hFont, nil
}

// paint draws one text command into pixels: the glyphs are rendered white
// into the scratch DIB and composited in the command's color.
func (r *gdiTextRenderer) paint(pixels []byte, bufW, bufH int, cmd drawCmd) {
	textRect := cmd.rect.Intersect(image.Rect(0, 0, bufW, bufH))
	if textRect.Empty() {
		return
	}

	// One pixel of padding around the rectangle for anti-aliasing at the
	// edges, clamped back onto the buffer.
	const pad = 1

	texW := textRect.Dx() + pad*2 //nolint:mnd
	texH := textRect.Dy() + pad*2 //nolint:mnd
	textX := textRect.Min.X - pad
	textY := textRect.Min.Y - pad

	if textX < 0 {
		texW += textX
		textX = 0
	}

	if textY < 0 {
		texH += textY
		textY = 0
	}

	texW = min(texW, bufW-textX)
	texH = min(texH, bufH-textY)

	if texW <= 0 || texH <= 0 {
		return
	}

	if r.ensureScratch(texW, texH) != nil {
		return
	}

	hFont, err := r.font(cmd.font, cmd.fontSize)
	if err != nil {
		return
	}

	if hFont != r.current {
		discardCall(procSelectObject.Call(r.scratch.dc, hFont))
		r.current = hFont
	}

	stride := r.scratch.width * bytesPerPixel
	zeroRect(r.scratch.pixels, r.scratch.width, r.scratch.height, image.Rect(0, 0, texW, texH))

	utf16Text, err := windows.UTF16FromString(cmd.text)
	if err != nil {
		return
	}

	drawRect := windows.Rect{
		Left:   int32(pad),
		Top:    int32(pad),
		Right:  int32(pad + textRect.Dx()),
		Bottom: int32(pad + textRect.Dy()),
	}

	discardCall(procDrawTextW.Call(
		r.scratch.dc,
		uintptr(unsafe.Pointer(&utf16Text[0])),
		uintptr(^uint32(0)),
		uintptr(unsafe.Pointer(&drawRect)),
		dtCenter|dtVCenter|dtSingleLine,
	))

	alphaCompositeTextAt(
		pixels, bufW, bufH,
		r.scratch.pixels, texW, texH, stride, textX, textY,
		cmd.color,
	)
}
