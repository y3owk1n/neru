//go:build windows

package windows

import (
	"context"
	"image"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Text recognition through Windows.Media.Ocr, the first-party WinRT engine
// every Windows 10 and 11 desktop ships. It answers the *text* half of the
// vision strategy the way tesseract does on Linux: macOS runs three Vision
// requests and an OCR engine answers the first, so the rectangle request stays
// macOS-only (docs/CROSS_PLATFORM.md, the Vision footnote).
//
// The engine is created from the user's profile languages and needs the OCR
// language pack for at least one of them, which Windows installs with a
// language's "Basic typing" feature. A machine without one gets
// CodeNotSupported naming that remedy, from OCRHealth and from RecognizeText,
// rather than a strategy that silently finds nothing.
//
// Everything below runs on one OS thread the process keeps in the WinRT
// multithreaded apartment for its lifetime, so the engine can be created once
// and reused: RoUninitialize on the last thread in an apartment tears it down
// together with every object in it, and the daemon activates hints far more
// often than it starts. Requests are serialized through that thread, which
// also serializes the engine, and Windows.Media.Ocr is not reentrant anyway.
//
// Privacy: the frame and the text read from it are screen content. Both
// reach the caller and nowhere else. Nothing here logs; OCRStats carries the
// duration a caller may log, and nothing that derives from what was read.

// OCRWord is one run of recognized text, in the coordinate space of the image
// it was read from — origin (0, 0) at the top-left of that buffer, not of the
// screen.
//
// Text is screen content and carries the same rules the capture buffer does:
// it is never logged, never written to disk, and never held past the detection
// that asked for it.
type OCRWord struct {
	Text   string
	Bounds image.Rectangle
	// Confidence is 0..1. Windows.Media.Ocr reports no per-word score, so
	// every word carries 1: the engine has already dropped what it could not
	// read, and a floor a caller compares against keeps everything.
	Confidence float64
}

// OCRStats describes the work one recognition did, so a caller can log it.
// Durations only — nothing here derives from what was on the screen.
type OCRStats struct {
	// Recognition is how long the engine spent on the frame.
	Recognition time.Duration
}

// OCRParams is what one recognition needs beyond the pixels.
type OCRParams struct {
	// WordLevel asks for per-word boxes instead of per-line ones, which is
	// what `neru hints --split-word` means.
	WordLevel bool
	// TimeoutMS bounds the recognition. Zero or less means no deadline.
	TimeoutMS int
}

const (
	ocrEngineClass      = "Windows.Media.Ocr.OcrEngine"
	softwareBitmapClass = "Windows.Graphics.Imaging.SoftwareBitmap"
	bufferClass         = "Windows.Storage.Streams.Buffer"

	// bitmapPixelFormatBgra8 is BitmapPixelFormat.Bgra8, the layout the DIB
	// capture already produces before it swaps into image.RGBA order.
	bitmapPixelFormatBgra8 = 87

	// Vtable slots past the IInspectable prefix, in header order.
	ocrEngineStaticsMaxImageDimension    = inspectableSlots + 0
	ocrEngineStaticsTryCreateUserProfile = inspectableSlots + 4
	ocrEngineRecognizeAsync              = inspectableSlots + 0
	ocrResultLines                       = inspectableSlots + 0
	ocrLineWords                         = inspectableSlots + 0
	ocrLineText                          = inspectableSlots + 1
	ocrWordBoundingRect                  = inspectableSlots + 0
	ocrWordText                          = inspectableSlots + 1
	softwareBitmapFactoryCreate          = inspectableSlots + 0
	softwareBitmapCopyFromBuffer         = inspectableSlots + 11
	bufferFactoryCreate                  = inspectableSlots + 0
	bufferPutLength                      = inspectableSlots + 2
	asyncOperationGetResults             = inspectableSlots + 2
	asyncInfoStatus                      = inspectableSlots + 1
	asyncInfoErrorCode                   = inspectableSlots + 2
	asyncInfoCancel                      = inspectableSlots + 3
	asyncInfoClose                       = inspectableSlots + 4

	// IBufferByteAccess derives from IUnknown, not IInspectable: Buffer is
	// its only method, right after Release.
	bufferByteAccessBuffer = 3

	// AsyncStatus values (asyncinfo.h).
	asyncStatusStarted   = 0
	asyncStatusCompleted = 1
	asyncStatusCanceled  = 2
	asyncStatusError     = 3

	// roundHalf turns a float edge into the nearest whole pixel.
	roundHalf = 0.5

	// ocrPollInterval is how often a recognition in flight is checked. The
	// engine takes tens of milliseconds on a window, so a millisecond of
	// granularity costs nothing a user can feel and needs no COM callback
	// object implemented in Go.
	ocrPollInterval = time.Millisecond
)

// Interface IDs, from the SDK headers named in winrt.go.
var (
	iidOcrEngineStatics      = mustGUID("{5BFFA85A-3384-3540-9940-699120D428A8}")
	iidSoftwareBitmapFactory = mustGUID("{C99FEB69-2D62-4D47-A6B3-4FDB6A07FDF8}")
	iidBufferFactory         = mustGUID("{71AF914D-C10F-484B-BC50-14BC623B3A27}")
	iidBufferByteAccess      = mustGUID("{905A0FEF-BC53-11DF-8C49-001E4FC686DA}")
	iidAsyncInfo             = mustGUID("{00000036-0000-0000-C000-000000000046}")
)

// winrtRect is Windows.Foundation.Rect: four floats, the shape OcrWord's
// BoundingRect writes into.
type winrtRect struct {
	X, Y, Width, Height float32
}

// ocrWorker is the one thread the apartment and the engine live on.
type ocrWorker struct {
	requests chan func()
	// initErr is why the worker could not start, read after start has
	// happened; nil means it is serving.
	initErr error

	// The fields below are touched only from the worker goroutine.
	engine   unsafe.Pointer
	maxImage int
}

var (
	ocrOnce   sync.Once
	ocrShared *ocrWorker
)

// worker starts the OCR thread on first use and returns it.
func worker() *ocrWorker {
	ocrOnce.Do(func() {
		ocrShared = &ocrWorker{requests: make(chan func())}
		started := make(chan struct{})

		go ocrShared.run(started)

		<-started
	})

	return ocrShared
}

// run is the worker goroutine: locked to its OS thread, joined to the
// multithreaded apartment, serving one request at a time until the process
// ends.
func (w *ocrWorker) run(started chan<- struct{}) {
	runtime.LockOSThread()

	w.initErr = roInitialize()

	close(started)

	if w.initErr != nil {
		return
	}

	for request := range w.requests {
		request()
	}
}

// do runs request on the worker thread and waits for it.
func (w *ocrWorker) do(request func() error) error {
	if w.initErr != nil {
		return derrors.Wrap(w.initErr, derrors.CodeInternal,
			"the Windows Runtime could not be initialized for text recognition")
	}

	done := make(chan error, 1)
	w.requests <- func() { done <- request() }

	return <-done
}

// ensureEngine creates the engine on first use. Worker thread only.
func (w *ocrWorker) ensureEngine() error {
	if w.engine != nil {
		return nil
	}

	statics, err := activationFactory(ocrEngineClass, &iidOcrEngineStatics)
	if err != nil {
		// Every Windows build Neru ships for carries this class, so its
		// absence is a broken install rather than an unsupported platform.
		return derrors.Wrap(err, derrors.CodeInternal,
			"Windows.Media.Ocr could not be reached on this Windows install")
	}
	defer winrtReleaseObject(statics)

	var maxImage uint32

	hresult := winrtCall(
		statics,
		ocrEngineStaticsMaxImageDimension,
		uintptr(unsafe.Pointer(&maxImage)),
	)
	if hresultFailed(hresult) || maxImage == 0 {
		return derrors.Wrap(hresultError("OcrEngine.MaxImageDimension", hresult),
			derrors.CodeInternal, "could not read the OCR engine's image limit")
	}

	var engine unsafe.Pointer

	hresult = winrtCall(
		statics,
		ocrEngineStaticsTryCreateUserProfile,
		uintptr(unsafe.Pointer(&engine)),
	)
	if hresultFailed(hresult) {
		return derrors.Wrap(hresultError("OcrEngine.TryCreateFromUserProfileLanguages", hresult),
			derrors.CodeInternal, "could not create the OCR engine")
	}

	// A null engine with S_OK is the documented answer for "none of the
	// profile languages has an OCR pack installed".
	if engine == nil {
		return derrors.New(derrors.CodeNotSupported,
			"no OCR language pack is installed for any of this account's languages; "+
				"in Settings > Time & language > Language & region, open a language's "+
				"options and install Basic typing, or run "+
				`Add-WindowsCapability -Online -Name "Language.OCR~~~en-US~0.0.1.0"`)
	}

	w.engine = engine
	w.maxImage = int(maxImage)

	return nil
}

// OCRHealth reports whether text recognition can run on this machine: the
// runtime is up and an OCR language pack is installed for one of the
// account's languages. A caller that asks is also warming the engine; the
// handle is kept for the process, and a later recognition finds it made.
func OCRHealth() error {
	bridge := worker()

	return bridge.do(bridge.ensureEngine)
}

// RecognizeText reads the text in img and returns each run with its box, in
// img's own pixel space. WordLevel asks for one run per word; otherwise a run
// is a line, and its box is the union of its words'.
//
// ctx is read while the engine works: a hint activation that is canceled or
// runs past its own deadline cancels the recognition rather than waiting on
// params.TimeoutMS, which is the user's budget for the engine alone.
//
// The engine caps both image dimensions (MaxImageDimension, 2600 on current
// builds), which a 4K monitor exceeds. A frame past the cap is box-averaged
// down by the smallest whole factor that fits, and the boxes are scaled back
// up before they are returned, so the caller never sees the resampling.
func RecognizeText(
	ctx context.Context,
	img *image.RGBA,
	params OCRParams,
) ([]OCRWord, OCRStats, error) {
	if img == nil || img.Rect.Empty() {
		return nil, OCRStats{}, derrors.New(derrors.CodeActionFailed,
			"text recognition needs a frame with pixels in it")
	}

	var (
		words []OCRWord
		stats OCRStats
	)

	bridge := worker()

	err := bridge.do(func() error {
		engineErr := bridge.ensureEngine()
		if engineErr != nil {
			return engineErr
		}

		var recognizeErr error

		words, stats, recognizeErr = bridge.recognize(ctx, img, params)

		return recognizeErr
	})
	if err != nil {
		return nil, stats, err
	}

	return words, stats, nil
}

// recognize is RecognizeText on the worker thread with the engine made.
func (w *ocrWorker) recognize(
	ctx context.Context,
	img *image.RGBA,
	params OCRParams,
) ([]OCRWord, OCRStats, error) {
	longest := max(img.Rect.Dx(), img.Rect.Dy())
	factor := (longest + w.maxImage - 1) / w.maxImage

	frame := img
	if factor > 1 {
		frame = downsample(img, factor)
	}

	bitmap, err := softwareBitmapFrom(frame)
	if err != nil {
		return nil, OCRStats{}, err
	}
	defer winrtReleaseObject(bitmap)

	started := time.Now()

	result, err := w.recognizeBitmap(ctx, bitmap, params.TimeoutMS)

	stats := OCRStats{Recognition: time.Since(started)}
	if err != nil {
		return nil, stats, err
	}
	defer winrtReleaseObject(result)

	words, err := collectWords(result, params.WordLevel, factor)
	if err != nil {
		return nil, stats, err
	}

	return words, stats, nil
}

// recognizeBitmap runs the engine over bitmap and waits for the result,
// polling the operation's status rather than registering a completion
// handler. Worker thread only.
func (w *ocrWorker) recognizeBitmap(
	ctx context.Context,
	bitmap unsafe.Pointer,
	timeoutMS int,
) (unsafe.Pointer, error) {
	var operation unsafe.Pointer

	hresult := winrtCall(
		w.engine,
		ocrEngineRecognizeAsync,
		uintptr(bitmap),
		uintptr(unsafe.Pointer(&operation)),
	)
	if hresultFailed(hresult) || operation == nil {
		return nil, derrors.Wrap(hresultError("OcrEngine.RecognizeAsync", hresult),
			derrors.CodeActionFailed, "the captured frame is not a shape the OCR engine can read")
	}
	defer winrtReleaseObject(operation)

	info, err := winrtQuery(operation, &iidAsyncInfo)
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeInternal,
			"the OCR operation does not expose its status")
	}
	defer func() {
		winrtCall(info, asyncInfoClose)
		winrtReleaseObject(info)
	}()

	var deadline time.Time
	if timeoutMS > 0 {
		deadline = time.Now().Add(time.Duration(timeoutMS) * time.Millisecond)
	}

	for {
		var status int32

		hresult = winrtCall(info, asyncInfoStatus, uintptr(unsafe.Pointer(&status)))
		if hresultFailed(hresult) {
			return nil, derrors.Wrap(hresultError("IAsyncInfo.get_Status", hresult),
				derrors.CodeInternal, "could not read the OCR operation's status")
		}

		switch status {
		case asyncStatusCompleted:
			var result unsafe.Pointer

			hresult = winrtCall(
				operation,
				asyncOperationGetResults,
				uintptr(unsafe.Pointer(&result)),
			)
			if hresultFailed(hresult) || result == nil {
				return nil, derrors.Wrap(hresultError("IAsyncOperation.GetResults", hresult),
					derrors.CodeActionFailed, "text recognition produced no result")
			}

			return result, nil
		case asyncStatusError:
			var code uintptr

			winrtCall(info, asyncInfoErrorCode, uintptr(unsafe.Pointer(&code)))

			return nil, derrors.Wrap(hresultError("OcrEngine.RecognizeAsync", code),
				derrors.CodeActionFailed, "text recognition failed")
		case asyncStatusCanceled:
			return nil, derrors.New(derrors.CodeActionFailed, "text recognition was canceled")
		}

		ctxErr := ctx.Err()
		if ctxErr != nil {
			winrtCall(info, asyncInfoCancel)

			return nil, derrors.Wrap(ctxErr, derrors.CodeContextCanceled,
				"text recognition canceled")
		}

		if !deadline.IsZero() && time.Now().After(deadline) {
			winrtCall(info, asyncInfoCancel)

			return nil, derrors.New(derrors.CodeActionFailed,
				"text recognition ran past hints.vision.request_timeout_ms; raise it, or "+
					"narrow what is being read by hinting a smaller window")
		}

		time.Sleep(ocrPollInterval)
	}
}

