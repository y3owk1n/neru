package overlay

import (
	"context"
	"image"

	"go.uber.org/zap"

	overlayHints "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/hint"
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

// ShowHints displays hint labels on the screen.
func (a *Adapter) ShowHints(ctx context.Context, hints []*hint.Interface) error {
	// Check context
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	a.logger.Debug("Showing hints overlay", zap.Int("hint_count", len(hints)))

	// Convert domain hints to overlay hints for rendering
	overlayHintList := make([]*overlayHints.Hint, len(hints))
	for index, hint := range hints {
		overlayHintList[index] = overlayHints.NewHint(
			hint.Label(),
			hint.Position(),
			hint.Bounds().Size(),
			hint.MatchedPrefix(),
		)
	}

	// Show the overlay window
	a.manager.Show()
	a.manager.SwitchTo("hints")

	drawHintsErr := a.manager.DrawHintsWithStyle(
		overlayHintList,
		ResolvedStyle(a.styles).Hints,
	)
	if drawHintsErr != nil {
		return derrors.Wrap(drawHintsErr, derrors.CodeOverlayFailed, "failed to draw hints")
	}

	a.logger.Debug("Hints overlay displayed", zap.Int("count", len(hints)))

	return nil
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

// Hide removes all overlays from the screen.
func (a *Adapter) Hide(ctx context.Context) error {
	// Check context
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	a.logger.Debug("Hiding overlay")
	a.manager.Hide()
	a.manager.SwitchTo("idle")
	a.logger.Debug("Overlay hidden")

	return nil
}

// IsVisible returns true if any overlay is currently visible.
func (a *Adapter) IsVisible() bool {
	return a.manager.Mode() != "idle"
}

// Refresh updates the overlay display.
func (a *Adapter) Refresh(ctx context.Context) error {
	// Check context
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
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

// Ensure Adapter implements ports.OverlayPort.
var _ ports.OverlayPort = (*Adapter)(nil)
