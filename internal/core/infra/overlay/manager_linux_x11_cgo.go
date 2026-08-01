//go:build linux && cgo

package overlay

/*
#cgo linux pkg-config: x11 xrender xfixes xext cairo
#include <stdlib.h>
#include "../platform/linux/x11_overlay.h"
*/
import "C"

import (
	"image"
	"strings"
	"sync"
	"time"
	"unsafe"

	"go.uber.org/zap"

	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/core/domain/recursivegrid"
	gridcomponent "github.com/y3owk1n/neru/internal/core/infra/overlay/render/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/core/infra/overlay/render/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/core/infra/overlay/render/recursivegrid"
	_ "github.com/y3owk1n/neru/internal/core/infra/platform/linux"
	"github.com/y3owk1n/neru/internal/core/ports"
)

type x11Overlay struct {
	raw *C.NeruX11Overlay
	// scale is the desktop-wide HiDPI UI factor from Xft.dpi (>= 1.0). X11 has a
	// single device-pixel coordinate space and no per-monitor scale, so hint/label
	// positions stay in device pixels and only element sizes (fonts, stroke widths,
	// badge geometry) are multiplied by this factor for legibility on HiDPI screens.
	scale          float64
	logger         *zap.Logger
	currentPrefix  string
	hideUnmatched  bool
	currentSubgrid *domainGrid.Cell
	sublayerKeys   string
	cachedGrid     *domainGrid.Grid
	cachedStyle    gridcomponent.Style

	// originOffset is the active screen's top-left origin in global device
	// pixels. Grid, recursive-grid and hint content arrives in screen-local
	// coordinates (origin 0,0); adding this offset places it on the correct
	// monitor of the desktop-spanning overlay window. Absolute-coordinate draws
	// (badges, monitor_select, the click indicator) do not apply it.
	originOffset image.Point

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

	scale := float64(C.neru_x11_overlay_scale(raw)) //nolint:nlreturn
	if scale <= 0 {
		scale = 1
	}

	return &x11Overlay{raw: raw, logger: logger, scale: scale}
}

// Scale exposes the overlay's HiDPI scale so the manager can size badge
// geometry (and its clear rects) consistently with what the overlay renders.
func (o *x11Overlay) Scale() float64 {
	return o.s()
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

	// Translate the screen-local bounds and virtual-pointer position onto the
	// active monitor. Everything downstream (cell rects, animation from/to
	// rects, the pointer) derives from these, so the whole frame lands on the
	// right monitor.
	bounds = o.offset(bounds)
	virtualPointer.Position = virtualPointer.Position.Add(o.originOffset)

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
	// Size the badge from the scaled font so it fits the text drawTextCentered
	// renders. The manager sizes its clear rect with the same factor (Scale()).
	scaledStyle := style
	scaledStyle.fontSize = fontSize * o.s()
	rect := badgeBounds(posX, posY, text, scaledStyle)

	o.drawRect(rect, colors.background, colors.border, max(style.borderWidth, 1))
	o.drawTextCentered(text, rect, style.fontFamily, fontSize, colors.text)
}

func (o *x11Overlay) Flush() {
	if o == nil || o.raw == nil {
		return
	}
	C.neru_x11_overlay_flush(o.raw)
}

