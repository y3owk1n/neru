//go:build linux && cgo

package linux

import (
	"image"
	"strings"
	"sync"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/motion"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
	"github.com/y3owk1n/neru/internal/ports"
)

// overlaySurface is the primitive contract a Linux overlay backend provides
// to the shared drawing code. Layout, animation, offsets and label logic live
// once on sharedOverlay. Buffer management, HiDPI scale and window lifecycle
// live behind this interface. The shared code never touches cgo, so
// primitives take Go types and do their own C marshaling.
type overlaySurface interface {
	// alive reports whether the backend still holds an open native handle. A
	// draw that reaches C through a closed one is a nil dereference inside
	// cgo, and the shared delegates cannot see the handle for themselves.
	alive() bool

	// surfaceScale is the HiDPI factor applied to fonts, stroke widths and
	// text-fitted geometry. X11 probes Xft.dpi and Wayland reports 1.
	surfaceScale() float64

	// ensureBuffers prepares the backend's drawing target without selecting
	// it (Wayland's setup_buffers, a no-op on X11). Some paths call it
	// without a subsequent frame selection, so preserve that.
	ensureBuffers()

	// beginFrame makes a drawable target current, reporting false when none
	// is available and the frame must be skipped (Wayland's buffer
	// acquisition with one sync-retry, X11 always succeeds).
	beginFrame() bool

	// surfaceClear clears the current target without any buffer selection.
	surfaceClear()

	// clearFrame clears inside an animation frame (X11's double-buffered
	// clear, plain clear on Wayland, which selected a buffer in beginFrame).
	clearFrame()

	surfaceClearRect(rect image.Rectangle)
	surfaceFlush()
	surfaceHide()

	// showIndicator maps the dedicated click-indicator surface before its
	// animation starts. Wayland must do this under the render lock and keep
	// the surface keyboard-passive. X11 just maps the window.
	showIndicator()

	// finishIndicator unmaps the indicator when its animation completes. The
	// caller holds the render lock. X11 also clears so no stale pixmap shows
	// on the next map. Wayland must not clear an unselected buffer.
	finishIndicator()

	// syncBeforeAnimation flushes pending display work before a grid
	// animation begins (Wayland sync, a no-op on X11).
	syncBeforeAnimation()

	rectPrim(bounds image.Rectangle, fill, border uint32, lineWidth float64)
	roundedRectPrim(bounds image.Rectangle, radius float64, fill, border uint32, lineWidth float64)
	hintBadgePrim(
		badgeRect image.Rectangle, radius float64, edge int, arrow badge.HintArrow,
		fill, border uint32, lineWidth float64,
	)
	textPrim(text, fontFamily string, centerX, centerY, fontSize float64, color uint32, bold bool)
}

// sharedOverlay is the backend-independent half of a Linux overlay: all
// drawing, layout and animation state, the logic that drives the
// overlaySurface primitives, and the exported methods the manager calls. X11
// and Wayland embed it, so those methods are promoted into both, which is what
// makes the manager's nil-checked dispatch load-bearing (../AGENTS.md).
type sharedOverlay struct {
	srf overlaySurface

	currentPrefix  string
	hideUnmatched  bool
	currentSubgrid *domainGrid.Cell
	sublayerKeys   string
	cachedGrid     *domainGrid.Grid
	cachedStyle    gridcomponent.Style

	// gridPointer is the pointer stand-in grid mode asked for. It is painted
	// in the same pass as the cells, like recursive grid's pointer, so every
	// repaint reads it. The zero value is off screen, which is what the mode
	// asks for on activation and exit.
	gridPointer recursivegridcomponent.VirtualPointerState

	// originOffset is the active screen's top-left origin in global device
	// pixels. Grid, recursive-grid and hint content arrives screen-local and
	// gets it added to land on the right monitor of the desktop-spanning
	// window. Absolute draws (indicator badges, monitor_select, the click
	// indicator) do not. The hints search badge does, because its frame is
	// placed against the screen the caller named.
	originOffset image.Point

	// lastHints is the hint set this surface was last painted with, so the
	// search badge can be taken off without erasing the labels. Badge and
	// hints share one Cairo target, so hiding the badge means repainting the
	// hints, since clearing a rectangle would take a bite out of them.
	//
	// clear() and hide() drop it, which is where every mode transition goes
	// (`Adapter.ClearFrame`). Only a badge draw sets the manager's
	// searchBadgeRect, which decides whether this is read at all.
	lastHints      []*hintscomponent.Hint
	lastHintStyle  hintscomponent.StyleMode
	lastHintOffset badge.HintOffset

	// renderMu serializes rendering with the manager (and, on Wayland, with
	// the keyboard poller's wl_display access). The manager owns it and holds
	// it across synchronous draws. Only the animation goroutines lock it here.
	renderMu *sync.Mutex

	cancelMu         sync.Mutex
	animStop         chan struct{}
	animDone         chan struct{}
	hasLast          bool
	lastBounds       image.Rectangle
	lastDepth        int
	lastRects        []image.Rectangle
	currentAnimRects []image.Rectangle
	// animSettled says the transition that painted currentAnimRects reached
	// its last frame. One that has not is continued rather than restarted.
	animSettled bool
	// animPointer is where the last transition frame painted the virtual
	// pointer, and lastPointer where the last settled frame did. The pointer
	// rides the zoom from one of them to where the new frame puts it.
	animPointer image.Point
	lastPointer recursivegridcomponent.VirtualPointerState
}

