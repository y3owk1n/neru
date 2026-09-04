//go:build windows && amd64

package windows

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The DirectComposition surface: the Windows counterpart of the CoreAnimation
// layer macOS draws on.
//
// The window is created with WS_EX_NOREDIRECTIONBITMAP, so it has no GDI
// backing store at all. A DXGI swapchain with premultiplied alpha is bound to
// a DirectComposition visual on it, Direct2D paints the frame's commands into
// the swapchain's back buffer on the GPU, and DWM composites the result — no
// pixel is copied through the CPU, and text comes from DirectWrite's glyph
// cache rather than a font GDI realizes per label.
//
// It is all COM, called through vtables the way the UIA adapter calls
// IUIAutomation: this build has no cgo. Direct2D takes floats, which Windows
// passes in XMM registers; Go's stdcall shim on amd64 mirrors the first four
// integer arguments into XMM0-XMM3 and the rest travel on the stack as their
// bit patterns, so a float32's bits in a uintptr reach the callee intact.
// windows/arm64 has no such mirror, which is why this file is amd64-only and
// overlay_dcomp_other.go answers there (docs/CROSS_PLATFORM.md owns that
// status).
//
// The frame is painted into a persistent canvas bitmap first and the canvas
// copied to the back buffer to present: a flip-model swapchain does not keep
// a back buffer's contents across presents, and the window's frames are
// incremental — a search badge is drawn over the hints already on screen.

const (
	d3dDriverTypeHardware = 1
	d3dDriverTypeWARP     = 5
	d3d11CreateDeviceBGRA = 0x20
	d3d11SDKVersion       = 7

	dxgiFormatB8G8R8A8UNorm     = 87
	dxgiUsageRenderTargetOutput = 0x20
	dxgiSwapEffectFlipSeq       = 3
	dxgiAlphaModePremultiplied  = 1
	dxgiSwapChainBuffers        = 2
	dcompTargetTopmost          = 1

	d2dFactoryTypeMultiThreaded = 1
	d2dAlphaModePremultiplied   = 1
	d2dBitmapOptionsTarget      = 0x1
	d2dBitmapOptionsCannotDraw  = 0x2
	d2dTextAntialiasGrayscale   = 2
	d2dCompositeModeSourceCopy  = 1
	d2dFigureBeginFilled        = 0
	d2dFigureEndClosed          = 1
	d2dDefaultDPI               = 96

	dwriteFactoryTypeShared    = 0
	dwriteFontWeightBold       = 700
	dwriteFontStretchNormal    = 5
	dwriteTextAlignmentCenter  = 2
	dwriteParagraphAlignCenter = 2
	dwriteWordWrappingNoWrap   = 1
	dwriteLocale               = "en-us"

	// vtable slots. IUnknown holds 0-2.
	vtblRelease = 2

	vtblDXGIObjectGetParent   = 6
	vtblDXGIDeviceGetAdapter  = 7
	vtblDXGISwapChainPresent  = 8
	vtblDXGISwapChainGetBuf   = 9
	vtblDXGIFactory2CreateSCC = 24

	vtblDCompDeviceCommit       = 3
	vtblDCompDeviceCreateTarget = 6
	vtblDCompDeviceCreateVisual = 7
	vtblDCompTargetSetRoot      = 3
	vtblDCompVisualSetContent   = 15

	vtblD2DFactoryCreatePathGeometry = 10
	vtblD2DFactory1CreateDevice      = 17
	vtblD2DDeviceCreateContext       = 4

	vtblD2DRTCreateSolidBrush     = 8
	vtblD2DRTDrawRectangle        = 16
	vtblD2DRTFillRectangle        = 17
	vtblD2DRTDrawRoundedRectangle = 18
	vtblD2DRTFillRoundedRectangle = 19
	vtblD2DRTFillGeometry         = 23
	vtblD2DRTDrawText             = 27
	vtblD2DRTSetTextAntialiasMode = 34
	vtblD2DRTClear                = 47
	vtblD2DRTBeginDraw            = 48
	vtblD2DRTEndDraw              = 49
	vtblD2DDCCreateBitmap         = 57
	vtblD2DDCCreateBitmapFromDXGI = 62
	vtblD2DDCSetTarget            = 74
	vtblD2DDCDrawImage            = 83

	vtblD2DBrushSetColor = 8

	vtblD2DPathGeometryOpen = 17
	vtblD2DSinkBeginFigure  = 5
	vtblD2DSinkAddLines     = 6
	vtblD2DSinkEndFigure    = 8
	vtblD2DSinkClose        = 9

	vtblDWriteFactoryCreateTextFormat = 15
	vtblDWriteFormatSetTextAlignment  = 3
	vtblDWriteFormatSetParagraphAlign = 4
	vtblDWriteFormatSetWordWrapping   = 5

	float32Bits = 32
)

