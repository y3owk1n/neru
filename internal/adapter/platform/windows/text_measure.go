//go:build windows

package windows

import (
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/y3owk1n/neru/internal/adapter/platform/fontcache"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

var procGetTextExtentPoint32W = gdi32.NewProc("GetTextExtentPoint32W")

// gdiSize mirrors SIZE.
type gdiSize struct {
	cx int32
	cy int32
}

// NewTextMeasurer returns a GDI-backed ports.TextMeasurer. Each string is
// measured on first use and remembered for the lifetime of the process.
//
// GDI measures for both renderers. The DirectComposition path draws with
// DirectWrite, whose advances differ from GDI's by a few percent at most. The
// fit policy keeps a margin that covers it, which is cheaper than a second
// measuring path that would have to be kept in step with this one.
func NewTextMeasurer() ports.TextMeasurer {
	return fontcache.NewMeasurer(measureText)
}

// measureText realizes the font the GDI renderer would draw with, on a memory
// device context of its own, so it is safe on any goroutine and never touches
// the overlay's render thread.
func measureText(text, family string, size float64, bold bool) (ports.TextMetrics, error) {
	if size <= 0 {
		return ports.TextMetrics{}, derrors.New(
			derrors.CodeInvalidInput,
			"text cannot be measured at a font size that is not positive",
		)
	}

	// GDI realizes whole pixel sizes, as gdiTextRenderer.font does. The answer
	// is scaled back to the size asked for so a fractional size is not
	// under-measured.
	pixelSize := max(int(size), 1)

	utf16Text, err := windows.UTF16FromString(text)
	if err != nil {
		return ports.TextMetrics{}, derrors.Wrap(
			err,
			derrors.CodeInvalidInput,
			"text is not valid UTF-16",
		)
	}

	hdc, _, _ := procCreateCompatibleDC.Call(0)
	if hdc == 0 {
		return ports.TextMetrics{}, derrors.New(derrors.CodeInternal, "CreateCompatibleDC failed")
	}

	defer func() { discardCall(procDeleteDC.Call(hdc)) }()

	hFont, err := createGDIFont(family, pixelSize, bold)
	if err != nil {
		return ports.TextMetrics{}, derrors.Wrap(
			err,
			derrors.CodeInternal,
			"GDI could not realize the font",
		)
	}

	defer func() { discardCall(procDeleteObject.Call(hFont)) }()

	previous, _, _ := procSelectObject.Call(hdc, hFont)
	defer func() { discardCall(procSelectObject.Call(hdc, previous)) }()

	var extent gdiSize

	// The length excludes the terminator UTF16FromString appends.
	measured, _, _ := procGetTextExtentPoint32W.Call(
		hdc,
		uintptr(unsafe.Pointer(&utf16Text[0])),
		uintptr(len(utf16Text)-1),
		uintptr(unsafe.Pointer(&extent)),
	)
	if measured == 0 {
		return ports.TextMetrics{}, derrors.New(
			derrors.CodeInternal,
			"GetTextExtentPoint32W failed",
		)
	}

	ratio := size / float64(pixelSize)

	return ports.TextMetrics{
		Width:  float64(extent.cx) * ratio,
		Height: float64(extent.cy) * ratio,
	}, nil
}