// The exported methods below are what the manager calls on a backend. All but
// Flush make no C call. Show, Resize and Destroy stay per-backend, because
// Wayland re-runs buffer setup before showing, layer shells auto-resize, and
// Wayland's Destroy waits on the keyboard poller. Each guards itself with
// drawable, but a nil backend pointer is the manager's nil check to catch
// (../AGENTS.md).

func (o *sharedOverlay) Hide() {
	if !o.drawable() {
		return
	}

	o.hide()
}

func (o *sharedOverlay) Clear() {
	if !o.drawable() {
		return
	}

	o.clear()
}

func (o *sharedOverlay) ClearRect(rect image.Rectangle) {
	if !o.drawable() || rect.Empty() {
		return
	}

	o.clearRect(rect)
}

func (o *sharedOverlay) UpdateGridMatches(prefix string) {
	if !o.drawable() {
		return
	}

	o.updateGridMatches(prefix)
}

// ShowSubgrid opens the finer grid inside one cell, with the pointer stand-in
// the same keystroke moved (#1492).
//
// The pointer is recorded after the nil-cell check and before the surface
// check, as in SetGridPointer. A call with no cell said nothing about this
// surface, and recording its pointer would let SetGridPointer's equality guard
// swallow the call that meant it. There is no equality guard here, because an
// open repaints the surface anyway.
func (o *sharedOverlay) ShowSubgrid(
	cell *domainGrid.Cell,
	_ gridcomponent.Style,
	pointer recursivegridcomponent.VirtualPointerState,
) {
	if cell == nil {
		return
	}

	o.gridPointer = pointer

	if !o.drawable() {
		return
	}

	o.showSubgrid(cell)
}

// SetHideUnmatched sets a Go field and reaches no surface, so it deliberately
// does not ask drawable.
func (o *sharedOverlay) SetHideUnmatched(hide bool) {
	o.hideUnmatched = hide
}

func (o *sharedOverlay) DrawGrid(g *domainGrid.Grid, input string, style gridcomponent.Style) {
	if !o.drawable() || g == nil {
		return
	}

	o.drawGrid(g, input, style)
}

// SetGridPointer records where grid mode's pointer stand-in belongs and
// repaints the grid surface so it appears there, or takes it off when the
// state is not visible. The record is kept even without a surface, since the
// next repaint reads it. An unchanged state paints nothing, because grid mode
// refreshes the pointer on every keystroke. It repaints the whole surface
// rather than erasing the old glyph, for the reason given at lastHints.
func (o *sharedOverlay) SetGridPointer(pointer recursivegridcomponent.VirtualPointerState) {
	if pointer == o.gridPointer {
		return
	}

	o.gridPointer = pointer

	if !o.drawable() {
		return
	}

	o.redrawGrid()
}

func (o *sharedOverlay) DrawRecursiveGridWithSubKeyPreview(
	bounds image.Rectangle,
	depth int,
	keys string,
	dims domain.GridDimensions,
	nextKeys string,
	nextDims domain.GridDimensions,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
	animEnabled bool,
	animDurationMS int,
) {
	if !o.drawable() || bounds.Empty() || dims.Cols <= 0 || dims.Rows <= 0 {
		return
	}

	o.drawRecursiveGridWithSubKeyPreview(
		bounds, depth, keys, dims,
		nextKeys, nextDims,
		style, virtualPointer, animEnabled, animDurationMS,
	)
}

func (o *sharedOverlay) DrawBadge(
	posX, posY int,
	text string,
	colors overlayColors,
	style overlayBadgeStyle,
) {
	if !o.drawable() || text == "" {
		return
	}

	o.drawBadge(posX, posY, text, colors, style)
}

// Flush is the only delegate that reaches C.
func (o *sharedOverlay) Flush() {
	if !o.drawable() {
		return
	}

	o.srf.surfaceFlush()
}

func (o *sharedOverlay) DrawMonitorSelect(
	targets []manager.MonitorSelectTarget,
	style manager.MonitorSelectStyle,
) {
	if !o.drawable() {
		return
	}

	o.drawMonitorSelect(targets, style)
}

func (o *sharedOverlay) DrawHints(
	hintsSlice []*hintscomponent.Hint,
	style hintscomponent.StyleMode,
	offset badge.HintOffset,
) {
	if !o.drawable() {
		return
	}

	o.drawHints(hintsSlice, style, offset)
}

func (o *sharedOverlay) DrawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
	if !o.drawable() {
		return
	}

	o.drawMouseActionIndicator(point, style)
}

// DrawHintSearchInput paints the hint-search badge and answers the rectangle it
// covered, so the manager knows what to put back when it is hidden. The empty
// rectangle means nothing was painted.
func (o *sharedOverlay) DrawHintSearchInput(
	label string,
	frame hintscomponent.SearchInputFrame,
	style hintscomponent.SearchInputStyle,
) image.Rectangle {
	if !o.drawable() {
		return image.Rectangle{}
	}

	return o.drawHintSearchInput(label, frame, style)
}

