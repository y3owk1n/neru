//go:build windows

// internal/ui/overlay/manager_windows_overlay.go
// Win32 overlay backend used by the Windows overlay manager for grid rendering.
// Does not manage singleton lifecycle or mode subscriptions.

package overlay

import (
	"image"
	"strings"
	"unsafe"

	"go.uber.org/zap"

	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	winplatform "github.com/y3owk1n/neru/internal/core/infra/platform/windows"
	"github.com/y3owk1n/neru/internal/core/ports"
)

const (
	winSubgridCols      = 3
	winSubgridRows      = 3
	winSubgridHalfPixel = 0.5
	winSubgridFontScale = 0.7
)

type winOverlay struct {
	window         *winplatform.OverlayWindow
	logger         *zap.Logger
	currentPrefix  string
	hideUnmatched  bool
	currentSubgrid *domainGrid.Cell
	sublayerKeys   string
	cachedGrid     *domainGrid.Grid
	cachedStyle    gridcomponent.Style
	suppressDraw   bool

	lastHints     []*hintscomponent.Hint
	lastHintStyle hintscomponent.StyleMode
}

func newWinOverlay(logger *zap.Logger) *winOverlay {
	window, err := winplatform.NewOverlayWindow()
	if err != nil {
		if logger != nil {
			logger.Error("failed to create Windows overlay window", zap.Error(err))
		}

		return nil
	}

	if logger != nil {
		bounds := window.Bounds()
		logger.Info(
			"Windows overlay window ready",
			zap.Int("x", bounds.Min.X),
			zap.Int("y", bounds.Min.Y),
			zap.Int("width", bounds.Dx()),
			zap.Int("height", bounds.Dy()),
		)
	}

	return &winOverlay{window: window, logger: logger}
}

func (o *winOverlay) Healthy() bool {
	return o != nil && o.window != nil && o.window.Healthy()
}

// WindowPtr returns nil on Windows. The native HWND is not a memory pointer,
// and no consumer dereferences this value, so the overlay window handle stays
// internal (reachable via the platform window) instead of being smuggled
// through an unsafe.Pointer.
func (o *winOverlay) WindowPtr() unsafe.Pointer {
	return nil
}

func (o *winOverlay) Show() {
	if o == nil {
		return
	}

	o.ensureWindowForDraw()

	if o.window == nil {
		if o.logger != nil {
			o.logger.Error("Show aborted, overlay window is nil")
		}

		return
	}

	o.suppressDraw = false

	if o.logger != nil {
		bounds := o.window.Bounds()
		o.logger.Debug("Show overlay window",
			zap.Uintptr("hwnd", uintptr(o.window.HWND())),
			zap.Int("x", bounds.Min.X),
			zap.Int("y", bounds.Min.Y),
			zap.Int("width", bounds.Dx()),
			zap.Int("height", bounds.Dy()),
		)
	}

	// Reopen after Esc: redraw from cache once the HWND is about to be shown.
	if o.cachedGrid != nil {
		o.redrawGridWithoutFlush()
	}

	o.window.Show()
	o.flushOverlay("show")

	if o.logger != nil {
		o.logger.Debug("Show overlay window done")
	}
}

// EnsureVisible makes the overlay HWND visible without flushing the pixel
// buffer. Used by indicator draw paths that need the window to be on-screen
// before composing a new frame, avoiding the blink that Show()'s intermediate
// flush would cause.
func (o *winOverlay) EnsureVisible() {
	if o == nil {
		return
	}

	o.ensureWindowForDraw()

	if o.window == nil {
		return
	}

	o.suppressDraw = false
	o.window.Show()
}

func (o *winOverlay) Hide() {
	if o == nil {
		return
	}

	o.suppressDraw = true
	o.currentSubgrid = nil

	if o.window != nil {
		o.window.Hide()
	}
}

func (o *winOverlay) Clear() {
	if o != nil && o.window != nil {
		o.window.Clear()
	}
}

// clearForIndicator clears the pixel buffer before drawing a transient
// indicator badge so that old badges at previous cursor positions are
// erased. When cached grid or hints content exists (grid/hints mode),
// the buffer is left untouched so the indicator draws on top of the
// existing content.
func (o *winOverlay) clearForIndicator() {
	if o == nil || o.window == nil {
		return
	}

	// In grid or hints mode, the cached content is already rendered in the
	// pixel buffer. Do not clear — the indicator badge is drawn on top.
	if o.cachedGrid != nil || len(o.lastHints) > 0 {
		return
	}

	o.window.Clear()
}

