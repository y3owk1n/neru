//go:build windows

package windows

import (
	"context"
	"errors"
	"image"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Text recognition through Windows.Media.Ocr, the first-party WinRT engine
// every Windows 10 and 11 desktop ships. It answers the *text* half of the
// vision strategy the way tesseract does on Linux. macOS runs three Vision
// requests and an OCR engine answers the first, so the rectangle request stays
// macOS-only (docs/contributing/platform-internals.md, the Vision footnote).
//
// The engine is created from the user's profile languages and needs the OCR
// language pack for at least one of them, which Windows installs with a
// language's "Basic typing" feature. A machine without one gets
// CodeNotSupported naming that remedy, from OCRHealth and from RecognizeText,
// rather than a strategy that silently finds nothing.
//
// That engine reads in the first profile language with a pack, so a Chinese
// account language below English goes unread. A second engine, for the first
// Chinese, Japanese or Korean language the account lists, reads beside it
// when asked (OCRParams.Wide), as macOS reads its preferred Chinese, Japanese
// or Korean language beside script detection.
//
// Each engine lives on an OS thread of its own that the process keeps in the
// WinRT multithreaded apartment for its lifetime, so it can be created once
// and reused, because RoUninitialize on the last thread in an apartment tears
// it down together with every object in it, and the daemon activates hints
// far more often than it starts. Requests are serialized through that thread,
// which also serializes the engine, and Windows.Media.Ocr is not reentrant
// anyway. An accurate read runs on threads of its own (OCRParams.Accurate),
// so a fast read never waits behind one.
//
// Privacy: the frame and the text read from it are screen content. Both
// reach the caller and nowhere else. Nothing here logs. OCRStats carries the
// duration a caller may log, and nothing that derives from what was read.

// OCRWord is one run of recognized text, in the coordinate space of the image
// it was read from, with origin (0, 0) at the top-left of that buffer, not of
// the screen.
//
// Text is screen content and carries the same rules the capture buffer does.
// It is never logged, never written to disk, and never held past the detection
// that asked for it.
type OCRWord struct {
	Text   string
	Bounds image.Rectangle
	// Confidence is 0..1. Windows.Media.Ocr reports no per-word score, so
	// every word carries 1. The engine has already dropped what it could not
	// read, and a floor a caller compares against keeps everything.
	Confidence float64
	// Line counts the engine's lines from 0, in the order it read them, so
	// the words of one line share it.
	Line int
}

// OCRStats describes the work one recognition did, so a caller can log it.
// It holds durations only, and nothing here derives from what was on the screen.
type OCRStats struct {
	// Recognition is how long the engine spent on the frame.
	Recognition time.Duration
}

// OCRParams is what one recognition needs beyond the pixels.
type OCRParams struct {
	// WordLevel asks for per-word boxes instead of per-line ones, which is
	// what `neru hints --split-word` means.
	WordLevel bool
	// Wide reads with the engine for the account's Chinese, Japanese or
	// Korean language instead of the profile engine. Ask WideOCRReady first.
	Wide bool
	// Accurate reads with a profile engine of the accurate read's own, on a
	// thread no fast read uses.
	Accurate bool
	// TimeoutMS bounds the recognition. Zero or less means no deadline.
	TimeoutMS int
}

const (
	ocrEngineClass          = "Windows.Media.Ocr.OcrEngine"
	softwareBitmapClass     = "Windows.Graphics.Imaging.SoftwareBitmap"
	bufferClass             = "Windows.Storage.Streams.Buffer"
	languageClass           = "Windows.Globalization.Language"
	globalizationPrefsClass = "Windows.System.UserProfile.GlobalizationPreferences"

	// bitmapPixelFormatBgra8 is BitmapPixelFormat.Bgra8, the layout the DIB
	// capture already produces before it swaps into image.RGBA order.
	bitmapPixelFormatBgra8 = 87

	// Vtable slots past the IInspectable prefix, in header order.
	ocrEngineStaticsMaxImageDimension    = inspectableSlots + 0
	ocrEngineStaticsIsLanguageSupported  = inspectableSlots + 2
	ocrEngineStaticsTryCreateFromLang    = inspectableSlots + 3
	ocrEngineStaticsTryCreateUserProfile = inspectableSlots + 4
	languageFactoryCreateLanguage        = inspectableSlots + 0
	globalizationPrefsLanguages          = inspectableSlots + 3
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

	// IBufferByteAccess derives from IUnknown, not IInspectable. Buffer is
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
	iidLanguageFactory       = mustGUID("{9B0252AC-0C27-44F8-B792-9793FB66C63E}")
	iidGlobalizationPrefs    = mustGUID("{01BF4326-ED37-4E96-B0E9-C1340D1EA158}")
)

// winrtRect is Windows.Foundation.Rect: four floats, the shape OcrWord's
// BoundingRect writes into.
type winrtRect struct {
	X, Y, Width, Height float32
}

// ocrWorker is the thread one engine lives on.
type ocrWorker struct {
	// create makes the engine from the OcrEngine statics, or returns
	// errNoEngine when there is none to make.
	create func(statics unsafe.Pointer) (unsafe.Pointer, error)

	once     sync.Once
	requests chan func()
	// initErr is why the worker could not start, read after start has
	// happened. nil means it is serving.
	initErr error

	// made is set once create has answered without an error. engine and
	// maxImage are written before it and never after, so a caller that sees
	// it set may read them from any thread.
	made atomic.Bool

	// The fields below are written only from the worker goroutine.
	engine   unsafe.Pointer
	maxImage int
}

// errNoEngine is create's answer when the account gives it no engine to
// make, which is not a failure.
var errNoEngine = errors.New("no engine to make")

var (
	// profileOCR reads in the account's profile languages, for hints and
	// every fast read.
	profileOCR = &ocrWorker{create: createProfileEngine}
	// accurateOCR reads as profileOCR does, for the accurate read.
	accurateOCR = &ocrWorker{create: createProfileEngine}
	// wideOCR reads in the account's Chinese, Japanese or Korean language.
	wideOCR = &ocrWorker{create: createWideEngine}
)

// start starts the worker's thread on first use and returns the worker.
func (w *ocrWorker) start() *ocrWorker {
	w.once.Do(func() {
		w.requests = make(chan func())
		started := make(chan struct{})

		go w.run(started)

		<-started
	})

	return w
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
	if w.made.Load() {
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

	engine, err := w.create(statics)
	if err != nil && !errors.Is(err, errNoEngine) {
		return err
	}

	w.engine = engine
	w.maxImage = int(maxImage)
	w.made.Store(true)

	return nil
}

// createProfileEngine makes the engine for the first profile language with
// an OCR pack.
func createProfileEngine(statics unsafe.Pointer) (unsafe.Pointer, error) {
	var engine unsafe.Pointer

	hresult := winrtCall(
		statics,
		ocrEngineStaticsTryCreateUserProfile,
		uintptr(unsafe.Pointer(&engine)),
	)
	if hresultFailed(hresult) {
		return nil, derrors.Wrap(
			hresultError("OcrEngine.TryCreateFromUserProfileLanguages", hresult),
			derrors.CodeInternal, "could not create the OCR engine")
	}

	// A null engine with S_OK is the documented answer for "none of the
	// profile languages has an OCR pack installed".
	if engine == nil {
		return nil, derrors.New(derrors.CodeNotSupported,
			"no OCR language pack is installed for any of this account's languages; "+
				"in Settings > Time & language > Language & region, open a language's "+
				"options and install Basic typing, or run "+
				`Add-WindowsCapability -Online -Name "Language.OCR~~~en-US~0.0.1.0"`)
	}

	return engine, nil
}

// createWideEngine makes the engine for the first Chinese, Japanese or
// Korean language in the account's list. It returns errNoEngine when the
// list has no such language, or when the profile engine already reads it
// because no language above it has an OCR pack.
func createWideEngine(statics unsafe.Pointer) (unsafe.Pointer, error) {
	tags, err := profileLanguages()
	if err != nil {
		return nil, err
	}

	profileReadsAbove := false

	for _, tag := range tags {
		wide := wideOCRTag(tag)
		if wide == "" {
			if !profileReadsAbove {
				profileReadsAbove, err = ocrLanguageSupported(statics, tag)
				if err != nil {
					return nil, err
				}
			}

			continue
		}

		engine, err := ocrEngineFor(statics, wide)
		if err != nil {
			return nil, err
		}

		if engine == nil {
			return nil, derrors.Newf(derrors.CodeNotSupported,
				"the account lists %s, but no OCR language pack reads it, so vim mode "+
					"cannot read it; in Settings > Time & language > Language & region, "+
					"open its options and install Basic typing, or install its "+
					"Language.OCR capability (Get-WindowsCapability -Online -Name 'Language.OCR*')",
				tag)
		}

		if !profileReadsAbove {
			winrtReleaseObject(engine)

			return nil, errNoEngine
		}

		return engine, nil
	}

	return nil, errNoEngine
}

// wideOCRTag is the language to build a wide engine from for the account
// language tag, or "" when tag is not Chinese, Japanese or Korean. It
// returns the tag unchanged, since the engine matches a pack by script, so a
// zh-Hant-HK tag gets the zh-TW pack. Cantonese has no pack of its own and
// reads as Traditional Chinese, as on macOS.
func wideOCRTag(tag string) string {
	primary, _, _ := strings.Cut(strings.ToLower(tag), "-")

	switch primary {
	case "zh", "ja", "ko":
		return tag
	case "yue":
		return "zh-Hant"
	default:
		return ""
	}
}

// profileLanguages is the account's language list, in order
// (GlobalizationPreferences.Languages).
func profileLanguages() ([]string, error) {
	prefs, err := activationFactory(globalizationPrefsClass, &iidGlobalizationPrefs)
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeInternal,
			"could not reach the account's language preferences")
	}
	defer winrtReleaseObject(prefs)

	var listPtr unsafe.Pointer

	hresult := winrtCall(prefs, globalizationPrefsLanguages, uintptr(unsafe.Pointer(&listPtr)))
	if hresultFailed(hresult) || listPtr == nil {
		return nil, derrors.Wrap(hresultError("GlobalizationPreferences.Languages", hresult),
			derrors.CodeInternal, "could not read the account's languages")
	}
	defer winrtReleaseObject(listPtr)

	list := vectorView{ptr: listPtr}

	count, err := list.size()
	if err != nil {
		return nil, derrors.Wrap(
			err,
			derrors.CodeInternal,
			"could not count the account's languages",
		)
	}

	tags := make([]string, 0, count)

	for index := range count {
		var tag hstring

		hresult = winrtCall(
			list.ptr,
			vectorViewGetAt,
			uintptr(index),
			uintptr(unsafe.Pointer(&tag)),
		)
		if hresultFailed(hresult) {
			return nil, derrors.Wrap(hresultError("IVectorView<HSTRING>::GetAt", hresult),
				derrors.CodeInternal, "could not read one of the account's languages")
		}

		tags = append(tags, tag.String())
		tag.delete()
	}

	return tags, nil
}

