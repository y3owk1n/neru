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
	"github.com/y3owk1n/neru/internal/core/ports"
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

	renderMu *sync.Mutex

	cancelMu         sync.Mutex
	animStop         chan struct{}
	animDone         chan struct{}
	hasLast          bool
	lastBounds       image.Rectangle
	lastCols         int
	lastRows         int
	lastDepth        int
	lastRects        []image.Rectangle
	currentAnimRects []image.Rectangle
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
		o.cancelAnimation()
		C.neru_x11_overlay_hide(o.raw)
	}
}

func (o *x11Overlay) Clear() {
	if o != nil && o.raw != nil {
		o.cancelAnimation()
		o.hasLast = false
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
		o.cancelAnimation()
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
	depth int,
	keys string,
	gridCols int,
	gridRows int,
	nextKeys string,
	nextGridCols int,
	nextGridRows int,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
	animEnabled bool,
	animDurationMS int,
) {
	o.DrawRecursiveGridWithSubKeyPreview(
		bounds, depth, keys, gridCols, gridRows,
		nextKeys, nextGridCols, nextGridRows,
		style, virtualPointer, animEnabled, animDurationMS,
	)
}

//nolint:mnd
func (o *x11Overlay) DrawRecursiveGridWithSubKeyPreview(
	bounds image.Rectangle,
	depth int,
	keys string,
	gridCols int,
	gridRows int,
	nextKeys string,
	nextGridCols int,
	nextGridRows int,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
	animEnabled bool,
	animDurationMS int,
) {
	if o == nil || o.raw == nil || bounds.Empty() || gridCols <= 0 || gridRows <= 0 {
		return
	}

	shouldAnimate := animEnabled && o.hasLast && depth != o.lastDepth &&
		!o.lastBounds.Empty()

	cellRects := recursivegrid.ComputeGridCells(bounds, gridCols, gridRows)

	if shouldAnimate {
		duration := time.Duration(animDurationMS) * time.Millisecond
		if duration <= 0 {
			duration = 50 * time.Millisecond
		}

		fromRects := o.buildFromRects(cellRects, bounds)
		keyRunes := []rune(strings.ToUpper(keys))
		nextKeyRunes := []rune(strings.ToUpper(nextKeys))

		animStop := make(chan struct{})
		animDone := make(chan struct{})
		o.animStop = animStop
		o.animDone = animDone

		o.startGridAnimation(
			fromRects, cellRects,
			keyRunes, nextKeyRunes,
			nextGridCols, nextGridRows,
			style, virtualPointer,
			duration, animStop, animDone,
		)
	} else {
		o.clearAndDraw(
			cellRects, keys, gridCols, gridRows,
			nextKeys, nextGridCols, nextGridRows,
			style, virtualPointer,
		)
	}

	o.hasLast = true
	o.lastBounds = bounds
	o.lastCols = gridCols
	o.lastRows = gridRows
	o.lastDepth = depth
	o.lastRects = make([]image.Rectangle, len(cellRects))
	copy(o.lastRects, cellRects)
}

func (o *x11Overlay) DrawBadge(
	posX, posY int,
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
}

func (o *x11Overlay) Flush() {
	if o == nil || o.raw == nil {
		return
	}
	C.neru_x11_overlay_flush(o.raw)
}

func (o *x11Overlay) DrawHints(hintsSlice []*hintscomponent.Hint, style hintscomponent.StyleMode) {
	if o == nil || o.raw == nil {
		return
	}

	o.cancelAnimation()
	o.hasLast = false
	C.neru_x11_overlay_clear(o.raw)
	fontSize := float64(max(style.FontSize(), 1))
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

		textColor := style.TextColor()
		if hint.MatchedPrefix() != "" {
			textColor = style.MatchedTextColor()
		}

		label := hint.Label()
		paddingX := resolveAutoPadding(fontSize, style.PaddingX(), true)
		paddingY := resolveAutoPadding(fontSize, style.PaddingY(), false)
		badgeWidth := estimateTextWidth(label, fontSize) + paddingX*paddingMultiplier
		badgeHeight := estimateTextHeight(fontSize) + paddingY*paddingMultiplier

		centerX := hint.Position().X + hint.Size().X/centeredRectDivisor
		centerY := hint.Position().Y + hint.Size().Y/centeredRectDivisor
		switch style.Placement() {
		case "top":
			centerY = hint.Position().Y
		case "bottom":
			centerY = hint.Position().Y + hint.Size().Y
		}

		badge := image.Rect(
			centerX-badgeWidth/centeredRectDivisor,
			centerY-badgeHeight/centeredRectDivisor,
			centerX+badgeWidth/centeredRectDivisor,
			centerY+badgeHeight/centeredRectDivisor,
		)
		radius := style.BorderRadius()
		if radius < 0 {
			radius = badgeHeight / centeredRectDivisor
		}
		if radius > 0 {
			o.drawRoundedRect(
				badge,
				float64(radius),
				parseHexColor(style.BackgroundColor()),
				parseHexColor(style.BorderColor()),
				float64(max(style.BorderWidth(), 0)),
			)
		} else {
			o.drawRect(
				badge,
				parseHexColor(style.BackgroundColor()),
				parseHexColor(style.BorderColor()),
				float64(max(style.BorderWidth(), 0)),
			)
		}
		o.drawTextCentered(
			label, badge,
			style.FontFamily(),
			fontSize,
			parseHexColor(textColor),
		)
	}

	C.neru_x11_overlay_flush(o.raw)
}

