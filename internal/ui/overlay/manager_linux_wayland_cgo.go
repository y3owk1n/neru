//go:build linux && cgo

package overlay

/*
#cgo linux pkg-config: wayland-client cairo xkbcommon
#include <stdlib.h>
#include "../../core/infra/platform/linux/overlay_wayland.h"
*/
import "C"

import (
	"image"
	"strings"
	"sync"
	"time"
	"unsafe"

	"go.uber.org/zap"

	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/core/domain/recursivegrid"
	_ "github.com/y3owk1n/neru/internal/core/infra/platform/linux"
	_ "github.com/y3owk1n/neru/internal/core/infra/platform/linux/wlr_protocol"
	"github.com/y3owk1n/neru/internal/core/ports"
)

type wlrootsOverlay struct {
	raw            *C.NeruWaylandOverlay
	logger         *zap.Logger
	currentPrefix  string
	hideUnmatched  bool
	currentSubgrid *domainGrid.Cell
	sublayerKeys   string // Configured sublayer keys (uppercase), set from grid config
	// Cached grid state for incremental redraws triggered by
	// UpdateGridMatches / ShowSubgrid (which don't receive the grid).
	cachedGrid  *domainGrid.Grid
	cachedStyle gridcomponent.Style

	// displayMu serializes all access to the underlying wl_display connection.
	// The Wayland client API is not thread-safe: concurrent wl_display_dispatch
	// (keyboard poller) and wl_display_flush / wl_display_roundtrip (rendering)
	// from different goroutines is undefined behavior. This mutex is shared with
	// the Manager's renderMu — the Manager sets it after construction via
	// setDisplayMu so that both the rendering path and the keyboard poller
	// serialize on the same lock.
	displayMu *sync.Mutex

	stopCh chan struct{} // signals keyboardPoller to exit
	doneCh chan struct{} // closed when keyboardPoller has exited
}

func init() {
	wlrootsKeyboardCh = make(chan string, keyboardChanBuffer)
}