// newLanguage makes a Windows.Globalization.Language for tag. The caller
// releases it.
func newLanguage(tag string) (unsafe.Pointer, error) {
	factory, err := activationFactory(languageClass, &iidLanguageFactory)
	if err != nil {
		return nil, derrors.Wrap(
			err,
			derrors.CodeInternal,
			"could not reach the WinRT language factory",
		)
	}
	defer winrtReleaseObject(factory)

	name, err := newHString(tag)
	if err != nil {
		return nil, derrors.Wrap(
			err,
			derrors.CodeInternal,
			"could not pass a language tag to WinRT",
		)
	}
	defer name.delete()

	var language unsafe.Pointer

	hresult := winrtCall(factory, languageFactoryCreateLanguage, uintptr(name),
		uintptr(unsafe.Pointer(&language)))
	if hresultFailed(hresult) || language == nil {
		return nil, derrors.Wrap(hresultError("Language.CreateLanguage", hresult),
			derrors.CodeInternal, "could not make a language from an account language tag")
	}

	return language, nil
}

// ocrLanguageSupported reports whether an OCR pack reads tag.
func ocrLanguageSupported(statics unsafe.Pointer, tag string) (bool, error) {
	language, err := newLanguage(tag)
	if err != nil {
		return false, err
	}
	defer winrtReleaseObject(language)

	var supported bool

	hresult := winrtCall(statics, ocrEngineStaticsIsLanguageSupported, uintptr(language),
		uintptr(unsafe.Pointer(&supported)))
	if hresultFailed(hresult) {
		return false, derrors.Wrap(hresultError("OcrEngine.IsLanguageSupported", hresult),
			derrors.CodeInternal, "could not ask whether an OCR pack reads a language")
	}

	return supported, nil
}

