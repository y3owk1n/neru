package grid

/*
#cgo CFLAGS: -x objective-c
#include "../../infra/bridge/overlay.h"
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
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"go.uber.org/zap"
)

var (
	gridCallbackID        uint64
	gridCallbackMap       = make(map[uint64]chan struct{}, 8) // Pre-size for typical usage
	gridCallbackLock      sync.Mutex
	gridCellSlicePool     sync.Pool
	gridLabelSlicePool    sync.Pool
	subgridCellSlicePool  sync.Pool
	subgridLabelSlicePool sync.Pool
	gridPoolOnce          sync.Once
)

//export gridResizeCompletionCallback
func gridResizeCompletionCallback(context unsafe.Pointer) {
	// Convert context to callback ID
	id := uint64(uintptr(context))

	gridCallbackLock.Lock()
	if done, ok := gridCallbackMap[id]; ok {
		close(done)
		delete(gridCallbackMap, id)
	}
	gridCallbackLock.Unlock()
}

// Overlay manages the rendering of grid overlays using native platform APIs.
type Overlay struct {
	window C.OverlayWindow
	config config.GridConfig
	logger *zap.Logger
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

// NewOverlay creates a new grid overlay instance with its own window and prewarms common grid sizes.
func NewOverlay(config config.GridConfig, logger *zap.Logger) *Overlay {
	window := C.createOverlayWindow()
	initGridPools()
	chars := config.Characters
	if strings.TrimSpace(chars) == "" {
		chars = config.Characters
	}
	go domainGrid.Prewarm(chars, []image.Rectangle{
		image.Rect(0, 0, 1280, 800),
		image.Rect(0, 0, 1366, 768),
		image.Rect(0, 0, 1440, 900),
		image.Rect(0, 0, 1920, 1080),
		image.Rect(0, 0, 2560, 1440),
		image.Rect(0, 0, 3440, 1440),
		image.Rect(0, 0, 3840, 2160),
	})

	return &Overlay{
		window: window,
		config: config,
		logger: logger,
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
	if strings.TrimSpace(chars) == "" {
		chars = config.Characters
	}
	go domainGrid.Prewarm(chars, []image.Rectangle{
		image.Rect(0, 0, 1280, 800),
		image.Rect(0, 0, 1366, 768),
		image.Rect(0, 0, 1440, 900),
		image.Rect(0, 0, 1920, 1080),
		image.Rect(0, 0, 2560, 1440),
		image.Rect(0, 0, 3440, 1440),
		image.Rect(0, 0, 3840, 2160),
	})

	return &Overlay{
		window: (C.OverlayWindow)(windowPtr),
		config: config,
		logger: logger,
	}
}

// GetWindow returns the overlay window.
func (o *Overlay) GetWindow() C.OverlayWindow {
	return o.window
}

// GetConfig returns the grid config.
func (o *Overlay) GetConfig() config.GridConfig {
	return o.config
}

// GetLogger returns the logger.
func (o *Overlay) GetLogger() *zap.Logger {
	return o.logger
}

// UpdateConfig updates the overlay's config (e.g., after config reload).
func (o *Overlay) UpdateConfig(config config.GridConfig) {
	o.config = config
}

// SetHideUnmatched sets whether to hide unmatched cells.
func (o *Overlay) SetHideUnmatched(hide bool) {
	C.NeruSetHideUnmatched(o.window, C.int(boolToInt(hide)))
}

// boolToInt converts a boolean to an integer (1 for true, 0 for false).
func boolToInt(b bool) int {
	if b {
		return 1
	}

	return 0
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

// ResizeToActiveScreen resizes the overlay window to the screen containing the mouse cursor.
func (o *Overlay) ResizeToActiveScreen() {
	C.NeruResizeOverlayToActiveScreen(o.window)
}

// ResizeToActiveScreenSync resizes the overlay window synchronously with callback notification.
func (o *Overlay) ResizeToActiveScreenSync() {
	done := make(chan struct{})

	// Generate unique ID for this callback
	callbackID := atomic.AddUint64(&gridCallbackID, 1)

	// Store channel in map
	gridCallbackLock.Lock()
	gridCallbackMap[callbackID] = done
	gridCallbackLock.Unlock()

	if o.logger != nil {
		o.logger.Debug("Grid overlay resize started", zap.Uint64("callback_id", callbackID))
	}

	// Pass ID as context (safe - no Go pointers)
	// Note: uintptr conversion must happen in same expression to satisfy go vet
	C.NeruResizeOverlayToActiveScreenWithCallback(
		o.window,
		(C.ResizeCompletionCallback)(
			unsafe.Pointer(C.gridResizeCompletionCallback), //nolint:unconvert
		),
		*(*unsafe.Pointer)(unsafe.Pointer(&callbackID)),
	)

	// Don't wait for callback - continue immediately for better UX
	// The resize operation is typically fast and visually complete before callback
	// Start a goroutine to handle cleanup when callback eventually arrives
	go func() {
		if o.logger != nil {
			o.logger.Debug(
				"Grid overlay resize background cleanup started",
				zap.Uint64("callback_id", callbackID),
			)
		}

		// Use timer instead of time.After to prevent memory leaks
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()

		select {
		case <-done:
			timer.Stop() // Stop timer immediately on success
			// Callback received, normal cleanup already handled in callback
			if o.logger != nil {
				o.logger.Debug(
					"Grid overlay resize callback received",
					zap.Uint64("callback_id", callbackID),
				)
			}
		case <-timer.C:
			// Long timeout for cleanup only - callback likely failed
			gridCallbackLock.Lock()
			delete(gridCallbackMap, callbackID)
			gridCallbackLock.Unlock()

			if o.logger != nil {
				o.logger.Debug("Grid overlay resize cleanup timeout - removed callback from map",
					zap.Uint64("callback_id", callbackID))
			}
		}
	}()
}

// Draw renders the flat grid with all 3-char cells visible.
func (o *Overlay) Draw(grid *domainGrid.Grid, currentInput string, style Style) error {
	o.logger.Debug("Drawing grid overlay",
		zap.Int("cell_count", len(grid.GetAllCells())),
		zap.String("current_input", currentInput))

	// Clear existing content
	o.Clear()

	cells := grid.GetAllCells()

	if len(cells) == 0 {
		o.logger.Debug("No cells to draw in grid overlay")

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

	o.logger.Debug("Grid overlay drawn successfully")

	return nil
}

// UpdateMatches updates matched state without redrawing all cells.
func (o *Overlay) UpdateMatches(prefix string) {
	o.logger.Debug("Updating grid matches", zap.String("prefix", prefix))

	cPrefix := C.CString(prefix)
	defer C.free(unsafe.Pointer(cPrefix)) //nolint:nlreturn
	C.NeruUpdateGridMatchPrefix(o.window, cPrefix)

	o.logger.Debug("Grid matches updated successfully")
}

// ShowSubgrid draws a 3x3 subgrid inside the selected cell.
func (o *Overlay) ShowSubgrid(cell *domainGrid.Cell, style Style) {
	o.logger.Debug("Showing subgrid",
		zap.Int("cell_x", cell.GetBounds().Min.X),
		zap.Int("cell_y", cell.GetBounds().Min.Y),
		zap.Int("cell_width", cell.GetBounds().Dx()),
		zap.Int("cell_height", cell.GetBounds().Dy()))

	keys := o.config.SublayerKeys
	if strings.TrimSpace(keys) == "" {
		keys = o.config.Characters
	}
	chars := []rune(keys)
	// Subgrid is always 3x3
	const rows = 3
	const cols = 3

	// If not enough characters, adjust count to available characters
	count := min(len(chars), 9)

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

	cellBounds := cell.Bounds
	// Build breakpoints that evenly distribute remainders to fully cover the cell
	xBreaks := make([]int, cols+1)
	yBreaks := make([]int, rows+1)
	xBreaks[0] = cellBounds.Min.X
	yBreaks[0] = cellBounds.Min.Y
	for breakIndex := 1; breakIndex <= cols; breakIndex++ {
		// round(i * width / cols)
		val := float64(breakIndex) * float64(cellBounds.Dx()) / float64(cols)
		xBreaks[breakIndex] = cellBounds.Min.X + int(val+0.5)
	}
	for breakIndex := 1; breakIndex <= rows; breakIndex++ {
		val := float64(breakIndex) * float64(cellBounds.Dy()) / float64(rows)
		yBreaks[breakIndex] = cellBounds.Min.Y + int(val+0.5)
	}

	// Ensure last break exactly matches bounds max to avoid 1px drift
	xBreaks[cols] = cellBounds.Max.X
	yBreaks[rows] = cellBounds.Max.Y

	for cellIndex := range cells {
		rowIndex := cellIndex / cols
		colIndex := cellIndex % cols
		label := strings.ToUpper(string(chars[cellIndex]))
		labels[cellIndex] = C.CString(label)
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

	fontFamily := C.CString(style.FontFamily)
	backgroundColor := C.CString(style.BackgroundColor)
	textColor := C.CString(style.TextColor)
	matchedTextColor := C.CString(style.MatchedTextColor)
	matchedBackgroundColor := C.CString(style.MatchedBackgroundColor)
	matchedBorderColor := C.CString(style.MatchedBorderColor)
	borderColor := C.CString(style.BorderColor)

	finalStyle := C.GridCellStyle{
		fontSize:               C.int(style.FontSize),
		fontFamily:             fontFamily,
		backgroundColor:        backgroundColor,
		textColor:              textColor,
		matchedTextColor:       matchedTextColor,
		matchedBackgroundColor: matchedBackgroundColor,
		matchedBorderColor:     matchedBorderColor,
		borderColor:            borderColor,
		borderWidth:            C.int(style.BorderWidth),
		backgroundOpacity:      C.double(style.Opacity),
		textOpacity:            C.double(1.0),
	}

	C.NeruClearOverlay(o.window)
	C.NeruDrawGridCells(o.window, &cells[0], C.int(len(cells)), finalStyle)

	for labelIndex := range labels {
		C.free(unsafe.Pointer(labels[labelIndex]))
	}
	*cellsPtr = (*cellsPtr)[:0]
	*labelsPtr = (*labelsPtr)[:0]
	subgridCellSlicePool.Put(cellsPtr)
	subgridLabelSlicePool.Put(labelsPtr)
	C.free(unsafe.Pointer(fontFamily))
	C.free(unsafe.Pointer(backgroundColor))
	C.free(unsafe.Pointer(textColor))
	C.free(unsafe.Pointer(matchedTextColor))
	C.free(unsafe.Pointer(matchedBackgroundColor))
	C.free(unsafe.Pointer(matchedBorderColor))
	C.free(unsafe.Pointer(borderColor))

	o.logger.Debug("Subgrid shown successfully")
}

// DrawScrollHighlight draws a scroll highlight.
func (o *Overlay) DrawScrollHighlight(
	xCoordinate, yCoordinate, width, height int,
	color string,
	borderWidth int,
) {
	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor)) //nolint:nlreturn
	// Build 4 border lines around the rectangle
	lines := make([]C.CGRect, 4)
	// Bottom
	lines[0] = C.CGRect{
		origin: C.CGPoint{x: C.double(xCoordinate), y: C.double(yCoordinate)},
		size:   C.CGSize{width: C.double(width), height: C.double(borderWidth)},
	}
	// Top
	lines[1] = C.CGRect{
		origin: C.CGPoint{
			x: C.double(xCoordinate),
			y: C.double(yCoordinate + height - borderWidth),
		},
		size: C.CGSize{width: C.double(width), height: C.double(borderWidth)},
	}
	// Left
	lines[2] = C.CGRect{
		origin: C.CGPoint{x: C.double(xCoordinate), y: C.double(yCoordinate)},
		size:   C.CGSize{width: C.double(borderWidth), height: C.double(height)},
	}
	// Right
	lines[3] = C.CGRect{
		origin: C.CGPoint{x: C.double(xCoordinate + width - borderWidth), y: C.double(yCoordinate)},
		size:   C.CGSize{width: C.double(borderWidth), height: C.double(height)},
	}
	C.NeruDrawGridLines(o.window, &lines[0], C.int(4), cColor, C.int(borderWidth), C.double(1.0))
}

// drawGridCells draws all grid cells with their labels.
func (o *Overlay) drawGridCells(cellsGo []*domainGrid.Cell, currentInput string, style Style) {
	o.logger.Debug("Drawing grid cells",
		zap.Int("cell_count", len(cellsGo)),
		zap.String("current_input", currentInput))

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
		cLabels[cellIndex] = C.CString(cell.GetCoordinate())

		isMatched := 0
		matchedPrefixLength := 0
		if currentInput != "" && strings.HasPrefix(cell.GetCoordinate(), currentInput) {
			isMatched = 1
			matchedCount++
			matchedPrefixLength = len(currentInput)
		}

		var cGridCell C.GridCell
		cGridCell.label = cLabels[cellIndex]
		cGridCell.bounds.origin.x = C.double(cell.GetBounds().Min.X)
		cGridCell.bounds.origin.y = C.double(cell.GetBounds().Min.Y)
		cGridCell.bounds.size.width = C.double(cell.GetBounds().Dx())
		cGridCell.bounds.size.height = C.double(cell.GetBounds().Dy())
		cGridCell.isMatched = C.int(isMatched)
		cGridCell.isSubgrid = C.int(0) // Mark as regular grid cell
		cGridCell.matchedPrefixLength = C.int(matchedPrefixLength)

		cGridCells[cellIndex] = cGridCell
	}

	o.logger.Debug("Grid cell match statistics",
		zap.Int("total_cells", len(cellsGo)),
		zap.Int("matched_cells", matchedCount))

	fontFamily := C.CString(style.FontFamily)
	backgroundColor := C.CString(style.BackgroundColor)
	textColor := C.CString(style.TextColor)
	matchedTextColor := C.CString(style.MatchedTextColor)
	matchedBackgroundColor := C.CString(style.MatchedBackgroundColor)
	matchedBorderColor := C.CString(style.MatchedBorderColor)
	borderColor := C.CString(style.BorderColor)

	finalStyle := C.GridCellStyle{
		fontSize:               C.int(style.FontSize),
		fontFamily:             fontFamily,
		backgroundColor:        backgroundColor,
		textColor:              textColor,
		matchedTextColor:       matchedTextColor,
		matchedBackgroundColor: matchedBackgroundColor,
		matchedBorderColor:     matchedBorderColor,
		borderColor:            borderColor,
		borderWidth:            C.int(style.BorderWidth),
		backgroundOpacity:      C.double(style.Opacity),
		textOpacity:            C.double(1.0),
	}

	C.NeruClearOverlay(o.window)
	C.NeruDrawGridCells(o.window, &cGridCells[0], C.int(len(cGridCells)), finalStyle)

	for labelIndex := range cLabels {
		C.free(unsafe.Pointer(cLabels[labelIndex]))
	}
	*cGridCellsPtr = (*cGridCellsPtr)[:0]
	*cLabelsPtr = (*cLabelsPtr)[:0]
	gridCellSlicePool.Put(cGridCellsPtr)
	gridLabelSlicePool.Put(cLabelsPtr)
	C.free(unsafe.Pointer(fontFamily))
	C.free(unsafe.Pointer(backgroundColor))
	C.free(unsafe.Pointer(textColor))
	C.free(unsafe.Pointer(matchedTextColor))
	C.free(unsafe.Pointer(matchedBackgroundColor))
	C.free(unsafe.Pointer(matchedBorderColor))
	C.free(unsafe.Pointer(borderColor))
}

// Style represents the visual style for grid cells.
type Style struct {
	FontSize               int
	FontFamily             string
	Opacity                float64
	BorderWidth            int
	BackgroundColor        string
	TextColor              string
	MatchedTextColor       string
	MatchedBackgroundColor string
	MatchedBorderColor     string
	BorderColor            string
}

// BuildStyle returns Style based on action name using the provided config.
func BuildStyle(cfg config.GridConfig) Style {
	style := Style{
		FontSize:               cfg.FontSize,
		FontFamily:             cfg.FontFamily,
		Opacity:                cfg.Opacity,
		BorderWidth:            cfg.BorderWidth,
		BackgroundColor:        cfg.BackgroundColor,
		TextColor:              cfg.TextColor,
		MatchedTextColor:       cfg.MatchedTextColor,
		MatchedBackgroundColor: cfg.MatchedBackgroundColor,
		MatchedBorderColor:     cfg.MatchedBorderColor,
		BorderColor:            cfg.BorderColor,
	}

	return style
}
