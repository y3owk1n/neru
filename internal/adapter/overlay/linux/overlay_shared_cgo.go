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

	// originOffset is the active screen's top-left origin in global device
	// pixels. Grid, recursive-grid and hint content arrives in screen-local
	// coordinates (origin 0,0); adding this offset places it on the correct
	// monitor of the desktop-spanning overlay window. Absolute-coordinate draws
	// (badges, monitor_select, the click indicator) do not apply it.
	originOffset image.Point

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
// live here rather than once per backend because thirteen of the fourteen make
// no C call at all — the whole of each was a nil-guard prologue and one
// delegate into the code beneath it — and the fourteenth, Flush, goes through
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
	o.srf.surfaceHide()
}

func (o *sharedOverlay) clear() {
	o.cancelAnimation()
	o.hasLast = false
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

	o.drawSubgrid(cell.Bounds(), o.cachedStyle)
	o.srf.surfaceFlush()
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

func (o *sharedOverlay) drawHints(
	hintsSlice []*hintscomponent.Hint,
	style hintscomponent.StyleMode,
	offset badge.HintOffset,
) {
	o.srf.ensureBuffers()
	o.cancelAnimation()
	o.hasLast = false

	if !o.srf.beginFrame() {
		return
	}

	o.srf.surfaceClear()

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
// It is the fourteenth of the delegates above and the one that was already
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
