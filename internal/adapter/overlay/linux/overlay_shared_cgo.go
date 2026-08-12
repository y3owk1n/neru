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
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
	"github.com/y3owk1n/neru/internal/ports"
)

// overlaySurface is the primitive contract a Linux overlay backend provides
// to the shared drawing implementation. Everything above these primitives —
// layout, animation, offsets, label logic — is identical between X11 and
// Wayland and lives once on sharedOverlay; everything genuinely different
// (buffer management, HiDPI scale, window lifecycle) lives behind this
// interface. The shared code never touches cgo: primitives take Go types and
// do their own C marshaling (including CString lifetimes).
type overlaySurface interface {
	// alive reports whether the backend still holds an open native handle.
	// It is the one question the shared delegates below cannot answer for
	// themselves: the handle is a *C.NeruX11Overlay on one side and a
	// *C.NeruWaylandOverlay on the other, and a draw that reaches C through a
	// closed one is a nil dereference inside cgo. Every exported delegate that
	// used to guard `o.raw != nil` per backend asks this instead.
	alive() bool

	// surfaceScale is the HiDPI factor applied to fonts, stroke widths and
	// text-fitted geometry. X11 probes Xft.dpi; Wayland reports 1.
	surfaceScale() float64

	// ensureBuffers prepares the backend's drawing target without selecting
	// it (Wayland's setup_buffers; a no-op on X11). Some paths call it
	// without a subsequent frame selection — preserve that.
	ensureBuffers()

	// beginFrame makes a drawable target current, reporting false when none
	// is available and the frame must be skipped (Wayland's buffer
	// acquisition with one sync-retry; X11 always succeeds).
	beginFrame() bool

	// surfaceClear clears the current target without any buffer selection.
	surfaceClear()

	// clearFrame clears inside an animation frame (X11's double-buffered
	// clear; plain clear on Wayland, which selected a buffer in beginFrame).
	clearFrame()

	surfaceClearRect(rect image.Rectangle)
	surfaceFlush()
	surfaceHide()

	// showIndicator maps the dedicated click-indicator surface before its
	// animation starts. Wayland must do this under the render lock and keep
	// the surface keyboard-passive; X11 just maps the window.
	showIndicator()

	// finishIndicator unmaps the indicator when its animation completes; the
	// caller holds the render lock. X11 also clears so no stale pixmap shows
	// on the next map; Wayland must not clear an unselected buffer.
	finishIndicator()

	// syncBeforeAnimation flushes pending display work before a grid
	// animation begins (Wayland sync; no-op on X11).
	syncBeforeAnimation()

	rectPrim(bounds image.Rectangle, fill, border uint32, lineWidth float64)
	roundedRectPrim(bounds image.Rectangle, radius float64, fill, border uint32, lineWidth float64)
	hintBadgePrim(
		badgeRect image.Rectangle, radius float64, edge int, arrow badge.HintArrow,
		fill, border uint32, lineWidth float64,
	)
	textPrim(text, fontFamily string, centerX, centerY, fontSize float64, color uint32)
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

	// gridPointer is the pointer stand-in grid mode has asked for, kept
	// because it is painted in the same pass as the cells rather than into a
	// window of its own — the same thing recursive grid does with the pointer
	// its frame carries, except that grid mode's arrives between draws instead
	// of on one. Every repaint of the grid surface reads it, so narrowing a
	// prefix or opening a subgrid keeps the pointer where the mode put it.
	//
	// The zero value is the pointer being off screen, which is what the mode
	// asks for on activation and again on exit.
	gridPointer recursivegridcomponent.VirtualPointerState

	// originOffset is the active screen's top-left origin in global device
	// pixels. Grid, recursive-grid and hint content arrives in screen-local
	// coordinates (origin 0,0); adding this offset places it on the correct
	// monitor of the desktop-spanning overlay window. Absolute-coordinate draws
	// (the indicator badges, monitor_select, the click indicator) do not apply
	// it. The hints search badge does, badge though it is: its frame is placed
	// against the screen the caller named, so it arrives screen-local like the
	// labels it sits beside.
	originOffset image.Point

	// lastHints is the hint set this surface was last painted with, kept so the
	// search badge can be taken off it without erasing the labels underneath.
	// Both surfaces are one Cairo target, so the badge is painted over the
	// hints and hiding it means repainting them rather than clearing a
	// rectangle out of them.
	//
	// It is dropped by clear() and hide(), which is where every mode transition
	// goes (`Adapter.ClearFrame`), so nothing is repainted that is no longer on
	// screen. The draws that clear the surface for themselves — grid, recursive
	// grid, monitor-select — do not drop it, and do not need to: what decides
	// whether any of this is read is the manager's searchBadgeRect, and only a
	// badge draw sets that.
	lastHints      []*hintscomponent.Hint
	lastHintStyle  hintscomponent.StyleMode
	lastHintOffset badge.HintOffset

	// renderMu serializes rendering with the manager (and, on Wayland, with
	// the keyboard poller's wl_display access). The manager owns the mutex
	// object and holds it across synchronous draw calls; only the animation
	// goroutines lock it here.
	renderMu *sync.Mutex

	cancelMu         sync.Mutex
	animStop         chan struct{}
	animDone         chan struct{}
	hasLast          bool
	lastBounds       image.Rectangle
	lastDepth        int
	lastRects        []image.Rectangle
	currentAnimRects []image.Rectangle
}

