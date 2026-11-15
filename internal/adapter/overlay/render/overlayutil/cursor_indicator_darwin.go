//go:build darwin

package overlayutil

/*
#include "../../../platform/darwin/overlay.h"
*/
import "C"

import (
	"image"
	"unsafe"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	"github.com/y3owk1n/neru/internal/config"
)

// ShowCursorIndicator draws the virtual pointer a grid mode shows on its own
// overlay window at point. appearance and textColor are what the manager
// handed the overlay through SetVirtualPointerConfig, already settled.
func ShowCursorIndicator(
	window unsafe.Pointer,
	point image.Point,
	size int,
	fillColor string,
	appearance config.VirtualPointerUI,
	textColor string,
) {
	cFillColor := C.CString(fillColor)
	cLabelChar := C.CString(appearance.Char)
	cFontFamily := C.CString(appearance.FontFamily)
	cTextColor := C.CString(textColor)

	defer darwin.FreeCString(unsafe.Pointer(cFillColor))
	defer darwin.FreeCString(unsafe.Pointer(cLabelChar))
	defer darwin.FreeCString(unsafe.Pointer(cFontFamily))
	defer darwin.FreeCString(unsafe.Pointer(cTextColor))

	C.NeruShowCursorIndicator(
		C.OverlayWindow(window),
		C.CGPoint{x: C.double(point.X), y: C.double(point.Y)},
		C.CursorIndicatorStyle{
			radius:     C.double(size),
			fillColor:  cFillColor,
			labelChar:  cLabelChar,
			fontFamily: cFontFamily,
			fontSize:   C.int(appearance.FontSize),
			textColor:  cTextColor,
		},
	)
}
