//go:build darwin

package darwin

/*
#include <stdlib.h>
#include "textmeasure.h"
*/
import "C"

import (
	"unsafe"

	"github.com/y3owk1n/neru/internal/adapter/platform/fontcache"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// NewTextMeasurer returns a CoreText-backed ports.TextMeasurer. Each string is
// measured on first use and remembered for the lifetime of the process.
func NewTextMeasurer() ports.TextMeasurer {
	return fontcache.NewMeasurer(measureText)
}

// measureText asks CoreText directly, on the calling thread. It must never
// reach the main queue. See NeruMeasureText.
func measureText(text, family string, size float64, bold bool) (ports.TextMetrics, error) {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	cFamily := C.CString(family)
	defer C.free(unsafe.Pointer(cFamily))

	cBold := C.int(0)
	if bold {
		cBold = 1
	}

	var width, height C.double
	if C.NeruMeasureText(cText, cFamily, C.double(size), cBold, &width, &height) == 0 {
		return ports.TextMetrics{}, derrors.New(
			derrors.CodeInternal,
			"CoreText could not measure the text",
		)
	}

	return ports.TextMetrics{Width: float64(width), Height: float64(height)}, nil
}
