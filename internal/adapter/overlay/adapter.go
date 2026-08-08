package overlay

import (
	"context"
	"image"
	"sync/atomic"

	"go.uber.org/zap"

	overlayHints "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	overlayRecursiveGrid "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

// Adapter implements ports.OverlayPort by wrapping the existing overlay.Manager.
type Adapter struct {
	manager ManagerInterface
	styles  StyleOwner
	logger  *zap.Logger

	// subgridDrawn is whether a subgrid is on the grid surface. It is the one
	// piece of screen state this adapter keeps, and it keeps it because it is
	// the only thing that knows: it drew the subgrid, and the next grid draw
	// has to erase it rather than trust a backend to diff it away.
	subgridDrawn atomic.Bool

	// monitorSelectDrawn is whether the monitor picker's panels are on screen,
	// kept for the same reason: they are not drawn on the shared surface, so
	// clearing that surface does not take them down and only this knows they
	// need it.
	monitorSelectDrawn atomic.Bool
}

// NewAdapter creates a new overlay adapter. styles is the resolved Style the
// adapter draws with; it is never rebuilt here, so a draw costs no theme
// lookup.
func NewAdapter(
	manager ManagerInterface,
	styles StyleOwner,
	logger *zap.Logger,
) *Adapter {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Adapter{
		manager: manager,
		styles:  styles,
		logger:  logger.Named("overlay"),
	}
}

// ShowFrame puts a Frame on screen. This is the only place the transition
// sequence exists: size the overlay to the active screen, show it, switch it
// to the frame's mode, then draw. It used to be open-coded at every call site,
// where getting the order wrong showed an empty overlay or left the previous
// mode's on screen.
func (a *Adapter) ShowFrame(ctx context.Context, frame ports.Frame) error {
	err := contextAlive(ctx)
	if err != nil {
		return err
	}

	mode, modeErr := overlayMode(frame.Mode())
	if modeErr != nil {
		return modeErr
	}

	a.logger.Debug("Showing overlay frame", zap.String("mode", string(mode)))

	a.clearSurfacesTheFrameDoesNotOwn(frame)

	if drawsOnSharedWindow(frame) {
		a.manager.ResizeToActiveScreen()
		a.manager.Show()
	}

	a.manager.SwitchTo(mode)

	return a.draw(frame, transitionDraw)
}

// drawsOnSharedWindow reports whether realizing a frame needs the shared
// overlay window sized and brought up.
//
// Monitor-select is the one that does not: its panels are surfaces of their
// own, one window per display on macOS, and on Linux its own draw brings the
// spanning window up. Bringing it up here would put a transparent
// always-on-top window behind the panels that nothing draws into.
//
// Scroll draws no content of its own but still needs the window, because on
// Linux the mode and sticky-modifier indicators are badges painted on that
// surface: the shared window's visibility is theirs. Deciding otherwise would
// encode macOS's one-window-per-indicator model in shared code and leave a
// Linux user in scroll mode with no indicator after a monitor move.
//
// Every mode is named rather than defaulted, so a mode added without an answer
// here fails the `exhaustive` linter instead of silently inheriting one. The
// trailing return exists because the compiler cannot see that, and is
// unreachable in a lint-clean tree.
func drawsOnSharedWindow(frame ports.Frame) bool {
	switch frame.Mode() {
	case domain.ModeMonitorSelect:
		return false
	case domain.ModeHints, domain.ModeGrid, domain.ModeRecursiveGrid,
		domain.ModeScroll, domain.ModeIdle:
		return true
	}

	return true
}

// RedrawFrame draws a frame whose overlay is already up. Hint labels narrow on
// every keystroke; the window sequence is skipped so a keystroke costs a draw
// and nothing else (ADR 0003).
func (a *Adapter) RedrawFrame(ctx context.Context, frame ports.Frame) error {
	err := contextAlive(ctx)
	if err != nil {
		return err
	}

	return a.draw(frame, updateDraw)
}

// ClearFrame takes the frame on screen off it and returns the overlay to idle.
func (a *Adapter) ClearFrame(ctx context.Context) error {
	err := contextAlive(ctx)
	if err != nil {
		return err
	}

	a.logger.Debug("Clearing overlay frame")

	a.manager.Clear()
	a.manager.ClearCache()
	a.hideMonitorSelect()
	a.manager.Hide()
	a.manager.SwitchTo(ModeIdle)
	a.subgridDrawn.Store(false)

	return nil
}

// SetActiveScreen names the display the overlay's screen-local content belongs
// to.
func (a *Adapter) SetActiveScreen(screen image.Rectangle) {
	a.manager.SetActiveScreenOrigin(screen.Min)
}

// Flush commits everything drawn since the last flush.
func (a *Adapter) Flush() {
	a.manager.Flush()
}

// DrawHintSearch draws the hint search input over the hints frame. Its
// geometry comes from the Style the overlay resolved and the screen the
// caller names, so no caller carries a position here.
//
// The anchor is placed before anything is drawn, and an anchor this overlay
// cannot place is reported instead of approximated: the search box would
// otherwise appear in a corner the user did not configure, with nothing
// anywhere saying so. The mode handler degrades quietly on CodeNotSupported —
// search itself keeps working, because the query reaches the hints through the
// key stream — so the warning that explains the missing box is logged here.
func (a *Adapter) DrawHintSearch(search ports.HintSearch) error {
	style := ResolvedStyle(a.styles)

	frame, frameErr := searchInputFrame(style.HintSearchLayout, search.Screen)
	if frameErr != nil {
		a.logger.Warn(
			"Hint search input anchor not placed by the overlay",
			zap.String("position", style.HintSearchLayout.Position),
		)

		return frameErr
	}

	drawErr := a.manager.DrawHintSearchInput(
		search.Query,
		search.ResultCount,
		frame,
		style.HintSearchInput,
	)
	if drawErr != nil {
		if derrors.IsNotSupported(drawErr) {
			return drawErr
		}

		return derrors.Wrap(drawErr, derrors.CodeOverlayFailed, "failed to draw hint search input")
	}

	return nil
}

// HideHintSearch takes the search input off the screen.
func (a *Adapter) HideHintSearch() {
	a.manager.HideHintSearchInput()
}

// HintSearchBounds reports where the search input sits on a screen.
//
// It answers a rectangle and has no way to report, so an anchor the placement
// refuses answers the empty one. That is the honest answer rather than a
// degradation of its own: the draw refused too, so there is no box on screen
// for the platform's IME field to be put over, and the empty rectangle is
// already what the handler falls back to when it has no overlay at all. The
// draw path logs the reason, which is why this one does not repeat it on every
// search that opens.
func (a *Adapter) HintSearchBounds(screen image.Rectangle) image.Rectangle {
	layout := ResolvedStyle(a.styles).HintSearchLayout

	placed, placeErr := searchInputFrame(layout, screen)
	if placeErr != nil {
		return image.Rectangle{}
	}

	return image.Rectangle{
		Min: placed.Position(),
		Max: placed.Position().Add(image.Pt(placed.Width(), layout.Height)),
	}
}

// UpdateGridMatches narrows the grid on screen to a prefix.
func (a *Adapter) UpdateGridMatches(prefix string) {
	a.manager.UpdateGridMatches(prefix)
}

// SetGridHideUnmatched says whether cells that no longer match disappear.
func (a *Adapter) SetGridHideUnmatched(hide bool) {
	a.manager.SetHideUnmatched(hide)
}

// ShowGridSubgrid opens the finer grid inside one cell, with the grid Style
// the overlay already resolved.
func (a *Adapter) ShowGridSubgrid(cell *domainGrid.Cell) {
	a.manager.ShowSubgrid(cell, ResolvedStyle(a.styles).Grid)
	a.subgridDrawn.Store(true)
}

// UpdateGridPointer places the pointer stand-in on a grid mode's surface, or
// takes it off. Its size and color come from the resolved Style, so no caller
// carries appearance here.
func (a *Adapter) UpdateGridPointer(mode domain.Mode, pointer ports.GridPointer) {
	surface, surfaceErr := overlayMode(mode)
	if surfaceErr != nil {
		a.logger.Debug("Grid pointer names no mode this adapter draws",
			zap.String("mode", domain.ModeString(mode)))

		return
	}

	if !pointer.Visible {
		a.manager.HideGridPointer(surface)

		return
	}

	style := ResolvedStyle(a.styles).VirtualPointer

	a.manager.DrawGridPointer(surface, pointer.Position, style.FontSize, style.FillColor)
}

// overlayMode translates a mode into the overlay's own name for it. The two
// vocabularies share their spelling deliberately; a mode this overlay has none
// for is reported rather than drawn in whatever mode happened to be current.
func overlayMode(mode domain.Mode) (Mode, error) {
	name := domain.ModeString(mode)
	if name == domain.UnknownMode {
		return ModeIdle, derrors.New(
			derrors.CodeNotSupported,
			"overlay frame names no mode this adapter knows",
		)
	}

	return Mode(name), nil
}

// contextAlive reports the caller's context as an overlay error if it has
// already been canceled.
func contextAlive(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
		return nil
	}
}

