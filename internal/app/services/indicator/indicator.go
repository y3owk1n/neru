package indicator

import (
	"context"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// Base is the shared half of an indicator service: it owns whether the
// indicator is on screen, how big its surface is, and the cursor read that
// every position update begins with. The service that embeds it adds the one
// thing indicators do differently — what they draw.
//
// A Base with no overlay is the "this indicator was never constructed" case:
// a disabled indicator, or a headless backend with nothing to draw on. Every
// method answers that itself, so mode logic can drive an indicator without
// first asking whether it exists.
type Base struct {
	indicator ports.Indicator
	system    ports.SystemPort
	overlay   ports.OverlayPort
}

// NewBase returns the shared half of the service for one indicator. Either
// port may be nil; an indicator reports nothing of its own, so there is no
// logger — what a caller would want logged is the mode change that drove it.
func NewBase(
	indicator ports.Indicator,
	system ports.SystemPort,
	overlay ports.OverlayPort,
) Base {
	return Base{
		indicator: indicator,
		system:    system,
		overlay:   overlay,
	}
}

// GetCursorPosition returns the current cursor position.
func (b *Base) GetCursorPosition(ctx context.Context) (int, int, error) {
	if b.system == nil {
		return 0, 0, derrors.New(derrors.CodeActionFailed, "system port not available")
	}

	point, err := b.system.CursorPosition(ctx)
	if err != nil {
		return 0, 0, derrors.WrapAccessibilityFailed(err, "get cursor position")
	}

	return point.X, point.Y, nil
}

// Show makes the indicator visible. It draws nothing: what appears is
// whatever the next position update draws.
func (b *Base) Show() {
	if b.overlay == nil {
		return
	}

	b.overlay.ShowIndicator(b.indicator)
}

// Hide takes the indicator off the screen, content and all.
func (b *Base) Hide() {
	if b.overlay == nil {
		return
	}

	b.overlay.HideIndicator(b.indicator)
}

// ResizeToActiveScreen sizes the indicator to the display the cursor is on.
// Modes that manage their own overlay windows skip the manager's whole-overlay
// resize, so an indicator can otherwise still be sized for the display it was
// last shown on.
func (b *Base) ResizeToActiveScreen() {
	if b.overlay == nil {
		return
	}

	b.overlay.ResizeIndicatorToActiveScreen(b.indicator)
}

// Overlay returns the port the concrete service draws through, or nil when
// this indicator has no overlay behind it.
func (b *Base) Overlay() ports.OverlayPort {
	return b.overlay
}
