//go:build linux && cgo

package linux

/*
#include "wlkbptr.h"
#include <stdlib.h>
*/
import "C"

import (
	"image"
	"unsafe"
)

// DetectWLKBPTRTargets detects interactive UI target bounding boxes from an RGBA
// image buffer using the wl-kbptr contour detection algorithm.
// Rectangles are returned in logical coordinates (divided by scale).
func DetectWLKBPTRTargets(img *image.RGBA, scale float64) []image.Rectangle {
	if img == nil || img.Rect.Dx() <= 0 || img.Rect.Dy() <= 0 {
		return nil
	}

	if scale <= 0 {
		scale = 1.0
	}

	var result C.NeruTargetResult
	status := C.neru_wlkbptr_detect(
		(*C.uchar)(unsafe.Pointer(&img.Pix[0])),
		C.int(img.Rect.Dx()),
		C.int(img.Rect.Dy()),
		C.int(img.Stride),
		C.double(scale),
		&result,
	)

	if status != C.NERU_WLKBPTR_OK || result.count == 0 || result.rects == nil {
		return nil
	}
	defer C.neru_wlkbptr_free(&result)

	count := int(result.count)
	cRects := unsafe.Slice(result.rects, count)
	rects := make([]image.Rectangle, 0, count)

	for _, r := range cRects {
		if r.w <= 0 || r.h <= 0 {
			continue
		}
		rects = append(rects, image.Rect(
			int(r.x),
			int(r.y),
			int(r.x+r.w),
			int(r.y+r.h),
		))
	}

	return rects
}
