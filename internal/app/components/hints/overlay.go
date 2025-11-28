package hints

/*
#cgo CFLAGS: -x objective-c
#include "../../../core/infra/bridge/overlay.h"
#include <stdlib.h>

// Callback function that Go can reference.
extern void resizeHintCompletionCallback(void* context);
*/
import "C"

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"go.uber.org/zap"
)

const (
	// DefaultCallbackMapSize is the default size for callback maps.
	DefaultCallbackMapSize = 8

	// DefaultTimerDuration is the default timer duration.
	DefaultTimerDuration = 2 * time.Second
)

var (
	hintCallbackID  uint64
	hintCallbackMap = make(
		map[uint64]chan struct{},
		DefaultCallbackMapSize,
	) // Pre-size for typical usage
	hintCallbackLock sync.Mutex
	hintDataPool     sync.Pool
	cLabelSlicePool  sync.Pool
	hintPoolOnce     sync.Once

	// Pre-allocated common errors.
	errCreateOverlayWindow = derrors.New(
		derrors.CodeOverlayFailed,
		"failed to create overlay window",
	)
)

//export resizeHintCompletionCallback
func resizeHintCompletionCallback(context unsafe.Pointer) {
	// Convert context to callback ID
	id := uint64(uintptr(context))

	hintCallbackLock.Lock()
	if done, ok := hintCallbackMap[id]; ok {
		close(done)
		delete(hintCallbackMap, id)
	}
	hintCallbackLock.Unlock()
}

// Overlay manages the rendering of hint overlays using native platform APIs.
type Overlay struct {
	window C.OverlayWindow
	config config.HintsConfig
	logger *zap.Logger

	// Cached C strings for style properties to reduce allocations
	cachedStyleMu          sync.RWMutex
	cachedFontFamily       *C.char
	cachedBgColor          *C.char
	cachedTextColor        *C.char
	cachedMatchedTextColor *C.char
	cachedBorderColor      *C.char
	cachedHighlightColor   *C.char
}

// StyleMode represents the visual styling configuration for hint overlays.
type StyleMode struct {
	fontSize         int
	fontFamily       string
	borderRadius     int
	padding          int
	borderWidth      int
	opacity          float64
	backgroundColor  string
	textColor        string
	matchedTextColor string
	borderColor      string
}

// FontSize returns the font size.
func (s StyleMode) FontSize() int {
	return s.fontSize
}

// FontFamily returns the font family.
func (s StyleMode) FontFamily() string {
	return s.fontFamily
}

// BorderRadius returns the border radius.
func (s StyleMode) BorderRadius() int {
	return s.borderRadius
}

// Padding returns the padding.
func (s StyleMode) Padding() int {
	return s.padding
}

// BorderWidth returns the border width.
func (s StyleMode) BorderWidth() int {
	return s.borderWidth
}

// Opacity returns the opacity.
func (s StyleMode) Opacity() float64 {
	return s.opacity
}

// BackgroundColor returns the background color.
func (s StyleMode) BackgroundColor() string {
	return s.backgroundColor
}

// TextColor returns the text color.
func (s StyleMode) TextColor() string {
	return s.textColor
}

// MatchedTextColor returns the matched text color.
func (s StyleMode) MatchedTextColor() string {
	return s.matchedTextColor
}

// BorderColor returns the border color.
func (s StyleMode) BorderColor() string {
	return s.borderColor
}

// initPools initializes the object pools once.
func initPools() {
	hintPoolOnce.Do(func() {
		hintDataPool = sync.Pool{New: func() any {
			s := make([]C.HintData, 0)

			return &s
		}}
		cLabelSlicePool = sync.Pool{New: func() any {
			s := make([]*C.char, 0)

			return &s
		}}
	})
}

// NewOverlay creates a new hint overlay instance with its own window.
func NewOverlay(config config.HintsConfig, logger *zap.Logger) (*Overlay, error) {
	window := C.createOverlayWindow()
	if window == nil {
		return nil, errCreateOverlayWindow
	}
	initPools()

	return &Overlay{
		window: window,
		config: config,
		logger: logger,
	}, nil
}

// NewOverlayWithWindow creates a hint overlay instance using a shared window.
func NewOverlayWithWindow(
	config config.HintsConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) (*Overlay, error) {
	initPools()

	return &Overlay{
		window: (C.OverlayWindow)(windowPtr),
		config: config,
		logger: logger,
	}, nil
}