// DrawModeIndicator draws a mode indicator at the specified position.
func (a *Adapter) DrawModeIndicator(x, y int) {
	a.manager.DrawModeIndicator(x, y)
}

// DrawStickyModifiersIndicator draws the sticky modifiers indicator at the specified position.
func (a *Adapter) DrawStickyModifiersIndicator(x, y int, symbols string) {
	a.manager.DrawStickyModifiersIndicator(x, y, symbols)
}

// DrawVirtualPointer draws the cursor-following virtual pointer. Its size and
// color come from the resolved Style the adapter already holds, so a caller
// never carries appearance through the mode layer to get here.
func (a *Adapter) DrawVirtualPointer(x, y int) {
	style := ResolvedStyle(a.styles).VirtualPointer

	a.manager.DrawVirtualPointer(x, y, style.FontSize, style.FillColor)
}

// ShowIndicator makes an indicator visible.
func (a *Adapter) ShowIndicator(indicator ports.Indicator) {
	a.manager.ShowIndicator(indicator)
}

// HideIndicator takes an indicator off the screen, content and all.
func (a *Adapter) HideIndicator(indicator ports.Indicator) {
	a.manager.HideIndicator(indicator)
}

// ResizeIndicatorToActiveScreen sizes an indicator to the active display.
func (a *Adapter) ResizeIndicatorToActiveScreen(indicator ports.Indicator) {
	a.manager.ResizeIndicatorToActiveScreen(indicator)
}

