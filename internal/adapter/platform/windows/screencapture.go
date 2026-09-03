//go:build windows

package windows

import (
	"context"
	"image"
	"unsafe"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Screen capture through GDI: BitBlt from the desktop DC into a top-down
// 32-bit DIB section. The desktop DC spans the whole virtual screen, so one
// call reads any rectangle on any monitor, and because the process is
// per-monitor-v2 DPI aware (the manifest go-winres embeds, `just
// generate-winres`) the coordinates it reads are the same physical pixels
// GetCursorPos and EnumDisplayMonitors report. That is what keeps a captured
// pixel mappable back to the screen point the hint pipeline clicks.
const (
	srcCopy    = 0x00CC0020
	captureBlt = 0x40000000
	biRGB      = 0

	// captureDIBBitCount is the DIB section depth: 32 bits, BGRA in memory,
	// which is what the desktop composites in and what the copy below expects.
	captureDIBBitCount = 32
)

var (
	procGetDC     = user32.NewProc("GetDC")
	procReleaseDC = user32.NewProc("ReleaseDC")
	procBitBlt    = gdi32.NewProc("BitBlt")
)

// bitmapInfoHeader is BITMAPINFOHEADER. A BI_RGB 32-bit section needs no color
// table, so this header alone stands in for the BITMAPINFO CreateDIBSection
// takes.
type bitmapInfoHeader struct {
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
}

// CaptureScreenRegion captures the pixels currently inside region and returns
// them as an RGBA image.
//
// region is in Neru's shared coordinate space: global origin, top-left, Y down,
// unscaled pixels. The zero rectangle means "the whole active screen", the
// monitor under the cursor, which is what ports.VisionPort.CaptureScreen means
// here; it is resolved in this function so the GDI path only ever sees a
// concrete rectangle. Honoring the region is the point of the parameter: the
// caller is normally the focused window, and reading a whole 4K desktop back
// to examine one window is the difference between usable and not.
//
// What comes back covers **exactly** the requested region. A rectangle that
// leaves the virtual screen fails rather than coming back clipped, because a
// clipped frame carries nothing that says where its top-left actually is. The
// image's own bounds start at (0, 0); the region passed in is what places
// those pixels. A region may span two monitors: the desktop DC is one surface
// and BitBlt reads across the seam.
//
// ctx is a check at the door rather than a deadline the call carries: BitBlt
// is a single synchronous GDI call that cannot be canceled once entered, and
// a Go deadline is only worth threading where the callee reads it
// (internal/app/modes/AGENTS.md).
//
// Privacy: the returned image is the only copy that outlives this call. The
// DIB section is wiped before it is deleted. Callers must never log it, derive
// log text from it, write it to disk, or hold it past the detection that asked
// for it.
func CaptureScreenRegion(ctx context.Context, region image.Rectangle) (*image.RGBA, error) {
	err := ctx.Err()
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeContextCanceled, "screen capture canceled")
	}

	resolved, err := resolveCaptureRegion(region)
	if err != nil {
		return nil, err
	}

	return bitBltRegion(resolved)
}

// resolveCaptureRegion turns the caller's request into the rectangle handed to
// BitBlt.
//
// Only the *zero* rectangle means "the whole active screen". A region that was
// asked for and came out degenerate is refused rather than widened, because
// the one thing a caller asking for a window must never get instead is
// everything on screen. A region that leaves the virtual screen is refused for
// the reason CaptureScreenRegion gives: a clipped frame cannot be placed.
func resolveCaptureRegion(region image.Rectangle) (image.Rectangle, error) {
	if region == (image.Rectangle{}) {
		bounds, err := activeScreenBounds()
		if err != nil {
			return image.Rectangle{}, derrors.Wrap(
				err,
				derrors.CodeActionFailed,
				"could not resolve the active screen to capture",
			)
		}

		bounds = bounds.Canon()
		if bounds.Empty() {
			return image.Rectangle{}, derrors.New(
				derrors.CodeActionFailed,
				"the active screen reports empty bounds; there is nothing to capture",
			)
		}

		return bounds, nil
	}

	canonical := region.Canon()
	if canonical.Empty() {
		return image.Rectangle{}, derrors.Newf(
			derrors.CodeActionFailed,
			"the requested capture region %v is empty; refusing to widen it to the whole screen",
			region,
		)
	}

	desktop, err := virtualScreenBounds()
	if err != nil {
		return image.Rectangle{}, err
	}

	if !canonical.In(desktop) {
		return image.Rectangle{}, derrors.Newf(
			derrors.CodeActionFailed,
			"the requested capture region %v lies outside the screen %v",
			canonical,
			desktop,
		)
	}

	return canonical, nil
}