// DrawMonitorSelect renders one centered, labeled panel per monitor for the
// interactive monitor picker. Panels reuse the existing rounded-rect + text
// primitives (no dedicated C), and are sized from the scaled font (see
// monitorSelectPanelLayout) so they stay legible on HiDPI. The label is drawn
// with the matched/selected color when it has a matched prefix or is selected.
func (o *x11Overlay) DrawMonitorSelect(targets []MonitorSelectTarget, style MonitorSelectStyle) {
	if o == nil || o.raw == nil {
		return
	}

	o.cancelAnimation()
	o.hasLast = false
	C.neru_x11_overlay_clear(o.raw)

	spec := newMonitorSelectDrawSpec(style)
	for _, target := range targets {
		if target.Bounds.Empty() {
			continue
		}

		if spec.hasBackdrop {
			o.drawRect(target.Bounds, spec.backdrop, 0, 0)
		}

		panel, labelRect, subtitleRect, radius := monitorSelectPanelLayout(
			target.Bounds, target.Label, target.Subtitle, style, o.s(),
		)
		o.drawRoundedRect(panel, radius, spec.background, spec.border, spec.borderWidth)

		o.drawTextCentered(target.Label, labelRect, style.FontFamily, spec.labelFont, spec.text)

		if target.Subtitle != "" {
			o.drawTextCentered(
				target.Subtitle, subtitleRect,
				monitorSelectSubtitleFamily(style), spec.subtitleFont, spec.subtitleText,
			)
		}
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
		// hint.Position() is the element center in screen-local coordinates;
		// translate it onto the active monitor.
		pos := hint.Position().Add(o.originOffset)
		if style.BoundaryHighlightEnabled() {
			boundary := image.Rect(
				pos.X-hint.Size().X/2,
				pos.Y-hint.Size().Y/2,
				pos.X+hint.Size().X/2,
				pos.Y+hint.Size().Y/2,
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
		// Size the badge from the scaled font so it fits the text drawTextCentered
		// renders (which applies the same o.s() factor). Position stays in device
		// pixels; the badge grows around the target.
		sfont := fontSize * o.s()
		paddingX := resolveAutoPadding(sfont, style.PaddingX(), true)
		paddingY := resolveAutoPadding(sfont, style.PaddingY(), false)
		badgeWidth := estimateTextWidth(label, sfont) + paddingX*paddingMultiplier
		badgeHeight := estimateTextHeight(sfont) + paddingY*paddingMultiplier

		radius := style.BorderRadius()
		if radius < 0 {
			radius = min(badgeHeight/centeredRectDivisor, hintAutoRadiusMax)
		}
		// Cap the radius so a top/bottom badge keeps a flat edge for the tail.
		radius = hintBadgeRadius(radius, badgeWidth, style.Placement())

		// pos is the element center (set in modes/hints.go). The badge is
		// centered horizontally on it and placed above / on / below the center
		// to match macOS; top/bottom placement also draws a connector arrow
		// pointing back at the target.
		badge, arrow, hasArrow := hintBadgePlacement(
			pos, badgeWidth, badgeHeight, radius, style.Placement(),
		)

		fill := parseHexColor(style.BackgroundColor())
		border := parseHexColor(style.BorderColor())
		borderWidth := float64(max(style.BorderWidth(), 0))

		// Badge and connector tail are drawn as one filled+stroked outline so
		// translucent colors don't double-composite at the junction.
		o.drawHintBadge(
			badge, float64(radius), hintTailEdge(badge, arrow, hasArrow), arrow,
			fill, border, borderWidth,
		)
		o.drawTextCentered(
			label, badge,
			style.FontFamily(),
			fontSize,
			parseHexColor(textColor),
		)
	}

	C.neru_x11_overlay_flush(o.raw)
}

// DrawMouseActionIndicator animates a transient click indicator centered on
// point. It runs on this overlay's dedicated indicator window, independent of
// the mode overlay's show/hide lifecycle, so it survives the mode exit that
// immediately follows a click.
func (o *x11Overlay) DrawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
	if o == nil || o.raw == nil {
		return
	}

	// Cancel any in-flight indicator animation before starting a new one.
	o.cancelAnimation()

	duration := time.Duration(style.DurationMS) * time.Millisecond
	if duration <= 0 {
		duration = defaultMouseActionDuration
	}

	animStop := make(chan struct{})
	animDone := make(chan struct{})
	o.animStop = animStop
	o.animDone = animDone

	C.neru_x11_overlay_show(o.raw)
	o.startMouseActionAnimation(point, style, duration, animStop, animDone)
}

func (o *x11Overlay) startMouseActionAnimation(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
	duration time.Duration,
	stopCh chan struct{},
	doneCh chan struct{},
) {
	startTime := time.Now()

	fillBase := parseHexColor(style.BackgroundColor)
	borderBase := parseHexColor(style.BorderColor)
	lineWidth := float64(max(style.BorderWidth, 0))
	baseSize := float64(max(style.Size, 1)) * o.s()
	isSquare := style.Shape == "square"

	renderFrame := func(rawProgress float64) {
		eased := applyEasing(style.Easing, rawProgress)
		scale := max(lerp(style.StartScale, style.EndScale, eased), 0)
		opacity := lerp(style.StartOpacity, style.EndOpacity, eased)
		diameter := baseSize * scale
		rect := mouseActionIndicatorRect(point, diameter)
		fill := applyOpacity(fillBase, opacity)
		border := applyOpacity(borderBase, opacity)

		C.neru_x11_overlay_clear_buffered(o.raw)
		if isSquare {
			o.drawRect(rect, fill, border, lineWidth)
		} else {
			o.drawRoundedRect(rect, diameter/centeredRectDivisor, fill, border, lineWidth)
		}
		C.neru_x11_overlay_flush(o.raw)
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
			rawProgress := min(float64(elapsed)/float64(duration), 1.0)

			renderStart := time.Now()

			if o.renderMu != nil {
				o.renderMu.Lock()
				select {
				case <-stopCh:
					o.renderMu.Unlock()

					return
				default:
				}
			}
			renderFrame(rawProgress)
			if o.renderMu != nil {
				o.renderMu.Unlock()
			}

			if rawProgress >= 1.0 {
				// Finished: clear and unmap the dedicated window so the fully
				// faded indicator does not linger on screen.
				if o.renderMu != nil {
					o.renderMu.Lock()
				}
				C.neru_x11_overlay_clear(o.raw)
				C.neru_x11_overlay_hide(o.raw)
				if o.renderMu != nil {
					o.renderMu.Unlock()
				}

				return
			}

			sleepFor := animationFrameDur - time.Since(renderStart)
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

// unexported helpers

// s returns the overlay's HiDPI scale factor, guarding against a zero value.
func (o *x11Overlay) s() float64 {
	if o == nil || o.scale <= 0 {
		return 1
	}

	return o.scale
}

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
		cellBounds := o.offset(cell.Bounds())
		o.drawRect(cellBounds, fill, border, style.LineWidth)
		o.drawTextCentered(label, cellBounds,
			style.LabelFontName, style.LabelFontSize, text)
	}

	if o.currentSubgrid != nil {
		o.drawSubgrid(o.currentSubgrid.Bounds(), style)
	}
	C.neru_x11_overlay_flush(o.raw)
}

func (o *x11Overlay) drawSubgrid(bounds image.Rectangle, style gridcomponent.Style) {
	// bounds is screen-local; place the subgrid on the active monitor.
	bounds = o.offset(bounds)
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

// Stroke widths scale with the HiDPI factor here so every draw path (grid,
// hints, badges, indicator) gets consistent line weight without per-call-site
// scaling. Element geometry that must fit scaled text is sized by the callers.
// setOriginOffset stores the active screen origin used to translate
// screen-local grid/recursive-grid/hint coordinates onto the correct monitor.
func (o *x11Overlay) setOriginOffset(origin image.Point) {
	if o == nil {
		return
	}

	o.originOffset = origin
}

// offset translates a screen-local rectangle into global desktop coordinates.
func (o *x11Overlay) offset(r image.Rectangle) image.Rectangle {
	return r.Add(o.originOffset)
}

func (o *x11Overlay) drawRect(
	bounds image.Rectangle,
	fill uint32, border uint32, lineWidth float64,
) {
	C.neru_x11_overlay_rect(
		o.raw,
		C.double(bounds.Min.X), C.double(bounds.Min.Y),
		C.double(bounds.Dx()), C.double(bounds.Dy()),
		C.uint(fill), C.uint(border), C.double(lineWidth*o.s()),
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
		C.uint(fill), C.uint(border), C.double(lineWidth*o.s()),
	)
}

func (o *x11Overlay) drawHintBadge(
	badge image.Rectangle, radius float64, edge int, arrow hintArrowTriangle,
	fill uint32, border uint32, lineWidth float64,
) {
	C.neru_x11_overlay_hint_badge(
		o.raw,
		C.double(badge.Min.X), C.double(badge.Min.Y),
		C.double(badge.Dx()), C.double(badge.Dy()),
		C.double(radius),
		C.int(edge),
		C.double(arrow.baseLeft.X), C.double(arrow.baseRight.X),
		C.double(arrow.tip.X), C.double(arrow.tip.Y),
		C.uint(fill), C.uint(border), C.double(lineWidth*o.s()),
	)
}

// drawTextCentered applies the HiDPI scale to the font size centrally, so every
// label/badge caller passes its base (logical) font size and text renders at the
// device-appropriate size. Callers that size geometry around the text must use
// the same scaled font (fontSize * o.s()).
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
		C.double(fontSize*o.s()), C.uint(color),
	)
}

func (o *x11Overlay) drawLabelBackground(
	label string, cell image.Rectangle,
	style recursivegridcomponent.Style,
) {
	// Match the scaled font that drawTextCentered renders for the label.
	fontSize := style.LabelFontSize * o.s()
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