var (
	errDCompUnavailable = errors.New("directcomposition overlay unavailable")

	d3d11  = windows.NewLazySystemDLL("d3d11.dll")
	dcomp  = windows.NewLazySystemDLL("dcomp.dll")
	d2d1   = windows.NewLazySystemDLL("d2d1.dll")
	dwrite = windows.NewLazySystemDLL("dwrite.dll")

	procD3D11CreateDevice        = d3d11.NewProc("D3D11CreateDevice")
	procDCompositionCreateDevice = dcomp.NewProc("DCompositionCreateDevice")
	procD2D1CreateFactory        = d2d1.NewProc("D2D1CreateFactory")
	procDWriteCreateFactory      = dwrite.NewProc("DWriteCreateFactory")

	//nolint:mnd // interface IDs, as the SDK headers spell them
	iidIDXGIDevice = windows.GUID{
		Data1: 0x54ec77fa, Data2: 0x1377, Data3: 0x44e6,
		Data4: [8]byte{0x8c, 0x32, 0x88, 0xfd, 0x5f, 0x44, 0xc8, 0x4c},
	}
	//nolint:mnd // interface ID, as the SDK header spells it
	iidIDXGIFactory2 = windows.GUID{
		Data1: 0x50c83a1c, Data2: 0xe072, Data3: 0x4c48,
		Data4: [8]byte{0x87, 0xb0, 0x36, 0x30, 0xfa, 0x36, 0xa6, 0xd0},
	}
	//nolint:mnd // interface ID, as the SDK header spells it
	iidIDXGISurface = windows.GUID{
		Data1: 0xcafcb56c, Data2: 0x6ac3, Data3: 0x4889,
		Data4: [8]byte{0xbf, 0x47, 0x9e, 0x23, 0xbb, 0xd2, 0x60, 0xc2},
	}
	//nolint:mnd // interface ID, as the SDK header spells it
	iidIDCompositionDevice = windows.GUID{
		Data1: 0xC37EA93A, Data2: 0xE7AA, Data3: 0x450D,
		Data4: [8]byte{0xB1, 0x6F, 0x97, 0x46, 0xCB, 0x04, 0x07, 0xF3},
	}
	//nolint:mnd // interface ID, as the SDK header spells it
	iidID2D1Factory1 = windows.GUID{
		Data1: 0xbb12d362, Data2: 0xdaee, Data3: 0x4b9a,
		Data4: [8]byte{0xaa, 0x1d, 0x14, 0xba, 0x40, 0x1c, 0xfa, 0x1f},
	}
	//nolint:mnd // interface ID, as the SDK header spells it
	iidIDWriteFactory = windows.GUID{
		Data1: 0xb859ee5a, Data2: 0xd838, Data3: 0x4b5b,
		Data4: [8]byte{0xa2, 0xe8, 0x1a, 0xdc, 0x7d, 0x93, 0xdb, 0x48},
	}
)

