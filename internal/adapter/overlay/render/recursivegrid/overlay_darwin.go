//go:build darwin

package recursivegrid

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#include "../../../platform/darwin/overlay.h"
#include <stdlib.h>

// Callback function that Go can reference.
extern void recursiveGridResizeCompletionCallback(void* context);
*/
import "C"

import (
	"image"
	"strings"
	"sync"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/overlayutil"
	// The overlay.h included above only declares its symbols; importing the
	// bridge is what links the Objective-C that defines them (ADR 0009).
	_ "github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
)

//export recursiveGridResizeCompletionCallback
func recursiveGridResizeCompletionCallback(context unsafe.Pointer) {
	// Read callback context from the C-heap-allocated CallbackContext
	ctx := *(*overlayutil.CallbackContext)(context)
	// Free the C-allocated context now that we've copied the values
	overlayutil.FreeCallbackContext(context)
	overlayutil.CompleteGlobalCallback(ctx.CallbackID, ctx.Generation)
}

const (
	// NSWindowSharingNone represents NSWindowSharingNone (0) - hidden from screen sharing.
	NSWindowSharingNone = 0
	// NSWindowSharingReadOnly represents NSWindowSharingReadOnly (1) - visible in screen sharing.
	NSWindowSharingReadOnly = 1
	// millisecondsPerSecond converts config milliseconds into native seconds.
	millisecondsPerSecond = 1000.0
)

// Overlay manages the rendering of recursive_grid overlays using native platform APIs.
type Overlay struct {
	window     C.OverlayWindow
	config     config.RecursiveGridConfig
	logger     *zap.Logger
	lastBounds image.Rectangle
	lastDepth  int
	hasLast    bool

	// configMu protects config from concurrent read/write.
	configMu sync.RWMutex

	callbackManager     *overlayutil.CallbackManager
	styleCache          *overlayutil.StyleCache
	labelCacheMu        sync.RWMutex
	cachedLabels        map[string]*C.char
	virtualPointerCfg   config.VirtualPointerUI
	virtualPointerColor string

	// drawMu serializes draw operations against cache invalidation.
	// Draw paths hold RLock; freeAllCaches holds Lock.
	drawMu sync.RWMutex
}

// NewOverlay creates a new recursive_grid overlay instance.
func NewOverlay(cfg config.RecursiveGridConfig, logger *zap.Logger) (*Overlay, error) {
	base, err := overlayutil.NewBaseOverlay(logger)
	if err != nil {
		return nil, err
	}
	base.CallbackManager.SetComponent("recursivegrid")

	return &Overlay{
		window:          C.OverlayWindow(base.Window),
		config:          cfg,
		logger:          logger,
		callbackManager: base.CallbackManager,
		styleCache:      base.StyleCache,
		cachedLabels:    make(map[string]*C.char),
	}, nil
}

// NewOverlayWithWindow creates a recursive_grid overlay instance using a shared window.
func NewOverlayWithWindow(
	cfg config.RecursiveGridConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) *Overlay {
	base := overlayutil.NewBaseOverlayWithWindow(logger, windowPtr)
	base.CallbackManager.SetComponent("recursivegrid")

	return &Overlay{
		window:          C.OverlayWindow(base.Window),
		config:          cfg,
		logger:          logger,
		callbackManager: base.CallbackManager,
		styleCache:      base.StyleCache,
		cachedLabels:    make(map[string]*C.char),
	}
}

// Window returns the overlay window.
func (o *Overlay) Window() C.OverlayWindow {
	return o.window
}

// Config returns the recursive_grid config.
func (o *Overlay) Config() config.RecursiveGridConfig {
	o.configMu.RLock()
	defer o.configMu.RUnlock()

	return o.config
}

// Logger returns the logger.
func (o *Overlay) Logger() *zap.Logger {
	return o.logger
}

// SetConfig updates the overlay's config.
func (o *Overlay) SetConfig(cfg config.RecursiveGridConfig) {
	o.configMu.Lock()
	o.config = cfg
	o.configMu.Unlock()

	o.freeAllCaches()
}