func newWlrootsOverlay(logger *zap.Logger) *wlrootsOverlay {
	raw := C.neru_wayland_overlay_new()
	if raw == nil {
		return nil
	}

	C.neru_wayland_overlay_setup_buffers(raw)

	overlay := &wlrootsOverlay{
		raw:    raw,
		logger: logger,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	// Do NOT start the keyboardPoller goroutine here. The caller must
	// call setDisplayMu first (to share renderMu), then startPoller.
	// Starting the goroutine before displayMu is set would be a data
	// race: the poller reads displayMu on every iteration.

	return overlay
}

func (o *wlrootsOverlay) Healthy() bool {
	return o != nil && o.raw != nil
}

func (o *wlrootsOverlay) WindowPtr() unsafe.Pointer {
	if o == nil {
		return nil
	}

	return unsafe.Pointer(o.raw)
}

func (o *wlrootsOverlay) Show() {
	if o != nil && o.raw != nil {
		// Re-setup buffers in case dimensions changed
		C.neru_wayland_overlay_setup_buffers(o.raw)
		C.neru_wayland_overlay_show(o.raw)
	}
}

func (o *wlrootsOverlay) Hide() {
	if o != nil && o.raw != nil {
		C.neru_wayland_overlay_hide(o.raw)
	}
}

func (o *wlrootsOverlay) Clear() {
	if o != nil && o.raw != nil {
		C.neru_wayland_overlay_clear(o.raw)
	}
}

func (o *wlrootsOverlay) ClearRect(rect image.Rectangle) {
	if o != nil && o.raw != nil && !rect.Empty() {
		C.neru_wayland_overlay_clear_rect(
			o.raw,
			C.double(rect.Min.X),
			C.double(rect.Min.Y),
			C.double(rect.Dx()),
			C.double(rect.Dy()),
		)
	}
}

func (o *wlrootsOverlay) Resize() {
	// Wayland layer shells auto-resize
}

func (o *wlrootsOverlay) Destroy() {
	if o == nil || o.raw == nil {
		return
	}

	// Signal the poller goroutine to stop and wait for it to exit
	// before freeing the C struct, preventing a use-after-free.
	close(o.stopCh)
	<-o.doneCh

	C.neru_wayland_overlay_destroy(o.raw)
	o.raw = nil
}

func (o *wlrootsOverlay) UpdateGridMatches(prefix string) {
	o.currentPrefix = strings.ToUpper(prefix)
	o.redrawGrid()
}

func (o *wlrootsOverlay) ShowSubgrid(cell *domainGrid.Cell, _ gridcomponent.Style) {
	if o == nil || o.raw == nil || cell == nil {
		return
	}

	o.currentSubgrid = cell
	C.neru_wayland_overlay_setup_buffers(o.raw)
	o.Clear()
	o.drawSubgrid(cell.Bounds(), o.cachedStyle)
	C.neru_wayland_overlay_flush(o.raw)
}

func (o *wlrootsOverlay) SetHideUnmatched(hide bool) {
	o.hideUnmatched = hide
}

func (o *wlrootsOverlay) DrawGrid(g *domainGrid.Grid, input string, style gridcomponent.Style) {
	if o == nil || o.raw == nil || g == nil {
		return
	}
	// Cache for incremental redraws from UpdateGridMatches / ShowSubgrid.
	o.cachedGrid = g
	o.cachedStyle = style
	o.currentPrefix = strings.ToUpper(input)
	// Clear subgrid — DrawGrid draws the main grid; ShowSubgrid sets it separately.
	o.currentSubgrid = nil

	o.redrawGrid()
}

func (o *wlrootsOverlay) DrawRecursiveGrid(
	bounds image.Rectangle,
	_ int,
	keys string,
	gridCols int,
	gridRows int,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
) {
	o.DrawRecursiveGridWithSubKeyPreview(
		bounds,
		keys,
		gridCols,
		gridRows,
		"",
		0,
		0,
		style,
		virtualPointer,
	)
}

func (o *wlrootsOverlay) DrawRecursiveGridWithSubKeyPreview(
	bounds image.Rectangle,
	keys string,
	gridCols int,
	gridRows int,
	nextKeys string,
	nextGridCols int,
	nextGridRows int,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
) {
	if o == nil || o.raw == nil || bounds.Empty() || gridCols <= 0 || gridRows <= 0 {
		return
	}
	C.neru_wayland_overlay_setup_buffers(o.raw)
	o.Clear()

	keyRunes := []rune(strings.ToUpper(keys))
	nextKeyRunes := []rune(strings.ToUpper(nextKeys))
	drawSubPreview := style.SubKeyPreview && len(nextKeyRunes) > 0 && nextGridCols > 0 &&
		nextGridRows > 0

	cellRects := recursivegrid.ComputeGridCells(bounds, gridCols, gridRows)
	for idx, cell := range cellRects {
		fill := style.HighlightColor
		if fill == 0 {
			fill = subgridCellBackground
		}

		o.drawRect(cell, fill, style.LineColor, style.LineWidth)
		if idx < len(keyRunes) {
			label := style.LabelChar
			if label == "" {
				label = string(keyRunes[idx])
			}

			if shouldShowLabel(cell, style) {
				if style.LabelBackground {
					o.drawLabelBackground(label, cell, style)
				}

				o.drawTextCentered(
					label,
					cell,
					style.LabelFontName,
					style.LabelFontSize,
					style.LabelFontColor,
				)
			}

			if drawSubPreview && shouldShowSubKeyPreview(cell, style, nextGridCols, nextGridRows) {
				o.drawSubKeyMiniGrid(
					cell,
					nextKeyRunes,
					nextGridCols,
					nextGridRows,
					style,
				)
			}
		}
	}

	if virtualPointer.Visible {
		vpChar := virtualPointer.Char
		if vpChar == "" {
			vpChar = "\u25CF"
		}

		fontName := ports.ResolveFont(virtualPointer.FontName, false)

		fontSize := float64(virtualPointer.Size)
		halfSize := max(virtualPointer.Size/2, 1) //nolint:mnd

		vpBounds := image.Rect(
			virtualPointer.Position.X-halfSize,
			virtualPointer.Position.Y-halfSize,
			virtualPointer.Position.X+halfSize,
			virtualPointer.Position.Y+halfSize,
		)
		o.drawTextCentered(
			vpChar,
			vpBounds,
			fontName,
			fontSize,
			parseHexColor(virtualPointer.FillColor),
		)
	}

	C.neru_wayland_overlay_flush(o.raw)
}

func (o *wlrootsOverlay) DrawBadge(
	posX,
	posY int,
	text string,
	colors overlayColors,
	style overlayBadgeStyle,
) {
	if o == nil || o.raw == nil || text == "" {
		return
	}
	C.neru_wayland_overlay_setup_buffers(o.raw)

	fontSize := style.fontSize
	if fontSize <= 0 {
		fontSize = 14
	}

	rect := badgeBounds(posX, posY, text, style)

	o.drawRect(rect, colors.background, colors.border, max(style.borderWidth, 1))
	o.drawTextCentered(text, rect, style.fontFamily, fontSize, colors.text)
}

func (o *wlrootsOverlay) Flush() {
	if o == nil || o.raw == nil {
		return
	}
	C.neru_wayland_overlay_flush(o.raw)
}

func (o *wlrootsOverlay) DrawHints(
	hintsSlice []*hintscomponent.Hint,
	style hintscomponent.StyleMode,
) {
	if o == nil || o.raw == nil {
		return
	}

	C.neru_wayland_overlay_setup_buffers(o.raw)
	o.Clear()
	for _, hint := range hintsSlice {
		if style.BoundaryHighlightEnabled() {
			boundary := image.Rect(
				hint.Position().X-hint.Size().X/2,
				hint.Position().Y-hint.Size().Y/2,
				hint.Position().X+hint.Size().X/2,
				hint.Position().Y+hint.Size().Y/2,
			)
			o.drawRect(
				boundary,
				parseHexColor(style.BoundaryBackgroundColor()),
				parseHexColor(style.BoundaryBorderColor()),
				float64(max(style.BoundaryBorderWidth(), 0)),
			)
		}

		bounds := hintLabelBounds(
			hint.Position().X,
			hint.Position().Y,
			hint.Label(),
			style,
		)

		textColor := style.TextColor()
		if hint.MatchedPrefix() != "" {
			textColor = style.MatchedTextColor()
		}

		o.drawRoundedRect(
			bounds,
			hintCornerRadius(style.BorderRadius(), bounds.Dy()),
			parseHexColor(style.BackgroundColor()),
			parseHexColor(style.BorderColor()),
			float64(max(style.BorderWidth(), 0)),
		)
		o.drawTextCentered(
			hint.Label(),
			bounds,
			style.FontFamily(),
			float64(max(style.FontSize(), 1)),
			parseHexColor(textColor),
		)
	}

	C.neru_wayland_overlay_flush(o.raw)
}

// keyboardPoller polls for keyboard events.
// It acquires displayMu around every wl_display access to prevent concurrent
// use with the rendering path (which also holds the same mutex via renderMu).
func (o *wlrootsOverlay) keyboardPoller() {
	defer close(o.doneCh)

	const pollInterval = 5 * time.Millisecond

	for {
		select {
		case <-o.stopCh:
			return
		default:
		}

		// Collect all buffered keys under a single lock acquisition.
		// wl_display_roundtrip (called by the rendering path) may
		// dispatch multiple keyboard events; the ring buffer preserves
		// them all so none are silently dropped.
		var keys []string

		if o.displayMu != nil {
			o.displayMu.Lock()
		}

		if C.neru_wayland_overlay_poll(o.raw) < 0 { //nolint:nlreturn
			if o.displayMu != nil {
				o.displayMu.Unlock()
			}

			return
		}

		for {
			key := C.neru_wayland_overlay_get_key(o.raw) //nolint:nlreturn
			if key == nil {
				break
			}

			keys = append(keys, C.GoString(key))
		}

		if o.displayMu != nil {
			o.displayMu.Unlock()
		}

		if len(keys) > 0 {
			for _, k := range keys {
				select {
				case wlrootsKeyboardCh <- k:
				default:
				}
			}
		} else {
			time.Sleep(pollInterval)
		}
	}
}

func (o *wlrootsOverlay) setKeyboardCaptureEnabled(enabled bool) {
	if o == nil || o.raw == nil {
		return
	}

	cEnabled := C.int(0)
	if enabled {
		cEnabled = 1
	}

	C.neru_wayland_overlay_set_keyboard_capture(o.raw, cEnabled)
}

// startPoller launches the keyboard polling goroutine.
// Must be called after setDisplayMu so the poller has a valid mutex.
func (o *wlrootsOverlay) startPoller() {
	go o.keyboardPoller()
}

// setDisplayMu sets the mutex used to serialize wl_display access.
// Must be called before the keyboard poller goroutine starts using it.
func (o *wlrootsOverlay) setDisplayMu(mu *sync.Mutex) {
	o.displayMu = mu
}

// redrawGrid performs the actual grid rendering using cached state.
func (o *wlrootsOverlay) redrawGrid() {
	if o == nil || o.raw == nil || o.cachedGrid == nil {
		return
	}
	C.neru_wayland_overlay_setup_buffers(o.raw)
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
		o.drawRect(cell.Bounds(), fill, border, style.LineWidth)
		o.drawTextCentered(label, cell.Bounds(), style.LabelFontName, style.LabelFontSize, text)
	}

	if o.currentSubgrid != nil {
		o.drawSubgrid(o.currentSubgrid.Bounds(), style)
	}
	C.neru_wayland_overlay_flush(o.raw)
}