// Window returns the underlying C overlay window.
func (o *Overlay) Window() C.OverlayWindow {
	return o.window
}

// Config returns the hints config.
func (o *Overlay) Config() config.HintsConfig { return o.config }

// Logger returns the logger.
func (o *Overlay) Logger() *zap.Logger { return o.logger }

// Show shows the overlay.
func (o *Overlay) Show() {
	C.NeruShowOverlayWindow(o.window)
}

// Hide hides the overlay.
func (o *Overlay) Hide() {
	C.NeruHideOverlayWindow(o.window)
}

// Clear clears all hints from the overlay.
func (o *Overlay) Clear() {
	C.NeruClearOverlay(o.window)
}

// ResizeToActiveScreen resizes the overlay window to the screen containing the mouse cursor.
func (o *Overlay) ResizeToActiveScreen() {
	C.NeruResizeOverlayToActiveScreen(o.window)
}

// ResizeToActiveScreenSync resizes the overlay window synchronously with callback notification.
func (o *Overlay) ResizeToActiveScreenSync() {
	done := make(chan struct{})

	// Generate unique ID for this callback
	callbackID := atomic.AddUint64(&hintCallbackID, 1)

	// Store channel in map
	hintCallbackLock.Lock()
	hintCallbackMap[callbackID] = done
	hintCallbackLock.Unlock()

	if o.logger != nil {
		o.logger.Debug("Hint overlay resize started", zap.Uint64("callback_id", callbackID))
	}

	// Pass ID as context (safe - no Go pointers)
	// Note: uintptr conversion must happen in same expression to satisfy go vet
	C.NeruResizeOverlayToActiveScreenWithCallback(
		o.window,
		(C.ResizeCompletionCallback)(
			unsafe.Pointer(C.resizeHintCompletionCallback), //nolint:unconvert
		),
		*(*unsafe.Pointer)(unsafe.Pointer(&callbackID)),
	)

	// Don't wait for callback - continue immediately for better UX
	// The resize operation is typically fast and visually complete before callback
	// Start a goroutine to handle cleanup when callback eventually arrives
	go func() {
		if o.logger != nil {
			o.logger.Debug(
				"Hint overlay resize background cleanup started",
				zap.Uint64("callback_id", callbackID),
			)
		}

		// Use timer instead of time.After to prevent memory leaks
		timer := time.NewTimer(DefaultTimerDuration)
		defer timer.Stop()

		select {
		case <-done:
			timer.Stop() // Stop timer immediately on success
			// Callback received, normal cleanup already handled in callback
			if o.logger != nil {
				o.logger.Debug(
					"Hint overlay resize callback received",
					zap.Uint64("callback_id", callbackID),
				)
			}
		case <-timer.C:
			// Long timeout for cleanup only - callback likely failed
			hintCallbackLock.Lock()
			delete(hintCallbackMap, callbackID)
			hintCallbackLock.Unlock()

			if o.logger != nil {
				o.logger.Debug("Hint overlay resize cleanup timeout - removed callback from map",
					zap.Uint64("callback_id", callbackID))
			}
		}
	}()
}

// DrawHintsWithStyle draws hints on the overlay with custom style.
func (o *Overlay) DrawHintsWithStyle(hints []*Hint, style StyleMode) error {
	return o.drawHintsInternal(hints, style, true)
}

// DrawTargetDot draws a small circular dot at the target position.
func (o *Overlay) DrawTargetDot(
	pointX, pointY int,
	radius float64,
	color, borderColor string,
	borderWidth float64,
) error {
	center := C.CGPoint{
		x: C.double(pointX),
		y: C.double(pointY),
	}

	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor)) //nolint:nlreturn

	var cBorderColor *C.char
	if borderColor != "" {
		cBorderColor = C.CString(borderColor)
		defer C.free(unsafe.Pointer(cBorderColor)) //nolint:nlreturn
	}

	C.NeruDrawTargetDot(
		o.window,
		center,
		C.double(radius),
		cColor,
		cBorderColor,
		C.double(borderWidth),
	)

	return nil
}

