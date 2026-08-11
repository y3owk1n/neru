//go:build linux && !cgo

package linux

import (
	"image"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The cgo twin lives in ocr_cgo.go. Tesseract is a C library bound through
// #cgo pkg-config, so a pure-Go build has no engine at all and says so rather
// than returning an empty word list a caller would read as "no text on screen".

func OCRHealth() error {
	return derrors.New(
		derrors.CodeNotSupported,
		"the vision hint strategy requires CGO-enabled Linux builds: text "+
			"recognition is linked against tesseract",
	)
}

func RecognizeText(img *image.RGBA, params OCRParams) ([]OCRWord, error) {
	_, _ = img, params

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"text recognition requires CGO-enabled Linux builds",
	)
}