// SetVirtualPointerConfig stores the virtual pointer UI config for char rendering.
// color is the resolved text color hex string (already resolved for the active theme).
func (o *Overlay) SetVirtualPointerConfig(cfg config.VirtualPointerUI, color string) {
	o.configMu.Lock()
	o.virtualPointerCfg = cfg
	o.virtualPointerColor = color
	o.configMu.Unlock()
}

// Show displays the overlay window.
func (o *Overlay) Show() {
	C.NeruShowOverlayWindow(o.window)
}

// Hide hides the overlay window.
func (o *Overlay) Hide() {
	C.NeruHideOverlayWindow(o.window)
}

// Clear clears the overlay window and resets state.
func (o *Overlay) Clear() {
	C.NeruClearOverlay(o.window)
	o.lastBounds = image.Rectangle{}
	o.lastDepth = 0
	o.hasLast = false
}

// ShowVirtualPointer renders a virtual pointer at the current selection point.
func (o *Overlay) ShowVirtualPointer(
	point image.Point,
	size int,
	fillColor string,
) {
	o.configMu.RLock()
	cfg := o.virtualPointerCfg
	color := o.virtualPointerColor
	o.configMu.RUnlock()

	cFillColor := C.CString(fillColor)
	defer C.free(unsafe.Pointer(cFillColor))

	cLabelChar := C.CString(cfg.Char)
	cFontFamily := C.CString(cfg.FontFamily)
	cTextColor := C.CString(color)
	defer C.free(unsafe.Pointer(cLabelChar))
	defer C.free(unsafe.Pointer(cFontFamily))
	defer C.free(unsafe.Pointer(cTextColor))

	indicatorStyle := C.CursorIndicatorStyle{
		radius:     C.double(size),
		fillColor:  cFillColor,
		labelChar:  cLabelChar,
		fontFamily: cFontFamily,
		fontSize:   C.int(cfg.FontSize),
		textColor:  cTextColor,
	}

	C.NeruShowCursorIndicator(
		o.window,
		C.CGPoint{x: C.double(point.X), y: C.double(point.Y)},
		indicatorStyle,
	)
}

// HideVirtualPointer removes the virtual pointer from the overlay.
func (o *Overlay) HideVirtualPointer() {
	C.NeruHideCursorIndicator(o.window)
}

// Cleanup frees Go-side resources (callbackManager, styleCache, labelCache)
// without destroying the native window. Use this for overlays that share a
// window managed by the overlay Manager.
func (o *Overlay) Cleanup() {
	if o.callbackManager != nil {
		o.callbackManager.Cleanup()
	}
	o.freeAllCaches()
}

// Destroy destroys the overlay window.
func (o *Overlay) Destroy() {
	o.Cleanup()

	if o.window != nil {
		C.NeruDestroyOverlayWindow(o.window)
		o.window = nil
	}
}

// ReplaceWindow atomically replaces the underlying overlay window.
func (o *Overlay) ReplaceWindow() {
	C.NeruReplaceOverlayWindow(&o.window)
}

// ResizeToMainScreen resizes the overlay window to the current main screen.
func (o *Overlay) ResizeToMainScreen() {
	C.NeruResizeOverlayToMainScreen(o.window)
}

// ResizeToActiveScreen resizes the overlay window with callback notification.
// Falls back to a non-callback resize if the callback ID pool is exhausted.
func (o *Overlay) ResizeToActiveScreen() {
	started := o.callbackManager.StartResizeOperation(func(callbackID uint64, generation uint64) {
		// Pass callback ID and generation as opaque pointer context for C callback.
		// Uses CallbackIDToPointer to convert in a way that go vet accepts.
		contextPtr := overlayutil.CallbackIDToPointer(callbackID, generation)
		C.NeruResizeOverlayToActiveScreenWithCallback(
			o.window,
			C.ResizeCompletionCallback(C.recursiveGridResizeCompletionCallback),
			contextPtr,
		)
	})
	if !started {
		// Pool exhausted — fall back to non-callback resize so the overlay
		// is still moved to the correct screen.
		C.NeruResizeOverlayToActiveScreen(o.window)
	}
}

