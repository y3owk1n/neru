package grid

/*
#cgo CFLAGS: -x objective-c
#include "../../../core/infra/bridge/overlay.h"
#include <stdlib.h>

// Callback function that Go can reference.
extern void gridResizeCompletionCallback(void* context);
*/
import "C"

import (
	"image"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/overlayutil"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	"go.uber.org/zap"
)

const (
	// DefaultGridLinesCount is the default number of grid lines.
	DefaultGridLinesCount = 4

	// GridMaxChars is the max chars for grid.
	GridMaxChars = 9

	// RoundingFactor is the factor for rounding.
	RoundingFactor = 0.5
)

//export gridResizeCompletionCallback
func gridResizeCompletionCallback(context unsafe.Pointer) {
	// Convert context to callback ID
	id := uint64(uintptr(context))

	overlayutil.CompleteGlobalCallback(id)
}

var (
	gridCellSlicePool     sync.Pool
	gridLabelSlicePool    sync.Pool
	subgridCellSlicePool  sync.Pool
	subgridLabelSlicePool sync.Pool
	gridPoolOnce          sync.Once
)

// Overlay manages the rendering of grid overlays using native platform APIs.
type Overlay struct {
	window C.OverlayWindow
	config config.GridConfig
	logger *zap.Logger

	callbackManager *overlayutil.CallbackManager

	// Cached C strings for style properties to reduce allocations
	cachedStyleMu            sync.RWMutex
	cachedFontFamily         *C.char
	cachedBgColor            *C.char
	cachedTextColor          *C.char
	cachedMatchedTextColor   *C.char
	cachedMatchedBgColor     *C.char
	cachedMatchedBorderColor *C.char
	cachedBorderColor        *C.char
	cachedHighlightColor     *C.char
	cachedLabels             map[string]*C.char

	// Pre-allocated buffer for grid lines (always 4 lines for highlights)
	gridLineBuffer [DefaultGridLinesCount]C.CGRect
}

// initGridPools initializes the grid object pools once.
func initGridPools() {
	gridPoolOnce.Do(func() {
		gridCellSlicePool = sync.Pool{New: func() any {
			s := make([]C.GridCell, 0)

			return &s
		}}
		gridLabelSlicePool = sync.Pool{New: func() any {
			s := make([]*C.char, 0)

			return &s
		}}
		subgridCellSlicePool = sync.Pool{New: func() any {
			s := make([]C.GridCell, 0)

			return &s
		}}
		subgridLabelSlicePool = sync.Pool{New: func() any {
			s := make([]*C.char, 0)

			return &s
		}}
	})
}

// getCommonGridSizes returns a list of common screen resolutions for grid prewarming.
func getCommonGridSizes() []image.Rectangle {
	return []image.Rectangle{
		image.Rect(0, 0, 1280, 800),  //nolint:mnd
		image.Rect(0, 0, 1366, 768),  //nolint:mnd
		image.Rect(0, 0, 1440, 900),  //nolint:mnd
		image.Rect(0, 0, 1920, 1080), //nolint:mnd
		image.Rect(0, 0, 2560, 1440), //nolint:mnd
		image.Rect(0, 0, 3440, 1440), //nolint:mnd
		image.Rect(0, 0, 3840, 2160), //nolint:mnd
	}
}

// NewOverlay creates a new grid overlay instance with its own window and prewarms common grid sizes.
func NewOverlay(config config.GridConfig, logger *zap.Logger) *Overlay {
	window := C.createOverlayWindow()
	initGridPools()
	chars := config.Characters

	if config.PrewarmEnabled {
		go domainGrid.Prewarm(chars, getCommonGridSizes())
	}

	return &Overlay{
		window:          window,
		config:          config,
		logger:          logger,
		callbackManager: overlayutil.NewCallbackManager(logger),
		cachedLabels:    make(map[string]*C.char),
	}
}