// HideHintSearchInput takes the badge painted over the given rectangle off the
// surface.
func (o *sharedOverlay) HideHintSearchInput(painted image.Rectangle) {
	if !o.drawable() || painted.Empty() {
		return
	}

	o.hideHintSearchInput(painted)
}

// setRenderMu wires the mutex that serializes rendering on this surface. On
// Wayland it also serializes wl_display access with the keyboard poller,
// because the Wayland client API is not thread-safe, so the manager wires it
// before starting that poller. The mode overlay gets the manager's renderMu
// and the click indicator its own indicatorRenderMu. They must never be the
// same lock (../AGENTS.md).
func (o *sharedOverlay) setRenderMu(mu *sync.Mutex) {
	o.renderMu = mu
}

// forgetGridPointer drops grid mode's pointer stand-in without repainting.
// It follows a Clear or Hide. A record kept past one would put the pointer
// back on the first repaint of whatever comes next.
func (o *sharedOverlay) forgetGridPointer() {
	o.gridPointer = recursivegridcomponent.VirtualPointerState{}
}

// drawable reports whether there is a native surface to draw on: a wired
// overlaySurface whose handle is still open. Only the constructors wire srf,
// so a backend built any other way (as in tests without a display server)
// has none, and calling through the nil interface would panic.
func (o *sharedOverlay) drawable() bool {
	return o.srf != nil && o.srf.alive()
}

func (o *sharedOverlay) hide() {
	o.cancelAnimation()
	o.lastHints = nil
	o.srf.surfaceHide()
}

func (o *sharedOverlay) clear() {
	o.cancelAnimation()
	o.hasLast = false
	o.lastHints = nil
	o.srf.surfaceClear()
}

func (o *sharedOverlay) clearRect(rect image.Rectangle) {
	o.srf.surfaceClearRect(rect)
}

func (o *sharedOverlay) updateGridMatches(prefix string) {
	o.currentPrefix = strings.ToUpper(prefix)
	o.redrawGrid()
}

// showSubgrid replaces what is on the grid surface with the finer grid inside
// one cell. It records the cell first, because currentSubgrid is what the
// surface shows until a Frame replaces it, and every repaint reads it.
func (o *sharedOverlay) showSubgrid(cell *domainGrid.Cell) {
	o.currentSubgrid = cell

	if !o.paintSubgridSurface(cell) {
		return
	}

	// Dropped only once painted, so a skipped frame leaves both describing
	// the screen. currentSubgrid is set outside the guard because it records
	// what the mode has open.
	o.hasLast = false
	o.lastHints = nil
}

// paintSubgridSurface paints the finer grid inside one cell, alone on the
// surface, and reports whether the frame was painted. It is the single copy
// of what an open subgrid looks like (#1491). Parent cells are not drawn
// behind it, matching macOS, whose ShowSubgrid replaces the cell array.
//
// beginFrame must come before the clear. It selects the writable buffer on
// Wayland, so clearing first wipes the buffer on screen instead.
//
// It clears the surface directly, not through clear(), whose animation
// cancel deadlocks under renderMu, which every caller holds (see
// cancelAnimation). Manager.ShowSubgrid cancels before taking the lock.
func (o *sharedOverlay) paintSubgridSurface(cell *domainGrid.Cell) bool {
	o.srf.ensureBuffers()

	if !o.srf.beginFrame() {
		return false
	}

	o.srf.surfaceClear()
	o.paintSubgridContent(cell)
	o.srf.surfaceFlush()

	return true
}

// paintSubgridContent draws the finer grid inside one cell, with the pointer
// riding the same pass, into a frame the caller has begun and cleared.
func (o *sharedOverlay) paintSubgridContent(cell *domainGrid.Cell) {
	o.drawSubgrid(cell.Bounds(), o.cachedStyle)
	o.paintGridPointer()
}

// paintGridPointer puts grid mode's pointer stand-in on the surface, in the
// pass the caller has already begun. Its position is screen-local like the
// cells, so it is translated onto the active monitor here.
func (o *sharedOverlay) paintGridPointer() {
	if !o.gridPointer.Visible {
		return
	}

	placed := o.gridPointer
	placed.Position = placed.Position.Add(o.originOffset)

	o.drawVirtualPointer(placed)
}

func (o *sharedOverlay) drawGrid(grid *domainGrid.Grid, input string, style gridcomponent.Style) {
	o.cachedGrid = grid
	o.cachedStyle = style
	o.currentPrefix = strings.ToUpper(input)
	o.currentSubgrid = nil

	o.redrawGrid()
}