// comObject is a COM interface pointer. Methods are reached through the
// vtable by index; every interface used here is called on the overlay UI
// thread only.
type comObject struct {
	ptr unsafe.Pointer
}

func (c comObject) call(index int, args ...uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(c.ptr)
	method := *(*uintptr)(unsafe.Add(vtbl, uintptr(index)*unsafe.Sizeof(uintptr(0))))

	full := make([]uintptr, 0, len(args)+1)
	full = append(full, uintptr(c.ptr))
	full = append(full, args...)

	ret, _, _ := syscall.SyscallN(method, full...)

	return ret
}

// hresult turns a COM return value into an error.
func (c comObject) hresult(what string, index int, args ...uintptr) error {
	return checkHRESULT(what, c.call(index, args...))
}

func (c comObject) release() {
	if c.ptr != nil {
		c.call(vtblRelease)
	}
}

func checkHRESULT(what string, ret uintptr) error {
	if int32(ret) < 0 {
		return fmt.Errorf("%w: %s failed: 0x%08X", errDCompUnavailable, what, uint32(ret))
	}

	return nil
}

func ptrArg(p unsafe.Pointer) uintptr { return uintptr(p) }

func objArg(c comObject) uintptr { return uintptr(c.ptr) }

func outArg(c *comObject) uintptr { return uintptr(unsafe.Pointer(&c.ptr)) }

func guidArg(g *windows.GUID) uintptr { return uintptr(unsafe.Pointer(g)) }

func floatArg(f float32) uintptr { return uintptr(math.Float32bits(f)) }

// packedPoint is a D2D1_POINT_2F passed by value: eight bytes, which the x64
// convention hands over in one register.
func packedPoint(x, y float32) uintptr {
	return uintptr(math.Float32bits(x)) | uintptr(math.Float32bits(y))<<float32Bits
}

// packedSizeU is a D2D1_SIZE_U passed by value, the same way.
func packedSizeU(width, height int) uintptr {
	return uintptr(uint32(width)) | uintptr(uint32(height))<<float32Bits
}

type dxgiSwapChainDesc1 struct {
	Width       uint32
	Height      uint32
	Format      uint32
	Stereo      int32
	SampleCount uint32
	SampleQual  uint32
	BufferUsage uint32
	BufferCount uint32
	Scaling     uint32
	SwapEffect  uint32
	AlphaMode   uint32
	Flags       uint32
}

type d2dRectF struct {
	Left, Top, Right, Bottom float32
}

type d2dRoundedRect struct {
	Rect             d2dRectF
	RadiusX, RadiusY float32
}

type d2dColorF struct {
	R, G, B, A float32
}

type d2dPoint2F struct {
	X, Y float32
}

type d2dBitmapProperties1 struct {
	Format       uint32
	AlphaMode    uint32
	DpiX         float32
	DpiY         float32
	Options      uint32
	ColorContext unsafe.Pointer
}

// dcompShared is the device-level state every DirectComposition window shares:
// one D3D device, one composition device, one Direct2D device and one
// DirectWrite factory with its text formats. Created on first use, on the
// overlay UI thread, and kept for the life of the process.
type dcompShared struct {
	d3dDevice     comObject
	dxgiDevice    comObject
	dxgiFactory   comObject
	dcompDevice   comObject
	d2dFactory    comObject
	d2dDevice     comObject
	dwriteFactory comObject
	formats       map[fontKey]comObject
}

var (
	dcompOnce   sync.Once
	dcompState  *dcompShared
	errDComp    error
	dcompBroken bool
)

// dcompAvailable reports whether DirectComposition can be used for new
// windows. UI thread only. The first call brings the devices up; a failure
// is remembered, and so is a device lost later.
func dcompAvailable() bool {
	dcompOnce.Do(func() {
		dcompState, errDComp = newDCompShared()
	})

	return errDComp == nil && !dcompBroken
}