// unexported helpers

func (o *x11Overlay) setRenderMu(mu *sync.Mutex) {
	o.renderMu = mu
}

func (o *x11Overlay) cancelAnimation() {
	o.cancelMu.Lock()

	var doneCh chan struct{}
	if o.animStop != nil {
		close(o.animStop)
		o.animStop = nil
	}
	if o.animDone != nil {
		doneCh = o.animDone
		o.animDone = nil
	}
	o.cancelMu.Unlock()

	if doneCh != nil {
		<-doneCh
	}
}

//nolint:mnd,varnamelen
func (o *x11Overlay) buildFromRects(
	toRects []image.Rectangle,
	bounds image.Rectangle,
) []image.Rectangle {
	if len(o.currentAnimRects) == len(toRects) {
		from := make([]image.Rectangle, len(o.currentAnimRects))
		copy(from, o.currentAnimRects)

		return from
	}

	if len(o.lastRects) == len(toRects) {
		from := make([]image.Rectangle, len(o.lastRects))
		copy(from, o.lastRects)

		return from
	}

	if o.lastBounds.Empty() {
		from := make([]image.Rectangle, len(toRects))
		for idx, rect := range toRects {
			cx := rect.Min.X + rect.Dx()/2
			cy := rect.Min.Y + rect.Dy()/2
			from[idx] = image.Rect(cx, cy, cx, cy)
		}

		return from
	}

	fromBounds := o.lastBounds
	fw := float64(fromBounds.Dx())
	fh := float64(fromBounds.Dy())
	dw := float64(bounds.Dx())
	dh := float64(bounds.Dy())
	from := make([]image.Rectangle, len(toRects))
	for idx, rect := range toRects {
		nx := (float64(rect.Min.X+rect.Dx()/2) - float64(bounds.Min.X)) / dw
		ny := (float64(rect.Min.Y+rect.Dy()/2) - float64(bounds.Min.Y)) / dh
		cx := int(float64(fromBounds.Min.X) + nx*fw)
		cy := int(float64(fromBounds.Min.Y) + ny*fh)
		rw := rect.Dx()
		rh := rect.Dy()
		from[idx] = image.Rect(
			cx-rw/2, cy-rh/2,
			cx+rw/2, cy+rh/2,
		)
	}

	return from
}