//nolint:mnd
func (o *sharedOverlay) drawRecursiveGridWithSubKeyPreview(
	bounds image.Rectangle,
	depth int,
	keys string,
	dims domain.GridDimensions,
	nextKeys string,
	nextDims domain.GridDimensions,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
	animEnabled bool,
	animDurationMS int,
) {
	// Translate the screen-local bounds and pointer onto the active monitor.
	// Everything downstream derives from these.
	bounds = o.offset(bounds)
	virtualPointer.Position = virtualPointer.Position.Add(o.originOffset)

	o.srf.ensureBuffers()

	shouldAnimate := animEnabled && o.hasLast && depth != o.lastDepth &&
		!o.lastBounds.Empty()

	cellRects := recursivegrid.ComputeGridCells(bounds, dims)

	if shouldAnimate {
		duration := time.Duration(animDurationMS) * time.Millisecond
		if duration <= 0 {
			duration = 50 * time.Millisecond
		}

		continuing := len(o.currentAnimRects) > 0 && !o.animSettled
		fromRects := o.buildFromRects(cellRects, bounds)
		fromPointer := motion.PointerOrigin(
			virtualPointer.Position, o.lastPointer.Position, o.animPointer,
			o.lastPointer.Visible, continuing,
		)
		keyRunes := []rune(strings.ToUpper(keys))
		nextKeyRunes := []rune(strings.ToUpper(nextKeys))

		animStop := make(chan struct{})
		animDone := make(chan struct{})
		o.animStop = animStop
		o.animDone = animDone

		o.startGridAnimation(
			fromRects, cellRects,
			keyRunes, nextKeyRunes,
			nextDims,
			style, virtualPointer, fromPointer, continuing,
			duration, animStop, animDone,
		)
	} else {
		o.clearAndDraw(
			cellRects, keys,
			nextKeys, nextDims,
			style, virtualPointer,
		)
	}

	o.hasLast = true
	o.lastBounds = bounds
	o.lastDepth = depth
	o.lastPointer = virtualPointer
	o.lastRects = make([]image.Rectangle, len(cellRects))
	copy(o.lastRects, cellRects)
}

func (o *sharedOverlay) drawBadge(
	posX, posY int,
	text string,
	colors overlayColors,
	style overlayBadgeStyle,
) {
	o.srf.ensureBuffers()

	fontSize := style.fontSize
	if fontSize <= 0 {
		fontSize = 14
	}
	// Size the badge from the scaled font so it fits the text drawTextCentered
	// renders. The manager sizes its clear rect with the same factor (Scale()).
	scaledStyle := style
	scaledStyle.fontSize = fontSize * o.srf.surfaceScale()
	rect := badgeBounds(posX, posY, text, scaledStyle)

	o.drawRect(rect, colors.background, colors.border, max(style.borderWidth, 1))
	o.drawTextCentered(text, rect, style.fontFamily, fontSize, colors.text, true)
}

// drawMonitorSelect renders one centered, labeled panel per monitor for the
// monitor picker. Panels are sized from the scaled font (see
// monitorSelectPanelLayout) so they stay legible on HiDPI.
func (o *sharedOverlay) drawMonitorSelect(
	targets []manager.MonitorSelectTarget,
	style manager.MonitorSelectStyle,
) {
	o.srf.ensureBuffers()
	o.cancelAnimation()
	o.hasLast = false

	if !o.srf.beginFrame() {
		return
	}

	o.srf.surfaceClear()

	spec := newMonitorSelectDrawSpec(style)
	for _, target := range targets {
		if target.Bounds.Empty() {
			continue
		}

		if spec.hasBackdrop {
			o.drawRect(target.Bounds, spec.backdrop, 0, 0)
		}

		// Each monitor fits its own text, and the panel is laid out from the
		// fitted style.
		fitted := style.FittedTo(target, o.srf.surfaceScale())

		panel, labelRect, subtitleRect, radius := monitorSelectPanelLayout(
			target.Bounds, target.Label, target.Subtitle, fitted, o.srf.surfaceScale(),
		)
		o.drawRoundedRect(panel, radius, spec.background, spec.border, spec.borderWidth)

		o.drawTextCentered(
			target.Label,
			labelRect,
			style.FontFamily,
			float64(fitted.FontSize),
			spec.text,
			true,
		)

		if target.Subtitle != "" {
			// The subtitle family is never empty. An unset one is settled to
			// the label's family with the rest of the Style.
			o.drawTextCentered(
				target.Subtitle, subtitleRect,
				style.SubtitleFontFamily, float64(fitted.SubtitleFontSize), spec.subtitleText,
				false,
			)
		}
	}

	o.srf.surfaceFlush()
}

// drawHints paints a hint set over the whole surface, stopping any animation
// that would paint over it. It must not run with renderMu held (see
// cancelAnimation). Callers already cancel through
// Manager.cancelBackendAnimation, so this is a backstop.
func (o *sharedOverlay) drawHints(
	hintsSlice []*hintscomponent.Hint,
	style hintscomponent.StyleMode,
	offset badge.HintOffset,
) {
	o.cancelAnimation()
	o.repaintHints(hintsSlice, style, offset)
}

