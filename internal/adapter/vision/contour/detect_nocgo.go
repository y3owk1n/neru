//go:build !cgo

package contour

import (
	"image"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Detect refuses: the detector is C, and a build without cgo has no way to
// run it. Returning nothing would read as a scan that found nothing.
func Detect(_ *image.RGBA, _ float64) ([]image.Rectangle, error) {
	return nil, derrors.New(derrors.CodeNotSupported, "contour detection needs a cgo build")
}