//nolint:varnamelen
func (o *x11Overlay) startGridAnimation(
	fromRects, toRects []image.Rectangle,
	keyRunes, nextKeyRunes []rune,
	nextGridCols, nextGridRows int,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
	duration time.Duration,
	stopCh chan struct{},
	doneCh chan struct{},
) {
	startTime := time.Now()

	renderFrame := func(rawProgress float64) {
		if rawProgress >= 1.0 {
			rawProgress = 1.0
		}
		progress := easeInOut(rawProgress)

		interpCells := make([]image.Rectangle, len(toRects))
		for i := range toRects {
			src := fromRects[i]
			dst := toRects[i]
			interpCells[i] = image.Rect(
				int(lerp(float64(src.Min.X), float64(dst.Min.X), progress)),
				int(lerp(float64(src.Min.Y), float64(dst.Min.Y), progress)),
				int(lerp(float64(src.Max.X), float64(dst.Max.X), progress)),
				int(lerp(float64(src.Max.Y), float64(dst.Max.Y), progress)),
			)
		}

		o.currentAnimRects = interpCells

		C.neru_x11_overlay_clear_buffered(o.raw)
		o.drawFrame(
			interpCells,
			keyRunes,
			nextKeyRunes,
			nextGridCols,
			nextGridRows,
			style,
			virtualPointer,
		)
	}

	go func() {
		defer close(doneCh)
		defer func() {
			o.cancelMu.Lock()
			if o.animStop == stopCh {
				o.animStop = nil
			}
			if o.animDone == doneCh {
				o.animDone = nil
			}
			o.cancelMu.Unlock()
		}()

		for {
			select {
			case <-stopCh:
				return
			default:
			}

			elapsed := time.Since(startTime)
			rawProgress := float64(elapsed) / float64(duration)
			if rawProgress >= 1.0 {
				rawProgress = 1.0
			}

			renderStart := time.Now()

			mu := o.renderMu
			if mu != nil {
				mu.Lock()
				select {
				case <-stopCh:
					mu.Unlock()

					return
				default:
				}
			}
			renderFrame(rawProgress)
			if mu != nil {
				mu.Unlock()
			}

			if rawProgress >= 1.0 {
				return
			}

			renderDur := time.Since(renderStart)
			sleepFor := animationFrameDur - renderDur
			if sleepFor > 0 {
				select {
				case <-stopCh:
					return
				case <-time.After(sleepFor):
				}
			}
		}
	}()
}

func (o *x11Overlay) clearAndDraw(
	cellRects []image.Rectangle,
	keys string, gridCols, gridRows int,
	nextKeys string, nextGridCols, nextGridRows int,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
) {
	if o == nil || o.raw == nil {
		return
	}

	o.currentAnimRects = nil

	keyRunes := []rune(strings.ToUpper(keys))
	nextKeyRunes := []rune(strings.ToUpper(nextKeys))

	C.neru_x11_overlay_clear(o.raw)
	o.drawFrame(
		cellRects,
		keyRunes,
		nextKeyRunes,
		nextGridCols,
		nextGridRows,
		style,
		virtualPointer,
	)
}

func (o *x11Overlay) drawFrame(
	cellRects []image.Rectangle,
	keyRunes, nextKeyRunes []rune,
	nextGridCols, nextGridRows int,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
) {
	drawSubPreview := style.SubKeyPreview && len(nextKeyRunes) > 0 &&
		nextGridCols > 0 && nextGridRows > 0

	for idx, cell := range cellRects {
		if cell.Empty() {
			continue
		}

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
					label, cell, style.LabelFontName,
					style.LabelFontSize, style.LabelFontColor,
				)
			}

			if drawSubPreview &&
				shouldShowSubKeyPreview(cell, style, nextGridCols, nextGridRows) {
				o.drawSubKeyMiniGrid(cell, nextKeyRunes,
					nextGridCols, nextGridRows, style)
			}
		}
	}

	if virtualPointer.Visible {
		o.drawVirtualPointer(virtualPointer)
	}

	C.neru_x11_overlay_flush(o.raw)
}

//nolint:mnd,varnamelen
func (o *x11Overlay) drawVirtualPointer(vp recursivegridcomponent.VirtualPointerState) {
	vpChar := vp.Char
	if vpChar == "" {
		vpChar = "\u25CF"
	}

	fontName := ports.ResolveFont(vp.FontName, false)
	fontSize := float64(vp.Size)
	halfSize := max(vp.Size/2, 1)
	vpBounds := image.Rect(
		vp.Position.X-halfSize,
		vp.Position.Y-halfSize,
		vp.Position.X+halfSize,
		vp.Position.Y+halfSize,
	)
	o.drawTextCentered(vpChar, vpBounds, fontName, fontSize,
		parseHexColor(vp.FillColor))
}