// repaintHints is drawHints without the cancel, for the one caller already
// inside the render lock: hiding the search badge.
func (o *sharedOverlay) repaintHints(
	hintsSlice []*hintscomponent.Hint,
	style hintscomponent.StyleMode,
	offset badge.HintOffset,
) {
	o.srf.ensureBuffers()
	o.hasLast = false

	if !o.srf.beginFrame() {
		return
	}

	o.srf.surfaceClear()

	// Remembered only once the frame is committed, so a skipped frame cannot
	// leave the cache describing a screen that was never painted.
	o.lastHints = hintsSlice
	o.lastHintStyle = style
	o.lastHintOffset = offset

	fontSize := float64(max(style.FontSize(), 1))
	for _, hint := range hintsSlice {
		// hint.Position() is the element center in screen-local coordinates;
		// translate it onto the active monitor.
		pos := hint.Position().Add(o.originOffset)
		if style.BoundaryHighlightEnabled() {
			boundary := badge.CenteredOn(pos, hint.Size().X, hint.Size().Y)
			o.drawRoundedRect(
				boundary,
				badge.BorderRadius(
					style.BoundaryBorderRadius(), boundary, hintBoundaryAutoRadiusMax,
				),
				badge.ParseHexARGB(style.BoundaryBackgroundColor()),
				badge.ParseHexARGB(style.BoundaryBorderColor()),
				float64(max(style.BoundaryBorderWidth(), 0)),
			)
		}

		textColor := style.TextColor()
		if hint.MatchedPrefix() != "" {
			textColor = style.MatchedTextColor()
		}

		label := hint.Label()
		// Size the badge from the scaled font so it fits the text
		// drawTextCentered renders. Position stays in device pixels.
		sfont := fontSize * o.srf.surfaceScale()
		paddingX := badge.AutoPadding(sfont, style.PaddingX(), true)
		paddingY := badge.AutoPadding(sfont, style.PaddingY(), false)
		badgeWidth := badge.TextWidth(label, style.FontFamily(), sfont, true) +
			paddingX*paddingMultiplier
		badgeHeight := badge.EstimateTextHeight(sfont) + paddingY*paddingMultiplier

		radius := style.BorderRadius()
		if radius < 0 {
			radius = min(badgeHeight/halfDivisor, hintAutoRadiusMax)
		}
		// Cap the radius so an offset badge keeps a flat edge for the tail.
		radius = badge.HintRadius(radius, badgeWidth, offset)

		// pos is the element center. The badge is centered on it horizontally
		// and placed above, on or below it to match macOS. An offset badge
		// also draws a connector arrow back at the target.
		badgeRect, arrow, hasArrow := badge.PlaceHint(
			pos, badgeWidth, badgeHeight, radius, offset,
		)

		fill := badge.ParseHexARGB(style.BackgroundColor())
		border := badge.ParseHexARGB(style.BorderColor())
		borderWidth := float64(max(style.BorderWidth(), 0))

		// Badge and connector tail are drawn as one filled+stroked outline so
		// translucent colors don't double-composite at the junction.
		o.drawHintBadge(
			badgeRect, float64(radius), hintTailEdge(badgeRect, arrow, hasArrow), arrow,
			fill, border, borderWidth,
		)
		o.drawTextCentered(
			label, badgeRect,
			style.FontFamily(),
			fontSize,
			badge.ParseHexARGB(textColor),
			true,
		)
	}

	o.srf.surfaceFlush()
}

// drawHintSearchInput paints the search badge over the hints already on the
// surface, with no buffer selection and no clear. It flushes for itself
// rather than waiting for the indicator tick, since a lagging query is the
// latency this badge exists to remove.
//
// Painting without clearing relies on every badge draw following a hints draw
// that cleared the surface (`applyHintSearchFilter`). Without that, a
// backspace would leave the tail of the longer label behind.
func (o *sharedOverlay) drawHintSearchInput(
	label string,
	frame hintscomponent.SearchInputFrame,
	style hintscomponent.SearchInputStyle,
) image.Rectangle {
	o.srf.ensureBuffers()

	fontSize := float64(max(style.FontSize(), 1))

	// Size the badge from the scaled font like every other badge here. The
	// configured width is a pixel measurement and stays unscaled.
	bounds := o.offset(badge.SearchBounds(
		frame.Position(),
		frame.Width(),
		label,
		badge.TextFont{Family: style.FontFamily(), Size: fontSize * o.srf.surfaceScale()},
		style.PaddingX(),
		style.PaddingY(),
	))

	o.drawRoundedRect(
		bounds,
		badge.BorderRadius(style.BorderRadius(), bounds, searchInputAutoRadiusMax),
		badge.ParseHexARGB(style.BackgroundColor()),
		badge.ParseHexARGB(style.BorderColor()),
		float64(max(style.BorderWidth(), 0)),
	)
	o.drawTextCentered(
		label, bounds,
		style.FontFamily(),
		fontSize,
		badge.ParseHexARGB(style.TextColor()),
		false,
	)

	o.srf.surfaceFlush()

	return bounds
}

// hideHintSearchInput puts back what the badge covered. With hints on the
// surface that means repainting them (see lastHints), since the confirm path
// hides the badge with the narrowed labels still showing. Otherwise the
// rectangle is erased.
func (o *sharedOverlay) hideHintSearchInput(painted image.Rectangle) {
	if len(o.lastHints) > 0 {
		o.repaintHints(o.lastHints, o.lastHintStyle, o.lastHintOffset)

		return
	}

	o.clearRect(painted)
	o.srf.surfaceFlush()
}

// drawMouseActionIndicator animates a transient click indicator centered on
// point, on its own indicator window, so it survives the mode exit that
// follows a click.
func (o *sharedOverlay) drawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
	// Must run without the render lock held (see cancelAnimation).
	o.cancelAnimation()

	duration := time.Duration(style.DurationMS) * time.Millisecond
	if duration <= 0 {
		duration = defaultMouseActionDuration
	}

	o.srf.showIndicator()

	animStop := make(chan struct{})
	animDone := make(chan struct{})
	o.animStop = animStop
	o.animDone = animDone

	o.startMouseActionAnimation(point, style, duration, animStop, animDone)
}