// ClearCache invalidates cached grid and hints state so that a subsequent
// Show() does not redraw stale content from a previous mode. This must be
// called when modes exit to prevent ghost artifacts (e.g. the old grid
// reappearing when a mode indicator is drawn).
func (o *winOverlay) ClearCache() {
	if o == nil {
		return
	}

	o.cachedGrid = nil
	o.cachedStyle = gridcomponent.Style{}
	o.currentPrefix = ""
	o.currentSubgrid = nil
	o.lastHints = nil
	o.lastHintStyle = hintscomponent.StyleMode{}
}

func (o *winOverlay) Resize() {
	if o == nil || o.window == nil {
		return
	}

	err := o.window.ResizeToActiveScreen()
	if err != nil && o.logger != nil {
		o.logger.Warn("failed to resize Windows overlay", zap.Error(err))
	}
}

func (o *winOverlay) Destroy() {
	if o != nil && o.window != nil {
		o.window.Destroy()
		o.window = nil
	}
}

func (o *winOverlay) UpdateGridMatches(prefix string) {
	if o == nil || o.cachedGrid == nil || o.suppressDraw {
		return
	}

	if o.window != nil && !o.window.Visible() {
		o.currentPrefix = strings.ToUpper(prefix)

		return
	}

	o.currentPrefix = strings.ToUpper(prefix)
	o.redrawGrid()
}

func (o *winOverlay) ShowSubgrid(cell *domainGrid.Cell, _ gridcomponent.Style) {
	if o == nil || o.window == nil || cell == nil {
		return
	}

	o.currentSubgrid = cell
	o.Clear()
	o.drawSubgrid(cell.Bounds(), o.cachedStyle)
	o.flushOverlay("subgrid")
}

func (o *winOverlay) SetHideUnmatched(hide bool) {
	o.hideUnmatched = hide
}

func (o *winOverlay) DrawGrid(gridValue *domainGrid.Grid, input string, style gridcomponent.Style) {
	if o == nil {
		return
	}

	o.ensureWindowForDraw()

	if o.window == nil {
		if o.logger != nil {
			o.logger.Error("DrawGrid aborted, overlay window is nil")
		}

		return
	}

	if gridValue == nil {
		if o.logger != nil {
			o.logger.Error("DrawGrid aborted, grid is nil")
		}

		return
	}

	o.cachedGrid = gridValue
	o.cachedStyle = style
	o.currentPrefix = strings.ToUpper(input)
	o.currentSubgrid = nil
	o.suppressDraw = false
	o.redrawGrid()
}

func (o *winOverlay) recreateWindow() {
	if o == nil {
		return
	}

	if o.window != nil {
		o.window.Destroy()
		o.window = nil
	}

	window, err := winplatform.NewOverlayWindow()
	if err != nil {
		if o.logger != nil {
			o.logger.Error("failed to recreate overlay window", zap.Error(err))
		}

		return
	}

	o.window = window

	if o.logger != nil {
		bounds := window.Bounds()
		o.logger.Debug(
			"recreated overlay window",
			zap.Uintptr("hwnd", uintptr(window.HWND())),
			zap.Int("width", bounds.Dx()),
			zap.Int("height", bounds.Dy()),
		)
	}
}

func (o *winOverlay) ensureWindowForDraw() {
	if o == nil {
		return
	}

	// HWND may be hidden between grid sessions; recreate only when invalid.
	if o.window == nil || !o.window.Healthy() {
		o.recreateWindow()
	}
}

func (o *winOverlay) screenBounds() (image.Rectangle, bool) {
	if o == nil || o.window == nil {
		return image.Rectangle{}, false
	}

	bounds := o.window.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return image.Rectangle{}, false
	}

	return bounds, true
}

func (o *winOverlay) redrawGrid() {
	o.redrawGridWithoutFlush()
	o.flushOverlay("grid")
}

