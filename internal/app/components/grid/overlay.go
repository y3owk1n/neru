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

	// NSWindowSharingNone represents NSWindowSharingNone (0) - hidden from screen sharing.
	NSWindowSharingNone = 0
	// NSWindowSharingReadOnly represents NSWindowSharingReadOnly (1) - visible in screen sharing.
	NSWindowSharingReadOnly = 1
)

//export gridResizeCompletionCallback
func gridResizeCompletionCallback(context unsafe.Pointer) {
	// Read callback ID from the pointer (points to a slice element in callbackIDStore)
	id := *(*uint64)(context)

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
	styleCache   *overlayutil.StyleCache
	labelCacheMu sync.RWMutex
	cachedLabels map[string]*C.char

	// Pre-allocated buffer for grid lines (always 4 lines for highlights)
	gridLineBuffer [DefaultGridLinesCount]C.CGRect

	// State tracking for dirty rectangle updates
	gridStateMu   sync.RWMutex
	previousGrid  *domainGrid.Grid
	previousInput string
	previousStyle Style

	// Viewport tracking for lazy rendering
	viewportMu sync.RWMutex
	viewport   image.Rectangle // Current visible area
	maxCells   int             // Maximum cells to render (0 = no limit)
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
func NewOverlay(config config.GridConfig, logger *zap.Logger) (*Overlay, error) {
	base, err := overlayutil.NewBaseOverlay(logger)
	if err != nil {
		return nil, err
	}
	initGridPools()
	chars := config.Characters

	if config.PrewarmEnabled {
		go domainGrid.Prewarm(chars, getCommonGridSizes())
	}

	return &Overlay{
		window:          (C.OverlayWindow)(base.Window),
		config:          config,
		logger:          logger,
		callbackManager: base.CallbackManager,
		styleCache:      base.StyleCache,
		cachedLabels:    make(map[string]*C.char),
	}, nil
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
	base := overlayutil.NewBaseOverlayWithWindow(logger, windowPtr)

	return &Overlay{
		window:          (C.OverlayWindow)(base.Window),
		config:          config,
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
	o.styleCache.Free()
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

// Clear clears the grid overlay and resets state.
func (o *Overlay) Clear() {
	C.NeruClearOverlay(o.window)
	// Reset previous state so next draw will be a full redraw
	o.gridStateMu.Lock()
	o.previousGrid = nil
	o.previousInput = ""
	o.previousStyle = Style{}
	o.gridStateMu.Unlock()
}

// Destroy destroys the grid overlay window.
func (o *Overlay) Destroy() {
	// Clean up callback manager first to stop background goroutines
	if o.callbackManager != nil {
		o.callbackManager.Cleanup()
	}

	o.styleCache.Free()
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
		// Uses CallbackIDToPointer to convert in a way that go vet accepts.
		contextPtr := overlayutil.CallbackIDToPointer(callbackID)

		C.NeruResizeOverlayToActiveScreenWithCallback(
			o.window,
			(C.ResizeCompletionCallback)(C.gridResizeCompletionCallback),
			contextPtr,
		)
	})
}

// DrawGrid renders the flat grid with all 3-char cells visible.
func (o *Overlay) DrawGrid(grid *domainGrid.Grid, currentInput string, style Style) error {
	cells := grid.AllCells()

	if len(cells) == 0 {
		o.Clear()

		return nil
	}

	start := time.Now()
	var msBefore runtime.MemStats
	runtime.ReadMemStats(&msBefore)

	// Check if we can do incremental updates (always try if we have previous state)
	o.gridStateMu.RLock()
	canIncrementalUpdate := o.previousGrid != nil
	o.gridStateMu.RUnlock()

	if canIncrementalUpdate {
		// Try incremental update (handles both input changes and structural changes)
		if o.drawGridIncremental(grid, currentInput, style) {
			// Update cached state on successful incremental update
			o.gridStateMu.Lock()
			o.previousGrid = grid
			o.previousInput = currentInput
			o.previousStyle = style
			o.gridStateMu.Unlock()

			o.logger.Debug("Grid incremental update successful")
			// Note: Show() should be called separately by the caller to ensure overlay is visible

			return nil
		}
		o.logger.Debug("Grid incremental update failed, falling back to full redraw")
	}

	// Full redraw
	o.Clear()
	visibleCells := o.filterCellsByViewport(cells)
	o.drawGridCells(visibleCells, currentInput, style)

	// Update cached state
	o.gridStateMu.Lock()
	o.previousGrid = grid
	o.previousInput = currentInput
	o.previousStyle = style
	o.gridStateMu.Unlock()

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
	cachedStyle := o.styleCache.Get(func(cached *overlayutil.CachedStyle) {
		cached.FontFamily = unsafe.Pointer(C.CString(style.FontFamily()))
		cached.BgColor = unsafe.Pointer(C.CString(style.BackgroundColor()))
		cached.TextColor = unsafe.Pointer(C.CString(style.TextColor()))
		cached.MatchedTextColor = unsafe.Pointer(C.CString(style.MatchedTextColor()))
		cached.MatchedBgColor = unsafe.Pointer(C.CString(style.MatchedBackgroundColor()))
		cached.MatchedBorderColor = unsafe.Pointer(C.CString(style.MatchedBorderColor()))
		cached.BorderColor = unsafe.Pointer(C.CString(style.BorderColor()))
	})

	finalStyle := C.GridCellStyle{
		fontSize:               C.int(style.FontSize()),
		fontFamily:             (*C.char)(cachedStyle.FontFamily),
		backgroundColor:        (*C.char)(cachedStyle.BgColor),
		textColor:              (*C.char)(cachedStyle.TextColor),
		matchedTextColor:       (*C.char)(cachedStyle.MatchedTextColor),
		matchedBackgroundColor: (*C.char)(cachedStyle.MatchedBgColor),
		matchedBorderColor:     (*C.char)(cachedStyle.MatchedBorderColor),
		borderColor:            (*C.char)(cachedStyle.BorderColor),
		borderWidth:            C.int(style.BorderWidth()),
	}

	C.NeruClearOverlay(o.window)
	C.NeruDrawGridCells(o.window, &cells[0], C.int(len(cells)), finalStyle)

	*cellsPtr = (*cellsPtr)[:0]
	*labelsPtr = (*labelsPtr)[:0]
	subgridCellSlicePool.Put(cellsPtr)
	subgridLabelSlicePool.Put(labelsPtr)
	// Note: We don't free cached style strings - they're reused across draws
}

// SetViewport sets the current viewport for lazy rendering and forces a full redraw on next draw.
func (o *Overlay) SetViewport(viewport image.Rectangle) {
	// Update viewport
	o.viewportMu.Lock()
	o.viewport = viewport
	o.viewportMu.Unlock()

	// Invalidate incremental grid state so the next DrawGrid performs a full redraw
	o.gridStateMu.Lock()
	o.previousGrid = nil
	o.previousInput = ""
	o.previousStyle = Style{}
	o.gridStateMu.Unlock()
}

// SetMaxCells sets the maximum number of cells to render (0 = no limit).
func (o *Overlay) SetMaxCells(maxCells int) {
	o.viewportMu.Lock()
	defer o.viewportMu.Unlock()
	o.maxCells = maxCells
}

// SetSharingType sets the window sharing type for screen sharing visibility.
func (o *Overlay) SetSharingType(hide bool) {
	sharingType := C.int(NSWindowSharingReadOnly)
	if hide {
		sharingType = C.int(NSWindowSharingNone)
	}

	C.NeruSetOverlaySharingType(o.window, sharingType)
}

// drawGridIncremental performs incremental updates by only redrawing changed cells.
func (o *Overlay) drawGridIncremental(
	grid *domainGrid.Grid,
	currentInput string,
	style Style,
) bool {
	o.gridStateMu.RLock()
	previousGrid := o.previousGrid
	previousInput := o.previousInput
	previousStyle := o.previousStyle
	o.gridStateMu.RUnlock()

	if previousGrid == nil {
		return false // No previous state to compare against
	}

	// Check if only the input changed (common case for typing)
	if o.gridsAreStructurallyEqual(grid, previousGrid) && style == previousStyle {
		// Only input changed - we can do incremental match updates
		if currentInput != previousInput {
			o.updateMatchesIncremental(grid, currentInput, previousInput)

			return true
		}
		// No changes at all - but we need to ensure overlay is actually visible
		// If the overlay was cleared between activations, we need to redraw even if nothing changed.
		// Since we can't easily check if overlay is empty from Go, we'll be conservative:
		// If this is the first draw after a potential clear (indicated by empty input and no previous input),
		// force a redraw to ensure visibility.
		if currentInput == "" && previousInput == "" {
			// This might be a fresh activation after clear - force redraw to be safe
			return false
		}
		// Otherwise, assume overlay is still showing and no redraw needed
		return true
	}

	// Handle structural changes (grid size/layout changes) using incremental C API
	return o.drawGridIncrementalStructural(
		grid,
		previousGrid,
		currentInput,
		style,
		previousInput,
		previousStyle,
	)
}

// gridsAreStructurallyEqual checks if two grids have the same structure (same cells in same positions).
func (o *Overlay) gridsAreStructurallyEqual(a, b *domainGrid.Grid) bool {
	aCells := a.AllCells()
	bCells := b.AllCells()

	if len(aCells) != len(bCells) {
		return false
	}

	// Check if all cells have the same coordinates and bounds
	for i, aCell := range aCells {
		bCell := bCells[i]
		if aCell.Coordinate() != bCell.Coordinate() ||
			aCell.Bounds() != bCell.Bounds() {
			return false
		}
	}

	return true
}

// updateMatchesIncremental updates match states incrementally when input changes.
func (o *Overlay) updateMatchesIncremental(grid *domainGrid.Grid, newInput, oldInput string) {
	// Use the existing UpdateMatches method which calls NeruUpdateGridMatchPrefix
	// This updates match states without clearing the entire overlay
	o.UpdateMatches(newInput)

	o.logger.Debug("Incremental match update",
		zap.String("old_input", oldInput),
		zap.String("new_input", newInput))
}

// drawGridIncrementalStructural handles structural changes using the incremental C API.
func (o *Overlay) drawGridIncrementalStructural(
	currentGrid *domainGrid.Grid,
	previousGrid *domainGrid.Grid,
	currentInput string,
	currentStyle Style,
	previousInput string,
	previousStyle Style,
) bool {
	currentCells := currentGrid.AllCells()
	previousCells := previousGrid.AllCells()

	// Build maps for efficient lookup
	previousCellMap := make(map[string]*domainGrid.Cell)
	for _, cell := range previousCells {
		previousCellMap[cell.Coordinate()] = cell
	}

	currentCellMap := make(map[string]*domainGrid.Cell)
	for _, cell := range currentCells {
		currentCellMap[cell.Coordinate()] = cell
	}

	// Find cells to add/update (in current but not in previous, or changed)
	var cellsToAdd []*domainGrid.Cell
	for _, cell := range currentCells {
		prevCell, exists := previousCellMap[cell.Coordinate()]
		if !exists { //nolint:gocritic
			// New cell
			cellsToAdd = append(cellsToAdd, cell)
		} else if cell.Bounds() != prevCell.Bounds() {
			// Cell bounds changed (position/size changed)
			cellsToAdd = append(cellsToAdd, cell)
		} else if currentInput != previousInput || currentStyle != previousStyle {
			// Cell exists but match state or style changed
			cellsToAdd = append(cellsToAdd, cell)
		}
	}

	// Find cells to remove (in previous but not in current)
	var cellsToRemove []image.Rectangle
	for _, cell := range previousCells {
		if _, exists := currentCellMap[cell.Coordinate()]; !exists {
			cellsToRemove = append(cellsToRemove, cell.Bounds())
		}
	}

	// If we need to add all cells (overlay is likely empty), do a full redraw instead
	// This handles the case where the overlay was cleared between activations
	if len(cellsToAdd) == len(currentCells) && len(cellsToRemove) == len(previousCells) {
		// All cells need to be added and all previous cells removed - this is effectively a full redraw
		// Fall back to full redraw for better performance and correctness
		return false
	}

	// If no changes, nothing to do
	if len(cellsToAdd) == 0 && len(cellsToRemove) == 0 {
		return true
	}

	// Convert cells to C structures
	cellsToAddC := o.convertCellsToC(cellsToAdd, currentInput)

	// Convert bounds to C structures
	var cellsToRemoveC []C.CGRect
	if len(cellsToRemove) > 0 {
		cellsToRemoveC = make([]C.CGRect, len(cellsToRemove))
		for i, bounds := range cellsToRemove {
			cellsToRemoveC[i] = C.CGRect{
				origin: C.CGPoint{
					x: C.double(bounds.Min.X),
					y: C.double(bounds.Min.Y),
				},
				size: C.CGSize{
					width:  C.double(bounds.Dx()),
					height: C.double(bounds.Dy()),
				},
			}
		}
	}

	// Get style strings
	cachedStyle := o.styleCache.Get(func(cached *overlayutil.CachedStyle) {
		cached.FontFamily = unsafe.Pointer(C.CString(currentStyle.FontFamily()))
		cached.BgColor = unsafe.Pointer(C.CString(currentStyle.BackgroundColor()))
		cached.TextColor = unsafe.Pointer(C.CString(currentStyle.TextColor()))
		cached.MatchedTextColor = unsafe.Pointer(C.CString(currentStyle.MatchedTextColor()))
		cached.MatchedBgColor = unsafe.Pointer(C.CString(currentStyle.MatchedBackgroundColor()))
		cached.MatchedBorderColor = unsafe.Pointer(C.CString(currentStyle.MatchedBorderColor()))
		cached.BorderColor = unsafe.Pointer(C.CString(currentStyle.BorderColor()))
	})

	finalStyle := C.GridCellStyle{
		fontSize:               C.int(currentStyle.FontSize()),
		fontFamily:             (*C.char)(cachedStyle.FontFamily),
		backgroundColor:        (*C.char)(cachedStyle.BgColor),
		textColor:              (*C.char)(cachedStyle.TextColor),
		matchedTextColor:       (*C.char)(cachedStyle.MatchedTextColor),
		matchedBackgroundColor: (*C.char)(cachedStyle.MatchedBgColor),
		matchedBorderColor:     (*C.char)(cachedStyle.MatchedBorderColor),
		borderColor:            (*C.char)(cachedStyle.BorderColor),
		borderWidth:            C.int(currentStyle.BorderWidth()),
	}

	// Call incremental C API
	var cellsToAddPtr *C.GridCell
	var cellsToRemovePtr *C.CGRect
	if len(cellsToAddC) > 0 {
		cellsToAddPtr = &cellsToAddC[0]
	}
	if len(cellsToRemoveC) > 0 {
		cellsToRemovePtr = &cellsToRemoveC[0]
	}

	C.NeruDrawIncrementGrid(
		o.window,
		cellsToAddPtr,
		C.int(len(cellsToAddC)),
		cellsToRemovePtr,
		C.int(len(cellsToRemoveC)),
		finalStyle,
	)

	o.logger.Debug("Incremental structural update",
		zap.Int("cells_added", len(cellsToAdd)),
		zap.Int("cells_removed", len(cellsToRemove)))

	return true
}

// convertCellsToC converts domain grid cells to C GridCell structures.
func (o *Overlay) convertCellsToC(cellsGo []*domainGrid.Cell, currentInput string) []C.GridCell {
	if len(cellsGo) == 0 {
		return nil
	}

	cGridCells := make([]C.GridCell, len(cellsGo))
	cLabels := make([]*C.char, len(cellsGo))

	for cellIndex, cell := range cellsGo {
		cLabels[cellIndex] = o.getOrCacheLabel(cell.Coordinate())

		isMatched := 0
		matchedPrefixLength := 0
		if currentInput != "" && strings.HasPrefix(cell.Coordinate(), currentInput) {
			isMatched = 1
			matchedPrefixLength = len(currentInput)
		}

		cGridCells[cellIndex] = C.GridCell{
			label: cLabels[cellIndex],
			bounds: C.CGRect{
				origin: C.CGPoint{
					x: C.double(cell.Bounds().Min.X),
					y: C.double(cell.Bounds().Min.Y),
				},
				size: C.CGSize{
					width:  C.double(cell.Bounds().Dx()),
					height: C.double(cell.Bounds().Dy()),
				},
			},
			isMatched:           C.int(isMatched),
			isSubgrid:           C.int(0), // Mark as regular grid cell
			matchedPrefixLength: C.int(matchedPrefixLength),
		}
	}

	return cGridCells
}

// filterCellsByViewport returns only cells that intersect with the viewport.
func (o *Overlay) filterCellsByViewport(cells []*domainGrid.Cell) []*domainGrid.Cell {
	o.viewportMu.RLock()
	viewport := o.viewport
	maxCells := o.maxCells
	o.viewportMu.RUnlock()

	if viewport.Empty() || maxCells == 0 {
		return cells
	}

	var visibleCells []*domainGrid.Cell

	// First pass: collect cells that intersect with viewport
	for _, cell := range cells {
		if cell.Bounds().Overlaps(viewport) {
			visibleCells = append(visibleCells, cell)
		}
	}

	// Second pass: limit to maxCells if specified
	if maxCells > 0 && len(visibleCells) > maxCells {
		visibleCells = visibleCells[:maxCells]
	}

	return visibleCells
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
	cachedStyle := o.styleCache.Get(func(cached *overlayutil.CachedStyle) {
		cached.FontFamily = unsafe.Pointer(C.CString(style.FontFamily()))
		cached.BgColor = unsafe.Pointer(C.CString(style.BackgroundColor()))
		cached.TextColor = unsafe.Pointer(C.CString(style.TextColor()))
		cached.MatchedTextColor = unsafe.Pointer(C.CString(style.MatchedTextColor()))
		cached.MatchedBgColor = unsafe.Pointer(C.CString(style.MatchedBackgroundColor()))
		cached.MatchedBorderColor = unsafe.Pointer(C.CString(style.MatchedBorderColor()))
		cached.BorderColor = unsafe.Pointer(C.CString(style.BorderColor()))
	})

	finalStyle := C.GridCellStyle{
		fontSize:               C.int(style.FontSize()),
		fontFamily:             (*C.char)(cachedStyle.FontFamily),
		backgroundColor:        (*C.char)(cachedStyle.BgColor),
		textColor:              (*C.char)(cachedStyle.TextColor),
		matchedTextColor:       (*C.char)(cachedStyle.MatchedTextColor),
		matchedBackgroundColor: (*C.char)(cachedStyle.MatchedBgColor),
		matchedBorderColor:     (*C.char)(cachedStyle.MatchedBorderColor),
		borderColor:            (*C.char)(cachedStyle.BorderColor),
		borderWidth:            C.int(style.BorderWidth()),
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