func (o *x11Overlay) redrawGrid() {
	if o == nil || o.raw == nil || o.cachedGrid == nil {
		return
	}

	C.neru_x11_overlay_clear(o.raw)
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
		o.drawTextCentered(label, cell.Bounds(),
			style.LabelFontName, style.LabelFontSize, text)
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
			float64(i)*float64(bounds.Dx())/float64(subgridCols)+
				subgridHalfPixel,
		)
	}
	for i := 1; i <= subgridRows; i++ {
		yBreaks[i] = bounds.Min.Y + int(
			float64(i)*float64(bounds.Dy())/float64(subgridRows)+
				subgridHalfPixel,
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
				xBreaks[col], yBreaks[row],
				xBreaks[col+1], yBreaks[row+1],
			)
			o.drawRect(cell, style.BackgroundColor,
				style.LineColor, style.LineWidth)
			o.drawTextCentered(
				string(keyRunes[index]), cell,
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
	fill uint32, border uint32, lineWidth float64,
) {
	C.neru_x11_overlay_rect(
		o.raw,
		C.double(bounds.Min.X), C.double(bounds.Min.Y),
		C.double(bounds.Dx()), C.double(bounds.Dy()),
		C.uint(fill), C.uint(border), C.double(lineWidth),
	)
}

func (o *x11Overlay) drawRoundedRect(
	bounds image.Rectangle,
	radius float64,
	fill uint32, border uint32, lineWidth float64,
) {
	C.neru_x11_overlay_rounded_rect(
		o.raw,
		C.double(bounds.Min.X), C.double(bounds.Min.Y),
		C.double(bounds.Dx()), C.double(bounds.Dy()),
		C.double(radius),
		C.uint(fill), C.uint(border), C.double(lineWidth),
	)
}

func (o *x11Overlay) drawTextCentered(
	text string, bounds image.Rectangle,
	fontFamily string, fontSize float64, color uint32,
) {
	cText := C.CString(text)
	cFontFamily := C.CString(fontFamily)

	defer C.free(unsafe.Pointer(cText))       //nolint:nlreturn
	defer C.free(unsafe.Pointer(cFontFamily)) //nolint:nlreturn

	C.neru_x11_overlay_text(
		o.raw, cText, cFontFamily,
		C.double(bounds.Min.X+bounds.Dx()/2),
		C.double(bounds.Min.Y+bounds.Dy()/2),
		C.double(fontSize), C.uint(color),
	)
}

func (o *x11Overlay) drawLabelBackground(
	label string, cell image.Rectangle,
	style recursivegridcomponent.Style,
) {
	fontSize := style.LabelFontSize
	paddingX := resolveAutoPadding(fontSize,
		style.LabelBackgroundPaddingX, true)
	paddingY := resolveAutoPadding(fontSize,
		style.LabelBackgroundPaddingY, false)
	width := estimateTextWidth(label, fontSize) +
		paddingX*paddingMultiplier
	height := estimateTextHeight(fontSize) +
		paddingY*paddingMultiplier
	rect := centeredRect(cell, width, height)
	o.drawRect(rect, style.LabelBackgroundColor,
		style.LineColor, max(style.LabelBackgroundBorderWidth, 0))
}

//nolint:mnd
func (o *x11Overlay) drawSubKeyMiniGrid(
	cell image.Rectangle,
	nextKeyRunes []rune,
	nextGridCols int, nextGridRows int,
	style recursivegridcomponent.Style,
) {
	subCells := recursivegrid.ComputeGridCells(cell, nextGridCols, nextGridRows)
	centerIdx := -1

	if nextGridCols%2 == 1 && nextGridRows%2 == 1 {
		centerIdx = (nextGridRows/2)*nextGridCols + nextGridCols/2
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
			subLabel, subCell,
			style.LabelFontName, style.SubKeyPreviewFontSize,
			style.SubKeyPreviewTextColor,
		)
		subIndex++
	}
}