// The exported methods below are what the manager calls on a backend. They
// live here rather than once per backend because seventeen of the eighteen make
// no C call at all — the whole of each was a nil-guard prologue and one
// delegate into the code beneath it — and the eighteenth, Flush, goes through
// the surfaceFlush primitive this seam already declares. Show, Resize and
// Destroy stay per-backend: Wayland re-runs buffer setup before showing, layer
// shells auto-resize so Resize is empty there, and Wayland's Destroy waits on
// the keyboard poller.
//
// What they guard for themselves is a surface worth drawing on (`drawable`
// below). What they cannot guard is a nil backend pointer, which the manager
// nil-checks before every dispatch — see ../AGENTS.md.

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

func (o *sharedOverlay) ShowSubgrid(cell *domainGrid.Cell, _ gridcomponent.Style) {
	if !o.drawable() || cell == nil {
		return
	}

	o.showSubgrid(cell)
}

// SetHideUnmatched sets a Go field and reaches no surface, so it deliberately
// does not ask drawable: a backend whose handle has been closed still answers
// the next draw with the flag it was last given.
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
// state it is handed is not visible.
//
// The record is kept whether or not there is a surface to paint on, because
// it is state the next repaint reads rather than a draw of its own — the same
// reason SetHideUnmatched keeps its flag. What asks drawable is the repaint.
//
// A state equal to the one already held paints nothing: grid mode refreshes
// the pointer on every keystroke and it moves only when a cell is chosen, so
// the common key would otherwise cost a second full repaint of the surface the
// narrowing has already repainted.
func (o *sharedOverlay) SetGridPointer(pointer recursivegridcomponent.VirtualPointerState) {
	if pointer == o.gridPointer {
		return
	}

	o.gridPointer = pointer

	if !o.drawable() {
		return
	}

	o.repaintGridSurface()
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

// Flush is the one of these that reaches C, through the primitive both
// backends already implement as the same one-line flush of their own handle.
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

// setRenderMu wires the mutex that serializes rendering on *this* surface. On
// Wayland it is also what serializes wl_display access between rendering and
// the keyboard poller — the Wayland client API is not thread-safe — which is
// why the manager wires it before starting that poller, and why this used to
// carry a second name (setDisplayMu) on that side. One operation, one name.
//
// Which mutex arrives is the caller's decision and not a detail: the mode
// overlay gets the manager's renderMu, the click indicator gets its own
// indicatorRenderMu, and the two must never be the same lock (../AGENTS.md).
func (o *sharedOverlay) setRenderMu(mu *sync.Mutex) {
	o.renderMu = mu
}

// forgetGridPointer drops grid mode's pointer stand-in without repainting. It
// follows a Clear or a Hide, which take the whole surface away: there is
// nothing left to paint it onto, and a record kept past one would put the
// pointer back on the first repaint of whatever comes next.
func (o *sharedOverlay) forgetGridPointer() {
	o.gridPointer = recursivegridcomponent.VirtualPointerState{}
}

// drawable reports whether there is a native surface to draw on: a backend
// whose overlaySurface was wired and whose handle is still open. It is the
// guard the exported delegates above share, standing in for the per-backend
// `o.raw != nil` prologue each of them used to carry.
//
// The srf half is not ceremony. Only the constructors wire it, so a backend
// value built any other way has no surface at all — which is what the tests
// that stand a Manager up without a display server attach — and reaching
// through a nil interface would panic where the old prologue returned.
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

func (o *sharedOverlay) showSubgrid(cell *domainGrid.Cell) {
	o.currentSubgrid = cell
	o.srf.ensureBuffers()
	o.clear()

	if !o.srf.beginFrame() {
		return
	}

	o.paintSubgridContent(cell)
	o.srf.surfaceFlush()
}

// repaintGridSurface paints the grid surface as it currently stands, which is
// either the subgrid one cell was opened into or the cells themselves. It is
// what a pointer move repaints through, so the pointer arrives over whichever
// of the two is on screen.
//
// Repainting the whole surface for one glyph is the deliberate half of the
// cost. The alternative is the sticky badge's incremental shape — clear the
// rectangle the pointer covered, paint it at its new place — and it is wrong
// here for the reason hiding the hint search badge repaints rather than
// erases: this surface is one Cairo target, so clearing a rectangle out of it
// takes a bite out of the cell borders underneath. The pointer moves only when
// a cell is chosen, and the equality guard in SetGridPointer keeps the
// narrowing keystroke at the one repaint it already had.
//
// Its sequence is redrawGrid's, and the order of the first two calls is the
// load-bearing part: beginFrame is what selects the writable buffer on Wayland,
// so clearing before it wipes the buffer that is on screen and leaves the one
// about to be shown holding whatever it last held.
//
// It also clears through the surface primitive rather than through clear():
// every caller reaches it with renderMu held, and clear() begins by canceling a
// running animation, which waits on a goroutine that takes renderMu on every
// frame. showSubgrid above calls clear() and is reached under renderMu too —
// that is older than this and not made worse here, because it only bites while
// an animation is painting the surface a subgrid is being opened on, which the
// grid frame coming up canceled. A second caller of clear() from this depth
// would be a new way to reach it, so this one does not add one.
func (o *sharedOverlay) repaintGridSurface() {
	if o.currentSubgrid == nil {
		o.redrawGrid()

		return
	}

	o.srf.ensureBuffers()

	if !o.srf.beginFrame() {
		return
	}

	o.srf.surfaceClear()
	o.paintSubgridContent(o.currentSubgrid)
	o.srf.surfaceFlush()
}

// paintSubgridContent draws the finer grid inside one cell, with the pointer
// riding the same pass, into a frame the caller has begun and cleared.
func (o *sharedOverlay) paintSubgridContent(cell *domainGrid.Cell) {
	o.drawSubgrid(cell.Bounds(), o.cachedStyle)
	o.paintGridPointer()
}

// paintGridPointer puts grid mode's pointer stand-in on the surface, in the
// pass the caller has already begun. The position it holds is screen-local
// like the cells it sits among, so it is translated onto the active monitor
// here — the same translation the recursive-grid draw makes on the pointer its
// frame carries.
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
	// Translate the screen-local bounds and virtual-pointer position onto the
	// active monitor. Everything downstream (cell rects, animation from/to
	// rects, the pointer) derives from these, so the whole frame lands on the
	// right monitor.
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
			nextDims,
			style, virtualPointer,
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
	o.drawTextCentered(text, rect, style.fontFamily, fontSize, colors.text)
}

