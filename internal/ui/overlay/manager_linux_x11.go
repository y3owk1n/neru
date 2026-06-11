//go:build linux && cgo

package overlay

/*
#cgo linux pkg-config: x11 xrender xfixes xext cairo
#include <stdlib.h>
#include "../../core/infra/platform/linux/x11_overlay.h"
*/
import "C"

import (
	"image"
	"strings"
	"unsafe"

	"go.uber.org/zap"

	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	_ "github.com/y3owk1n/neru/internal/core/infra/platform/linux"
)

type x11Overlay struct {
	raw            *C.NeruX11Overlay
	logger         *zap.Logger
	currentPrefix  string
	hideUnmatched  bool
	currentSubgrid *domainGrid.Cell
	sublayerKeys   string
	cachedGrid     *domainGrid.Grid
	cachedStyle    gridcomponent.Style
}

func newX11Overlay(logger *zap.Logger) *x11Overlay {
	raw := C.neru_x11_overlay_new()
	if raw == nil {
		return nil
	}

	return &x11Overlay{raw: raw, logger: logger}
}

func (o *x11Overlay) Healthy() bool {
	return o != nil && o.raw != nil
}

func (o *x11Overlay) WindowPtr() unsafe.Pointer {
	if o == nil {
		return nil
	}

	return unsafe.Pointer(o.raw)
}

func (o *x11Overlay) Show() {
	if o != nil && o.raw != nil {
		C.neru_x11_overlay_show(o.raw)
	}
}

func (o *x11Overlay) Hide() {
	if o != nil && o.raw != nil {
		C.neru_x11_overlay_hide(o.raw)
	}
}

func (o *x11Overlay) Clear() {
	if o != nil && o.raw != nil {
		C.neru_x11_overlay_clear(o.raw)
	}
}

func (o *x11Overlay) ClearRect(rect image.Rectangle) {
	if o != nil && o.raw != nil && !rect.Empty() {
		C.neru_x11_overlay_clear_rect(
			o.raw,
			C.int(rect.Min.X),
			C.int(rect.Min.Y),
			C.int(rect.Dx()),
			C.int(rect.Dy()),
		)
	}
}

func (o *x11Overlay) Resize() {
	if o != nil && o.raw != nil {
		C.neru_x11_overlay_resize(o.raw)
	}
}

func (o *x11Overlay) Destroy() {
	if o != nil && o.raw != nil {
		C.neru_x11_overlay_destroy(o.raw)
		o.raw = nil
	}
}

func (o *x11Overlay) UpdateGridMatches(prefix string) {
	o.currentPrefix = strings.ToUpper(prefix)
	o.redrawGrid()
}

func (o *x11Overlay) ShowSubgrid(cell *domainGrid.Cell, _ gridcomponent.Style) {
	if o == nil || o.raw == nil || cell == nil {
		return
	}

	o.currentSubgrid = cell
	o.Clear()
	o.drawSubgrid(cell.Bounds(), o.cachedStyle)
	C.neru_x11_overlay_flush(o.raw)
}

func (o *x11Overlay) SetHideUnmatched(hide bool) {
	o.hideUnmatched = hide
}

func (o *x11Overlay) DrawGrid(g *domainGrid.Grid, input string, style gridcomponent.Style) {
	if o == nil || o.raw == nil || g == nil {
		return
	}
	o.cachedGrid = g
	o.cachedStyle = style
	o.currentPrefix = strings.ToUpper(input)
	o.currentSubgrid = nil

	o.redrawGrid()
}

func (o *x11Overlay) DrawRecursiveGrid(
	bounds image.Rectangle,
	_ int,
	keys string,
	gridCols int,
	gridRows int,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
) {
	if o == nil || o.raw == nil || bounds.Empty() || gridCols <= 0 || gridRows <= 0 {
		return
	}
	o.Clear()

	keyRunes := []rune(strings.ToUpper(keys))
	cellWidth := bounds.Dx() / gridCols
	cellHeight := bounds.Dy() / gridRows
	index := 0
	for row := range gridRows {
		for col := range gridCols {
			cell := image.Rect(
				bounds.Min.X+col*cellWidth,
				bounds.Min.Y+row*cellHeight,
				bounds.Min.X+(col+1)*cellWidth,
				bounds.Min.Y+(row+1)*cellHeight,
			)
			if col == gridCols-1 {
				cell.Max.X = bounds.Max.X
			}
			if row == gridRows-1 {
				cell.Max.Y = bounds.Max.Y
			}

			fill := style.HighlightColor
			if fill == 0 {
				fill = subgridCellBackground
			}

			o.drawRect(cell, fill, style.LineColor, style.LineWidth)
			if index < len(keyRunes) {
				label := string(keyRunes[index])
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

				if shouldShowSubKeyPreview(cell, style) {
					o.drawSubKeyPreview(label, cell, style)
				}
			}
			index++
		}
	}

	if virtualPointer.Visible {
		vpBounds := image.Rect(
			virtualPointer.Position.X-virtualPointer.Size/2,
			virtualPointer.Position.Y-virtualPointer.Size/2,
			virtualPointer.Position.X+virtualPointer.Size/2,
			virtualPointer.Position.Y+virtualPointer.Size/2,
		)
		o.drawRect(
			vpBounds,
			parseHexColor(virtualPointer.FillColor),
			style.LineColor,
			subgridLineWidth,
		)
	}

	C.neru_x11_overlay_flush(o.raw)
}