func newDCompShared() (*dcompShared, error) {
	for _, proc := range []*windows.LazyProc{
		procD3D11CreateDevice,
		procDCompositionCreateDevice,
		procD2D1CreateFactory,
		procDWriteCreateFactory,
	} {
		err := proc.Find()
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errDCompUnavailable, err)
		}
	}

	shared := &dcompShared{formats: make(map[fontKey]comObject)}

	err := shared.init()
	if err != nil {
		shared.release()

		return nil, err
	}

	return shared, nil
}

func (s *dcompShared) init() error {
	err := s.createD3DDevice(d3dDriverTypeHardware)
	if err != nil {
		// No GPU for this session (a remote desktop, a VM): WARP still
		// composes through DWM without touching GDI.
		err = s.createD3DDevice(d3dDriverTypeWARP)
	}

	if err != nil {
		return err
	}

	var adapter comObject

	err = s.dxgiDevice.hresult(
		"IDXGIDevice::GetAdapter",
		vtblDXGIDeviceGetAdapter,
		outArg(&adapter),
	)
	if err != nil {
		return err
	}

	defer adapter.release()

	err = adapter.hresult(
		"IDXGIAdapter::GetParent",
		vtblDXGIObjectGetParent,
		guidArg(&iidIDXGIFactory2),
		outArg(&s.dxgiFactory),
	)
	if err != nil {
		return err
	}

	ret, _, _ := procDCompositionCreateDevice.Call(
		objArg(s.dxgiDevice),
		guidArg(&iidIDCompositionDevice),
		outArg(&s.dcompDevice),
	)

	err = checkHRESULT("DCompositionCreateDevice", ret)
	if err != nil {
		return err
	}

	ret, _, _ = procD2D1CreateFactory.Call(
		d2dFactoryTypeMultiThreaded,
		guidArg(&iidID2D1Factory1),
		0,
		outArg(&s.d2dFactory),
	)

	err = checkHRESULT("D2D1CreateFactory", ret)
	if err != nil {
		return err
	}

	err = s.d2dFactory.hresult(
		"ID2D1Factory1::CreateDevice",
		vtblD2DFactory1CreateDevice,
		objArg(s.dxgiDevice),
		outArg(&s.d2dDevice),
	)
	if err != nil {
		return err
	}

	ret, _, _ = procDWriteCreateFactory.Call(
		dwriteFactoryTypeShared,
		guidArg(&iidIDWriteFactory),
		outArg(&s.dwriteFactory),
	)

	return checkHRESULT("DWriteCreateFactory", ret)
}

func (s *dcompShared) createD3DDevice(driverType uintptr) error {
	var device comObject

	ret, _, _ := procD3D11CreateDevice.Call(
		0,
		driverType,
		0,
		d3d11CreateDeviceBGRA,
		0,
		0,
		d3d11SDKVersion,
		outArg(&device),
		0,
		0,
	)

	err := checkHRESULT("D3D11CreateDevice", ret)
	if err != nil {
		return err
	}

	var dxgiDevice comObject

	err = device.hresult(
		"ID3D11Device::QueryInterface(IDXGIDevice)",
		0,
		guidArg(&iidIDXGIDevice),
		outArg(&dxgiDevice),
	)
	if err != nil {
		device.release()

		return err
	}

	s.d3dDevice = device
	s.dxgiDevice = dxgiDevice

	return nil
}

func (s *dcompShared) release() {
	for _, format := range s.formats {
		format.release()
	}

	s.formats = nil

	for _, obj := range []*comObject{
		&s.dwriteFactory,
		&s.d2dDevice,
		&s.d2dFactory,
		&s.dcompDevice,
		&s.dxgiFactory,
		&s.dxgiDevice,
		&s.d3dDevice,
	} {
		obj.release()
		obj.ptr = nil
	}
}

