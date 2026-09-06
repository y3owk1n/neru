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
//
// Destroy is final: every method below no-ops once it has run, the way
// eventtap.Adapter's do since #1514. The shutdown is what needs it — the event
// tap's teardown drains its dispatcher, and that drain delivers whatever key
// was still queued into the mode handler, which draws — so a caller can reach
// this adapter after the backend it wraps has been freed. The ordering in
// App.Cleanup answers that for the caller it knows about; this answers it for
// every caller, including the ones a later change adds.
type Adapter struct {
	manager ManagerInterface
	styles  StyleOwner
	logger  *zap.Logger

	// destroyed is set by Destroy before it releases the backend. It is an
	// atomic rather than the mutex the event tap adapter guards the same state
	// with, because this adapter holds no lock of its own and must not grow
	// one: a draw runs under the mode handler's h.mu and may block in the
	// backend's renderMu, so a lock between them would sit on the h.mu ->
	// renderMu edge that every keystroke's draw crosses.
	destroyed atomic.Bool

	// teardownDone is closed once the backend has been released. A second
	// caller waits on it rather than returning early, so Destroy keeps its
	// postcondition — the overlay is released when it returns — for the caller
	// that raced the first one. It is allocated by NewAdapter rather than by
	// the claim, which is where the event tap adapter allocates its own: that
	// one claims under a mutex it already had, and this one claims with a
	// compare-and-swap because the flag above is an atomic. NewAdapter is the
	// only constructor — every other field is unexported and a zero Adapter has
	// no backend to release.
	teardownDone chan struct{}

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
		manager:      manager,
		styles:       styles,
		logger:       logger.Named("overlay"),
		teardownDone: make(chan struct{}),
	}
}

// ShowFrame puts a Frame on screen. This is the only place the transition
// sequence exists: size the overlay to the active screen, show it, switch it
// to the frame's mode, then draw. It used to be open-coded at every call site,
// where getting the order wrong showed an empty overlay or left the previous
// mode's on screen.
func (a *Adapter) ShowFrame(ctx context.Context, frame ports.Frame) error {
	if a.releasedReporting("ShowFrame") {
		return nil
	}

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
// Linux user in scroll mode with no indicator after a monitor move. A declared
// mode draws nothing of its own either, and needs the window for the same
// reason.
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
		domain.ModeScroll, domain.ModeCustom, domain.ModeIdle:
		return true
	}

	return true
}

// RedrawFrame draws a frame whose overlay is already up. Hint labels narrow on
// every keystroke; the window sequence is skipped so a keystroke costs a draw
// and nothing else (ADR 0003).
func (a *Adapter) RedrawFrame(ctx context.Context, frame ports.Frame) error {
	if a.releasedReporting("RedrawFrame") {
		return nil
	}

	err := contextAlive(ctx)
	if err != nil {
		return err
	}

	return a.draw(frame, updateDraw)
}

// ClearFrame takes the frame on screen off it and returns the overlay to idle.
//
// It also drops what the grid's incremental calls left behind, which grid mode
// used to reset for itself on the way out (#1492). Both run after the surface
// has been cleared, which is what makes them cost nothing where the reset used
// to cost a repaint: the hide-unmatched flag is a flag, and a backend that
// repaints on a pointer change has already forgotten the pointer, so the hide
// is the statement without the repaint. What each backend does beyond that is
// its own — macOS marks its emptied view for redisplay either way — and this is
// teardown, not a keystroke. The match prefix needs no reset at all: a grid
// coming back up is a transition, which clears and redraws in full.
//
// The pair is unconditional rather than gated on grid mode having been the one
// on screen. The leaving half does not ask which mode it is leaving — that is
// what stops a caller from having to remember — and neither call means anything
// to a surface no grid was drawn on.
func (a *Adapter) ClearFrame(ctx context.Context) error {
	if a.releasedReporting("ClearFrame") {
		return nil
	}

	err := contextAlive(ctx)
	if err != nil {
		return err
	}

	a.logger.Debug("Clearing overlay frame")

	a.manager.Clear()
	a.manager.ClearCache()
	a.hideMonitorSelect()
	a.manager.SetHideUnmatched(false)
	a.manager.HideGridPointer(ModeGrid)
	a.manager.Hide()
	a.manager.SwitchTo(ModeIdle)
	a.subgridDrawn.Store(false)

	return nil
}