// DrawMouseActionIndicator draws a transient mouse action indicator.
func (a *Adapter) DrawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
	a.manager.DrawMouseActionIndicator(point, style)
}

// IsVisible returns true if any overlay is currently visible.
func (a *Adapter) IsVisible() bool {
	return a.manager.Mode() != ModeIdle
}

// Refresh updates the overlay display.
func (a *Adapter) Refresh(ctx context.Context) error {
	err := contextAlive(ctx)
	if err != nil {
		return err
	}

	a.logger.Debug("Refreshing overlay")
	a.manager.ResizeToActiveScreen()
	a.logger.Debug("Overlay refreshed")

	return nil
}

// ApplyConfig hands the overlay a configuration that has just changed, so it
// re-resolves every Style and pushes the new configuration to the components it
// draws through. This is the single notification a config reload owes the
// overlay; the fan-out it replaced is what left an overlay in the old colors
// when a call site was missed.
func (a *Adapter) ApplyConfig(cfg *config.Config) {
	if a.styles == nil {
		return
	}

	a.styles.Apply(cfg)
}

// RefreshStyles re-resolves those Styles against the configuration the overlay
// already holds. A light/dark change goes through here.
func (a *Adapter) RefreshStyles() {
	if a.styles == nil {
		return
	}

	a.styles.Refresh()
}

// SetHiddenInScreenShare excludes the overlay from screen captures, or stops
// excluding it. Backends that cannot exclude themselves ignore it.
func (a *Adapter) SetHiddenInScreenShare(hidden bool) {
	a.manager.SetSharingType(hidden)
}

// Destroy releases everything the overlay owns.
func (a *Adapter) Destroy() {
	a.manager.Destroy()
}