// textFormat returns the DirectWrite format for a family and size, creating it
// once. A format is device-independent, so every window shares it.
func (s *dcompShared) textFormat(family string, fontSize float64) (comObject, error) {
	pixelSize := int(fontSize)
	if pixelSize == 0 {
		pixelSize = defaultFontSize
	}

	key := fontKey{family: family, size: pixelSize}
	if format, ok := s.formats[key]; ok {
		return format, nil
	}

	familyPtr, err := windows.UTF16PtrFromString(family)
	if err != nil {
		return comObject{}, err
	}

	localePtr, err := windows.UTF16PtrFromString(dwriteLocale)
	if err != nil {
		return comObject{}, err
	}

	var format comObject

	err = s.dwriteFactory.hresult(
		"IDWriteFactory::CreateTextFormat",
		vtblDWriteFactoryCreateTextFormat,
		ptrArg(unsafe.Pointer(familyPtr)),
		0,
		dwriteFontWeightBold,
		0,
		dwriteFontStretchNormal,
		floatArg(float32(pixelSize)),
		ptrArg(unsafe.Pointer(localePtr)),
		outArg(&format),
	)
	if err != nil {
		return comObject{}, err
	}

	format.call(vtblDWriteFormatSetTextAlignment, dwriteTextAlignmentCenter)
	format.call(vtblDWriteFormatSetParagraphAlign, dwriteParagraphAlignCenter)
	format.call(vtblDWriteFormatSetWordWrapping, dwriteWordWrappingNoWrap)

	s.formats[key] = format

	return format, nil
}

// dcompSurface is one window's swapchain, visual and Direct2D context.
type dcompSurface struct {
	shared    *dcompShared
	hwnd      windows.HWND
	width     int
	height    int
	swapchain comObject
	target    comObject
	visual    comObject
	context   comObject
	backBuf   comObject
	canvas    comObject
	brush     comObject
	// painted is whether anything has been drawn on the canvas since it was
	// last cleared; a clear of an empty canvas is skipped.
	painted bool
}

func newDCompSurface(hwnd windows.HWND, width, height int) (overlaySurface, error) {
	if !dcompAvailable() {
		if errDComp != nil {
			return nil, errDComp
		}

		return nil, errDCompUnavailable
	}

	surface := &dcompSurface{shared: dcompState, hwnd: hwnd}

	err := surface.create(width, height)
	if err != nil {
		surface.destroy()

		return nil, err
	}

	return surface, nil
}

func (s *dcompSurface) name() string { return "direct2d" }