// DrawRecursiveGrid renders the recursive_grid with current bounds, depth, keys
// and the dimensions the region is divided into.
// nextKeys/nextDims describe the *next* depth's layout and are used
// for the sub-key preview mini-grid inside each cell.
//
// The dimensions arrive as domain.GridDimensions rather than as a column count
// beside a row count so that this backend has no pair to transpose on its way
// to ComputeGridCells (#1313).
func (o *Overlay) DrawRecursiveGrid(
	bounds image.Rectangle,
	depth int,
	keys string,
	dims domain.GridDimensions,
	nextKeys string,
	nextDims domain.GridDimensions,
	style Style,
	virtualPointer VirtualPointerState,
) error {
	if bounds.Empty() {
		o.Clear()

		return nil
	}

	if ce := o.logger.Check(zap.DebugLevel, "Drawing recursive-grid"); ce != nil {
		ce.Write(
			zap.Int("bounds_x", bounds.Min.X),
			zap.Int("bounds_y", bounds.Min.Y),
			zap.Int("bounds_width", bounds.Dx()),
			zap.Int("bounds_height", bounds.Dy()),
			zap.Int("depth", depth),
			zap.Int("grid_cols", dims.Cols),
			zap.Int("grid_rows", dims.Rows),
			zap.String("keys", keys),
		)
	}

	// A shape that cannot narrow anything is replaced with the default one;
	// recursivegrid.UsableDimensions owns that rule for the manager and for
	// this draw alike. The key mapping goes with it: the one handed over was
	// cut for the shape being replaced, so it is the wrong length for the
	// fallback and would leave cells unlabelled.
	usableDims, asGiven := recursivegrid.UsableDimensions(dims)
	if !asGiven {
		keys = recursivegrid.DefaultKeys
	}

	dims = usableDims
	keyCount := dims.CellCount()

	// Validate keys length matches grid dimensions
	keyRunes := []rune(keys)
	if len(keyRunes) != keyCount {
		o.logger.Warn(
			"Keys length mismatch in DrawRecursiveGrid, some cells will have empty labels",
			zap.Int("key_count", len(keyRunes)),
			zap.Int("expected", keyCount),
		)
	} // Hold drawMu.RLock for the entire span from label lookup through the C
	// draw call so that freeLabelCache cannot free labels mid-draw.
	o.drawMu.RLock()

	// Compute cell positions using the shared helper (same as Divide()).
	cellRects := recursivegrid.ComputeGridCells(bounds, dims)

	cells := make([]C.GridCell, keyCount)

	for idx, cellRect := range cellRects {
		labelStr := ""
		if idx < len(keyRunes) {
			labelStr = string(keyRunes[idx])
		}
		label := style.LabelChar()
		if label == "" {
			label = strings.ToUpper(labelStr)
		}
		cells[idx] = C.GridCell{
			label:               o.getOrCacheLabel(label),
			bounds:              o.rectToCRect(cellRect),
			isMatched:           C.int(0),
			isSubgrid:           C.int(0),
			matchedPrefixLength: C.int(0),
		}
	}

	// Build sub-key preview labels.
	// When a label char override is set, repeat it to match the grid cell count
	// so the native renderer gets the expected number of labels.
	subKeyLabel := style.SubKeyPreviewLabelChar()
	if subKeyLabel != "" {
		subKeyLabel = strings.Repeat(subKeyLabel, nextDims.CellCount())
	} else {
		subKeyLabel = strings.ToUpper(nextKeys)
	}

	cachedStyle := o.styleCache.Get(func(cached *overlayutil.CachedStyle) {
		cached.FontFamily = unsafe.Pointer(C.CString(style.FontFamily()))
		cached.BgColor = unsafe.Pointer(C.CString(style.HighlightColor()))
		cached.LabelBgColor = unsafe.Pointer(C.CString(style.LabelBackgroundColor()))
		cached.TextColor = unsafe.Pointer(C.CString(style.TextColor()))
		cached.MatchedTextColor = unsafe.Pointer(C.CString(style.TextColor()))
		cached.MatchedBgColor = unsafe.Pointer(C.CString(style.HighlightColor()))
		cached.MatchedBorderColor = unsafe.Pointer(C.CString(style.LineColor()))
		cached.BorderColor = unsafe.Pointer(C.CString(style.LineColor()))
		cached.SubKeyTextColor = unsafe.Pointer(C.CString(style.SubKeyPreviewTextColor()))
		cached.SubKeyFontFamily = unsafe.Pointer(C.CString(style.FontFamily()))
	})

	finalStyle := C.GridCellStyle{
		fontSize:                    C.int(style.FontSize()),
		fontFamily:                  (*C.char)(cachedStyle.FontFamily),
		backgroundColor:             (*C.char)(cachedStyle.BgColor),
		labelBackgroundColor:        (*C.char)(cachedStyle.LabelBgColor),
		textColor:                   (*C.char)(cachedStyle.TextColor),
		matchedTextColor:            (*C.char)(cachedStyle.MatchedTextColor),
		matchedBackgroundColor:      (*C.char)(cachedStyle.MatchedBgColor),
		matchedBorderColor:          (*C.char)(cachedStyle.MatchedBorderColor),
		borderColor:                 (*C.char)(cachedStyle.BorderColor),
		borderWidth:                 C.int(style.LineWidth()),
		drawLabelBackground:         C.int(boolToInt(style.LabelBackground())),
		labelBackgroundPaddingX:     C.int(style.LabelBackgroundPaddingX()),
		labelBackgroundPaddingY:     C.int(style.LabelBackgroundPaddingY()),
		labelBackgroundBorderRadius: C.int(style.LabelBackgroundBorderRadius()),
		labelBackgroundBorderWidth:  C.int(style.LabelBackgroundBorderWidth()),
		subKeyGridCols:              C.int(nextDims.Cols),
		subKeyGridRows:              C.int(nextDims.Rows),
		drawSubKeyPreview: C.int(boolToInt(
			style.PreviewsNextDepth(len(nextKeys), nextDims),
		)),
		labelAutohideMultiplier:  C.float(style.LabelAutohideMultiplier()),
		subKeyFontSize:           C.int(style.SubKeyPreviewFontSize()),
		subKeyFontFamily:         (*C.char)(cachedStyle.SubKeyFontFamily),
		subKeyAutohideMultiplier: C.float(style.SubKeyPreviewAutohideMultiplier()),
		subKeyTextColor:          (*C.char)(cachedStyle.SubKeyTextColor),
		subKeyKeys:               o.getOrCacheLabel(subKeyLabel),
	}

	shouldAnimate := o.Config().Animation.Enabled && o.hasLast && depth != o.lastDepth &&
		!o.lastBounds.Empty()
	transitionDurationSeconds := float64(o.Config().Animation.DurationMS) / millisecondsPerSecond
	if shouldAnimate {
		C.NeruAnimateRecursiveGridTransition(
			o.window,
			&cells[0],
			C.int(len(cells)),
			finalStyle,
			C.double(transitionDurationSeconds),
		)
	} else {
		C.NeruDrawGridCells(o.window, &cells[0], C.int(len(cells)), finalStyle)
	}

	if virtualPointer.Visible {
		o.ShowVirtualPointer(
			virtualPointer.Position,
			virtualPointer.Size,
			virtualPointer.FillColor,
		)
	} else {
		o.HideVirtualPointer()
	}

	o.drawMu.RUnlock()
	o.lastBounds = bounds
	o.lastDepth = depth
	o.hasLast = true

	return nil
}

// SetSharingType sets the window sharing type for screen sharing visibility.
func (o *Overlay) SetSharingType(hide bool) {
	sharingType := C.int(NSWindowSharingReadOnly)
	if hide {
		sharingType = C.int(NSWindowSharingNone)
	}

	C.NeruSetOverlaySharingType(o.window, sharingType)
}

// rectToCRect converts a Go image.Rectangle to a C CGRect.
func (o *Overlay) rectToCRect(rect image.Rectangle) C.CGRect {
	return C.CGRect{
		origin: C.CGPoint{
			x: C.double(rect.Min.X),
			y: C.double(rect.Min.Y),
		},
		size: C.CGSize{
			width:  C.double(rect.Dx()),
			height: C.double(rect.Dy()),
		},
	}
}

// freeAllCaches frees both the style cache and the label cache under drawMu
// so that no in-flight draw can reference freed C pointers.
func (o *Overlay) freeAllCaches() {
	o.drawMu.Lock()
	defer o.drawMu.Unlock()

	o.styleCache.Free()
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
	o.cachedLabels = make(map[string]*C.char)
}

// getOrCacheLabel returns a cached C string for the label.
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

func boolToInt(v bool) int {
	if v {
		return 1
	}

	return 0
}