// drawMonitorSelect renders one centered, labeled panel per monitor for the
// interactive monitor picker. Panels reuse the existing rounded-rect + text
// primitives (no dedicated C), and are sized from the scaled font (see
// monitorSelectPanelLayout) so they stay legible on HiDPI. The label is drawn
// with the matched/selected color when it has a matched prefix or is selected.
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

		panel, labelRect, subtitleRect, radius := monitorSelectPanelLayout(
			target.Bounds, target.Label, target.Subtitle, style, o.srf.surfaceScale(),
		)
		o.drawRoundedRect(panel, radius, spec.background, spec.border, spec.borderWidth)

		o.drawTextCentered(target.Label, labelRect, style.FontFamily, spec.labelFont, spec.text)

		if target.Subtitle != "" {
			// The subtitle family is never empty: an unset one is settled to
			// the label's family with the rest of the Style.
			o.drawTextCentered(
				target.Subtitle, subtitleRect,
				style.SubtitleFontFamily, spec.subtitleFont, spec.subtitleText,
			)
		}
	}

	o.srf.surfaceFlush()
}

// drawHints paints a hint set over the whole surface, stopping any animation
// that would otherwise paint over it.
//
// The cancel is why this is separate from repaintHints below: it waits for the
// animation goroutine to exit and that goroutine takes renderMu on every frame,
// so it must not run with renderMu held (see cancelAnimation). Every caller of
// this one cancels through Manager.cancelBackendAnimation before taking the
// lock, and the second cancel here is the belt to that pair of braces.
func (o *sharedOverlay) drawHints(
	hintsSlice []*hintscomponent.Hint,
	style hintscomponent.StyleMode,
	offset badge.HintOffset,
) {
	o.cancelAnimation()
	o.repaintHints(hintsSlice, style, offset)
}

