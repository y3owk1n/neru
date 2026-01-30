package scroll

/*
#cgo CFLAGS: -x objective-c
#include "../../../core/infra/bridge/overlay.h"
#include <stdlib.h>

// Callback function that Go can reference.
extern void resizeScrollCompletionCallback(void* context);
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/overlayutil"
	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"go.uber.org/zap"
)

const (
	// defaultIndicatorWidth is the default width for the scroll indicator.
	defaultIndicatorWidth = 60
	// defaultIndicatorHeight is the default height for the scroll indicator.
	defaultIndicatorHeight = 20
)

//export resizeScrollCompletionCallback
func resizeScrollCompletionCallback(context unsafe.Pointer) {
	// Read callback ID from the pointer (points to a slice element in callbackIDStore)
	id := *(*uint64)(context)

	// Delegate to global callback manager
	overlayutil.CompleteGlobalCallback(id)
}

// Overlay manages the rendering of scroll mode overlays using native platform APIs.
type Overlay struct {
	window          C.OverlayWindow
	config          config.ScrollConfig
	logger          *zap.Logger
	callbackManager *overlayutil.CallbackManager

	// Cached C strings for style properties
	cachedStyleMu          sync.RWMutex
	cachedFontFamily       *C.char
	cachedBgColor          *C.char
	cachedTextColor        *C.char
	cachedMatchedTextColor *C.char
	cachedBorderColor      *C.char
}

// NewOverlay initializes a new scroll overlay instance with its own window.
func NewOverlay(config config.ScrollConfig, logger *zap.Logger) (*Overlay, error) {
	window := C.createOverlayWindow()
	if window == nil {
		return nil, derrors.New(derrors.CodeOverlayFailed, "failed to create overlay window")
	}

	return &Overlay{
		window:          window,
		config:          config,
		logger:          logger,
		callbackManager: overlayutil.NewCallbackManager(logger),
	}, nil
}

// NewOverlayWithWindow initializes a scroll overlay instance using a shared window.
func NewOverlayWithWindow(
	config config.ScrollConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) (*Overlay, error) {
	return &Overlay{
		window:          (C.OverlayWindow)(windowPtr),
		config:          config,
		logger:          logger,
		callbackManager: overlayutil.NewCallbackManager(logger),
	}, nil
}

// Window returns the underlying C overlay window.
func (o *Overlay) Window() C.OverlayWindow {
	return o.window
}

// Config returns the scroll configuration.
func (o *Overlay) Config() config.ScrollConfig {
	return o.config
}

// Logger returns the logger.
func (o *Overlay) Logger() *zap.Logger {
	return o.logger
}

// Show displays the scroll overlay window.
func (o *Overlay) Show() {
	C.NeruShowOverlayWindow(o.window)
}

// Hide conceals the scroll overlay window.
func (o *Overlay) Hide() {
	C.NeruHideOverlayWindow(o.window)
}

// Clear removes all scroll highlights from the overlay.
func (o *Overlay) Clear() {
	C.NeruClearOverlay(o.window)
}

// ResizeToActiveScreen adjusts the overlay window size with callback notification.
func (o *Overlay) ResizeToActiveScreen() {
	o.callbackManager.StartResizeOperation(func(callbackID uint64) {
		// Pass integer ID as opaque pointer context for C callback.
		// Uses CallbackIDToPointer to convert in a way that go vet accepts.
		contextPtr := overlayutil.CallbackIDToPointer(callbackID)

		C.NeruResizeOverlayToActiveScreenWithCallback(
			o.window,
			(C.ResizeCompletionCallback)(C.resizeScrollCompletionCallback),
			contextPtr,
		)
	})
}

// DrawScrollIndicator draws a "Scroll" indicator at the specified position.
func (o *Overlay) DrawScrollIndicator(xCoordinate, yCoordinate int) {
	// Offset from cursor to avoid covering it
	xOffset := o.config.IndicatorXOffset
	yOffset := o.config.IndicatorYOffset

	label := C.CString("Scroll")
	defer C.free(unsafe.Pointer(label)) //nolint:nlreturn

	// Create a single hint for the indicator
	hint := C.HintData{
		label: label,
		position: C.CGPoint{
			x: C.double(xCoordinate + xOffset),
			y: C.double(yCoordinate + yOffset),
		},
		size: C.CGSize{
			// Size is not strictly needed for just drawing the label, but providing reasonable defaults
			width:  defaultIndicatorWidth,
			height: defaultIndicatorHeight,
		},
		matchedPrefixLength: 0,
	}

	// Use cached style strings to avoid repeated allocations and fix use-after-free
	cFontFamily, cBgColor, cTextColor, cMatchedTextColor, cBorderColor := o.getCachedStyleStrings()

	style := C.HintStyle{
		fontSize:         C.int(o.config.FontSize),
		fontFamily:       cFontFamily,
		backgroundColor:  cBgColor,
		textColor:        cTextColor,
		matchedTextColor: cMatchedTextColor,
		borderColor:      cBorderColor,
		borderRadius:     C.int(o.config.BorderRadius),
		borderWidth:      C.int(o.config.BorderWidth),
		padding:          C.int(o.config.Padding),
		opacity:          C.double(o.config.Opacity),
		showArrow:        0, // No arrow for scroll indicator
	}

	// Reuse NeruDrawHints which can draw arbitrary labels
	C.NeruDrawHints(o.window, &hint, 1, style)
}

// UpdateConfig updates the overlay configuration.
func (o *Overlay) UpdateConfig(config config.ScrollConfig) {
	o.config = config
	// Invalidate style cache when config changes
	o.freeStyleCache()
}

// Destroy releases the overlay window resources.
func (o *Overlay) Destroy() {
	// Clean up callback manager first to stop background goroutines
	if o.callbackManager != nil {
		o.callbackManager.Cleanup()
	}

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

	bridge.FreeCString(unsafe.Pointer(o.cachedFontFamily))
	o.cachedFontFamily = nil
	bridge.FreeCString(unsafe.Pointer(o.cachedBgColor))
	o.cachedBgColor = nil
	bridge.FreeCString(unsafe.Pointer(o.cachedTextColor))
	o.cachedTextColor = nil
	bridge.FreeCString(unsafe.Pointer(o.cachedMatchedTextColor))
	o.cachedMatchedTextColor = nil
	bridge.FreeCString(unsafe.Pointer(o.cachedBorderColor))
	o.cachedBorderColor = nil
}

// updateStyleCacheLocked updates cached C strings for the current style.
// Must be called with cachedStyleMu write lock held.
func (o *Overlay) updateStyleCacheLocked() {
	bridge.FreeCString(unsafe.Pointer(o.cachedFontFamily))
	bridge.FreeCString(unsafe.Pointer(o.cachedBgColor))
	bridge.FreeCString(unsafe.Pointer(o.cachedTextColor))
	bridge.FreeCString(unsafe.Pointer(o.cachedMatchedTextColor))
	bridge.FreeCString(unsafe.Pointer(o.cachedBorderColor))

	o.cachedFontFamily = C.CString(o.config.FontFamily)
	o.cachedBgColor = C.CString(o.config.BackgroundColor)
	o.cachedTextColor = C.CString(o.config.TextColor)
	o.cachedMatchedTextColor = C.CString(o.config.TextColor) // No matching in scroll mode
	o.cachedBorderColor = C.CString(o.config.BorderColor)
}

// getCachedStyleStrings returns cached C strings for style, updating cache if needed.
func (o *Overlay) getCachedStyleStrings() (*C.char, *C.char, *C.char, *C.char, *C.char) {
	o.cachedStyleMu.RLock()
	// Check if cache needs rebuild or is invalid
	if o.cachedFontFamily == nil {
		o.cachedStyleMu.RUnlock()
		o.cachedStyleMu.Lock()
		// Double-check after acquiring write lock
		if o.cachedFontFamily == nil {
			o.updateStyleCacheLocked()
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
