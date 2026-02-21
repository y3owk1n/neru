package recursivegrid

/*
#cgo CFLAGS: -x objective-c
#include "../../../core/infra/bridge/overlay.h"
#include <stdlib.h>
*/
import "C"

import (
	"image"
	"strings"
	"sync"
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/overlayutil"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/recursivegrid"
	"go.uber.org/zap"
)

const (
	// NSWindowSharingNone represents NSWindowSharingNone (0) - hidden from screen sharing.
	NSWindowSharingNone = 0
	// NSWindowSharingReadOnly represents NSWindowSharingReadOnly (1) - visible in screen sharing.
	NSWindowSharingReadOnly = 1
)

// Overlay manages the rendering of recursive_grid overlays using native platform APIs.
type Overlay struct {
	window C.OverlayWindow
	config config.RecursiveGridConfig
	logger *zap.Logger

	callbackManager *overlayutil.CallbackManager
	styleCache      *overlayutil.StyleCache
	labelCacheMu    sync.RWMutex
	cachedLabels    map[string]*C.char
}

// NewOverlay creates a new recursive_grid overlay instance.
func NewOverlay(cfg config.RecursiveGridConfig, logger *zap.Logger) (*Overlay, error) {
	base, err := overlayutil.NewBaseOverlay(logger)
	if err != nil {
		return nil, err
	}

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
	return o.config
}

// Logger returns the logger.
func (o *Overlay) Logger() *zap.Logger {
	return o.logger
}

// SetConfig updates the overlay's config.
func (o *Overlay) SetConfig(cfg config.RecursiveGridConfig) {
	o.config = cfg
	o.styleCache.Free()
	o.freeLabelCache()
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
}