// repaintHints is drawHints without that cancel, for the one caller that is
// already inside the render lock when it decides to repaint: hiding the search
// badge. Reaching drawHints from there would take renderMu into a call
// documented as needing it released.
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

	// Remembered after the surface is committed to this set, so a frame that
	// was skipped above cannot leave the cache describing a screen that was
	// never painted.
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
		// Size the badge from the scaled font so it fits the text drawTextCentered
		// renders (which applies the same surface scale). Position stays in device
		// pixels; the badge grows around the target.
		sfont := fontSize * o.srf.surfaceScale()
		paddingX := badge.AutoPadding(sfont, style.PaddingX(), true)
		paddingY := badge.AutoPadding(sfont, style.PaddingY(), false)
		badgeWidth := badge.EstimateTextWidth(label, sfont) + paddingX*paddingMultiplier
		badgeHeight := badge.EstimateTextHeight(sfont) + paddingY*paddingMultiplier

		radius := style.BorderRadius()
		if radius < 0 {
			radius = min(badgeHeight/halfDivisor, hintAutoRadiusMax)
		}
		// Cap the radius so an offset badge keeps a flat edge for the tail.
		radius = badge.HintRadius(radius, badgeWidth, offset)

		// pos is the element center (set in modes/hints.go). The badge is
		// centered horizontally on it and placed above / on / below the center
		// to match macOS; an offset badge also draws a connector arrow pointing
		// back at the target.
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
		)
	}

	o.srf.surfaceFlush()
}