// SetActiveScreen names the display the overlay's screen-local content belongs
// to.
func (a *Adapter) SetActiveScreen(screen image.Rectangle) {
	if a.released() {
		return
	}

	a.manager.SetActiveScreenOrigin(screen.Min)
}

// Flush commits everything drawn since the last flush.
func (a *Adapter) Flush() {
	if a.released() {
		return
	}

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
	if a.releasedReporting("DrawHintSearch") {
		return nil
	}

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
	if a.released() {
		return
	}

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
	// A destroyed overlay answers the empty rectangle for the same reason a
	// refused anchor does: there is no box on screen for the platform's IME
	// field to be put over.
	if a.released() {
		return image.Rectangle{}
	}

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
	if a.released() {
		return
	}

	a.manager.UpdateGridMatches(prefix)
}

// SetGridHideUnmatched says whether cells that no longer match disappear.
func (a *Adapter) SetGridHideUnmatched(hide bool) {
	if a.released() {
		return
	}

	a.manager.SetHideUnmatched(hide)
}

// ShowGridSubgrid opens the finer grid inside one cell, with the grid Style
// the overlay already resolved and the pointer stand-in that belongs on the
// same surface.
//
// The pointer rides the open rather than following it in a call of its own
// (#1492), which is what makes the keystroke that picks a cell cost one repaint
// on a backend that paints the two into one surface. Meeting it with the
// resolved Style happens here, as it does for the pointer a recursive-grid
// frame carries — appearance never travels from a mode.
func (a *Adapter) ShowGridSubgrid(cell *domainGrid.Cell, pointer ports.GridPointer) {
	if a.released() {
		return
	}

	style := ResolvedStyle(a.styles)

	a.manager.ShowSubgrid(cell, style.Grid, gridSurfacePointer(pointer, style.VirtualPointer))
	a.subgridDrawn.Store(true)
}

// UpdateGridPointer places the pointer stand-in on a grid mode's surface, or
// takes it off. Its whole appearance — size, fill, char and family — comes from
// the resolved Style, so no caller carries any of it here.
func (a *Adapter) UpdateGridPointer(mode domain.Mode, pointer ports.GridPointer) {
	if a.released() {
		return
	}

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

	a.manager.DrawGridPointer(
		surface,
		pointer.Position,
		pointerAppearance(ResolvedStyle(a.styles).VirtualPointer),
	)
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
	if a.released() {
		return
	}

	a.manager.DrawModeIndicator(x, y)
}

// DrawStickyModifiersIndicator draws the sticky modifiers indicator at the specified position.
func (a *Adapter) DrawStickyModifiersIndicator(x, y int, symbols string) {
	if a.released() {
		return
	}

	a.manager.DrawStickyModifiersIndicator(x, y, symbols)
}

// DrawVirtualPointer draws the cursor-following virtual pointer. Its size and
// color come from the resolved Style the adapter already holds, so a caller
// never carries appearance through the mode layer to get here.
func (a *Adapter) DrawVirtualPointer(posX, posY int) {
	if a.released() {
		return
	}

	style := ResolvedStyle(a.styles).VirtualPointer

	a.manager.DrawVirtualPointer(posX, posY, style.FontSize, style.FillColor)
}

// ShowIndicator makes an indicator visible.
func (a *Adapter) ShowIndicator(indicator ports.Indicator) {
	if a.released() {
		return
	}

	a.manager.ShowIndicator(indicator)
}

// HideIndicator takes an indicator off the screen, content and all.
func (a *Adapter) HideIndicator(indicator ports.Indicator) {
	if a.released() {
		return
	}

	a.manager.HideIndicator(indicator)
}

// ResizeIndicatorToActiveScreen sizes an indicator to the active display.
func (a *Adapter) ResizeIndicatorToActiveScreen(indicator ports.Indicator) {
	if a.released() {
		return
	}

	a.manager.ResizeIndicatorToActiveScreen(indicator)
}

// DrawMouseActionIndicator draws a transient mouse action indicator.
func (a *Adapter) DrawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
	if a.released() {
		return
	}

	a.manager.DrawMouseActionIndicator(point, style)
}

