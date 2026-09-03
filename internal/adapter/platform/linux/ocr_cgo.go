//go:build linux && cgo

package linux

/*
// Only tesseract, not tesseract plus lept: ocr.c includes <tesseract/capi.h>,
// which forward-declares `struct Pix` on the C path and needs no leptonica
// header, and every symbol the bridge calls is a Tess* one. libtesseract.so
// links leptonica itself, so the runtime dependency is there either way —
// requiring lept.pc as well would only add a build that fails on a machine
// where the library is present and its pkg-config file is not.
#cgo linux pkg-config: tesseract
#include <stdlib.h>
#include "ocr.h"
*/
import "C"

import (
	"image"
	"runtime"
	"time"
	"unsafe"
)

// A Go test file cannot use cgo, so the NERU_OCR_* codes are mirrored as Go
// constants in ocr_common.go and the error vocabulary is written against those.
// These constant expressions keep the two halves honest: each converts a
// difference to uint in both directions, so a value that drifts stops compiling
// here rather than silently mapping the wrong failure to the wrong sentence.
const (
	_ = uint(C.NERU_OCR_OK-ocrStatusOK) + uint(ocrStatusOK-C.NERU_OCR_OK)
	_ = uint(C.NERU_OCR_ERR_INIT-ocrStatusInit) + uint(ocrStatusInit-C.NERU_OCR_ERR_INIT)
	_ = uint(C.NERU_OCR_ERR_ALLOC-ocrStatusAlloc) + uint(ocrStatusAlloc-C.NERU_OCR_ERR_ALLOC)
	_ = uint(C.NERU_OCR_ERR_IMAGE-ocrStatusImage) + uint(ocrStatusImage-C.NERU_OCR_ERR_IMAGE)
	_ = uint(C.NERU_OCR_ERR_RECOGNIZE-ocrStatusRecognize) +
		uint(ocrStatusRecognize-C.NERU_OCR_ERR_RECOGNIZE)
	_ = uint(C.NERU_OCR_ERR_BUSY-ocrStatusBusy) + uint(ocrStatusBusy-C.NERU_OCR_ERR_BUSY)
	_ = uint(
		C.NERU_OCR_ERR_TIMEOUT-ocrStatusTimeout,
	) + uint(
		ocrStatusTimeout-C.NERU_OCR_ERR_TIMEOUT,
	)
)

// ocrConfig builds the native config for one call. The returned free function
// releases the two C strings it owns and must run on every path.
func ocrConfig(datapath string, params OCRParams) (C.NeruOCRConfig, func()) {
	cDatapath := C.CString(datapath)
	cLanguage := C.CString(ocrLanguage)

	var wordLevel C.int
	if params.WordLevel {
		wordLevel = 1
	}

	config := C.NeruOCRConfig{
		datapath:  cDatapath,
		language:  cLanguage,
		wordLevel: wordLevel,
		timeoutMS: C.int(params.TimeoutMS),
	}

	return config, func() {
		C.free(unsafe.Pointer(cDatapath))
		C.free(unsafe.Pointer(cLanguage))
	}
}

// OCRHealth reports whether text recognition can run on this machine.
//
// It resolves the language data and then initializes the engine, so a caller
// learns about a missing tessdata directory and about an unreadable one, and
// the first real recognition finds a warm engine rather than paying the model
// load itself.
func OCRHealth() error {
	datapath, err := tessdataDir()
	if err != nil {
		return err
	}

	config, free := ocrConfig(datapath, OCRParams{})
	defer free()

	if status := C.neru_ocr_probe(&config); status != C.NERU_OCR_OK {
		return ocrError(ocrStatus(status))
	}

	return nil
}

// RecognizeText reads the text in img and returns one entry per word or per
// line, positioned in img's own coordinate space.
//
// img must be RGBA8888, which is what CaptureScreenRegion returns. The pixels
// cross into C for the duration of this call only: tesseract copies them into
// its own buffer before recognition starts, and the engine is cleared before
// the call returns, so nothing native holds the frame afterwards.
//
// The second result describes the work rather than the frame — a duration, so
// a caller can log how long a recognition took and, when one fails, whether it
// ran to the budget or gave up at once. It is filled on both paths.
//
// Nothing derived from the recognized text is logged here or by the caller — an
// OCR result is a transcript of the user's screen.
func RecognizeText(img *image.RGBA, params OCRParams) ([]OCRWord, OCRStats, error) {
	var stats OCRStats

	datapath, err := tessdataDir()
	if err != nil {
		return nil, stats, err
	}

	if img == nil || img.Rect.Dx() <= 0 || img.Rect.Dy() <= 0 {
		return nil, stats, ocrError(ocrStatusImage)
	}

	// The buffer has to hold every row the geometry claims. C reads it by
	// stride, so a Pix shorter than the geometry describes — a hand-built image,
	// a sub-image sliced wrong — is an out-of-bounds read inside tesseract
	// rather than a Go panic. Checked here because this is the last place the
	// Go type system can see it.
	if len(img.Pix) < (img.Rect.Dy()-1)*img.Stride+img.Rect.Dx()*bytesPerCapturedPixel {
		return nil, stats, ocrError(ocrStatusImage)
	}

	config, free := ocrConfig(datapath, params)
	defer free()

	var result C.NeruOCRResult

	status := C.neru_ocr_recognize(
		(*C.uchar)(unsafe.Pointer(&img.Pix[0])),
		C.int(img.Rect.Dx()),
		C.int(img.Rect.Dy()),
		C.int(img.Stride),
		&config,
		&result,
	)

	runtime.KeepAlive(img)

	stats.Recognition = time.Duration(result.elapsedMS) * time.Millisecond

	if status != C.NERU_OCR_OK {
		return nil, stats, ocrError(ocrStatus(status))
	}

	defer C.neru_ocr_result_free(&result)

	count := int(result.count)
	if count == 0 || result.words == nil {
		return nil, stats, nil
	}

	words := make([]OCRWord, 0, count)

	for _, cWord := range unsafe.Slice(result.words, count) {
		if cWord.text == nil {
			continue
		}

		words = append(words, OCRWord{
			Text: C.GoString(cWord.text),
			Bounds: image.Rect(
				int(cWord.x),
				int(cWord.y),
				int(cWord.x+cWord.width),
				int(cWord.y+cWord.height),
			),
			Confidence: float64(cWord.confidence),
		})
	}

	return words, stats, nil
}