// drawHintSearchInput paints the search badge over the hints already on the
// surface, the way an indicator badge is painted: no buffer selection and no
// clear, because the labels beneath it are what the user is searching and the
// badge is one more thing on the same Cairo target.
//
// It flushes for itself rather than waiting for the indicator tick's Flush. A
// query the user typed that appears a poll later is the latency this badge
// exists to remove.
//
// Painting over the surface rather than into a cleared rectangle rests on the
// order the caller keeps: every badge draw follows a hints draw, which clears
// the whole surface, because the mode handler filters the hints and then draws
// the badge (`applyHintSearchFilter`) and the hint manager delivers that update
// inside the call. Without it a backspace would leave the tail of the longer
// label behind.
func (o *sharedOverlay) drawHintSearchInput(
	label string,
	frame hintscomponent.SearchInputFrame,
	style hintscomponent.SearchInputStyle,
) image.Rectangle {
	o.srf.ensureBuffers()

	fontSize := float64(max(style.FontSize(), 1))

	// Size the badge from the scaled font so it fits the text drawTextCentered
	// renders, the way every other badge here is sized. The configured width is
	// a pixel measurement like an indicator's offsets, and stays unscaled.
	bounds := o.offset(badge.SearchBounds(
		frame.Position(),
		frame.Width(),
		label,
		fontSize*o.srf.surfaceScale(),
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
	)

	o.srf.surfaceFlush()

	return bounds
}

// hideHintSearchInput puts back what the badge covered.
//
// Where there are hints on the surface that means repainting them: the badge
// sits on the same target as the labels, so erasing its rectangle alone would
// take a bite out of the hints the user is about to type — and the confirm path
// hides the badge with the narrowed labels still on screen. Where there are no
// hints there is nothing to put back and the rectangle is simply erased.
func (o *sharedOverlay) hideHintSearchInput(painted image.Rectangle) {
	if len(o.lastHints) > 0 {
		o.repaintHints(o.lastHints, o.lastHintStyle, o.lastHintOffset)

		return
	}

	o.clearRect(painted)
	o.srf.surfaceFlush()
}

// drawMouseActionIndicator animates a transient click indicator centered on
// point. It runs on this overlay's dedicated indicator window, independent of
// the mode overlay's show/hide lifecycle, so it survives the mode exit that
// immediately follows a click.
func (o *sharedOverlay) drawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
	// Cancel any in-flight indicator animation before starting a new one.
	// (Must run without the render lock held: the animation goroutine may be
	// blocked acquiring it, and cancelAnimation waits for the goroutine.)
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
		scale := max(lerp(style.StartScale, style.EndScale, eased), 0)
		opacity := lerp(style.StartOpacity, style.EndOpacity, eased)
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
				// Parent may have closed stopCh while we were waiting for the
				// lock. Check here to avoid deadlock: parent holds the lock and
				// waits for animDone; we hold nothing yet the parent needs.
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
				// Finished: unmap the dedicated window so the fully faded
				// indicator does not linger on screen.
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
// exit. The caller must not hold renderMu: the goroutine takes it on every
// frame and would never reach the stop signal.
//
// The nil-receiver guard matches every other method a backend exposes to the
// manager, and buys consistency rather than safety: this one is promoted from
// the embedded sharedOverlay, so calling it through a nil *x11Overlay /
// *wlrootsOverlay panics on the promotion before the guard runs (see
// ../AGENTS.md). What keeps a nil backend out of here is Manager.cancelBackendAnimation,
// which captures the pointer under renderMu.
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

//nolint:mnd,varnamelen
func (o *sharedOverlay) buildFromRects(
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

func (o *sharedOverlay) startGridAnimation(
	fromRects, toRects []image.Rectangle,
	keyRunes, nextKeyRunes []rune,
	nextDims domain.GridDimensions,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
	duration time.Duration,
	stopCh chan struct{},
	doneCh chan struct{},
) {
	o.srf.syncBeforeAnimation()

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

		if !o.srf.beginFrame() {
			return
		}

		o.currentAnimRects = interpCells

		o.srf.clearFrame()
		o.drawFrame(
			interpCells,
			keyRunes,
			nextKeyRunes,
			nextDims,
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

			renderMu := o.renderMu
			if renderMu != nil {
				renderMu.Lock()
				// Parent may have closed stopCh while we were waiting
				// for the lock. Check here to avoid deadlock:
				//   parent holds the render lock, waits for animDone
				//   we   hold the render lock, parent waits for it
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
		virtualPointer,
	)
}

func (o *sharedOverlay) drawFrame(
	cellRects []image.Rectangle,
	keyRunes, nextKeyRunes []rune,
	nextDims domain.GridDimensions,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
) {
	drawSubPreview := style.PreviewsNextDepth(len(nextKeyRunes), nextDims)

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

			if style.ShowLabelIn(cell) {
				if style.LabelBackground() {
					o.drawLabelBackground(label, cell, style)
				}

				o.drawTextCentered(
					label, cell, style.FontFamily(),
					style.LabelFontSize(), style.TextColorARGB(),
				)
			}

			if drawSubPreview &&
				style.ShowSubKeyPreviewIn(cell, nextDims) {
				o.drawSubKeyMiniGrid(cell, nextKeyRunes, nextDims, style)
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

	// FontName arrives resolved: it comes from the Style, which settles every
	// family it hands out. Resolving again here would be a lock and a cache
	// lookup per drawn frame for the same answer.
	fontName := vp.FontName
	fontSize := float64(vp.Size)
	// Not badge.CenteredOn: the half is floored at 1 so a pointer configured to
	// size 0 or 1 still has a box to draw its glyph in.
	halfSize := max(vp.Size/2, 1)
	vpBounds := image.Rect(
		vp.Position.X-halfSize,
		vp.Position.Y-halfSize,
		vp.Position.X+halfSize,
		vp.Position.Y+halfSize,
	)
	o.drawTextCentered(vpChar, vpBounds, fontName, fontSize,
		badge.ParseHexARGB(vp.FillColor))
}

func (o *sharedOverlay) redrawGrid() {
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
			style.FontFamily(), style.LabelFontSize(), text)
	}

	if o.currentSubgrid != nil {
		o.drawSubgrid(o.currentSubgrid.Bounds(), style)
	}

	o.paintGridPointer()

	o.srf.surfaceFlush()
}

func (o *sharedOverlay) drawSubgrid(bounds image.Rectangle, style gridcomponent.Style) {
	// bounds is screen-local; place the subgrid on the active monitor.
	bounds = o.offset(bounds)

	// The keys the subgrid is drawn with, which are the keys the mode layer
	// selects on (internal/domain/grid/subgrid_keys.go).
	keyRunes := domainGrid.SubgridKeys(o.sublayerKeys, domainGrid.MaxKeyIndex)

	// The rectangles they are drawn on, which are the rectangles the mode layer
	// moves the cursor into (internal/domain/grid/subgrid_cells.go).
	cells := domainGrid.SubgridCells(bounds, domain.SubgridDimensions())

	// One cell per key, and fewer keys than cells is a configuration that
	// leaves the last cells unlabelled: the key set is capped at the same count
	// the division produces, which is what MaxKeyIndex is.
	for index, key := range keyRunes {
		cell := cells[index]

		o.drawRect(cell, style.BackgroundColorARGB(),
			style.LineColorARGB(), style.LineWidth())
		o.drawTextCentered(
			string(key), cell,
			style.FontFamily(),
			style.LabelFontSize()*subgridFontScale,
			style.TextColorARGB(),
		)
	}
}

// setOriginOffset stores the active screen origin used to translate
// screen-local grid/recursive-grid/hint coordinates onto the correct monitor.
//
// It is the last of the delegates above and the one that was already
// here: the per-backend wrappers only added the receiver guard promotion makes
// unreachable. Like SetHideUnmatched it touches no surface, so a closed handle
// does not stop it recording where the next draw belongs.
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

// drawTextCentered applies the HiDPI scale to the font size centrally, so every
// label/badge caller passes its base (logical) font size and text renders at the
// device-appropriate size. Callers that size geometry around the text must use
// the same scaled font (fontSize * surfaceScale()).
func (o *sharedOverlay) drawTextCentered(
	text string, bounds image.Rectangle,
	fontFamily string, fontSize float64, color uint32,
) {
	o.srf.textPrim(
		text, fontFamily,
		float64(bounds.Min.X+bounds.Dx()/2),
		float64(bounds.Min.Y+bounds.Dy()/2),
		fontSize*o.srf.surfaceScale(), color,
	)
}

func (o *sharedOverlay) drawLabelBackground(
	label string, cell image.Rectangle,
	style recursivegridcomponent.Style,
) {
	// Match the scaled font that drawTextCentered renders for the label.
	fontSize := style.LabelFontSize() * o.srf.surfaceScale()
	paddingX := badge.AutoPadding(fontSize,
		style.LabelBackgroundPaddingX(), true)
	paddingY := badge.AutoPadding(fontSize,
		style.LabelBackgroundPaddingY(), false)
	width := badge.EstimateTextWidth(label, fontSize) +
		paddingX*paddingMultiplier
	height := badge.EstimateTextHeight(fontSize) +
		paddingY*paddingMultiplier
	rect := badge.CenteredIn(cell, width, height)
	// A zero auto cap means a full pill: left at -1, the plate rounds to its
	// own half height. That is what the shared resolver documents for a label
	// background and what Windows draws. macOS caps the same auto radius at
	// 6 px (drawGridLabel: in overlay_darwin.m) — a divergence that predates
	// this call site and is not settled here.
	o.drawRoundedRect(rect,
		badge.BorderRadius(style.LabelBackgroundBorderRadius(), rect, 0),
		style.LabelBackgroundColorARGB(),
		style.LineColorARGB(), max(style.LabelBackgroundBorderWidthF(), 0))
}

// drawSubKeyMiniGrid paints the next depth's keys inside one cell, each on the
// sub-cell it would select.
//
// Where they go is not this backend's arithmetic: Style.SubKeyPreviewCells
// divides the cell and decides which sub-cell is left blank, and the GDI backend
// draws the same list (ADR 0007). What is left here is the painting — the shared
// text primitive, which is where the HiDPI scale is applied.
func (o *sharedOverlay) drawSubKeyMiniGrid(
	cell image.Rectangle,
	nextKeyRunes []rune,
	nextDims domain.GridDimensions,
	style recursivegridcomponent.Style,
) {
	for _, subCell := range style.SubKeyPreviewCells(cell, nextKeyRunes, nextDims) {
		o.drawTextCentered(
			subCell.Label, subCell.Bounds,
			style.FontFamily(), style.SubKeyPreviewFontSizeF(),
			style.SubKeyPreviewTextColorARGB(),
		)
	}
}