// IsVisible returns true if any overlay is currently visible. A destroyed
// overlay answers false: nothing it drew is on screen any more.
func (a *Adapter) IsVisible() bool {
	if a.released() {
		return false
	}

	return a.manager.Mode() != ModeIdle
}

// Refresh updates the overlay display.
func (a *Adapter) Refresh(ctx context.Context) error {
	if a.releasedReporting("Refresh") {
		return nil
	}

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
	if a.released() || a.styles == nil {
		return
	}

	a.styles.Apply(cfg)
}

// RefreshStyles re-resolves those Styles against the configuration the overlay
// already holds. A light/dark change goes through here.
func (a *Adapter) RefreshStyles() {
	if a.released() || a.styles == nil {
		return
	}

	a.styles.Refresh()
}

// SetHiddenInScreenShare excludes the overlay from screen captures, or stops
// excluding it. Backends that cannot exclude themselves ignore it.
func (a *Adapter) SetHiddenInScreenShare(hidden bool) {
	if a.released() {
		return
	}

	a.manager.SetSharingType(hidden)
}

// Destroy releases everything the overlay owns, and is safe to call twice.
//
// The adapter marks itself destroyed before the backend is released rather
// than after, so a caller arriving during the teardown finds an overlay that
// has already stopped answering instead of one whose native window is being
// freed underneath it. What the flag does not do is close that window: a
// caller already past the check when this runs is still inside the backend, and
// nothing here waits for it. Serializing the two would mean a lock across every
// draw, which is exactly what this adapter must not have — the release reaches
// a backend that may block (Linux takes renderMu, macOS dispatch_syncs to the
// main queue) and this adapter sits on the mode handler's h.mu -> renderMu
// edge. What closes the window is the caller: App.Cleanup tears the event tap
// down first, so the drain that delivers a queued key has finished before this
// runs.
//
// The one call that arrives on a goroutine of its own is
// SetHiddenInScreenShare — AppState publishes screen-share state with a
// goroutine per subscriber — so ordering cannot be what closes that one, and
// unsubscribing does not either: a goroutine already launched is past the
// point where removing the subscription reaches it. It is closed where the
// state it touches lives, in the darwin manager, which takes the mutex
// dedicated to that flag across its own teardown (darwin/manager.go). Every
// other backend answers SetSharingType with a no-op and has no such pair.
func (a *Adapter) Destroy() {
	if !a.destroyed.CompareAndSwap(false, true) {
		<-a.teardownDone

		return
	}

	defer close(a.teardownDone)

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
	if a.released() {
		return
	}

	controller, ok := a.manager.(KeyboardCaptureController)
	if !ok {
		return
	}

	controller.SetKeyboardCaptureEnabled(enabled)
}

// Health checks if the overlay manager is responsive.
//
// A destroyed overlay reports CodeNotSupported rather than answering healthy:
// there is no surface left, and the code is the one every caller already
// degrades on.
func (a *Adapter) Health(_ context.Context) error {
	if a.released() {
		return derrors.New(derrors.CodeNotSupported, "the overlay has been destroyed")
	}

	if reporter, ok := a.manager.(CapabilityReporter); ok {
		capability := reporter.OverlayCapabilities()
		if !capability.Supported() {
			return derrors.New(derrors.CodeNotSupported, capability.Detail)
		}
	}

	return nil
}

// released reports whether the overlay has been destroyed, which every method
// on the port answers by doing nothing.
func (a *Adapter) released() bool {
	return a.destroyed.Load()
}

// releasedReporting is released for the methods that answer an error. They
// report a success they did not achieve, the way the event tap adapter's
// Enable and Disable do, so the refusal is logged and the log is what explains
// the difference afterwards. The methods that answer nothing have nothing to
// correct and ask the plain question.
func (a *Adapter) releasedReporting(call string) bool {
	if !a.released() {
		return false
	}

	a.logger.Debug("Overlay call ignored: the overlay has been destroyed",
		zap.String("call", call))

	return true
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
		gridSurfacePointer(frame.Pointer, style.VirtualPointer),
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

// gridSurfacePointer meets the pointer a frame or a subgrid open describes with
// the Style the overlay resolved. Position is the caller's; everything else is
// appearance, and appearance never travels from a mode.
func gridSurfacePointer(
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