// softwareBitmapFrom copies img into a Bgra8 SoftwareBitmap the engine reads.
// The pixels go through a Windows.Storage.Streams.Buffer because that is the
// one input SoftwareBitmap takes without locking its own planes.
func softwareBitmapFrom(img *image.RGBA) (unsafe.Pointer, error) {
	width, height := img.Rect.Dx(), img.Rect.Dy()
	byteCount := width * height * bytesPerPixel

	bufferFactory, err := activationFactory(bufferClass, &iidBufferFactory)
	if err != nil {
		return nil, derrors.Wrap(
			err,
			derrors.CodeInternal,
			"could not reach the WinRT buffer factory",
		)
	}
	defer winrtReleaseObject(bufferFactory)

	var buffer unsafe.Pointer

	hresult := winrtCall(
		bufferFactory,
		bufferFactoryCreate,
		uintptr(byteCount),
		uintptr(unsafe.Pointer(&buffer)),
	)
	if hresultFailed(hresult) || buffer == nil {
		return nil, derrors.Wrap(hresultError("Buffer.Create", hresult),
			derrors.CodeInternal, "could not allocate a buffer for the frame")
	}
	defer winrtReleaseObject(buffer)

	byteAccess, err := winrtQuery(buffer, &iidBufferByteAccess)
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeInternal, "the WinRT buffer exposes no bytes")
	}
	defer winrtReleaseObject(byteAccess)

	var bytes unsafe.Pointer

	hresult = winrtCall(byteAccess, bufferByteAccessBuffer, uintptr(unsafe.Pointer(&bytes)))
	if hresultFailed(hresult) || bytes == nil {
		return nil, derrors.Wrap(hresultError("IBufferByteAccess.Buffer", hresult),
			derrors.CodeInternal, "could not map the frame buffer")
	}

	dst := unsafe.Slice((*byte)(bytes), byteCount)

	for y := range height {
		row := img.Pix[y*img.Stride : y*img.Stride+width*bytesPerPixel]
		out := dst[y*width*bytesPerPixel:]

		for offset := 0; offset < len(row); offset += bytesPerPixel {
			out[offset] = row[offset+2]
			out[offset+1] = row[offset+1]
			out[offset+2] = row[offset]
			out[offset+3] = 0xFF
		}
	}

	hresult = winrtCall(buffer, bufferPutLength, uintptr(byteCount))
	if hresultFailed(hresult) {
		return nil, derrors.Wrap(hresultError("IBuffer.put_Length", hresult),
			derrors.CodeInternal, "could not size the frame buffer")
	}

	bitmapFactory, err := activationFactory(softwareBitmapClass, &iidSoftwareBitmapFactory)
	if err != nil {
		return nil, derrors.Wrap(
			err,
			derrors.CodeInternal,
			"could not reach the WinRT bitmap factory",
		)
	}
	defer winrtReleaseObject(bitmapFactory)

	var bitmap unsafe.Pointer

	hresult = winrtCall(bitmapFactory, softwareBitmapFactoryCreate,
		bitmapPixelFormatBgra8, uintptr(width), uintptr(height), uintptr(unsafe.Pointer(&bitmap)))
	if hresultFailed(hresult) || bitmap == nil {
		return nil, derrors.Wrap(hresultError("SoftwareBitmap.Create", hresult),
			derrors.CodeInternal, "could not create a bitmap for the frame")
	}

	hresult = winrtCall(bitmap, softwareBitmapCopyFromBuffer, uintptr(buffer))
	if hresultFailed(hresult) {
		winrtReleaseObject(bitmap)

		return nil, derrors.Wrap(hresultError("SoftwareBitmap.CopyFromBuffer", hresult),
			derrors.CodeInternal, "could not copy the frame into its bitmap")
	}

	// The buffer's bytes are screen content; the bitmap now holds the only
	// copy that matters, so this one is wiped before it is released.
	clear(dst)

	return bitmap, nil
}