// NewOverlayWithWindow creates a grid overlay instance using a shared window and prewarms common grid sizes.
func NewOverlayWithWindow(
	config config.GridConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) *Overlay {
	initGridPools()
	chars := config.Characters

	if config.PrewarmEnabled {
		go domainGrid.Prewarm(chars, getCommonGridSizes())
	}

	return &Overlay{
		window:          (C.OverlayWindow)(windowPtr),
		config:          config,
		logger:          logger,
		callbackManager: overlayutil.NewCallbackManager(logger),
		cachedLabels:    make(map[string]*C.char),
	}
}

// Window returns the overlay window.
func (o *Overlay) Window() C.OverlayWindow {
	return o.window
}

// Config returns the grid config.
func (o *Overlay) Config() config.GridConfig {
	return o.config
}

// Logger returns the logger.
func (o *Overlay) Logger() *zap.Logger {
	return o.logger
}

// SetConfig updates the overlay's config (e.g., after config reload).
func (o *Overlay) SetConfig(config config.GridConfig) {
	o.config = config
	// Invalidate caches when config changes
	o.freeStyleCache()
	o.freeLabelCache()
}

// SetHideUnmatched sets whether to hide unmatched cells.
func (o *Overlay) SetHideUnmatched(hide bool) {
	hideInt := 0
	if hide {
		hideInt = 1
	}
	C.NeruSetHideUnmatched(o.window, C.int(hideInt))
}

// Show displays the grid overlay.
func (o *Overlay) Show() {
	C.NeruShowOverlayWindow(o.window)
}

// Hide hides the grid overlay.
func (o *Overlay) Hide() {
	C.NeruHideOverlayWindow(o.window)
}

// Clear clears the grid overlay.
func (o *Overlay) Clear() {
	C.NeruClearOverlay(o.window)
}

// Destroy destroys the grid overlay window.
func (o *Overlay) Destroy() {
	// Clean up callback manager first to stop background goroutines
	if o.callbackManager != nil {
		o.callbackManager.Cleanup()
	}

	o.freeStyleCache()
	o.freeLabelCache()
	C.NeruDestroyOverlayWindow(o.window)
}

// ReplaceWindow atomically replaces the underlying overlay window on the main thread.
func (o *Overlay) ReplaceWindow() {
	C.NeruReplaceOverlayWindow(&o.window)
}

// ResizeToMainScreen resizes the overlay window to the current main screen.
func (o *Overlay) ResizeToMainScreen() {
	C.NeruResizeOverlayToMainScreen(o.window)
}

// ResizeToActiveScreen resizes the overlay window with callback notification.
func (o *Overlay) ResizeToActiveScreen() {
	o.callbackManager.StartResizeOperation(func(callbackID uint64) {
		// Pass integer ID as opaque pointer context for C callback.
		// Safe: ID is a primitive value that C treats as opaque and Go round-trips via uintptr.
		// Assumes 64-bit pointers (guaranteed on macOS amd64/arm64, the only supported platforms).
		// Note: go vet complains about unsafe.Pointer misuse, but this is intentional and safe.
		C.NeruResizeOverlayToActiveScreenWithCallback(
			o.window,
			(C.ResizeCompletionCallback)(C.gridResizeCompletionCallback),
			unsafe.Pointer(uintptr(callbackID)), //nolint:govet
		)
	})
}

// DrawGrid renders the flat grid with all 3-char cells visible.
func (o *Overlay) DrawGrid(grid *domainGrid.Grid, currentInput string, style Style) error {
	// Clear existing content
	o.Clear()

	cells := grid.AllCells()

	if len(cells) == 0 {
		return nil
	}

	start := time.Now()
	var msBefore runtime.MemStats
	runtime.ReadMemStats(&msBefore)

	o.drawGridCells(cells, currentInput, style)

	var msAfter runtime.MemStats
	runtime.ReadMemStats(&msAfter)
	o.logger.Info("Grid draw perf",
		zap.Int("cell_count", len(cells)),
		zap.Duration("duration", time.Since(start)),
		zap.Uint64("alloc_bytes_delta", msAfter.Alloc-msBefore.Alloc),
		zap.Uint64("sys_bytes_delta", msAfter.Sys-msBefore.Sys))

	return nil
}

