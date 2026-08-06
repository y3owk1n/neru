package overlay

import (
	"context"
	"image"

	"go.uber.org/zap"

	overlayHints "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/ports"
)

// Adapter implements ports.OverlayPort by wrapping the existing overlay.Manager.
type Adapter struct {
	manager ManagerInterface
	styles  StyleSource
	logger  *zap.Logger
}

// NewAdapter creates a new overlay adapter. styles is the resolved Style the
// adapter draws with; it is never rebuilt here, so a draw costs no theme
// lookup.
func NewAdapter(
	manager ManagerInterface,
	styles StyleSource,
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

	mode, modeErr := frameMode(frame)
	if modeErr != nil {
		return modeErr
	}

	a.logger.Debug("Showing overlay frame", zap.String("mode", string(mode)))

	a.manager.ResizeToActiveScreen()
	a.manager.Show()
	a.manager.SwitchTo(mode)

	return a.draw(frame)
}

// RedrawFrame draws a frame whose overlay is already up. Hint labels narrow on
// every keystroke; the window sequence is skipped so a keystroke costs a draw
// and nothing else (ADR 0003).
func (a *Adapter) RedrawFrame(ctx context.Context, frame ports.Frame) error {
	err := contextAlive(ctx)
	if err != nil {
		return err
	}

	return a.draw(frame)
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
	a.manager.Hide()
	a.manager.SwitchTo(ModeIdle)

	return nil
}

// DrawHintSearch draws the hint search input over the hints frame. Its
// geometry comes from the Style the overlay resolved and the screen the
// caller names, so no caller carries a position here.
func (a *Adapter) DrawHintSearch(search ports.HintSearch) error {
	style := ResolvedStyle(a.styles)

	drawErr := a.manager.DrawHintSearchInput(
		search.Query,
		search.ResultCount,
		searchInputFrame(style.HintSearchLayout, search.Screen),
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
func (a *Adapter) HintSearchBounds(screen image.Rectangle) image.Rectangle {
	layout := ResolvedStyle(a.styles).HintSearchLayout
	placed := searchInputFrame(layout, screen)

	return image.Rectangle{
		Min: placed.Position(),
		Max: placed.Position().Add(image.Pt(placed.Width(), layout.Height)),
	}
}

// frameMode translates the mode a frame names into the overlay's own name for
// it. The two vocabularies share their spelling deliberately; a frame naming a
// mode this overlay has none for is reported rather than drawn in whatever
// mode happened to be current.
func frameMode(frame ports.Frame) (Mode, error) {
	name := domain.ModeString(frame.Mode())
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

// draw renders a frame's content. It is where the frame's domain values become
// render models and meet the Style the overlay resolved — the one direction
// this conversion runs, so no caller upstream names either.
func (a *Adapter) draw(frame ports.Frame) error {
	switch drawn := frame.(type) {
	case ports.HintsFrame:
		return a.drawHints(drawn)
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

// searchInputFrame places the search input on a screen. The anchor, size and
// insets are configuration, resolved with the rest of the Style; only the
// screen it lands on arrives with the draw. The result is clamped so a bad
// offset cannot push the input off the display.
func searchInputFrame(
	layout SearchInputLayout,
	screen image.Rectangle,
) overlayHints.SearchInputFrame {
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
	case overlayHints.SearchInputTopCenter:
		xOffset = centered
	case overlayHints.SearchInputTopRight:
		xOffset = screenWidth - width - layout.XOffset
	case overlayHints.SearchInputCenter:
		xOffset = centered
		yOffset = (screenHeight-height)/config.DefaultSearchInputCenterDivisor + layout.YOffset
	case overlayHints.SearchInputBottomLeft:
		yOffset = screenHeight - height - layout.YOffset
	case overlayHints.SearchInputBottomCenter:
		xOffset = centered
		yOffset = screenHeight - height - layout.YOffset
	case overlayHints.SearchInputBottomRight:
		xOffset = screenWidth - width - layout.XOffset
		yOffset = screenHeight - height - layout.YOffset
	case overlayHints.SearchInputTopLeft:
		fallthrough
	default:
	}

	xOffset = max(xOffset, 0)
	yOffset = max(yOffset, 0)

	if screenWidth > 0 && xOffset+width > screenWidth {
		xOffset = screenWidth - width
	}

	if screenHeight > 0 && yOffset+height > screenHeight {
		yOffset = screenHeight - height
	}

	return overlayHints.NewSearchInputFrame(image.Point{X: xOffset, Y: yOffset}, width)
}

// Ensure Adapter implements ports.OverlayPort.
var _ ports.OverlayPort = (*Adapter)(nil)