// collectWords walks the result's lines and words into OCRWords, scaling each
// box by factor to undo the downsample the frame went through.
func collectWords(result unsafe.Pointer, wordLevel bool, factor int) ([]OCRWord, error) {
	var linesPtr unsafe.Pointer

	hresult := winrtCall(result, ocrResultLines, uintptr(unsafe.Pointer(&linesPtr)))
	if hresultFailed(hresult) || linesPtr == nil {
		return nil, derrors.Wrap(hresultError("OcrResult.Lines", hresult),
			derrors.CodeInternal, "could not read the recognized lines")
	}
	defer winrtReleaseObject(linesPtr)

	lines := vectorView{ptr: linesPtr}

	lineCount, err := lines.size()
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeInternal, "could not count the recognized lines")
	}

	var words []OCRWord

	for i := range lineCount {
		line, lineErr := lines.at(i)
		if lineErr != nil {
			return nil, derrors.Wrap(
				lineErr,
				derrors.CodeInternal,
				"could not read a recognized line",
			)
		}

		words, err = appendLine(words, line, wordLevel, factor)
		winrtReleaseObject(line)

		if err != nil {
			return nil, err
		}
	}

	return words, nil
}

// appendLine appends line's words, or the line as one run, to words.
func appendLine(
	words []OCRWord,
	line unsafe.Pointer,
	wordLevel bool,
	factor int,
) ([]OCRWord, error) {
	var wordsPtr unsafe.Pointer

	hresult := winrtCall(line, ocrLineWords, uintptr(unsafe.Pointer(&wordsPtr)))
	if hresultFailed(hresult) || wordsPtr == nil {
		return nil, derrors.Wrap(hresultError("OcrLine.Words", hresult),
			derrors.CodeInternal, "could not read a line's words")
	}
	defer winrtReleaseObject(wordsPtr)

	view := vectorView{ptr: wordsPtr}

	count, err := view.size()
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeInternal, "could not count a line's words")
	}

	var lineBounds image.Rectangle

	for i := range count {
		word, wordErr := view.at(i)
		if wordErr != nil {
			return nil, derrors.Wrap(
				wordErr,
				derrors.CodeInternal,
				"could not read a recognized word",
			)
		}

		var rect winrtRect

		hresult = winrtCall(word, ocrWordBoundingRect, uintptr(unsafe.Pointer(&rect)))
		if hresultFailed(hresult) {
			winrtReleaseObject(word)

			return nil, derrors.Wrap(hresultError("OcrWord.BoundingRect", hresult),
				derrors.CodeInternal, "could not read a word's box")
		}

		bounds := image.Rect(
			int(rect.X)*factor,
			int(rect.Y)*factor,
			int(rect.X+rect.Width+roundHalf)*factor,
			int(rect.Y+rect.Height+roundHalf)*factor,
		)

		if wordLevel {
			words = append(words, OCRWord{
				Text:       inspectableText(word, ocrWordText),
				Bounds:     bounds,
				Confidence: 1,
			})
		} else {
			lineBounds = lineBounds.Union(bounds)
		}

		winrtReleaseObject(word)
	}

	if !wordLevel && count > 0 {
		words = append(words, OCRWord{
			Text:       inspectableText(line, ocrLineText),
			Bounds:     lineBounds,
			Confidence: 1,
		})
	}

	return words, nil
}

