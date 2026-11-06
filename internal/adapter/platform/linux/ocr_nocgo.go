//go:build linux && !cgo

package linux

import (
	"image"

	"github.com/y3owk1n/neru/internal/derrors"
)

// OCRHealth reports that text recognition cannot run in this build. It always
// returns derrors.CodeNotSupported, because tesseract is reached only through
// cgo.
func OCRHealth() error {
	return derrors.New(
		derrors.CodeNotSupported,
		"the vision hint strategy requires CGO-enabled Linux builds: text "+
			"recognition is linked against tesseract",
	)
}

// RecognizeText recognizes nothing in this build. It always returns
// derrors.CodeNotSupported, so a caller never reads an empty word list as a
// screen with no text on it.
func RecognizeText(img *image.RGBA, params OCRParams) ([]OCRWord, OCRStats, error) {
	_, _ = img, params

	return nil, OCRStats{}, derrors.New(
		derrors.CodeNotSupported,
		"text recognition requires CGO-enabled Linux builds",
	)
}