func (o *sharedOverlay) startMouseActionAnimation(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
	duration time.Duration,
	stopCh chan struct{},
	doneCh chan struct{},
) {
	startTime := time.Now()

	fillBase := badge.ParseHexARGB(style.BackgroundColor)
	borderBase := badge.ParseHexARGB(style.BorderColor)
	lineWidth := float64(max(style.BorderWidth, 0))
	baseSize := float64(max(style.Size, 1)) * o.srf.surfaceScale()
	isSquare := style.Shape == "square"

	renderFrame := func(rawProgress float64) {
		if !o.srf.beginFrame() {
			return
		}

		eased := applyEasing(style.Easing, rawProgress)
		scale := max(motion.Lerp(style.StartScale, style.EndScale, eased), 0)
		opacity := motion.Lerp(style.StartOpacity, style.EndOpacity, eased)
		diameter := baseSize * scale
		rect := mouseActionIndicatorRect(point, diameter)
		fill := applyOpacity(fillBase, opacity)
		border := applyOpacity(borderBase, opacity)

		o.srf.clearFrame()

		if isSquare {
			o.drawRect(rect, fill, border, lineWidth)
		} else {
			o.drawRoundedRect(rect, diameter/halfDivisor, fill, border, lineWidth)
		}

		o.srf.surfaceFlush()
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

			renderMu := o.renderMu
			if renderMu != nil {
				renderMu.Lock()
				// Parent may have closed stopCh while we waited for the lock.
				// It holds the lock and waits for animDone, so check first.
				select {
				case <-stopCh:
					renderMu.Unlock()

					return
				default:
				}
			}

			renderFrame(rawProgress)

			if renderMu != nil {
				renderMu.Unlock()
			}

			if rawProgress >= 1.0 {
				// Finished: unmap so the faded indicator does not linger.
				if renderMu != nil {
					renderMu.Lock()
				}

				o.srf.finishIndicator()

				if renderMu != nil {
					renderMu.Unlock()
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

// cancelAnimation stops a running animation and waits for its goroutine to
// exit. The caller must not hold renderMu, which the goroutine takes every
// frame.
//
// The nil-receiver guard is for consistency, not safety. This method is
// promoted from the embedded sharedOverlay, so a nil *x11Overlay or
// *wlrootsOverlay panics on the promotion first (../AGENTS.md).
// Manager.cancelBackendAnimation keeps a nil backend out of here by capturing
// the pointer under renderMu.
func (o *sharedOverlay) cancelAnimation() {
	if o == nil {
		return
	}

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

// buildFromRects answers where each cell of the new depth starts its zoom
// (motion.TransitionOrigins), from the cells a running transition was
// interrupted on or the ones the last depth drew.
func (o *sharedOverlay) buildFromRects(
	toRects []image.Rectangle,
	bounds image.Rectangle,
) []image.Rectangle {
	return motion.TransitionOrigins(toRects, bounds, o.currentAnimRects, o.lastRects, o.lastBounds)
}

func (o *sharedOverlay) startGridAnimation(
	fromRects, toRects []image.Rectangle,
	keyRunes, nextKeyRunes []rune,
	nextDims domain.GridDimensions,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
	fromPointer image.Point,
	continuing bool,
	duration time.Duration,
	stopCh chan struct{},
	doneCh chan struct{},
) {
	o.srf.syncBeforeAnimation()

	startTime := time.Now()
	o.animSettled = false

	// Both are fitted once before the first frame. held is what the
	// transition draws at, and settled is what its final frame draws at.
	scale := o.srf.surfaceScale()
	held := style.FitTransition(scale, fromRects, toRects, nextDims)
	settled := style.FitDraw(scale, toRects, nextDims)

	// Called under renderMu. Progress is read here, after the lock, so a frame
	// that waited paints where the cells are now.
	renderFrame := func() float64 {
		rawProgress := min(float64(time.Since(startTime))/float64(duration), 1)
		progress := motion.Eased(rawProgress, continuing)

		interpCells := motion.LerpRects(fromRects, toRects, progress)
		pointer := virtualPointer
		pointer.Position = motion.LerpPoint(fromPointer, virtualPointer.Position, progress)

		if !o.srf.beginFrame() {
			return rawProgress
		}

		o.currentAnimRects = interpCells
		o.animPointer = pointer.Position
		o.animSettled = rawProgress >= 1

		sizes := held
		if o.animSettled {
			sizes = settled
		}

		o.srf.clearFrame()
		o.drawFrame(
			interpCells,
			keyRunes,
			nextKeyRunes,
			nextDims,
			style,
			sizes,
			pointer,
		)

		return rawProgress
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

			renderStart := time.Now()

			renderMu := o.renderMu
			if renderMu != nil {
				renderMu.Lock()
				// Parent may have closed stopCh while we waited for the lock
				// (see drawMouseActionIndicator).
				select {
				case <-stopCh:
					renderMu.Unlock()

					return
				default:
				}
			}

			rawProgress := renderFrame()

			if renderMu != nil {
				renderMu.Unlock()
			}

			if rawProgress >= 1 {
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

func (o *sharedOverlay) clearAndDraw(
	cellRects []image.Rectangle,
	keys string,
	nextKeys string, nextDims domain.GridDimensions,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
) {
	o.currentAnimRects = nil

	keyRunes := []rune(strings.ToUpper(keys))
	nextKeyRunes := []rune(strings.ToUpper(nextKeys))

	if !o.srf.beginFrame() {
		return
	}

	o.srf.surfaceClear()
	o.drawFrame(
		cellRects,
		keyRunes,
		nextKeyRunes,
		nextDims,
		style,
		style.FitDraw(o.srf.surfaceScale(), cellRects, nextDims),
		virtualPointer,
	)
}

func (o *sharedOverlay) drawFrame(
	cellRects []image.Rectangle,
	keyRunes, nextKeyRunes []rune,
	nextDims domain.GridDimensions,
	style recursivegridcomponent.Style,
	sizes recursivegridcomponent.FittedSizes,
	virtualPointer recursivegridcomponent.VirtualPointerState,
) {
	// sizes is one answer for the whole frame (Style.FitDraw), so every label
	// matches and nothing is fitted per cell.
	drawSubPreview := sizes.ShowPreview &&
		style.PreviewsNextDepth(len(nextKeyRunes), nextDims)

	for idx, cell := range cellRects {
		if cell.Empty() {
			continue
		}

		fill := style.HighlightColorARGB()
		if fill == 0 {
			fill = subgridCellBackground
		}

		o.drawRect(cell, fill, style.LineColorARGB(), style.LineWidthF())

		if idx < len(keyRunes) {
			label := style.LabelChar()
			if label == "" {
				label = string(keyRunes[idx])
			}

			if sizes.ShowLabel {
				if style.LabelBackground() {
					o.drawLabelBackground(label, cell, style, sizes.LabelSize)
				}

				o.drawTextCentered(
					label, cell, style.FontFamily(),
					sizes.LabelSize, style.TextColorARGB(),
					false,
				)
			}

			if drawSubPreview {
				o.drawSubKeyMiniGrid(cell, nextKeyRunes, nextDims, style, sizes.PreviewSize)
			}
		}
	}

	if virtualPointer.Visible {
		o.drawVirtualPointer(virtualPointer)
	}

	o.srf.surfaceFlush()
}

//nolint:mnd,varnamelen
func (o *sharedOverlay) drawVirtualPointer(vp recursivegridcomponent.VirtualPointerState) {
	vpChar := vp.Char
	if vpChar == "" {
		vpChar = "\u25CF"
	}

	// FontName arrives resolved from the Style. Resolving again would cost a
	// lock and a cache lookup per frame.
	fontName := vp.FontName
	fontSize := float64(vp.Size)
	// Not badge.CenteredOn, because the half is floored at 1 so a size 0 or 1
	// pointer still has a box for its glyph.
	halfSize := max(vp.Size/2, 1)
	vpBounds := image.Rect(
		vp.Position.X-halfSize,
		vp.Position.Y-halfSize,
		vp.Position.X+halfSize,
		vp.Position.Y+halfSize,
	)
	o.drawTextCentered(vpChar, vpBounds, fontName, fontSize,
		badge.ParseHexARGB(vp.FillColor), false)
}

// redrawGrid paints the grid surface as it stands, either the open subgrid or
// the cells, so a narrowing keystroke, a pointer move and fresh cells all
// arrive at the same screen.
//
// The subgrid is the whole surface while open (#1491). The guard lives here
// because updateGridMatches reaches this on a keystroke, and putting the
// parent cells back under the subgrid would change the screen unasked.
func (o *sharedOverlay) redrawGrid() {
	if o.currentSubgrid != nil {
		o.paintSubgridSurface(o.currentSubgrid)

		return
	}

	if o.cachedGrid == nil {
		return
	}

	o.srf.ensureBuffers()

	if !o.srf.beginFrame() {
		return
	}

	o.srf.surfaceClear()
	style := o.cachedStyle
	prefix := o.currentPrefix

	// One size for the whole grid, fitted once per draw rather than per cell.
	// The cells are device pixels, and drawTextCentered applies the surface scale.
	labelSize := style.LabelFontSizeFor(o.srf.surfaceScale(), o.cachedGrid)

	for _, cell := range o.cachedGrid.AllCells() {
		label := strings.ToUpper(cell.Coordinate())

		matched := strings.HasPrefix(label, prefix)
		if o.hideUnmatched && prefix != "" && !matched {
			continue
		}

		fill := style.BackgroundColorARGB()
		text := style.TextColorARGB()

		border := style.LineColorARGB()
		if matched && prefix != "" {
			fill = style.MatchedBackgroundColorARGB()
			text = style.MatchedTextColorARGB()
			border = style.MatchedBorderColorARGB()
		}

		cellBounds := o.offset(cell.Bounds())
		o.drawRect(cellBounds, fill, border, style.LineWidth())
		o.drawTextCentered(label, cellBounds,
			style.FontFamily(), labelSize, text, false)
	}

	o.paintGridPointer()

	o.srf.surfaceFlush()
}

func (o *sharedOverlay) drawSubgrid(bounds image.Rectangle, style gridcomponent.Style) {
	// bounds is screen-local, so place the subgrid on the active monitor.
	bounds = o.offset(bounds)

	// The keys the mode layer selects on (internal/domain/grid/subgrid_keys.go).
	keyRunes := domainGrid.SubgridKeys(o.sublayerKeys, domainGrid.MaxKeyIndex)

	// The rectangles the mode layer moves the cursor into
	// (internal/domain/grid/subgrid_cells.go).
	cells := domainGrid.SubgridCells(bounds, domain.SubgridDimensions())
	labelSize := style.SubgridLabelFontSizeIn(o.srf.surfaceScale(), subgridFontScale, cells)

	// One cell per key. Fewer keys than cells leaves the last cells
	// unlabelled. MaxKeyIndex caps the keys at the cell count.
	for index, key := range keyRunes {
		cell := cells[index]

		o.drawRect(cell, style.BackgroundColorARGB(),
			style.LineColorARGB(), style.LineWidth())
		o.drawTextCentered(
			string(key), cell,
			style.FontFamily(),
			labelSize,
			style.TextColorARGB(),
			false,
		)
	}
}

// setOriginOffset stores the active screen origin used to translate
// screen-local grid, recursive-grid and hint coordinates onto the right
// monitor. It touches no surface, so a closed handle does not stop it.
func (o *sharedOverlay) setOriginOffset(origin image.Point) {
	o.originOffset = origin
}

// offset translates a screen-local rectangle into global desktop coordinates.
func (o *sharedOverlay) offset(rect image.Rectangle) image.Rectangle {
	return rect.Add(o.originOffset)
}

// Stroke widths scale with the HiDPI factor here so every draw path (grid,
// hints, badges, indicator) gets consistent line weight without per-call-site
// scaling. Element geometry that must fit scaled text is sized by the callers.
func (o *sharedOverlay) drawRect(
	bounds image.Rectangle,
	fill uint32, border uint32, lineWidth float64,
) {
	o.srf.rectPrim(bounds, fill, border, lineWidth*o.srf.surfaceScale())
}

func (o *sharedOverlay) drawRoundedRect(
	bounds image.Rectangle,
	radius float64,
	fill uint32, border uint32, lineWidth float64,
) {
	o.srf.roundedRectPrim(bounds, radius, fill, border, lineWidth*o.srf.surfaceScale())
}

func (o *sharedOverlay) drawHintBadge(
	badgeRect image.Rectangle, radius float64, edge int, arrow badge.HintArrow,
	fill uint32, border uint32, lineWidth float64,
) {
	o.srf.hintBadgePrim(
		badgeRect,
		radius,
		edge,
		arrow,
		fill,
		border,
		lineWidth*o.srf.surfaceScale(),
	)
}

// drawTextCentered applies the HiDPI scale to the font size, so callers pass
// the logical size. Callers that size geometry around the text must use the
// same scaled font (fontSize * surfaceScale()).
func (o *sharedOverlay) drawTextCentered(
	text string, bounds image.Rectangle,
	fontFamily string, fontSize float64, color uint32,
	bold bool,
) {
	o.srf.textPrim(
		text, fontFamily,
		float64(bounds.Min.X+bounds.Dx()/2),
		float64(bounds.Min.Y+bounds.Dy()/2),
		fontSize*o.srf.surfaceScale(), color, bold,
	)
}

func (o *sharedOverlay) drawLabelBackground(
	label string, cell image.Rectangle,
	style recursivegridcomponent.Style,
	labelSize float64,
) {
	// Match the scaled font that drawTextCentered renders for the label.
	fontSize := labelSize * o.srf.surfaceScale()
	paddingX := badge.AutoPadding(fontSize,
		style.LabelBackgroundPaddingX(), true)
	paddingY := badge.AutoPadding(fontSize,
		style.LabelBackgroundPaddingY(), false)
	width := badge.TextWidth(label, style.FontFamily(), fontSize, false) +
		paddingX*paddingMultiplier
	height := badge.EstimateTextHeight(fontSize) +
		paddingY*paddingMultiplier
	rect := badge.CenteredIn(cell, width, height)
	// A zero auto cap means a full pill. Left at -1, the plate rounds to half
	// its height, as the shared resolver documents and Windows draws. macOS
	// caps it at 6 px (drawGridLabel: in overlay_darwin.m), a divergence not
	// settled here.
	o.drawRoundedRect(rect,
		badge.BorderRadius(style.LabelBackgroundBorderRadius(), rect, 0),
		style.LabelBackgroundColorARGB(),
		style.LineColorARGB(), max(style.LabelBackgroundBorderWidthF(), 0))
}

// drawSubKeyMiniGrid paints the next depth's keys inside one cell, each on
// the sub-cell it would select. Style.SubKeyPreviewCells decides placement,
// and the GDI backend draws the same list (ADR 0007). Only the painting, and
// the HiDPI scale, happen here.
func (o *sharedOverlay) drawSubKeyMiniGrid(
	cell image.Rectangle,
	nextKeyRunes []rune,
	nextDims domain.GridDimensions,
	style recursivegridcomponent.Style,
	previewSize float64,
) {
	for _, subCell := range style.SubKeyPreviewCells(cell, nextKeyRunes, nextDims) {
		o.drawTextCentered(
			subCell.Label, subCell.Bounds,
			style.FontFamily(), previewSize,
			style.SubKeyPreviewTextColorARGB(),
			false,
		)
	}
}