// virtualScreenBounds is the rectangle every monitor fits inside, from the
// same enumeration ScreenNames and ScreenBoundsByName answer with, so a region
// accepted here is one those callers could have produced.
func virtualScreenBounds() (image.Rectangle, error) {
	monitors, err := enumerateMonitors()
	if err != nil {
		return image.Rectangle{}, derrors.Wrap(
			err,
			derrors.CodeActionFailed,
			"could not enumerate monitors to validate the capture region",
		)
	}

	desktop := monitors[0].bounds
	for _, monitor := range monitors[1:] {
		desktop = desktop.Union(monitor.bounds)
	}

	return desktop, nil
}

// bitBltRegion reads region off the desktop DC into a fresh image.RGBA.
//
// The DIB is requested top-down (negative height) so its rows are in the order
// image.RGBA wants, and BGRA so that only the red and blue channels swap on the
// way out. Alpha is forced opaque: the desktop composites to an opaque surface
// and image.RGBA is alpha-premultiplied, so anything else would render darker
// than the screen it came from.
func bitBltRegion(region image.Rectangle) (*image.RGBA, error) {
	width, height := region.Dx(), region.Dy()

	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return nil, derrors.New(
			derrors.CodeActionFailed,
			"could not open the desktop for reading; is there an interactive desktop session?",
		)
	}
	defer func() { discardCall(procReleaseDC.Call(0, hdcScreen)) }()

	hdcMem, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	if hdcMem == 0 {
		return nil, derrors.New(
			derrors.CodeInternal,
			"could not create a memory device context for the screen capture",
		)
	}
	defer func() { discardCall(procDeleteDC.Call(hdcMem)) }()

	header := bitmapInfoHeader{
		Width:       int32(width),
		Height:      -int32(height),
		Planes:      1,
		BitCount:    captureDIBBitCount,
		Compression: biRGB,
	}
	header.Size = uint32(unsafe.Sizeof(header))

	var bits unsafe.Pointer

	hDib, _, _ := procCreateDIBSection.Call(
		hdcScreen,
		uintptr(unsafe.Pointer(&header)),
		0, // DIB_RGB_COLORS
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if hDib == 0 || bits == nil {
		return nil, derrors.New(
			derrors.CodeInternal,
			"could not allocate a buffer for the screen capture",
		)
	}
	defer func() { discardCall(procDeleteObject.Call(hDib)) }()

	prevObj, _, _ := procSelectObject.Call(hdcMem, hDib)
	if prevObj == 0 {
		return nil, derrors.New(
			derrors.CodeInternal,
			"could not select the capture buffer into its device context",
		)
	}
	defer func() { discardCall(procSelectObject.Call(hdcMem, prevObj)) }()

	ret, _, err := procBitBlt.Call(
		hdcMem,
		0,
		0,
		uintptr(width),
		uintptr(height),
		hdcScreen,
		uintptr(region.Min.X),
		uintptr(region.Min.Y),
		srcCopy|captureBlt,
	)

	callErr := win32Bool(ret, err)
	if callErr != nil {
		return nil, derrors.Wrap(
			callErr,
			derrors.CodeActionFailed,
			"BitBlt failed to capture the screen",
		)
	}

	// The DIB is BGRA, tightly packed: a 32-bit row is already a multiple of the
	// four-byte alignment GDI pads to.
	src := unsafe.Slice((*byte)(bits), width*height*bytesPerPixel)
	defer clear(src)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for offset := 0; offset < len(src); offset += bytesPerPixel {
		img.Pix[offset] = src[offset+2]
		img.Pix[offset+1] = src[offset+1]
		img.Pix[offset+2] = src[offset]
		img.Pix[offset+3] = 0xFF
	}

	return img, nil
}