func (o *wlrootsOverlay) drawSubgrid(bounds image.Rectangle, style gridcomponent.Style) {
	const (
		cols = 3
		rows = 3
	)

	// Use configured sublayer keys; fall back to default
	keyRunes := []rune("ASDFGHJKL")
	if o.sublayerKeys != "" {
		keyRunes = []rune(strings.ToUpper(o.sublayerKeys))
	}
	maxKeys := min(len(keyRunes), cols*rows)

	// Build breakpoints that evenly distribute remainders to fully cover the cell
	// (matches macOS ShowSubgrid implementation).
	xBreaks := make([]int, cols+1)
	yBreaks := make([]int, rows+1)
	xBreaks[0] = bounds.Min.X
	yBreaks[0] = bounds.Min.Y
	for i := 1; i <= cols; i++ {
		xBreaks[i] = bounds.Min.X + int(
			float64(i)*float64(bounds.Dx())/float64(cols)+subgridHalfPixel,
		)
	}
	for i := 1; i <= rows; i++ {
		yBreaks[i] = bounds.Min.Y + int(
			float64(i)*float64(bounds.Dy())/float64(rows)+subgridHalfPixel,
		)
	}
	// Ensure last breaks exactly match bounds to avoid 1px drift
	xBreaks[cols] = bounds.Max.X
	yBreaks[rows] = bounds.Max.Y

	index := 0
	for row := range rows {
		for col := range cols {
			if index >= maxKeys {
				break
			}
			cell := image.Rect(
				xBreaks[col],
				yBreaks[row],
				xBreaks[col+1],
				yBreaks[row+1],
			)
			// Use a visible semi-opaque fill so subgrid cells are clearly distinct
			o.drawRect(cell, style.BackgroundColor, style.LineColor, style.LineWidth)
			o.drawTextCentered(
				string(keyRunes[index]),
				cell,
				style.LabelFontName,
				style.LabelFontSize*subgridFontScale,
				style.LabelFontColor,
			)
			index++
		}
	}
}

