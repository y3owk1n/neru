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
	sublayerKeys   string
	cachedGrid     *domainGrid.Grid
	cachedStyle    gridcomponent.Style

	displayMu *sync.Mutex

	stopCh chan struct{}
	doneCh chan struct{}

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
		C.neru_wayland_overlay_setup_buffers(o.raw)
		C.neru_wayland_overlay_show(o.raw)
	}
}

func (o *wlrootsOverlay) Hide() {
	if o != nil && o.raw != nil {
		o.cancelAnimation()
		C.neru_wayland_overlay_hide(o.raw)
	}
}

func (o *wlrootsOverlay) Clear() {
	if o != nil && o.raw != nil {
		o.cancelAnimation()
		o.hasLast = false
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

	o.cancelAnimation()
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
	if !o.selectAvailableBuffer() {
		return
	}
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
	o.cachedGrid = g
	o.cachedStyle = style
	o.currentPrefix = strings.ToUpper(input)
	o.currentSubgrid = nil

	o.redrawGrid()
}

func (o *wlrootsOverlay) DrawRecursiveGrid(
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
func (o *wlrootsOverlay) DrawRecursiveGridWithSubKeyPreview(
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

	C.neru_wayland_overlay_setup_buffers(o.raw)
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

func (o *wlrootsOverlay) DrawBadge(
	posX, posY int,
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
	o.cancelAnimation()
	o.hasLast = false
	if !o.selectAvailableBuffer() {
		return
	}
	C.neru_wayland_overlay_clear(o.raw)
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

	C.neru_wayland_overlay_flush(o.raw)
}

// selectAvailableBuffer picks a buffer that the compositor has released.
// Falls back to sync (roundtrip) if none free, which forces release processing.
func (o *wlrootsOverlay) selectAvailableBuffer() bool {
	if o == nil || o.raw == nil {
		return false
	}
	C.neru_wayland_overlay_dispatch_pending(o.raw)
	bufIdx := C.neru_wayland_overlay_available_buffer(o.raw) //nolint:nlreturn
	if bufIdx < 0 {
		C.neru_wayland_overlay_sync(o.raw)
		bufIdx = C.neru_wayland_overlay_available_buffer(o.raw) //nolint:nlreturn
	}
	if bufIdx < 0 {
		return false
	}
	C.neru_wayland_overlay_select_buffer(o.raw, bufIdx)

	return true
}

// unexported helpers

func (o *wlrootsOverlay) setDisplayMu(mu *sync.Mutex) {
	o.displayMu = mu
}

func (o *wlrootsOverlay) startPoller() {
	go o.keyboardPoller()
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

func (o *wlrootsOverlay) cancelAnimation() {
	if o.animStop != nil {
		close(o.animStop)
		o.animStop = nil
	}
	if o.animDone != nil {
		<-o.animDone
		o.animDone = nil
	}
}

func (o *wlrootsOverlay) keyboardPoller() {
	defer close(o.doneCh)

	const pollInterval = 5 * time.Millisecond

	for {
		select {
		case <-o.stopCh:
			return
		default:
		}

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

//nolint:mnd,varnamelen
func (o *wlrootsOverlay) buildFromRects(
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

func (o *wlrootsOverlay) startGridAnimation(
	fromRects, toRects []image.Rectangle,
	keyRunes, nextKeyRunes []rune,
	nextGridCols, nextGridRows int,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
	duration time.Duration,
	stopCh chan struct{},
	doneCh chan struct{},
) {
	C.neru_wayland_overlay_sync(o.raw)

	startTime := time.Now()

	renderFrame := func(rawProgress float64) bool {
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

		C.neru_wayland_overlay_dispatch_pending(o.raw)
		bufIdx := C.neru_wayland_overlay_available_buffer(o.raw) //nolint:nlreturn
		if bufIdx < 0 {
			C.neru_wayland_overlay_sync(o.raw)
			bufIdx = C.neru_wayland_overlay_available_buffer(o.raw) //nolint:nlreturn
		}
		if bufIdx < 0 {
			return false
		}
		C.neru_wayland_overlay_select_buffer(o.raw, bufIdx)

		o.currentAnimRects = interpCells

		C.neru_wayland_overlay_clear(o.raw)
		o.drawFrame(
			interpCells, keyRunes, nextKeyRunes,
			nextGridCols, nextGridRows, style, virtualPointer,
		)

		return true
	}

	go func() {
		defer close(doneCh)

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

			if o.displayMu != nil {
				o.displayMu.Lock()
				// Parent may have closed stopCh while we were waiting
				// for the lock. Check here to avoid deadlock:
				//   parent holds displayMu, waits for animDone
				//   we   hold displayMu, parent waits for displayMu
				select {
				case <-stopCh:
					o.displayMu.Unlock()

					return
				default:
				}
			}
			_ = renderFrame(rawProgress)
			if o.displayMu != nil {
				o.displayMu.Unlock()
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

func (o *wlrootsOverlay) clearAndDraw(
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

	if !o.selectAvailableBuffer() {
		return
	}
	C.neru_wayland_overlay_clear(o.raw)
	o.drawFrame(cellRects, keyRunes, nextKeyRunes,
		nextGridCols, nextGridRows, style, virtualPointer)
}

func (o *wlrootsOverlay) drawFrame(
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

	C.neru_wayland_overlay_flush(o.raw)
}

//nolint:mnd,varnamelen
func (o *wlrootsOverlay) drawVirtualPointer(vp recursivegridcomponent.VirtualPointerState) {
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

func (o *wlrootsOverlay) redrawGrid() {
	if o == nil || o.raw == nil || o.cachedGrid == nil {
		return
	}

	C.neru_wayland_overlay_setup_buffers(o.raw)
	if !o.selectAvailableBuffer() {
		return
	}
	C.neru_wayland_overlay_clear(o.raw)
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
	C.neru_wayland_overlay_flush(o.raw)
}

func (o *wlrootsOverlay) drawSubgrid(bounds image.Rectangle, style gridcomponent.Style) {
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

func (o *wlrootsOverlay) drawRect(
	bounds image.Rectangle,
	fill uint32, border uint32, lineWidth float64,
) {
	C.neru_wayland_overlay_rect(
		o.raw,
		C.double(bounds.Min.X), C.double(bounds.Min.Y),
		C.double(bounds.Dx()), C.double(bounds.Dy()),
		C.uint(fill), C.uint(border), C.double(lineWidth),
	)
}

func (o *wlrootsOverlay) drawRoundedRect(
	bounds image.Rectangle,
	radius float64,
	fill uint32, border uint32, lineWidth float64,
) {
	C.neru_wayland_overlay_rounded_rect(
		o.raw,
		C.double(bounds.Min.X), C.double(bounds.Min.Y),
		C.double(bounds.Dx()), C.double(bounds.Dy()),
		C.double(radius),
		C.uint(fill), C.uint(border), C.double(lineWidth),
	)
}

func (o *wlrootsOverlay) drawTextCentered(
	text string, bounds image.Rectangle,
	fontFamily string, fontSize float64, color uint32,
) {
	cText := C.CString(text)
	cFontFamily := C.CString(fontFamily)

	defer C.free(unsafe.Pointer(cText))       //nolint:nlreturn
	defer C.free(unsafe.Pointer(cFontFamily)) //nolint:nlreturn

	C.neru_wayland_overlay_text(
		o.raw, cText, cFontFamily,
		C.double(bounds.Min.X+bounds.Dx()/2),
		C.double(bounds.Min.Y+bounds.Dy()/2),
		C.double(fontSize), C.uint(color),
	)
}

func (o *wlrootsOverlay) drawLabelBackground(
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
func (o *wlrootsOverlay) drawSubKeyMiniGrid(
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
