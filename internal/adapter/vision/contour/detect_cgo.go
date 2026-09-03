//go:build cgo

package contour

/*
#cgo LDFLAGS: -lm
#include "contour.h"
#include <stdlib.h>
*/
import "C"

import (
	"image"
	"unsafe"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Detect detects interactive UI target bounding boxes from an RGBA
// image buffer using the contour detection algorithm ported from wl-kbptr.
// Rectangles are returned in logical coordinates (divided by scale). An empty
// frame is refused; a frame with nothing in it returns no rectangles and no
// error.
func Detect(img *image.RGBA, scale float64) ([]image.Rectangle, error) {
	if img == nil || img.Rect.Dx() <= 0 || img.Rect.Dy() <= 0 {
		return nil, derrors.New(
			derrors.CodeInvalidInput,
			"contour detection needs a non-empty frame",
		)
	}

	if scale <= 0 {
		scale = 1.0
	}

	var result C.NeruTargetResult
	status := C.neru_contour_detect(
		(*C.uchar)(unsafe.Pointer(&img.Pix[0])),
		C.int(img.Rect.Dx()),
		C.int(img.Rect.Dy()),
		C.int(img.Stride),
		C.double(scale),
		&result,
	)

	if status != C.NERU_CONTOUR_OK {
		return nil, derrors.Newf(
			derrors.CodeInternal,
			"contour detector failed with status %d",
			int(status),
		)
	}

	// Freed even when empty: malloc(0) may hand back a live pointer.
	defer C.neru_contour_free(&result)

	if result.count == 0 || result.rects == nil {
		return nil, nil
	}

	count := int(result.count)
	cRects := unsafe.Slice(result.rects, count)
	rects := make([]image.Rectangle, 0, count)

	for _, rect := range cRects {
		if rect.w <= 0 || rect.h <= 0 {
			continue
		}

		rects = append(rects, image.Rect(
			int(rect.x),
			int(rect.y),
			int(rect.x+rect.w),
			int(rect.y+rect.h),
		))
	}

	return rects, nil
}