// SetKeyboardCaptureEnabled holds or releases the keyboard on the backends
// whose surface can, and does nothing on the ones that cannot.
//
// Asking is always safe, so this is a plain port method rather than an
// optional capability: what confines the behavior to Linux is the caller's own
// gate on the event tap, and a backend with no grab to release implements
// manager.KeyboardCaptureController as a no-op or not at all.
func (a *Adapter) SetKeyboardCaptureEnabled(enabled bool) {
	controller, ok := a.manager.(KeyboardCaptureController)
	if !ok {
		return
	}

	controller.SetKeyboardCaptureEnabled(enabled)
}

// Health checks if the overlay manager is responsive.
func (a *Adapter) Health(_ context.Context) error {
	if reporter, ok := a.manager.(CapabilityReporter); ok {
		capability := reporter.OverlayCapabilities()
		if !capability.Supported() {
			return derrors.New(derrors.CodeNotSupported, capability.Detail)
		}
	}

	return nil
}

// drawKind says whether a draw is bringing its frame up or repainting one
// already on screen. Only the surfaces whose backend keeps state between draws
// care, and each of them says why below.
type drawKind int

const (
	// transitionDraw is the first draw of a frame, after the window sequence.
	transitionDraw drawKind = iota
	// updateDraw repaints a frame already on screen.
	updateDraw
)

// draw renders a frame's content. It is where the frame's domain values become
// render models and meet the Style the overlay resolved — the one direction
// this conversion runs, so no caller upstream names either.
func (a *Adapter) draw(frame ports.Frame, kind drawKind) error {
	switch drawn := frame.(type) {
	case ports.HintsFrame:
		return a.drawHints(drawn)
	case ports.GridFrame:
		return a.drawGrid(drawn, kind)
	case ports.RecursiveGridFrame:
		return a.drawRecursiveGrid(drawn, kind)
	case ports.MonitorSelectFrame:
		return a.drawMonitorSelect(drawn)
	case ports.ScrollFrame:
		return a.drawScroll(kind)
	default:
		return derrors.New(derrors.CodeNotSupported, "overlay frame cannot be drawn")
	}
}

// drawHints draws the hint labels a frame carries. Hints arrive in global
// coordinates and the overlay draws in screen-local ones.
func (a *Adapter) drawHints(frame ports.HintsFrame) error {
	origin := frame.Screen.Min

	drawn := make([]*overlayHints.Hint, len(frame.Hints))
	for index, label := range frame.Hints {
		drawn[index] = overlayHints.NewHint(
			label.Label(),
			image.Point{
				X: label.Position().X - origin.X,
				Y: label.Position().Y - origin.Y,
			},
			label.Bounds().Size(),
			label.MatchedPrefix(),
		)
	}

	drawErr := a.manager.DrawHintsWithStyle(drawn, ResolvedStyle(a.styles).Hints)
	if drawErr != nil {
		// A backend with no hint surface reports CodeNotSupported, and the
		// caller degrades on that code — so it travels unwrapped.
		if derrors.IsNotSupported(drawErr) {
			return drawErr
		}

		return derrors.Wrap(drawErr, derrors.CodeOverlayFailed, "failed to draw hints")
	}

	return nil
}

// drawGrid draws the cells a grid frame carries.
//
// The surface is cleared first when there is something on it this grid cannot
// replace by being drawn over: the frame is coming up for the first time, or a
// subgrid is on screen. A subgrid is the interesting case — the backend's
// incremental path compares grids, so it cannot diff one away — and this is
// the only place that knows one is up, because this is what drew it.
//
// Clearing otherwise would be worse than wasteful: the mode and
// sticky-modifier indicators are painted on the same surface on Linux, so a
// theme change that cleared would blink them out until the next poll.
func (a *Adapter) drawGrid(frame ports.GridFrame, kind drawKind) error {
	if frame.Grid == nil {
		return derrors.New(derrors.CodeNotSupported, "grid frame carries no grid")
	}

	if a.subgridDrawn.Swap(false) || kind == transitionDraw {
		a.manager.Clear()
	}

	drawErr := a.manager.DrawGrid(frame.Grid, frame.Input, ResolvedStyle(a.styles).Grid)
	if drawErr != nil {
		if derrors.IsNotSupported(drawErr) {
			return drawErr
		}

		return derrors.Wrap(drawErr, derrors.CodeOverlayFailed, "failed to draw grid")
	}

	return nil
}