// DrawScrollHighlight draws a highlight around a scroll area.
func (o *Overlay) DrawScrollHighlight(
	xCoordinate, yCoordinate, width, height int,
	color string,
	borderWidth int,
) {
	renderBounds := C.CGRect{
		origin: C.CGPoint{
			x: C.double(xCoordinate),
			y: C.double(yCoordinate),
		},
		size: C.CGSize{
			width:  C.double(width),
			height: C.double(height),
		},
	}

	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor)) //nolint:nlreturn

	C.NeruDrawScrollHighlight(o.window, renderBounds, cColor, C.int(borderWidth))
}

// BuildStyle returns StyleMode based on action name using the provided config.
func BuildStyle(cfg config.HintsConfig) StyleMode {
	style := StyleMode{
		fontSize:         cfg.FontSize,
		fontFamily:       cfg.FontFamily,
		borderRadius:     cfg.BorderRadius,
		padding:          cfg.Padding,
		borderWidth:      cfg.BorderWidth,
		opacity:          cfg.Opacity,
		backgroundColor:  cfg.BackgroundColor,
		textColor:        cfg.TextColor,
		matchedTextColor: cfg.MatchedTextColor,
		borderColor:      cfg.BorderColor,
	}

	return style
}

// UpdateConfig updates the overlay configuration.
func (o *Overlay) UpdateConfig(config config.HintsConfig) {
	o.config = config
	// Invalidate style cache when config changes
	o.freeStyleCache()
}

// Destroy destroys the overlay.
func (o *Overlay) Destroy() {
	if o.window != nil {
		o.freeStyleCache()
		C.NeruDestroyOverlayWindow(o.window)
		o.window = nil
	}
}

// freeStyleCache frees all cached C strings.
func (o *Overlay) freeStyleCache() {
	o.cachedStyleMu.Lock()
	defer o.cachedStyleMu.Unlock()

	if o.cachedFontFamily != nil {
		C.free(unsafe.Pointer(o.cachedFontFamily))
		o.cachedFontFamily = nil
	}
	if o.cachedBgColor != nil {
		C.free(unsafe.Pointer(o.cachedBgColor))
		o.cachedBgColor = nil
	}
	if o.cachedTextColor != nil {
		C.free(unsafe.Pointer(o.cachedTextColor))
		o.cachedTextColor = nil
	}
	if o.cachedMatchedTextColor != nil {
		C.free(unsafe.Pointer(o.cachedMatchedTextColor))
		o.cachedMatchedTextColor = nil
	}
	if o.cachedBorderColor != nil {
		C.free(unsafe.Pointer(o.cachedBorderColor))
		o.cachedBorderColor = nil
	}
	if o.cachedHighlightColor != nil {
		C.free(unsafe.Pointer(o.cachedHighlightColor))
		o.cachedHighlightColor = nil
	}
}

// updateStyleCacheLocked updates cached C strings for the current style.
// Must be called with cachedStyleMu write lock held.
func (o *Overlay) updateStyleCacheLocked(style StyleMode) {
	// Free old cached strings
	if o.cachedFontFamily != nil {
		C.free(unsafe.Pointer(o.cachedFontFamily))
	}
	if o.cachedBgColor != nil {
		C.free(unsafe.Pointer(o.cachedBgColor))
	}
	if o.cachedTextColor != nil {
		C.free(unsafe.Pointer(o.cachedTextColor))
	}
	if o.cachedMatchedTextColor != nil {
		C.free(unsafe.Pointer(o.cachedMatchedTextColor))
	}
	if o.cachedBorderColor != nil {
		C.free(unsafe.Pointer(o.cachedBorderColor))
	}

	// Create new cached strings
	o.cachedFontFamily = C.CString(style.FontFamily())
	o.cachedBgColor = C.CString(style.BackgroundColor())
	o.cachedTextColor = C.CString(style.TextColor())
	o.cachedMatchedTextColor = C.CString(style.MatchedTextColor())
	o.cachedBorderColor = C.CString(style.BorderColor())
}