func (s *dcompSurface) create(width, height int) error {
	s.width, s.height = width, height
	shared := s.shared

	desc := dxgiSwapChainDesc1{
		Width:       uint32(width),
		Height:      uint32(height),
		Format:      dxgiFormatB8G8R8A8UNorm,
		SampleCount: 1,
		BufferUsage: dxgiUsageRenderTargetOutput,
		BufferCount: dxgiSwapChainBuffers,
		SwapEffect:  dxgiSwapEffectFlipSeq,
		AlphaMode:   dxgiAlphaModePremultiplied,
	}

	err := shared.dxgiFactory.hresult(
		"IDXGIFactory2::CreateSwapChainForComposition",
		vtblDXGIFactory2CreateSCC,
		objArg(shared.d3dDevice),
		ptrArg(unsafe.Pointer(&desc)),
		0,
		outArg(&s.swapchain),
	)
	if err != nil {
		return err
	}

	err = shared.dcompDevice.hresult(
		"IDCompositionDevice::CreateTargetForHwnd",
		vtblDCompDeviceCreateTarget,
		uintptr(s.hwnd),
		dcompTargetTopmost,
		outArg(&s.target),
	)
	if err != nil {
		return err
	}

	err = shared.dcompDevice.hresult(
		"IDCompositionDevice::CreateVisual",
		vtblDCompDeviceCreateVisual,
		outArg(&s.visual),
	)
	if err != nil {
		return err
	}

	err = s.visual.hresult(
		"IDCompositionVisual::SetContent",
		vtblDCompVisualSetContent,
		objArg(s.swapchain),
	)
	if err != nil {
		return err
	}

	err = s.target.hresult("IDCompositionTarget::SetRoot", vtblDCompTargetSetRoot, objArg(s.visual))
	if err != nil {
		return err
	}

	err = shared.dcompDevice.hresult("IDCompositionDevice::Commit", vtblDCompDeviceCommit)
	if err != nil {
		return err
	}

	err = shared.d2dDevice.hresult(
		"ID2D1Device::CreateDeviceContext",
		vtblD2DDeviceCreateContext,
		0,
		outArg(&s.context),
	)
	if err != nil {
		return err
	}

	s.context.call(vtblD2DRTSetTextAntialiasMode, d2dTextAntialiasGrayscale)

	var surface comObject

	err = s.swapchain.hresult(
		"IDXGISwapChain::GetBuffer",
		vtblDXGISwapChainGetBuf,
		0,
		guidArg(&iidIDXGISurface),
		outArg(&surface),
	)
	if err != nil {
		return err
	}

	defer surface.release()

	targetProps := d2dBitmapProperties1{
		Format:    dxgiFormatB8G8R8A8UNorm,
		AlphaMode: d2dAlphaModePremultiplied,
		DpiX:      d2dDefaultDPI,
		DpiY:      d2dDefaultDPI,
		Options:   d2dBitmapOptionsTarget | d2dBitmapOptionsCannotDraw,
	}

	err = s.context.hresult(
		"ID2D1DeviceContext::CreateBitmapFromDxgiSurface",
		vtblD2DDCCreateBitmapFromDXGI,
		objArg(surface),
		ptrArg(unsafe.Pointer(&targetProps)),
		outArg(&s.backBuf),
	)
	if err != nil {
		return err
	}

	canvasProps := d2dBitmapProperties1{
		Format:    dxgiFormatB8G8R8A8UNorm,
		AlphaMode: d2dAlphaModePremultiplied,
		DpiX:      d2dDefaultDPI,
		DpiY:      d2dDefaultDPI,
		Options:   d2dBitmapOptionsTarget,
	}

	err = s.context.hresult(
		"ID2D1DeviceContext::CreateBitmap",
		vtblD2DDCCreateBitmap,
		packedSizeU(width, height),
		0,
		0,
		ptrArg(unsafe.Pointer(&canvasProps)),
		outArg(&s.canvas),
	)
	if err != nil {
		return err
	}

	transparent := d2dColorF{}

	err = s.context.hresult(
		"ID2D1RenderTarget::CreateSolidColorBrush",
		vtblD2DRTCreateSolidBrush,
		ptrArg(unsafe.Pointer(&transparent)),
		0,
		outArg(&s.brush),
	)
	if err != nil {
		return err
	}

	// Start from a transparent canvas: a fresh bitmap's contents are
	// undefined.
	s.context.call(vtblD2DDCSetTarget, objArg(s.canvas))
	s.context.call(vtblD2DRTBeginDraw)
	s.context.call(vtblD2DRTClear, 0)

	return s.context.hresult("ID2D1RenderTarget::EndDraw", vtblD2DRTEndDraw, 0, 0)
}

func (s *dcompSurface) releaseObjects() {
	for _, obj := range []*comObject{
		&s.brush,
		&s.canvas,
		&s.backBuf,
		&s.context,
		&s.visual,
		&s.target,
		&s.swapchain,
	} {
		obj.release()
		obj.ptr = nil
	}
}

// resize rebuilds the swapchain and its bitmaps at the new size. A monitor
// change is the only caller, so rebuilding beats resizing the chain in place.
func (s *dcompSurface) resize(width, height int) error {
	s.releaseObjects()
	s.painted = false

	err := s.create(width, height)
	if err != nil {
		s.releaseObjects()

		dcompBroken = true
	}

	return err
}

