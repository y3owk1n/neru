//go:build linux && cgo

package linux

/*
// Only tesseract, not lept. ocr.c includes <tesseract/capi.h>, which needs no
// leptonica header, and libtesseract.so links leptonica itself. Requiring
// lept.pc would only fail builds where the library exists without its .pc file.
#cgo linux pkg-config: tesseract
#include <stdlib.h>
#include "ocr.h"
*/
import "C"

import (
	"image"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

// Test files cannot use cgo, so ocr_common.go mirrors the NERU_OCR_* codes as Go
// constants. Each line converts the difference to uint both ways, so a drifted
// value stops compiling rather than mapping a failure to the wrong sentence.
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
	_ = uint(
		C.NERU_OCR_ERR_CANCELED-ocrStatusCanceled,
	) + uint(
		ocrStatusCanceled-C.NERU_OCR_ERR_CANCELED,
	)
)

// ocrConfig builds the native config for one call. The returned free function
// releases the two C strings it owns and must run on every path.
func ocrConfig(datapath string, params OCRParams) (C.NeruOCRConfig, func()) {
	language, engine := ocrLanguage, C.int(0)
	if params.Language != "" && params.Language != ocrLanguage {
		language, engine = params.Language, 1
	}

	cDatapath := C.CString(datapath)
	cLanguage := C.CString(language)

	var wordLevel C.int
	if params.WordLevel {
		wordLevel = 1
	}

	config := C.NeruOCRConfig{
		datapath:  cDatapath,
		language:  cLanguage,
		wordLevel: wordLevel,
		timeoutMS: C.int(params.TimeoutMS),
		engine:    engine,
	}

	return config, func() {
		C.free(unsafe.Pointer(cDatapath))
		C.free(unsafe.Pointer(cLanguage))
	}
}

// OCRHealth reports whether text recognition can run on this machine. It
// resolves the language data and initializes the engine, so a missing or
// unreadable tessdata surfaces here and the first recognition finds a warm
// engine.
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

// RecognizeText reads the text in img and returns one entry per word or line,
// in img's own coordinates. img must be RGBA8888, as CaptureScreenRegion
// returns. The pixels cross into C for this call only, because tesseract copies
// them and the engine is cleared before the call returns.
//
// OCRStats is filled on both paths, so a caller can log how long recognition
// took and whether a failure ran to the budget. Never log anything derived from
// the recognized text, which is a transcript of the user's screen.
func RecognizeText(img *image.RGBA, params OCRParams) ([]OCRWord, OCRStats, error) {
	var stats OCRStats

	datapath, err := tessdataDir()
	if err != nil {
		return nil, stats, err
	}

	if !readableImage(img) {
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

// RecognizeLines reads the text in img as RecognizeText does, and returns one
// entry per character with its line and word, for a caller that lays the text
// out as it stands on screen. Never log anything derived from the result.
func RecognizeLines(img *image.RGBA, params OCRParams) ([]OCRSymbol, OCRStats, error) {
	var stats OCRStats

	datapath, err := tessdataDir()
	if err != nil {
		return nil, stats, err
	}

	if !readableImage(img) {
		return nil, stats, ocrError(ocrStatusImage)
	}

	config, free := ocrConfig(datapath, params)
	defer free()

	stopWatching := watchForCancel(&config, params.Done)
	defer stopWatching()

	var lines C.NeruOCRLines

	status := C.neru_ocr_recognize_lines(
		(*C.uchar)(unsafe.Pointer(&img.Pix[0])),
		C.int(img.Rect.Dx()),
		C.int(img.Rect.Dy()),
		C.int(img.Stride),
		&config,
		&lines,
	)

	runtime.KeepAlive(img)

	stats.Recognition = time.Duration(lines.elapsedMS) * time.Millisecond

	if status != C.NERU_OCR_OK {
		return nil, stats, ocrError(ocrStatus(status))
	}

	defer C.neru_ocr_lines_free(&lines)

	count := int(lines.count)
	if count == 0 || lines.symbols == nil {
		return nil, stats, nil
	}

	symbols := make([]OCRSymbol, 0, count)

	for _, symbol := range unsafe.Slice(lines.symbols, count) {
		if symbol.text == nil {
			continue
		}

		symbols = append(symbols, OCRSymbol{
			Text: C.GoString(symbol.text),
			Bounds: image.Rect(
				int(symbol.x),
				int(symbol.y),
				int(symbol.x+symbol.width),
				int(symbol.y+symbol.height),
			),
			Line: int(symbol.line),
			Word: int(symbol.word),
		})
	}

	return symbols, stats, nil
}

// watchForCancel gives config a cancel flag in C memory and sets it once done
// closes, so a recognition stops early. The returned function stops watching
// and frees the flag, after the watcher has let go of it. A nil done watches
// nothing.
func watchForCancel(config *C.NeruOCRConfig, done <-chan struct{}) func() {
	if done == nil {
		return func() {}
	}

	flag := (*C.int)(C.calloc(1, C.size_t(unsafe.Sizeof(C.int(0)))))
	config.cancel = flag

	stop := make(chan struct{})

	var watcher sync.WaitGroup

	watcher.Go(func() {
		select {
		case <-done:
			C.neru_ocr_cancel(flag)
		case <-stop:
		}
	})

	return func() {
		close(stop)
		watcher.Wait()
		C.free(unsafe.Pointer(flag))
	}
}

// readableImage reports whether img has pixels for its whole geometry. C reads
// the buffer by stride, so a Pix shorter than the geometry claims is an
// out-of-bounds read inside tesseract rather than a Go panic. This is the last
// place Go can catch it.
func readableImage(img *image.RGBA) bool {
	return img != nil && img.Rect.Dx() > 0 && img.Rect.Dy() > 0 &&
		len(img.Pix) >= (img.Rect.Dy()-1)*img.Stride+img.Rect.Dx()*bytesPerCapturedPixel
}

// LinesReadable reports why RecognizeLines cannot run on this machine, or nil
// when it can: the tesseract language data has to be installed. It reads no
// pixels and starts no engine, so a caller can ask before it captures.
func LinesReadable() error {
	_, err := tessdataDir()

	return err
}