// getCachedStyleStrings returns cached C strings for style, updating cache if needed.
func (o *Overlay) getCachedStyleStrings(
	style StyleMode,
) (*C.char, *C.char, *C.char, *C.char, *C.char) {
	o.cachedStyleMu.RLock()
	// Check if cache needs rebuild or is invalid
	if o.cachedFontFamily == nil {
		o.cachedStyleMu.RUnlock()
		o.cachedStyleMu.Lock()
		// Double-check after acquiring write lock
		if o.cachedFontFamily == nil {
			o.updateStyleCacheLocked(style)
		}
		fontFamily := o.cachedFontFamily
		bgColor := o.cachedBgColor
		textColor := o.cachedTextColor
		matchedTextColor := o.cachedMatchedTextColor
		borderColor := o.cachedBorderColor
		o.cachedStyleMu.Unlock()

		return fontFamily, bgColor, textColor, matchedTextColor, borderColor
	}

	fontFamily := o.cachedFontFamily
	bgColor := o.cachedBgColor
	textColor := o.cachedTextColor
	matchedTextColor := o.cachedMatchedTextColor
	borderColor := o.cachedBorderColor
	o.cachedStyleMu.RUnlock()

	return fontFamily, bgColor, textColor, matchedTextColor, borderColor
}

// drawHintsInternal is the internal implementation for drawing hints.
func (o *Overlay) drawHintsInternal(hints []*Hint, style StyleMode, showArrow bool) error {
	if len(hints) == 0 {
		o.Clear()

		return nil
	}

	start := time.Now()
	tmpHints := hintDataPool.Get()
	cHintsPtr, _ := tmpHints.(*[]C.HintData)
	if cap(*cHintsPtr) < len(hints) {
		s := make([]C.HintData, len(hints))
		cHintsPtr = &s
	} else {
		*cHintsPtr = (*cHintsPtr)[:len(hints)]
	}
	cHints := *cHintsPtr
	tmpCLables := cLabelSlicePool.Get()
	cLabelsPtr, _ := tmpCLables.(*[]*C.char)
	if cap(*cLabelsPtr) < len(hints) {
		s := make([]*C.char, len(hints))
		cLabelsPtr = &s
	} else {
		*cLabelsPtr = (*cLabelsPtr)[:len(hints)]
	}
	cLabels := *cLabelsPtr

	matchedCount := 0
	for i, hint := range hints {
		cLabels[i] = C.CString(hint.Label())
		cHints[i] = C.HintData{
			label: cLabels[i],
			position: C.CGPoint{
				x: C.double(hint.Position().X),
				y: C.double(hint.Position().Y),
			},
			size: C.CGSize{
				width:  C.double(hint.Size().X),
				height: C.double(hint.Size().Y),
			},
			matchedPrefixLength: C.int(len(hint.MatchedPrefix())),
		}

		if len(hint.MatchedPrefix()) > 0 {
			matchedCount++
		}
	}

	o.logger.Debug("Hint match statistics",
		zap.Int("total_hints", len(hints)),
		zap.Int("matched_hints", matchedCount))

	// Use cached style strings to avoid repeated allocations
	cFontFamily, cBgColor, cTextColor, cMatchedTextColor, cBorderColor := o.getCachedStyleStrings(
		style,
	)

	arrowFlag := 0
	if showArrow {
		arrowFlag = 1
	}

	finalStyle := C.HintStyle{
		fontSize:         C.int(style.FontSize()),
		fontFamily:       cFontFamily,
		backgroundColor:  cBgColor,
		textColor:        cTextColor,
		matchedTextColor: cMatchedTextColor,
		borderColor:      cBorderColor,
		borderRadius:     C.int(style.BorderRadius()),
		borderWidth:      C.int(style.BorderWidth()),
		padding:          C.int(style.Padding()),
		opacity:          C.double(style.Opacity()),
		showArrow:        C.int(arrowFlag),
	}

	// Draw hints
	C.NeruDrawHints(o.window, &cHints[0], C.int(len(cHints)), finalStyle)

	// Free all C strings
	for _, cLabel := range cLabels {
		C.free(unsafe.Pointer(cLabel))
	}
	*cHintsPtr = (*cHintsPtr)[:0]
	*cLabelsPtr = (*cLabelsPtr)[:0]
	hintDataPool.Put(cHintsPtr)
	cLabelSlicePool.Put(cLabelsPtr)
	// Note: We don't free cached style strings - they're reused across draws

	o.logger.Debug("Hints drawn successfully",
		zap.Duration("duration", time.Since(start)))

	return nil
}