// drawRecursiveGrid draws the region a recursive-grid frame carries, with the
// pointer that rides the same surface.
//
// A transition clears first and a redraw does not: the backend animates from
// the bounds it last drew, so a fresh activation must not zoom out of the
// region the previous one left behind, and every keystroke after it must.
func (a *Adapter) drawRecursiveGrid(frame ports.RecursiveGridFrame, kind drawKind) error {
	if kind == transitionDraw {
		a.manager.Clear()
	}

	style := ResolvedStyle(a.styles)

	drawErr := a.manager.DrawRecursiveGrid(
		frame.Bounds,
		frame.Depth,
		frame.Layout.Keys,
		frame.Layout.Dimensions,
		frame.NextLayout.Keys,
		frame.NextLayout.Dimensions,
		style.RecursiveGrid,
		recursiveGridPointer(frame.Pointer, style.VirtualPointer),
	)
	if drawErr != nil {
		if derrors.IsNotSupported(drawErr) {
			return drawErr
		}

		return derrors.Wrap(drawErr, derrors.CodeOverlayFailed, "failed to draw recursive grid")
	}

	return nil
}

// drawMonitorSelect draws the panels a monitor-select frame carries, through
// the backend's optional monitor-select capability.
//
// That capability is reached by type assertion rather than by widening the
// port, which is the extension pattern every optional backend capability here
// follows: a backend without it reports CodeNotSupported and the mode refuses
// to activate rather than engaging with nothing on screen.
func (a *Adapter) drawMonitorSelect(frame ports.MonitorSelectFrame) error {
	selector, capable := a.manager.(MonitorSelector)
	if !capable {
		return derrors.New(
			derrors.CodeNotSupported,
			"monitor_select overlay is unavailable on this backend",
		)
	}

	if len(frame.Targets) == 0 {
		a.hideMonitorSelect()

		return nil
	}

	targets := make([]MonitorSelectTarget, len(frame.Targets))
	for index, target := range frame.Targets {
		targets[index] = MonitorSelectTarget{
			Bounds:           target.Bounds,
			Label:            target.Label,
			Subtitle:         target.Name,
			Selected:         target.Selected,
			MatchedPrefixLen: target.MatchedPrefixLen,
		}
	}

	drawErr := selector.DrawMonitorSelect(targets, ResolvedStyle(a.styles).MonitorSelect)
	if drawErr != nil {
		if derrors.IsNotSupported(drawErr) {
			return drawErr
		}

		return derrors.Wrap(drawErr, derrors.CodeOverlayFailed, "failed to draw monitor_select")
	}

	a.monitorSelectDrawn.Store(true)

	return nil
}

// clearSurfacesTheFrameDoesNotOwn takes down anything still on screen that the
// incoming frame neither owns nor draws over.
//
// Only the monitor picker qualifies. Every other surface is the shared window,
// which a transition draw clears; the picker's panels are windows of their
// own, so a mode coming up over them clears nothing that removes them. Most
// transitions leave a mode through ClearFrame, which already takes them down —
// but scroll is entered without leaving the previous mode first, so this is
// what keeps the picker from outliving the mode that opened it.
func (a *Adapter) clearSurfacesTheFrameDoesNotOwn(frame ports.Frame) {
	if frame.Mode() == domain.ModeMonitorSelect {
		return
	}

	a.hideMonitorSelect()
}

// hideMonitorSelect takes the monitor picker's panels off the screen, if this
// adapter put any there. They do not live on the shared surface, so clearing
// that surface leaves them behind — which is how an overlay outlives the mode
// that owned it.
func (a *Adapter) hideMonitorSelect() {
	if !a.monitorSelectDrawn.Swap(false) {
		return
	}

	if selector, capable := a.manager.(MonitorSelector); capable {
		selector.HideMonitorSelect()
	}
}