// ocrEngineFor makes an engine for tag, or nil when no OCR pack reads it.
func ocrEngineFor(statics unsafe.Pointer, tag string) (unsafe.Pointer, error) {
	language, err := newLanguage(tag)
	if err != nil {
		return nil, err
	}
	defer winrtReleaseObject(language)

	var engine unsafe.Pointer

	hresult := winrtCall(statics, ocrEngineStaticsTryCreateFromLang, uintptr(language),
		uintptr(unsafe.Pointer(&engine)))
	if hresultFailed(hresult) {
		return nil, derrors.Wrap(hresultError("OcrEngine.TryCreateFromLanguage", hresult),
			derrors.CodeInternal, "could not create an OCR engine for an account language")
	}

	return engine, nil
}

// WideOCRReady reports whether the account lists a Chinese, Japanese or
// Korean language that a wide read would add, meaning an OCR pack reads it
// and the profile engine does not already read in it. It errors, with what
// to install, when the account lists one that no pack reads. After one call
// succeeds, later calls return without queuing behind a wide read.
func WideOCRReady() (bool, error) {
	bridge := wideOCR.start()

	if !bridge.made.Load() {
		err := bridge.do(bridge.ensureEngine)
		if err != nil {
			return false, err
		}
	}

	return bridge.engine != nil, nil
}