// UpdateMatches updates matched state without redrawing all cells.
func (o *Overlay) UpdateMatches(prefix string) {
	cPrefix := C.CString(prefix)
	defer C.free(unsafe.Pointer(cPrefix)) //nolint:nlreturn
	C.NeruUpdateGridMatchPrefix(o.window, cPrefix)
}

// ShowSubgrid draws a 3x3 subgrid inside the selected cell.
func (o *Overlay) ShowSubgrid(cell *domainGrid.Cell, style Style) {
	keys := o.config.SublayerKeys
	if strings.TrimSpace(keys) == "" {
		keys = o.config.Characters
	}
	chars := []rune(keys)
	// Subgrid is always 3x3
	const rows = 3
	const cols = 3

	// If not enough characters, adjust count to available characters
	count := min(len(chars), GridMaxChars)

	tmpCells := subgridCellSlicePool.Get()
	cellsPtr, _ := tmpCells.(*[]C.GridCell)
	if cap(*cellsPtr) < count {
		s := make([]C.GridCell, count)
		cellsPtr = &s
	} else {
		*cellsPtr = (*cellsPtr)[:count]
	}
	cells := *cellsPtr
	tmpLabels := subgridLabelSlicePool.Get()
	labelsPtr, _ := tmpLabels.(*[]*C.char)
	if cap(*labelsPtr) < count {
		s := make([]*C.char, count)
		labelsPtr = &s
	} else {
		*labelsPtr = (*labelsPtr)[:count]
	}
	labels := *labelsPtr

	cellBounds := cell.Bounds()
	// Build breakpoints that evenly distribute remainders to fully cover the cell
	xBreaks := make([]int, cols+1)
	yBreaks := make([]int, rows+1)
	xBreaks[0] = cellBounds.Min.X
	yBreaks[0] = cellBounds.Min.Y
	for breakIndex := 1; breakIndex <= cols; breakIndex++ {
		// round(i * width / cols)
		val := float64(breakIndex) * float64(cellBounds.Dx()) / float64(cols)
		xBreaks[breakIndex] = cellBounds.Min.X + int(val+RoundingFactor)
	}
	for breakIndex := 1; breakIndex <= rows; breakIndex++ {
		val := float64(breakIndex) * float64(cellBounds.Dy()) / float64(rows)
		yBreaks[breakIndex] = cellBounds.Min.Y + int(val+RoundingFactor)
	}

	// Ensure last break exactly matches bounds max to avoid 1px drift
	xBreaks[cols] = cellBounds.Max.X
	yBreaks[rows] = cellBounds.Max.Y

	for cellIndex := range cells {
		rowIndex := cellIndex / cols
		colIndex := cellIndex % cols
		label := strings.ToUpper(string(chars[cellIndex]))
		labels[cellIndex] = o.getOrCacheLabel(label)
		left := xBreaks[colIndex]
		right := xBreaks[colIndex+1]
		top := yBreaks[rowIndex]
		bottom := yBreaks[rowIndex+1]

		var gridCell C.GridCell
		gridCell.label = labels[cellIndex]
		gridCell.bounds.origin.x = C.double(left)
		gridCell.bounds.origin.y = C.double(top)
		gridCell.bounds.size.width = C.double(right - left)
		gridCell.bounds.size.height = C.double(bottom - top)
		gridCell.isMatched = C.int(0)
		gridCell.isSubgrid = C.int(1)           // Mark as subgrid cell
		gridCell.matchedPrefixLength = C.int(0) // Subgrid cells don't have matched prefixes

		cells[cellIndex] = gridCell
	}

	// Use cached style strings to avoid repeated allocations
	fontFamily, backgroundColor, textColor, matchedTextColor,
		matchedBackgroundColor, matchedBorderColor, borderColor := o.getCachedStyleStrings(style)

	finalStyle := C.GridCellStyle{
		fontSize:               C.int(style.FontSize()),
		fontFamily:             fontFamily,
		backgroundColor:        backgroundColor,
		textColor:              textColor,
		matchedTextColor:       matchedTextColor,
		matchedBackgroundColor: matchedBackgroundColor,
		matchedBorderColor:     matchedBorderColor,
		borderColor:            borderColor,
		borderWidth:            C.int(style.BorderWidth()),
		backgroundOpacity:      C.double(style.Opacity()),
		textOpacity:            C.double(1.0),
	}

	C.NeruClearOverlay(o.window)
	C.NeruDrawGridCells(o.window, &cells[0], C.int(len(cells)), finalStyle)

	*cellsPtr = (*cellsPtr)[:0]
	*labelsPtr = (*labelsPtr)[:0]
	subgridCellSlicePool.Put(cellsPtr)
	subgridLabelSlicePool.Put(labelsPtr)
	// Note: We don't free cached style strings - they're reused across draws
}