// drawScroll draws a scroll frame, which has nothing of its own to draw.
// Coming up it still clears the shared surface: scroll puts no content there,
// so whatever the previous mode left would stay on screen under the scroll
// indicator. The window is up by then and empty either way, so there is
// nothing for the clear to blink out.
func (a *Adapter) drawScroll(kind drawKind) error {
	if kind == transitionDraw {
		a.manager.Clear()
		a.manager.ClearCache()
	}

	return nil
}

// recursiveGridPointer meets the pointer a frame describes with the Style the
// overlay resolved. Position is the caller's; everything else is appearance,
// and appearance never travels on a frame.
func recursiveGridPointer(
	pointer ports.GridPointer,
	style VirtualPointerStyle,
) overlayRecursiveGrid.VirtualPointerState {
	if !pointer.Visible {
		return overlayRecursiveGrid.VirtualPointerState{}
	}

	return overlayRecursiveGrid.VirtualPointerState{
		Visible:   true,
		Position:  pointer.Position,
		Size:      style.FontSize,
		FillColor: style.FillColor,
		Char:      style.Char,
		FontName:  style.FontFamily,
	}
}

// searchInputFrame places the search input on a screen. The anchor, size and
// insets are configuration, resolved with the rest of the Style; only the
// screen it lands on arrives with the draw. The result is clamped so a bad
// offset cannot push the input off the display.
//
// Every anchor config.SearchInputPositions() declares has a branch below, and
// an anchor outside it is reported rather than placed. `top_left` is one of
// those branches on purpose: it is "the insets are the position outright", and
// it used to reach that answer by falling through to the same empty default an
// unrecognized anchor did — right for `top_left` by coincidence, and silent for
// an anchor added to the vocabulary without a branch here, which would validate
// and then be drawn in a corner nobody chose.
//
// The empty string is refused with the rest. Unlike `hints.ui.placement` there
// is no fallback to settle it to: the validator refuses an unset anchor rather
// than filling one in (config/search_input_position.go), so the only Style that
// carries one is a Style that was never resolved — whose width and height are
// zero as well.
func searchInputFrame(
	layout SearchInputLayout,
	screen image.Rectangle,
) (overlayHints.SearchInputFrame, error) {
	screenWidth := screen.Dx()
	screenHeight := screen.Dy()

	width := layout.Width
	if screenWidth > 0 && width > screenWidth {
		width = screenWidth
	}

	height := layout.Height
	xOffset := layout.XOffset
	yOffset := layout.YOffset

	centered := (screenWidth-width)/config.DefaultSearchInputCenterDivisor + layout.XOffset

	switch layout.Position {
	case config.SearchInputPositionTopLeft:
		// The configured insets are the position outright; nothing to derive
		// from the screen.
	case config.SearchInputPositionTopCenter:
		xOffset = centered
	case config.SearchInputPositionTopRight:
		xOffset = screenWidth - width - layout.XOffset
	case config.SearchInputPositionCenter:
		xOffset = centered
		yOffset = (screenHeight-height)/config.DefaultSearchInputCenterDivisor + layout.YOffset
	case config.SearchInputPositionBottomLeft:
		yOffset = screenHeight - height - layout.YOffset
	case config.SearchInputPositionBottomCenter:
		xOffset = centered
		yOffset = screenHeight - height - layout.YOffset
	case config.SearchInputPositionBottomRight:
		xOffset = screenWidth - width - layout.XOffset
		yOffset = screenHeight - height - layout.YOffset
	default:
		return overlayHints.SearchInputFrame{}, derrors.New(
			derrors.CodeNotSupported,
			"hint search input anchor is not placed by the overlay",
		)
	}

	xOffset = max(xOffset, 0)
	yOffset = max(yOffset, 0)

	if screenWidth > 0 && xOffset+width > screenWidth {
		xOffset = screenWidth - width
	}

	if screenHeight > 0 && yOffset+height > screenHeight {
		yOffset = screenHeight - height
	}

	return overlayHints.NewSearchInputFrame(image.Point{X: xOffset, Y: yOffset}, width), nil
}

// Ensure Adapter implements ports.OverlayPort.
var _ ports.OverlayPort = (*Adapter)(nil)