func (s *dcompSurface) destroy() {
	s.releaseObjects()
}

// render paints the frame into the canvas, copies the canvas to the back
// buffer and presents it.
func (s *dcompSurface) render(pending *frame) (FrameStats, error) {
	if s.context.ptr == nil {
		return FrameStats{}, errDCompUnavailable
	}

	started := time.Now()

	s.context.call(vtblD2DDCSetTarget, objArg(s.canvas))
	s.context.call(vtblD2DRTBeginDraw)

	if pending.clear && s.painted {
		s.context.call(vtblD2DRTClear, 0)
		s.painted = false
	}

	dirty := image.Rectangle{}
	if pending.clear {
		dirty = image.Rect(0, 0, s.width, s.height)
	}

	for _, cmd := range pending.cmds {
		s.paint(cmd)

		dirty = dirty.Union(cmd.bounds().Intersect(image.Rect(0, 0, s.width, s.height)))
		s.painted = true
	}

	err := s.context.hresult("ID2D1RenderTarget::EndDraw", vtblD2DRTEndDraw, 0, 0)
	if err != nil {
		dcompBroken = true

		return FrameStats{}, err
	}

	rastered := time.Now()

	if dirty.Empty() {
		return FrameStats{Backend: s.name(), Raster: rastered.Sub(started)}, nil
	}

	err = s.present()
	if err != nil {
		dcompBroken = true

		return FrameStats{}, err
	}

	return FrameStats{
		Backend:  s.name(),
		Commands: len(pending.cmds),
		Dirty:    dirty,
		Raster:   rastered.Sub(started),
		Present:  time.Since(rastered),
	}, nil
}

// present copies the canvas onto the back buffer and hands it to DWM.
func (s *dcompSurface) present() error {
	s.context.call(vtblD2DDCSetTarget, objArg(s.backBuf))
	s.context.call(vtblD2DRTBeginDraw)
	s.context.call(vtblD2DDCDrawImage, objArg(s.canvas), 0, 0, 0, d2dCompositeModeSourceCopy)

	err := s.context.hresult("ID2D1RenderTarget::EndDraw", vtblD2DRTEndDraw, 0, 0)
	if err != nil {
		return err
	}

	return s.swapchain.hresult("IDXGISwapChain::Present", vtblDXGISwapChainPresent, 0, 0)
}

func (s *dcompSurface) setColor(color uint32) {
	const channelMax = float32(alphaMax)

	colorF := d2dColorF{
		R: float32((color>>redShift)&byteMask) / channelMax,
		G: float32((color>>greenShift)&byteMask) / channelMax,
		B: float32(color&byteMask) / channelMax,
		A: float32(color>>alphaShift) / channelMax,
	}

	s.brush.call(vtblD2DBrushSetColor, ptrArg(unsafe.Pointer(&colorF)))
}

func rectF(rect image.Rectangle) d2dRectF {
	return d2dRectF{
		Left:   float32(rect.Min.X),
		Top:    float32(rect.Min.Y),
		Right:  float32(rect.Max.X),
		Bottom: float32(rect.Max.Y),
	}
}

// strokeRect is the rectangle a Direct2D stroke of the given width is drawn
// on so that it lands inside rect: Direct2D centers a stroke on the path,
// the software rasterizer keeps it inside the rectangle, and the two must
// paint the same cell border.
func strokeRect(rect image.Rectangle, width int) d2dRectF {
	half := float32(width) / 2 //nolint:mnd // centered stroke

	return d2dRectF{
		Left:   float32(rect.Min.X) + half,
		Top:    float32(rect.Min.Y) + half,
		Right:  float32(rect.Max.X) - half,
		Bottom: float32(rect.Max.Y) - half,
	}
}