// DrawScrollHighlight draws a scroll highlight.
func (o *Overlay) DrawScrollHighlight(
	xCoordinate, yCoordinate, width, height int,
	color string,
	borderWidth int,
) {
	// Cache color string if needed
	o.cachedStyleMu.Lock()
	if o.cachedHighlightColor != nil {
		C.free(unsafe.Pointer(o.cachedHighlightColor))
	}
	o.cachedHighlightColor = C.CString(color)
	cColor := o.cachedHighlightColor
	o.cachedStyleMu.Unlock()

	// Use pre-allocated buffer for grid lines (always 4 lines for highlights)
	// Bottom
	o.gridLineBuffer[0] = C.CGRect{
		origin: C.CGPoint{x: C.double(xCoordinate), y: C.double(yCoordinate)},
		size:   C.CGSize{width: C.double(width), height: C.double(borderWidth)},
	}
	// Top
	o.gridLineBuffer[1] = C.CGRect{
		origin: C.CGPoint{
			x: C.double(xCoordinate),
			y: C.double(yCoordinate + height - borderWidth),
		},
		size: C.CGSize{width: C.double(width), height: C.double(borderWidth)},
	}
	// Left
	o.gridLineBuffer[2] = C.CGRect{
		origin: C.CGPoint{x: C.double(xCoordinate), y: C.double(yCoordinate)},
		size:   C.CGSize{width: C.double(borderWidth), height: C.double(height)},
	}
	// Right
	o.gridLineBuffer[3] = C.CGRect{
		origin: C.CGPoint{x: C.double(xCoordinate + width - borderWidth), y: C.double(yCoordinate)},
		size:   C.CGSize{width: C.double(borderWidth), height: C.double(height)},
	}
	C.NeruDrawWindowBorder(
		o.window,
		&o.gridLineBuffer[0],
		C.int(DefaultGridLinesCount),
		cColor,
		C.int(borderWidth),
		C.double(1.0),
	)
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
	if o.cachedMatchedBgColor != nil {
		C.free(unsafe.Pointer(o.cachedMatchedBgColor))
		o.cachedMatchedBgColor = nil
	}
	if o.cachedMatchedBorderColor != nil {
		C.free(unsafe.Pointer(o.cachedMatchedBorderColor))
		o.cachedMatchedBorderColor = nil
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

// freeLabelCache frees all cached label C strings.
func (o *Overlay) freeLabelCache() {
	o.cachedStyleMu.Lock()
	defer o.cachedStyleMu.Unlock()

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
	o.cachedStyleMu.RLock()
	if cStr, ok := o.cachedLabels[label]; ok {
		o.cachedStyleMu.RUnlock()

		return cStr
	}
	o.cachedStyleMu.RUnlock()

	o.cachedStyleMu.Lock()
	defer o.cachedStyleMu.Unlock()

	// Double-check
	if cStr, ok := o.cachedLabels[label]; ok {
		return cStr
	}

	cStr := C.CString(label)
	o.cachedLabels[label] = cStr

	return cStr
}

// updateStyleCacheLocked updates cached C strings for the current style.
// Must be called with cachedStyleMu write lock held.
func (o *Overlay) updateStyleCacheLocked(style Style) {
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
	if o.cachedMatchedBgColor != nil {
		C.free(unsafe.Pointer(o.cachedMatchedBgColor))
	}
	if o.cachedMatchedBorderColor != nil {
		C.free(unsafe.Pointer(o.cachedMatchedBorderColor))
	}
	if o.cachedBorderColor != nil {
		C.free(unsafe.Pointer(o.cachedBorderColor))
	}

	// Create new cached strings
	o.cachedFontFamily = C.CString(style.FontFamily())
	o.cachedBgColor = C.CString(style.BackgroundColor())
	o.cachedTextColor = C.CString(style.TextColor())
	o.cachedMatchedTextColor = C.CString(style.MatchedTextColor())
	o.cachedMatchedBgColor = C.CString(style.MatchedBackgroundColor())
	o.cachedMatchedBorderColor = C.CString(style.MatchedBorderColor())
	o.cachedBorderColor = C.CString(style.BorderColor())
}

// getCachedStyleStrings returns cached C strings for style, updating cache if needed.
func (o *Overlay) getCachedStyleStrings(
	style Style,
) (*C.char, *C.char, *C.char, *C.char, *C.char, *C.char, *C.char) {
	o.cachedStyleMu.RLock()
	// Check if cache is valid (simple check - if any is nil, rebuild all)
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
		matchedBgColor := o.cachedMatchedBgColor
		matchedBorderColor := o.cachedMatchedBorderColor
		borderColor := o.cachedBorderColor
		o.cachedStyleMu.Unlock()

		return fontFamily, bgColor, textColor, matchedTextColor, matchedBgColor, matchedBorderColor, borderColor
	}

	fontFamily := o.cachedFontFamily
	bgColor := o.cachedBgColor
	textColor := o.cachedTextColor
	matchedTextColor := o.cachedMatchedTextColor
	matchedBgColor := o.cachedMatchedBgColor
	matchedBorderColor := o.cachedMatchedBorderColor
	borderColor := o.cachedBorderColor
	o.cachedStyleMu.RUnlock()

	return fontFamily, bgColor, textColor, matchedTextColor, matchedBgColor, matchedBorderColor, borderColor
}

// drawGridCells draws all grid cells with their labels.
func (o *Overlay) drawGridCells(cellsGo []*domainGrid.Cell, currentInput string, style Style) {
	tmpCells := gridCellSlicePool.Get()
	cGridCellsPtr, ok := tmpCells.(*[]C.GridCell)
	if !ok {
		// If type assertion fails, create a new slice
		s := make([]C.GridCell, len(cellsGo))
		cGridCellsPtr = &s
	} else {
		if cap(*cGridCellsPtr) < len(cellsGo) {
			s := make([]C.GridCell, len(cellsGo))
			cGridCellsPtr = &s
		} else {
			*cGridCellsPtr = (*cGridCellsPtr)[:len(cellsGo)]
		}
	}
	cGridCells := *cGridCellsPtr
	tmpLabels := gridLabelSlicePool.Get()
	cLabelsPtr, typeOk := tmpLabels.(*[]*C.char)
	if !typeOk {
		// If type assertion fails, create a new slice
		s := make([]*C.char, len(cellsGo))
		cLabelsPtr = &s
	} else {
		if cap(*cLabelsPtr) < len(cellsGo) {
			s := make([]*C.char, len(cellsGo))
			cLabelsPtr = &s
		} else {
			*cLabelsPtr = (*cLabelsPtr)[:len(cellsGo)]
		}
	}
	cLabels := *cLabelsPtr

	matchedCount := 0
	for cellIndex, cell := range cellsGo {
		cLabels[cellIndex] = o.getOrCacheLabel(cell.Coordinate())

		isMatched := 0
		matchedPrefixLength := 0
		if currentInput != "" && strings.HasPrefix(cell.Coordinate(), currentInput) {
			isMatched = 1
			matchedCount++
			matchedPrefixLength = len(currentInput)
		}

		var cGridCell C.GridCell
		cGridCell.label = cLabels[cellIndex]
		cGridCell.bounds.origin.x = C.double(cell.Bounds().Min.X)
		cGridCell.bounds.origin.y = C.double(cell.Bounds().Min.Y)
		cGridCell.bounds.size.width = C.double(cell.Bounds().Dx())
		cGridCell.bounds.size.height = C.double(cell.Bounds().Dy())
		cGridCell.isMatched = C.int(isMatched)
		cGridCell.isSubgrid = C.int(0) // Mark as regular grid cell
		cGridCell.matchedPrefixLength = C.int(matchedPrefixLength)

		cGridCells[cellIndex] = cGridCell
	}

	o.logger.Debug("Grid cell match statistics",
		zap.Int("total_cells", len(cellsGo)),
		zap.Int("matched_cells", matchedCount))

	// Use cached style strings to avoid repeated allocations
	fontFamily, backgroundColor, textColor, matchedTextColor,
		matchedBackgroundColor, matchedBorderColor, borderColor := o.getCachedStyleStrings(style)

	finalStyle := C.GridCellStyle{
		fontSize:               C.int(style.FontSize()),
		fontFamily:             fontFamily,
		backgroundColor:        backgroundColor,
		textColor:              textColor,
		matchedTextColor:       matchedTextColor,
		matchedBackgroundColor: matchedBackgroundColor,
		matchedBorderColor:     matchedBorderColor,
		borderColor:            borderColor,
		borderWidth:            C.int(style.BorderWidth()),
		backgroundOpacity:      C.double(style.Opacity()),
		textOpacity:            C.double(1.0),
	}

	C.NeruClearOverlay(o.window)
	C.NeruDrawGridCells(o.window, &cGridCells[0], C.int(len(cGridCells)), finalStyle)

	*cGridCellsPtr = (*cGridCellsPtr)[:0]
	*cLabelsPtr = (*cLabelsPtr)[:0]
	gridCellSlicePool.Put(cGridCellsPtr)
	gridLabelSlicePool.Put(cLabelsPtr)
	// Note: We don't free cached style strings - they're reused across draws
}

// Style represents the visual style for grid cells.
type Style struct {
	fontSize               int
	fontFamily             string
	opacity                float64
	borderWidth            int
	backgroundColor        string
	textColor              string
	matchedTextColor       string
	matchedBackgroundColor string
	matchedBorderColor     string
	borderColor            string
}

// FontSize returns the font size.
func (s Style) FontSize() int {
	return s.fontSize
}

// FontFamily returns the font family.
func (s Style) FontFamily() string {
	return s.fontFamily
}

// Opacity returns the opacity.
func (s Style) Opacity() float64 {
	return s.opacity
}

// BorderWidth returns the border width.
func (s Style) BorderWidth() int {
	return s.borderWidth
}

// BackgroundColor returns the background color.
func (s Style) BackgroundColor() string {
	return s.backgroundColor
}

// TextColor returns the text color.
func (s Style) TextColor() string {
	return s.textColor
}

// MatchedTextColor returns the matched text color.
func (s Style) MatchedTextColor() string {
	return s.matchedTextColor
}

// MatchedBackgroundColor returns the matched background color.
func (s Style) MatchedBackgroundColor() string {
	return s.matchedBackgroundColor
}

// MatchedBorderColor returns the matched border color.
func (s Style) MatchedBorderColor() string {
	return s.matchedBorderColor
}

// BorderColor returns the border color.
func (s Style) BorderColor() string {
	return s.borderColor
}

// BuildStyle returns Style based on action name using the provided config.
func BuildStyle(cfg config.GridConfig) Style {
	style := Style{
		fontSize:               cfg.FontSize,
		fontFamily:             cfg.FontFamily,
		opacity:                cfg.Opacity,
		borderWidth:            cfg.BorderWidth,
		backgroundColor:        cfg.BackgroundColor,
		textColor:              cfg.TextColor,
		matchedTextColor:       cfg.MatchedTextColor,
		matchedBackgroundColor: cfg.MatchedBackgroundColor,
		matchedBorderColor:     cfg.MatchedBorderColor,
		borderColor:            cfg.BorderColor,
	}

	return style
}