func (o *winOverlay) redrawGridWithoutFlush() {
	if o == nil {
		return
	}

	if o.window == nil {
		if o.logger != nil {
			o.logger.Error("redrawGrid aborted, overlay window is nil")
		}

		return
	}

	if o.cachedGrid == nil {
		if o.logger != nil {
			o.logger.Error("redrawGrid aborted, cached grid is nil")
		}

		return
	}

	o.Clear()

	style := o.cachedStyle
	prefix := o.currentPrefix

	for _, cell := range o.cachedGrid.AllCells() {
		label := strings.ToUpper(cell.Coordinate())

		matched := strings.HasPrefix(label, prefix)
		if o.hideUnmatched && prefix != "" && !matched {
			continue
		}

		fill := style.BackgroundColor
		text := style.LabelFontColor
		border := style.LineColor
		if matched && prefix != "" {
			fill = style.MatchedBackgroundColor
			text = style.MatchedTextColor
			border = style.MatchedBorderColor
		}

		o.drawCellFill(cell.Bounds(), fill)
		o.drawCellBorder(cell.Bounds(), border, style.LineWidth)

		if style.ShowLabels {
			o.drawTextCentered(label, cell.Bounds(), ports.ResolveFont(style.LabelFontName, false), style.LabelFontSize, text)
		}
	}

	if o.currentSubgrid != nil {
		o.drawSubgrid(o.currentSubgrid.Bounds(), style)
	}

	if o.logger != nil {
		o.logger.Debug(
			"redraw complete",
			zap.Int("cells", len(o.cachedGrid.AllCells())),
			zap.Bool("healthy", o.window.Healthy()),
		)
	}
}

func (o *winOverlay) flushOverlay(context string) {
	if o == nil || o.window == nil {
		return
	}

	err := o.window.Flush()
	if err != nil {
		if o.logger != nil {
			o.logger.Error(
				"overlay paint failed",
				zap.String("context", context),
				zap.Error(err),
			)
		}

		return
	}
}

func (o *winOverlay) drawSubgrid(bounds image.Rectangle, style gridcomponent.Style) {
	keyRunes := []rune("ASDFGHJKL")
	if o.sublayerKeys != "" {
		keyRunes = []rune(strings.ToUpper(o.sublayerKeys))
	}

	maxKeys := min(len(keyRunes), winSubgridCols*winSubgridRows)

	xBreaks := make([]int, winSubgridCols+1)
	yBreaks := make([]int, winSubgridRows+1)
	xBreaks[0] = bounds.Min.X

	yBreaks[0] = bounds.Min.Y
	for i := 1; i <= winSubgridCols; i++ {
		xBreaks[i] = bounds.Min.X + int(
			float64(i)*float64(bounds.Dx())/float64(winSubgridCols)+winSubgridHalfPixel,
		)
	}

	for i := 1; i <= winSubgridRows; i++ {
		yBreaks[i] = bounds.Min.Y + int(
			float64(i)*float64(bounds.Dy())/float64(winSubgridRows)+winSubgridHalfPixel,
		)
	}

	xBreaks[winSubgridCols] = bounds.Max.X
	yBreaks[winSubgridRows] = bounds.Max.Y

	index := 0
	for row := range winSubgridRows {
		for col := range winSubgridCols {
			if index >= maxKeys {
				break
			}

			cell := image.Rect(
				xBreaks[col],
				yBreaks[row],
				xBreaks[col+1],
				yBreaks[row+1],
			)
			o.drawCellBorder(cell, style.LineColor, style.LineWidth)
			o.drawTextCentered(
				string(keyRunes[index]),
				cell,
				ports.ResolveFont(style.LabelFontName, false),
				style.LabelFontSize*winSubgridFontScale,
				style.LabelFontColor,
			)
			index++
		}
	}
}

func (o *winOverlay) drawCellFill(bounds image.Rectangle, fill uint32) {
	if o == nil || o.window == nil {
		return
	}

	o.window.FillRect(bounds, fill)
}

// drawCellBorder draws only the grid outline; cell interiors stay color-key transparent.
func (o *winOverlay) drawCellBorder(
	bounds image.Rectangle,
	border uint32,
	lineWidth float64,
) {
	if o == nil || o.window == nil || lineWidth <= 0 {
		return
	}

	o.window.StrokeRect(bounds, border, lineWidth)
}

func (o *winOverlay) drawTextCentered(
	text string,
	bounds image.Rectangle,
	fontFamily string,
	fontSize float64,
	color uint32,
) {
	if o == nil || o.window == nil {
		return
	}

	o.window.DrawTextCentered(text, bounds, fontFamily, fontSize, color)
}