// OCRHealth reports whether text recognition can run on this machine, meaning
// the runtime is up and an OCR language pack is installed for one of the
// account's languages. A caller that asks is also warming the engine. The
// handle is kept for the process, and a later recognition finds it made.
func OCRHealth() error {
	bridge := profileOCR.start()

	return bridge.do(bridge.ensureEngine)
}

// RecognizeText reads the text in img and returns each run with its box, in
// img's own pixel space. WordLevel asks for one run per word. Otherwise a run
// is a line, and its box is the union of its words'.
//
// ctx is read while the engine works. A hint activation that is canceled or
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

	bridge := profileOCR

	switch {
	case params.Wide:
		bridge = wideOCR
	case params.Accurate:
		bridge = accurateOCR
	}

	bridge.start()

	err := bridge.do(func() error {
		// A read canceled while it waited behind another on the worker is
		// dropped before it converts the frame or starts the engine.
		ctxErr := ctx.Err()
		if ctxErr != nil {
			return derrors.Wrap(ctxErr, derrors.CodeContextCanceled, "text recognition canceled")
		}

		engineErr := bridge.ensureEngine()
		if engineErr != nil {
			return engineErr
		}

		if bridge.engine == nil {
			return derrors.New(derrors.CodeNotSupported,
				"the account lists no Chinese, Japanese or Korean language to read in")
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

	// The buffer's bytes are screen content. The bitmap now holds the only
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

	for index := range lineCount {
		line, lineErr := lines.at(index)
		if lineErr != nil {
			return nil, derrors.Wrap(
				lineErr,
				derrors.CodeInternal,
				"could not read a recognized line",
			)
		}

		words, err = appendLine(words, line, int(index), wordLevel, factor)
		winrtReleaseObject(line)

		if err != nil {
			return nil, err
		}
	}

	return words, nil
}

// appendLine appends line's words, or the line as one run, to words, each
// numbered index.
func appendLine(
	words []OCRWord,
	line unsafe.Pointer,
	index int,
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
				Line:       index,
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
			Line:       index,
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