func (s *dcompSurface) paint(cmd drawCmd) {
	if cmd.color>>alphaShift == 0 {
		return
	}

	s.setColor(cmd.color)

	switch cmd.kind {
	case drawFill:
		rect := rectF(cmd.rect)
		s.context.call(vtblD2DRTFillRectangle, ptrArg(unsafe.Pointer(&rect)), objArg(s.brush))
	case drawRoundedFill:
		rounded := d2dRoundedRect{
			Rect:    rectF(cmd.rect),
			RadiusX: float32(cmd.radius),
			RadiusY: float32(cmd.radius),
		}
		s.context.call(
			vtblD2DRTFillRoundedRectangle,
			ptrArg(unsafe.Pointer(&rounded)),
			objArg(s.brush),
		)
	case drawStroke:
		rect := strokeRect(cmd.rect, cmd.width)
		s.context.call(
			vtblD2DRTDrawRectangle,
			ptrArg(unsafe.Pointer(&rect)),
			objArg(s.brush),
			floatArg(float32(cmd.width)),
			0,
		)
	case drawRoundedStroke:
		inner := float32(math.Max(cmd.radius-float64(cmd.width)/2, 0))
		rounded := d2dRoundedRect{
			Rect:    strokeRect(cmd.rect, cmd.width),
			RadiusX: inner,
			RadiusY: inner,
		}
		s.context.call(
			vtblD2DRTDrawRoundedRectangle,
			ptrArg(unsafe.Pointer(&rounded)),
			objArg(s.brush),
			floatArg(float32(cmd.width)),
			0,
		)
	case drawTriangle:
		s.paintTriangle(cmd)
	case drawText:
		s.paintText(cmd)
	}
}

// paintTriangle fills a path geometry through the three vertices. The
// geometry is built and released per arrow: there are a handful per frame
// and only when hints are placed off their targets.
func (s *dcompSurface) paintTriangle(cmd drawCmd) {
	var geometry comObject

	err := s.shared.d2dFactory.hresult(
		"ID2D1Factory::CreatePathGeometry",
		vtblD2DFactoryCreatePathGeometry,
		outArg(&geometry),
	)
	if err != nil {
		return
	}

	defer geometry.release()

	var sink comObject

	err = geometry.hresult("ID2D1PathGeometry::Open", vtblD2DPathGeometryOpen, outArg(&sink))
	if err != nil {
		return
	}

	first := cmd.vertices[0]
	rest := [2]d2dPoint2F{
		{X: float32(cmd.vertices[1].X), Y: float32(cmd.vertices[1].Y)},
		{X: float32(cmd.vertices[2].X), Y: float32(cmd.vertices[2].Y)},
	}

	sink.call(
		vtblD2DSinkBeginFigure,
		packedPoint(float32(first.X), float32(first.Y)),
		d2dFigureBeginFilled,
	)
	sink.call(vtblD2DSinkAddLines, ptrArg(unsafe.Pointer(&rest[0])), uintptr(len(rest)))
	sink.call(vtblD2DSinkEndFigure, d2dFigureEndClosed)
	sink.call(vtblD2DSinkClose)
	sink.release()

	s.context.call(vtblD2DRTFillGeometry, objArg(geometry), objArg(s.brush), 0)
}

func (s *dcompSurface) paintText(cmd drawCmd) {
	format, err := s.shared.textFormat(cmd.font, cmd.fontSize)
	if err != nil {
		return
	}

	utf16Text, err := windows.UTF16FromString(cmd.text)
	if err != nil || len(utf16Text) < 2 {
		return
	}

	layout := rectF(cmd.rect)

	s.context.call(
		vtblD2DRTDrawText,
		ptrArg(unsafe.Pointer(&utf16Text[0])),
		uintptr(len(utf16Text)-1),
		objArg(format),
		ptrArg(unsafe.Pointer(&layout)),
		objArg(s.brush),
		0,
		0,
	)
}
