//go:build darwin

package recursivegrid

/*
#cgo CFLAGS: -x objective-c
#include "../../../core/infra/platform/darwin/overlay.h"
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

	"github.com/y3owk1n/neru/internal/app/components/overlayutil"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/recursivegrid"
	"github.com/y3owk1n/neru/internal/core/ports"
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
	// cellCenterDivisor is the divisor used when centering a cell grid within its bounds.
	cellCenterDivisor = 2
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

	callbackManager *overlayutil.CallbackManager
	styleCache      *overlayutil.StyleCache
	labelCacheMu    sync.RWMutex
	cachedLabels    map[string]*C.char

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
		window:          (C.OverlayWindow)(base.Window),
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
		window:          (C.OverlayWindow)(base.Window),
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
	cFillColor := C.CString(fillColor)
	defer C.free(unsafe.Pointer(cFillColor)) //nolint:nlreturn

	indicatorStyle := C.CursorIndicatorStyle{
		radius:    C.double(size),
		fillColor: cFillColor,
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
			(C.ResizeCompletionCallback)(C.recursiveGridResizeCompletionCallback),
			contextPtr,
		)
	})
	if !started {
		// Pool exhausted — fall back to non-callback resize so the overlay
		// is still moved to the correct screen.
		C.NeruResizeOverlayToActiveScreen(o.window)
	}
}

// DrawRecursiveGrid renders the recursive_grid with current bounds, depth, keys, gridCols, and gridRows.
// nextKeys/nextGridCols/nextGridRows describe the *next* depth's layout and are used
// for the sub-key preview mini-grid inside each cell.
func (o *Overlay) DrawRecursiveGrid(
	bounds image.Rectangle,
	depth int,
	keys string,
	gridCols int,
	gridRows int,
	nextKeys string,
	nextGridCols int,
	nextGridRows int,
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
			zap.Int("grid_cols", gridCols),
			zap.Int("grid_rows", gridRows),
			zap.String("keys", keys))
	}

	// Use the provided dimensions and calculate key count
	keyCount := gridCols * gridRows

	// Validate grid dimensions (must be at least 1, and total cells >= 2)
	if gridCols < recursivegrid.MinGridDimension ||
		gridRows < recursivegrid.MinGridDimension ||
		gridCols*gridRows < 2 {
		// Fallback to default 2x2 if invalid or degenerate (1×1)
		gridCols = recursivegrid.DefaultGridCols
		gridRows = recursivegrid.DefaultGridRows
		keyCount = gridCols * gridRows
		keys = recursivegrid.DefaultKeys
	}

	// Validate keys length matches grid dimensions
	keyRunes := []rune(keys)
	if len(keyRunes) != keyCount {
		o.logger.Warn(
			"Keys length mismatch in DrawRecursiveGrid, some cells will have empty labels",
			zap.Int("key_count", len(keyRunes)),
			zap.Int("expected", keyCount),
		)
	}

	// All cells are exactly the same size: floor(W/cols) × floor(H/rows).
	// Leftover pixels from uneven division become uniform padding around the grid.
	cellW := bounds.Dx() / gridCols
	cellH := bounds.Dy() / gridRows
	gridW := cellW * gridCols
	gridH := cellH * gridRows
	offX := (bounds.Dx() - gridW) / cellCenterDivisor
	offY := (bounds.Dy() - gridH) / cellCenterDivisor

	// Hold drawMu.RLock for the entire span from label lookup through the C
	// draw call so that freeLabelCache cannot free labels mid-draw.
	o.drawMu.RLock()

	// Create grid cells dynamically
	cells := make([]C.GridCell, keyCount)

	for row := range gridRows {
		for col := range gridCols {
			idx := row*gridCols + col

			x := bounds.Min.X + offX + col*cellW
			y := bounds.Min.Y + offY + row*cellH

			cell := image.Rect(x, y, x+cellW, y+cellH)

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
				bounds:              o.rectToCRect(cell),
				isMatched:           C.int(0),
				isSubgrid:           C.int(0),
				matchedPrefixLength: C.int(0),
			}
		}
	}

	// Build sub-key preview labels.
	// When a label char override is set, repeat it to match the grid cell count
	// so the native renderer gets the expected number of labels.
	subKeyLabel := style.SubKeyPreviewLabelChar()
	if subKeyLabel != "" {
		subKeyLabel = strings.Repeat(subKeyLabel, nextGridCols*nextGridRows)
	} else {
		subKeyLabel = strings.ToUpper(nextKeys)
	}

	// Get cached style
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
		subKeyGridCols:              C.int(nextGridCols),
		subKeyGridRows:              C.int(nextGridRows),
		drawSubKeyPreview:           C.int(boolToInt(style.SubKeyPreview() && nextKeys != "")),
		subKeyFontSize:              C.int(style.SubKeyPreviewFontSize()),
		subKeyFontFamily:            (*C.char)(cachedStyle.SubKeyFontFamily),
		subKeyAutohideMultiplier:    C.float(style.SubKeyPreviewAutohideMultiplier()),
		subKeyTextColor:             (*C.char)(cachedStyle.SubKeyTextColor),
		subKeyKeys:                  o.getOrCacheLabel(subKeyLabel),
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

// Style represents the visual style for recursive_grid.
type Style struct {
	lineColor                       string
	lineWidth                       int
	highlightColor                  string
	textColor                       string
	fontSize                        int
	fontFamily                      string
	labelBackground                 bool
	labelBackgroundColor            string
	labelBackgroundPaddingX         int
	labelBackgroundPaddingY         int
	labelBackgroundBorderRadius     int
	labelBackgroundBorderWidth      int
	labelChar                       string
	subKeyPreview                   bool
	subKeyPreviewFontSize           int
	subKeyPreviewAutohideMultiplier float64
	subKeyPreviewTextColor          string
	subKeyPreviewLabelChar          string
}

// LineColor returns the line color.
func (s Style) LineColor() string {
	return s.lineColor
}

// LineWidth returns the line width.
func (s Style) LineWidth() int {
	return s.lineWidth
}

// HighlightColor returns the highlight color.
func (s Style) HighlightColor() string {
	return s.highlightColor
}

// TextColor returns the text color.
func (s Style) TextColor() string {
	return s.textColor
}

// FontSize returns the font size.
func (s Style) FontSize() int {
	return s.fontSize
}

// FontFamily returns the font family.
func (s Style) FontFamily() string {
	return s.fontFamily
}

// LabelBackground returns whether labels should render with a badge background.
func (s Style) LabelBackground() bool {
	return s.labelBackground
}

// LabelBackgroundColor returns the label background color.
func (s Style) LabelBackgroundColor() string {
	return s.labelBackgroundColor
}

// LabelBackgroundPaddingX returns the horizontal badge padding.
func (s Style) LabelBackgroundPaddingX() int {
	return s.labelBackgroundPaddingX
}

// LabelBackgroundPaddingY returns the vertical badge padding.
func (s Style) LabelBackgroundPaddingY() int {
	return s.labelBackgroundPaddingY
}

// LabelBackgroundBorderRadius returns the badge border radius.
func (s Style) LabelBackgroundBorderRadius() int {
	return s.labelBackgroundBorderRadius
}

// LabelBackgroundBorderWidth returns the badge border width.
func (s Style) LabelBackgroundBorderWidth() int {
	return s.labelBackgroundBorderWidth
}

// LabelChar returns the label character override (empty = use key character).
func (s Style) LabelChar() string {
	return s.labelChar
}

// SubKeyPreviewLabelChar returns the sub-key preview label character override (empty = use key character).
func (s Style) SubKeyPreviewLabelChar() string {
	return s.subKeyPreviewLabelChar
}

// SubKeyPreview returns whether sub-key preview is enabled.
func (s Style) SubKeyPreview() bool {
	return s.subKeyPreview
}

// SubKeyPreviewFontSize returns the font size for sub-key preview labels.
func (s Style) SubKeyPreviewFontSize() int {
	return s.subKeyPreviewFontSize
}

// SubKeyPreviewAutohideMultiplier returns the minimum cell size multiplier for sub-key preview autohide.
func (s Style) SubKeyPreviewAutohideMultiplier() float64 {
	return s.subKeyPreviewAutohideMultiplier
}

// SubKeyPreviewTextColor returns the text color for sub-key preview labels.
func (s Style) SubKeyPreviewTextColor() string {
	return s.subKeyPreviewTextColor
}

// BuildStyle creates a Style from RecursiveGridConfig.
// The theme parameter is used to resolve theme-aware colors when they are not
// explicitly specified in the configuration (empty string = default).
func BuildStyle(cfg config.RecursiveGridConfig, theme config.ThemeProvider) Style {
	return Style{
		lineColor: cfg.UI.LineColor.ForTheme(
			theme,
			config.RecursiveGridLineColorLight,
			config.RecursiveGridLineColorDark,
		),
		lineWidth: cfg.UI.LineWidth,
		highlightColor: cfg.UI.HighlightColor.ForTheme(
			theme,
			config.RecursiveGridHighlightColorLight,
			config.RecursiveGridHighlightColorDark,
		),
		textColor: cfg.UI.TextColor.ForTheme(
			theme,
			config.RecursiveGridTextColorLight,
			config.RecursiveGridTextColorDark,
		),
		fontSize:        cfg.UI.FontSize,
		fontFamily:      ports.ResolveFont(cfg.UI.FontFamily, true),
		labelBackground: cfg.UI.LabelBackground,
		labelBackgroundColor: cfg.UI.LabelBackgroundColor.ForTheme(
			theme,
			config.RecursiveGridLabelBackgroundColorLight,
			config.RecursiveGridLabelBackgroundColorDark,
		),
		labelBackgroundPaddingX:         cfg.UI.LabelBackgroundPaddingX,
		labelBackgroundPaddingY:         cfg.UI.LabelBackgroundPaddingY,
		labelBackgroundBorderRadius:     cfg.UI.LabelBackgroundBorderRadius,
		labelBackgroundBorderWidth:      cfg.UI.LabelBackgroundBorderWidth,
		labelChar:                       cfg.UI.LabelChar,
		subKeyPreview:                   cfg.UI.SubKeyPreview,
		subKeyPreviewFontSize:           cfg.UI.SubKeyPreviewFontSize,
		subKeyPreviewAutohideMultiplier: cfg.UI.SubKeyPreviewAutohideMultiplier,
		subKeyPreviewTextColor: cfg.UI.SubKeyPreviewTextColor.ForTheme(
			theme,
			config.RecursiveGridSubKeyPreviewTextColorLight,
			config.RecursiveGridSubKeyPreviewTextColorDark,
		),
		subKeyPreviewLabelChar: cfg.UI.SubKeyPreviewLabelChar,
	}
}