func (o *wlrootsOverlay) drawRect(
	bounds image.Rectangle,
	fill uint32,
	border uint32,
	lineWidth float64,
) {
	C.neru_wayland_overlay_rect(
		o.raw,
		C.double(bounds.Min.X),
		C.double(bounds.Min.Y),
		C.double(bounds.Dx()),
		C.double(bounds.Dy()),
		C.uint(fill),
		C.uint(border),
		C.double(lineWidth),
	)
}

func (o *wlrootsOverlay) drawRoundedRect(
	bounds image.Rectangle,
	radius float64,
	fill uint32,
	border uint32,
	lineWidth float64,
) {
	C.neru_wayland_overlay_rounded_rect(
		o.raw,
		C.double(bounds.Min.X),
		C.double(bounds.Min.Y),
		C.double(bounds.Dx()),
		C.double(bounds.Dy()),
		C.double(radius),
		C.uint(fill),
		C.uint(border),
		C.double(lineWidth),
	)
}

func (o *wlrootsOverlay) drawTextCentered(
	text string,
	bounds image.Rectangle,
	fontFamily string,
	fontSize float64,
	color uint32,
) {
	cText := C.CString(text)
	cFontFamily := C.CString(fontFamily)
	defer C.free(unsafe.Pointer(cText))       //nolint:nlreturn
	defer C.free(unsafe.Pointer(cFontFamily)) //nolint:nlreturn

	C.neru_wayland_overlay_text(
		o.raw,
		cText,
		cFontFamily,
		C.double(bounds.Min.X+bounds.Dx()/2),
		C.double(bounds.Min.Y+bounds.Dy()/2),
		C.double(fontSize),
		C.uint(color),
	)
}