func (o *x11Overlay) DrawBadge(
	posX,
	posY int,
	text string,
	colors overlayColors,
	style overlayBadgeStyle,
) {
	if o == nil || o.raw == nil || text == "" {
		return
	}

	fontSize := style.fontSize
	if fontSize <= 0 {
		fontSize = 14
	}

	rect := badgeBounds(posX, posY, text, style)

	o.drawRect(rect, colors.background, colors.border, max(style.borderWidth, 1))
	o.drawTextCentered(text, rect, style.fontFamily, fontSize, colors.text)
	C.neru_x11_overlay_flush(o.raw)
}

func (o *x11Overlay) DrawHints(hintsSlice []*hintscomponent.Hint, style hintscomponent.StyleMode) {
	if o == nil || o.raw == nil {
		return
	}

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

		bounds := image.Rect(
			hint.Position().X,
			hint.Position().Y,
			hint.Position().X+hint.Size().X,
			hint.Position().Y+hint.Size().Y,
		)

		textColor := style.TextColor()
		if hint.MatchedPrefix() != "" {
			textColor = style.MatchedTextColor()
		}

		o.drawRect(
			bounds,
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

	C.neru_x11_overlay_flush(o.raw)
}

func (o *x11Overlay) redrawGrid() {
	if o == nil || o.raw == nil || o.cachedGrid == nil {
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
		o.drawRect(cell.Bounds(), fill, border, style.LineWidth)
		o.drawTextCentered(label, cell.Bounds(), style.LabelFontName, style.LabelFontSize, text)
	}

	if o.currentSubgrid != nil {
		o.drawSubgrid(o.currentSubgrid.Bounds(), style)
	}
	C.neru_x11_overlay_flush(o.raw)
}

func (o *x11Overlay) drawSubgrid(bounds image.Rectangle, style gridcomponent.Style) {
	keyRunes := []rune("ASDFGHJKL")
	if o.sublayerKeys != "" {
		keyRunes = []rune(strings.ToUpper(o.sublayerKeys))
	}
	maxKeys := min(len(keyRunes), subgridCols*subgridRows)

	xBreaks := make([]int, subgridCols+1)
	yBreaks := make([]int, subgridRows+1)
	xBreaks[0] = bounds.Min.X
	yBreaks[0] = bounds.Min.Y
	for i := 1; i <= subgridCols; i++ {
		xBreaks[i] = bounds.Min.X + int(
			float64(i)*float64(bounds.Dx())/float64(subgridCols)+subgridHalfPixel,
		)
	}
	for i := 1; i <= subgridRows; i++ {
		yBreaks[i] = bounds.Min.Y + int(
			float64(i)*float64(bounds.Dy())/float64(subgridRows)+subgridHalfPixel,
		)
	}
	xBreaks[subgridCols] = bounds.Max.X
	yBreaks[subgridRows] = bounds.Max.Y

	index := 0
	for row := range subgridRows {
		for col := range subgridCols {
			if index >= maxKeys {
				break
			}
			cell := image.Rect(
				xBreaks[col],
				yBreaks[row],
				xBreaks[col+1],
				yBreaks[row+1],
			)
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

func (o *x11Overlay) drawRect(
	bounds image.Rectangle,
	fill uint32,
	border uint32,
	lineWidth float64,
) {
	C.neru_x11_overlay_rect(
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

func (o *x11Overlay) drawTextCentered(
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

	C.neru_x11_overlay_text(
		o.raw,
		cText,
		cFontFamily,
		C.double(bounds.Min.X+bounds.Dx()/2),
		C.double(bounds.Min.Y+bounds.Dy()/2),
		C.double(fontSize),
		C.uint(color),
	)
}

func (o *x11Overlay) drawLabelBackground(
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

func (o *x11Overlay) drawSubKeyPreview(
	label string,
	cell image.Rectangle,
	style recursivegridcomponent.Style,
) {
	previewRect := image.Rect(
		cell.Min.X,
		cell.Max.Y-estimateTextHeight(style.SubKeyPreviewFontSize)-subKeyPreviewPaddingBottom,
		cell.Max.X,
		cell.Max.Y,
	)

	o.drawTextCentered(
		label,
		previewRect,
		style.LabelFontName,
		style.SubKeyPreviewFontSize,
		style.SubKeyPreviewTextColor,
	)
}