// inspectableText reads the HSTRING property at slot on this and returns a
// trimmed copy. A read that fails is an empty run, which the caller drops.
func inspectableText(this unsafe.Pointer, slot int) string {
	var text hstring

	hresult := winrtCall(this, slot, uintptr(unsafe.Pointer(&text)))
	if hresultFailed(hresult) {
		return ""
	}
	defer text.delete()

	return strings.TrimSpace(text.String())
}

// downsample box-averages img by a whole factor. The last partial row and
// column are dropped rather than padded, so every output pixel averages a
// full block.
func downsample(img *image.RGBA, factor int) *image.RGBA {
	width, height := img.Rect.Dx()/factor, img.Rect.Dy()/factor
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	block := factor * factor

	for row := range height {
		for column := range width {
			var sum [bytesPerPixel]int

			for dy := range factor {
				offset := img.PixOffset(img.Rect.Min.X+column*factor, img.Rect.Min.Y+row*factor+dy)
				for dx := range factor {
					px := img.Pix[offset+dx*bytesPerPixel : offset+dx*bytesPerPixel+bytesPerPixel]
					for channel := range bytesPerPixel {
						sum[channel] += int(px[channel])
					}
				}
			}

			dst := out.PixOffset(column, row)
			for channel := range bytesPerPixel {
				out.Pix[dst+channel] = byte(sum[channel] / block)
			}
		}
	}

	return out
}