// Cleanup frees Go-side resources (callbackManager, styleCache, labelCache)
// without destroying the native window. Use this for overlays that share a
// window managed by the overlay Manager.
func (o *Overlay) Cleanup() {
	if o.callbackManager != nil {
		o.callbackManager.Cleanup()
	}
	o.styleCache.Free()
	o.freeLabelCache()
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

// ResizeToActiveScreen resizes the overlay window to the active screen.
func (o *Overlay) ResizeToActiveScreen() {
	C.NeruResizeOverlayToActiveScreen(o.window)
}

// DrawRecursiveGrid renders the recursive_grid with current bounds, depth, keys, gridCols, and gridRows.
func (o *Overlay) DrawRecursiveGrid(
	bounds image.Rectangle,
	depth int,
	keys string,
	gridCols int,
	gridRows int,
	style Style,
) error {
	if bounds.Empty() {
		o.Clear()

		return nil
	}

	o.logger.Debug("Drawing recursive-grid",
		zap.Int("bounds_x", bounds.Min.X),
		zap.Int("bounds_y", bounds.Min.Y),
		zap.Int("bounds_width", bounds.Dx()),
		zap.Int("bounds_height", bounds.Dy()),
		zap.Int("depth", depth),
		zap.Int("grid_cols", gridCols),
		zap.Int("grid_rows", gridRows),
		zap.String("keys", keys))

	// Clear previous drawing
	o.Clear()

	// Use the provided dimensions and calculate key count
	keyCount := gridCols * gridRows

	// Validate grid dimensions (must be at least 2)
	if gridCols < recursivegrid.MinGridDimension {
		// Fallback to default 2x2 if invalid
		gridCols = recursivegrid.MinGridDimension
		gridRows = recursivegrid.MinGridDimension
		keyCount = gridCols * gridRows
		keys = "uijk"
	}

	if gridRows < recursivegrid.MinGridDimension {
		// Fallback to default 2x2 if invalid
		gridCols = recursivegrid.MinGridDimension
		gridRows = recursivegrid.MinGridDimension
		keyCount = gridCols * gridRows
		keys = "uijk"
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

	// Calculate cell dimensions
	cellWidth := bounds.Dx() / gridCols
	cellHeight := bounds.Dy() / gridRows

	// Create grid cells dynamically
	cells := make([]C.GridCell, keyCount)

	for row := range gridRows {
		for col := range gridCols {
			idx := row*gridCols + col

			maxX := bounds.Min.X + (col+1)*cellWidth
			if col == gridCols-1 {
				maxX = bounds.Max.X
			}

			maxY := bounds.Min.Y + (row+1)*cellHeight
			if row == gridRows-1 {
				maxY = bounds.Max.Y
			}

			cell := image.Rectangle{
				Min: image.Point{
					X: bounds.Min.X + col*cellWidth,
					Y: bounds.Min.Y + row*cellHeight,
				},
				Max: image.Point{
					X: maxX,
					Y: maxY,
				},
			}

			labelStr := ""
			if idx < len(keyRunes) {
				labelStr = string(keyRunes[idx])
			}
			label := strings.ToUpper(labelStr)
			cells[idx] = C.GridCell{
				label:               o.getOrCacheLabel(label),
				bounds:              o.rectToCRect(cell),
				isMatched:           C.int(0),
				isSubgrid:           C.int(0),
				matchedPrefixLength: C.int(0),
			}
		}
	}

	// Get cached style
	cachedStyle := o.styleCache.Get(func(cached *overlayutil.CachedStyle) {
		cached.FontFamily = unsafe.Pointer(C.CString(style.LabelFontFamily()))
		cached.BgColor = unsafe.Pointer(C.CString(style.HighlightColor()))
		cached.TextColor = unsafe.Pointer(C.CString(style.LabelColor()))
		cached.MatchedTextColor = unsafe.Pointer(C.CString(style.LabelColor()))
		cached.MatchedBgColor = unsafe.Pointer(C.CString(style.HighlightColor()))
		cached.MatchedBorderColor = unsafe.Pointer(C.CString(style.LineColor()))
		cached.BorderColor = unsafe.Pointer(C.CString(style.LineColor()))
	})

	finalStyle := C.GridCellStyle{
		fontSize:               C.int(style.LabelFontSize()),
		fontFamily:             (*C.char)(cachedStyle.FontFamily),
		backgroundColor:        (*C.char)(cachedStyle.BgColor),
		textColor:              (*C.char)(cachedStyle.TextColor),
		matchedTextColor:       (*C.char)(cachedStyle.MatchedTextColor),
		matchedBackgroundColor: (*C.char)(cachedStyle.MatchedBgColor),
		matchedBorderColor:     (*C.char)(cachedStyle.MatchedBorderColor),
		borderColor:            (*C.char)(cachedStyle.BorderColor),
		borderWidth:            C.int(style.LineWidth()),
	}

	// Draw the grid cells
	C.NeruDrawGridCells(o.window, &cells[0], C.int(len(cells)), finalStyle)

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

// freeLabelCache frees all cached label C strings.
func (o *Overlay) freeLabelCache() {
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

// Style represents the visual style for recursive_grid.
type Style struct {
	lineColor       string
	lineWidth       int
	highlightColor  string
	labelColor      string
	labelFontSize   int
	labelFontFamily string
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

// LabelColor returns the label color.
func (s Style) LabelColor() string {
	return s.labelColor
}

// LabelFontSize returns the label font size.
func (s Style) LabelFontSize() int {
	return s.labelFontSize
}

// LabelFontFamily returns the label font family.
func (s Style) LabelFontFamily() string {
	return s.labelFontFamily
}

// BuildStyle creates a Style from RecursiveGridConfig.
func BuildStyle(cfg config.RecursiveGridConfig) Style {
	return Style{
		lineColor:       cfg.LineColor,
		lineWidth:       cfg.LineWidth,
		highlightColor:  cfg.HighlightColor,
		labelColor:      cfg.LabelColor,
		labelFontSize:   cfg.LabelFontSize,
		labelFontFamily: cfg.LabelFontFamily,
	}
}