func (o *wlrootsOverlay) drawLabelBackground(
	label string,
	cell image.Rectangle,
	style recursivegridcomponent.Style,
) {
	fontSize := style.LabelFontSize
	paddingX := resolveAutoPadding(fontSize, style.LabelBackgroundPaddingX, true)
	paddingY := resolveAutoPadding(fontSize, style.LabelBackgroundPaddingY, false)
	width := estimateTextWidth(label, fontSize) + paddingX*paddingMultiplier
	height := estimateTextHeight(fontSize) + paddingY*paddingMultiplier
	rect := centeredRect(cell, width, height)

	o.drawRect(
		rect,
		style.LabelBackgroundColor,
		style.LineColor,
		max(style.LabelBackgroundBorderWidth, 0),
	)
}

func (o *wlrootsOverlay) drawSubKeyMiniGrid(
	cell image.Rectangle,
	nextKeyRunes []rune,
	nextGridCols int,
	nextGridRows int,
	style recursivegridcomponent.Style,
) {
	subCells := recursivegrid.ComputeGridCells(cell, nextGridCols, nextGridRows)

	// Center cell index for odd grids; skip it in preview for visual clarity.
	centerIdx := -1
	if nextGridCols%2 == 1 && nextGridRows%2 == 1 {
		centerIdx = (nextGridRows/2)*nextGridCols + nextGridCols/2 //nolint:mnd
	}

	subIndex := 0
	for idx, subCell := range subCells {
		if idx == centerIdx {
			subIndex++

			continue
		}

		if subIndex >= len(nextKeyRunes) {
			return
		}

		subLabel := style.SubKeyPreviewLabelChar
		if subLabel == "" {
			subLabel = string(nextKeyRunes[subIndex])
		}

		o.drawTextCentered(
			subLabel,
			subCell,
			style.LabelFontName,
			style.SubKeyPreviewFontSize,
			style.SubKeyPreviewTextColor,
		)
		subIndex++
	}
}
