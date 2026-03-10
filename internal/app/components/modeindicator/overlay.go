// Package modeindicator provides the mode indicator overlay component.
package modeindicator

/*
#cgo CFLAGS: -x objective-c
#include "../../../core/infra/bridge/overlay.h"
#include <stdlib.h>

// Callback function that Go can reference.
extern void resizeModeIndicatorCompletionCallback(void* context);
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/overlayutil"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"go.uber.org/zap"
)

const (
	// defaultIndicatorWidth is the default width for the mode indicator.
	defaultIndicatorWidth = 60
	// defaultIndicatorHeight is the default height for the mode indicator.
	defaultIndicatorHeight = 20

	// NSWindowSharingNone represents NSWindowSharingNone (0) - hidden from screen sharing.
	NSWindowSharingNone = 0
	// NSWindowSharingReadOnly represents NSWindowSharingReadOnly (1) - visible in screen sharing.
	NSWindowSharingReadOnly = 1
)

//export resizeModeIndicatorCompletionCallback
func resizeModeIndicatorCompletionCallback(context unsafe.Pointer) {
	// Read callback context from the C-heap-allocated CallbackContext
	ctx := *(*overlayutil.CallbackContext)(context)

	// Free the C-allocated context now that we've copied the values
	overlayutil.FreeCallbackContext(context)

	// Delegate to global callback manager
	overlayutil.CompleteGlobalCallback(ctx.CallbackID, ctx.Generation)
}

// Overlay manages the rendering of mode indicator overlays using native platform APIs.
type Overlay struct {
	window          C.OverlayWindow
	indicatorConfig config.ModeIndicatorConfig
	theme           config.ThemeProvider
	logger          *zap.Logger
	callbackManager *overlayutil.CallbackManager
	styleCache      *overlayutil.StyleCache

	// configMu protects indicatorConfig from concurrent read/write.
	configMu sync.RWMutex

	// lastDrawnMode tracks the mode whose colors are currently cached in
	// styleCache. When the mode changes the cache is invalidated so that
	// per-mode color overrides take effect.
	lastDrawnMode string

	// Cached C strings for indicator labels to avoid malloc/free per draw
	labelCacheMu sync.RWMutex
	cachedLabels map[string]*C.char
	// drawMu serializes draw operations against cache invalidation.
	// Draw paths hold RLock; freeAllCaches holds Lock.
	drawMu sync.RWMutex
}

// NewOverlay initializes a new mode indicator overlay instance with its own window.
func NewOverlay(
	indicatorCfg config.ModeIndicatorConfig,
	theme config.ThemeProvider,
	logger *zap.Logger,
) (*Overlay, error) {
	base, err := overlayutil.NewBaseOverlay(logger)
	if err != nil {
		return nil, err
	}

	return &Overlay{
		window:          (C.OverlayWindow)(base.Window),
		indicatorConfig: indicatorCfg,
		theme:           theme,
		logger:          logger,
		callbackManager: base.CallbackManager,
		styleCache:      base.StyleCache,
		cachedLabels:    make(map[string]*C.char),
	}, nil
}

// NewOverlayWithWindow initializes a mode indicator overlay instance using a shared window.
func NewOverlayWithWindow(
	indicatorCfg config.ModeIndicatorConfig,
	theme config.ThemeProvider,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) (*Overlay, error) {
	base := overlayutil.NewBaseOverlayWithWindow(logger, windowPtr)

	return &Overlay{
		window:          (C.OverlayWindow)(base.Window),
		indicatorConfig: indicatorCfg,
		theme:           theme,
		logger:          logger,
		callbackManager: base.CallbackManager,
		styleCache:      base.StyleCache,
		cachedLabels:    make(map[string]*C.char),
	}, nil
}

// Window returns the underlying C overlay window.
func (o *Overlay) Window() C.OverlayWindow {
	return o.window
}

// Logger returns the logger.
func (o *Overlay) Logger() *zap.Logger {
	return o.logger
}

// Show displays the mode indicator overlay window.
func (o *Overlay) Show() {
	C.NeruShowOverlayWindow(o.window)
}

// Hide conceals the mode indicator overlay window.
func (o *Overlay) Hide() {
	C.NeruHideOverlayWindow(o.window)
}

// Clear removes all highlights from the overlay.
func (o *Overlay) Clear() {
	C.NeruClearOverlay(o.window)
}

// ResizeToActiveScreen adjusts the overlay window size with callback notification.
// Falls back to a non-callback resize if the callback ID pool is exhausted.
func (o *Overlay) ResizeToActiveScreen() {
	started := o.callbackManager.StartResizeOperation(func(callbackID uint64, generation uint64) {
		// Pass callback ID and generation as opaque pointer context for C callback.
		// Uses CallbackIDToPointer to convert in a way that go vet accepts.
		contextPtr := overlayutil.CallbackIDToPointer(callbackID, generation)

		C.NeruResizeOverlayToActiveScreenWithCallback(
			o.window,
			(C.ResizeCompletionCallback)(C.resizeModeIndicatorCompletionCallback),
			contextPtr,
		)
	})
	if !started {
		// Pool exhausted — fall back to non-callback resize so the overlay
		// is still moved to the correct screen.
		C.NeruResizeOverlayToActiveScreen(o.window)
	}
}

// DrawModeIndicator draws a mode label at the specified position.
// The mode parameter is the overlay mode string matching domain.ModeName*
// constants (e.g. "hints", "grid", "scroll", "recursive_grid").
// The label text is resolved from config,
// allowing users to customize (or hide) each mode's indicator text.
// The caller is responsible for calling Show() once before the first draw
// (e.g. in startModeIndicatorPolling) rather than showing every tick.
func (o *Overlay) DrawModeIndicator(mode string, xCoordinate, yCoordinate int) {
	// Hold configMu.RLock for entire draw to prevent SetConfig from
	// writing to indicatorConfig while we read it.
	o.configMu.RLock()
	defer o.configMu.RUnlock()

	labelText := o.resolveLabelText(mode)
	if labelText == "" {
		return
	}

	// Offset from cursor to avoid covering it
	xOffset := o.indicatorConfig.UI.IndicatorXOffset
	yOffset := o.indicatorConfig.UI.IndicatorYOffset

	// Invalidate the style cache when the mode changes so that per-mode
	// color overrides are re-resolved instead of reusing stale values.
	// Both the read and write of lastDrawnMode must happen under drawMu
	// to avoid racing with freeAllCaches (which also writes lastDrawnMode
	// and frees the cache under drawMu.Lock).
	o.drawMu.RLock()
	needsInvalidation := mode != o.lastDrawnMode
	o.drawMu.RUnlock()
	if needsInvalidation {
		o.drawMu.Lock()
		// Double-check after acquiring the write lock, in case
		// freeAllCaches ran between the check and the lock acquisition.
		if mode != o.lastDrawnMode {
			o.styleCache.Free()
			o.lastDrawnMode = mode
		}
		o.drawMu.Unlock()
	}

	// Hold drawMu.RLock for the entire span from label lookup through the C
	// draw call so that freeAllCaches (which takes drawMu.Lock) cannot free
	// the C strings while they are still referenced.
	o.drawMu.RLock()
	label := o.getOrCacheLabel(labelText)

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

	// Resolve per-mode color overrides (falls back to UI defaults).
	modeConfig := o.resolveModeConfig(mode)

	if modeConfig == nil {
		o.drawMu.RUnlock()

		return
	}

	// Use cached style strings to avoid repeated allocations and fix use-after-free
	cachedStyle := o.styleCache.Get(func(cached *overlayutil.CachedStyle) {
		cached.FontFamily = unsafe.Pointer(C.CString(o.indicatorConfig.UI.FontFamily))
		cached.BgColor = unsafe.Pointer(
			C.CString(
				config.ResolveColorWithOverride(
					modeConfig.BackgroundColorLight,
					modeConfig.BackgroundColorDark,
					o.indicatorConfig.UI.BackgroundColorLight,
					o.indicatorConfig.UI.BackgroundColorDark,
					o.theme,
					config.ModeIndicatorBackgroundColorLight,
					config.ModeIndicatorBackgroundColorDark,
				),
			),
		)
		cached.TextColor = unsafe.Pointer(
			C.CString(
				config.ResolveColorWithOverride(
					modeConfig.TextColorLight,
					modeConfig.TextColorDark,
					o.indicatorConfig.UI.TextColorLight,
					o.indicatorConfig.UI.TextColorDark,
					o.theme,
					config.ModeIndicatorTextColorLight,
					config.ModeIndicatorTextColorDark,
				),
			),
		)
		cached.MatchedTextColor = unsafe.Pointer(
			C.CString(
				config.ResolveColorWithOverride(
					modeConfig.TextColorLight,
					modeConfig.TextColorDark,
					o.indicatorConfig.UI.TextColorLight,
					o.indicatorConfig.UI.TextColorDark,
					o.theme,
					config.ModeIndicatorTextColorLight,
					config.ModeIndicatorTextColorDark,
				),
			),
		) // No matching in indicator mode
		cached.BorderColor = unsafe.Pointer(
			C.CString(
				config.ResolveColorWithOverride(
					modeConfig.BorderColorLight,
					modeConfig.BorderColorDark,
					o.indicatorConfig.UI.BorderColorLight,
					o.indicatorConfig.UI.BorderColorDark,
					o.theme,
					config.ModeIndicatorBorderColorLight,
					config.ModeIndicatorBorderColorDark,
				),
			),
		)
	})

	style := C.HintStyle{
		fontSize:         C.int(o.indicatorConfig.UI.FontSize),
		fontFamily:       (*C.char)(cachedStyle.FontFamily),
		backgroundColor:  (*C.char)(cachedStyle.BgColor),
		textColor:        (*C.char)(cachedStyle.TextColor),
		matchedTextColor: (*C.char)(cachedStyle.MatchedTextColor),
		borderColor:      (*C.char)(cachedStyle.BorderColor),
		borderRadius:     C.int(o.indicatorConfig.UI.BorderRadius),
		borderWidth:      C.int(o.indicatorConfig.UI.BorderWidth),
		paddingX:         C.int(o.indicatorConfig.UI.PaddingX),
		paddingY:         C.int(o.indicatorConfig.UI.PaddingY),
		showArrow:        0, // No arrow for mode indicator
	}

	// Reuse NeruDrawHints which can draw arbitrary labels
	C.NeruDrawHints(o.window, &hint, 1, style)

	o.drawMu.RUnlock()
}

// SetConfig sets the overlay configuration.
func (o *Overlay) SetConfig(indicatorCfg config.ModeIndicatorConfig) {
	o.configMu.Lock()
	o.indicatorConfig = indicatorCfg
	o.configMu.Unlock()
	// Invalidate caches when config changes
	o.freeAllCaches()
}

// SetSharingType sets the window sharing type for screen sharing visibility.
func (o *Overlay) SetSharingType(hide bool) {
	sharingType := C.int(NSWindowSharingReadOnly)
	if hide {
		sharingType = C.int(NSWindowSharingNone)
	}

	C.NeruSetOverlaySharingType(o.window, sharingType)
}

// Cleanup frees Go-side resources (callbackManager, styleCache, labelCache)
// without destroying the native window.
func (o *Overlay) Cleanup() {
	if o.callbackManager != nil {
		o.callbackManager.Cleanup()
	}
	o.freeAllCaches()
}

// Destroy releases the overlay window resources.
func (o *Overlay) Destroy() {
	o.Cleanup()

	if o.window != nil {
		C.NeruDestroyOverlayWindow(o.window)
		o.window = nil
	}
}

// resolveModeConfig returns the per-mode config for the given mode.
// Caller must hold configMu.RLock.
func (o *Overlay) resolveModeConfig(mode string) *config.ModeIndicatorModeConfig {
	switch mode {
	case domain.ModeNameHints:
		return &o.indicatorConfig.Hints
	case domain.ModeNameGrid:
		return &o.indicatorConfig.Grid
	case domain.ModeNameScroll:
		return &o.indicatorConfig.Scroll
	case domain.ModeNameRecursiveGrid:
		return &o.indicatorConfig.RecursiveGrid
	default:
		return nil
	}
}

// resolveLabelText returns the configured label text for the given mode.
// Caller must hold configMu.RLock.
func (o *Overlay) resolveLabelText(mode string) string {
	mc := o.resolveModeConfig(mode)
	if mc == nil {
		return ""
	}

	return mc.Text
}

// freeAllCaches frees both the style cache and the label cache under drawMu
// so that no in-flight draw can reference freed C pointers.
func (o *Overlay) freeAllCaches() {
	o.drawMu.Lock()
	defer o.drawMu.Unlock()
	o.styleCache.Free()
	o.lastDrawnMode = ""
	o.freeLabelCacheLocked()
}

// freeLabelCacheLocked frees all cached label C strings.
// Caller must hold drawMu.Lock.
func (o *Overlay) freeLabelCacheLocked() {
	o.labelCacheMu.Lock()
	defer o.labelCacheMu.Unlock()
	for _, cStr := range o.cachedLabels {
		if cStr != nil {
			C.free(unsafe.Pointer(cStr))
		}
	}
	// Re-initialize map to clear references
	o.cachedLabels = make(map[string]*C.char)
}

// getOrCacheLabel returns a cached C string for the label, creating it if needed.
func (o *Overlay) getOrCacheLabel(label string) *C.char {
	o.labelCacheMu.RLock()
	if cStr, ok := o.cachedLabels[label]; ok {
		o.labelCacheMu.RUnlock()

		return cStr
	}
	o.labelCacheMu.RUnlock()
	o.labelCacheMu.Lock()
	defer o.labelCacheMu.Unlock()
	// Double-check
	if cStr, ok := o.cachedLabels[label]; ok {
		return cStr
	}
	cStr := C.CString(label)
	o.cachedLabels[label] = cStr

	return cStr
}
