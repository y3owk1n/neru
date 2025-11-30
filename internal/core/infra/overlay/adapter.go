package overlay

import (
	"context"
	"image"

	gridFeature "github.com/y3owk1n/neru/internal/app/components/grid"
	overlayHints "github.com/y3owk1n/neru/internal/app/components/hints"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"github.com/y3owk1n/neru/internal/core/ports"
	uiOverlay "github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// Adapter implements ports.OverlayPort by wrapping the existing overlay.Manager.
type Adapter struct {
	manager uiOverlay.ManagerInterface
	logger  *zap.Logger
}

// NewAdapter creates a new overlay adapter.
func NewAdapter(manager uiOverlay.ManagerInterface, logger *zap.Logger) *Adapter {
	return &Adapter{
		manager: manager,
		logger:  logger,
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

	// Draw hints using the overlay manager
	// Retrieve config from overlay to build current style
	var style overlayHints.StyleMode
	if hintOverlay := a.manager.HintOverlay(); hintOverlay != nil {
		style = overlayHints.BuildStyle(hintOverlay.Config())
	}

	drawHintsErr := a.manager.DrawHintsWithStyle(overlayHintList, style)
	if drawHintsErr != nil {
		return derrors.Wrap(drawHintsErr, derrors.CodeOverlayFailed, "failed to draw hints")
	}

	a.logger.Info("Hints overlay displayed", zap.Int("count", len(hints)))

	return nil
}

// ShowGrid displays the grid overlay.
func (a *Adapter) ShowGrid(ctx context.Context) error {
	// Check context
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	// Get screen bounds
	bounds := bridge.ActiveScreenBounds()

	// Create grid
	grid := domainGrid.NewGrid("abcdefghijklmnopqrstuvwxyz", bounds, a.logger)

	// Draw grid
	drawGridErr := a.manager.DrawGrid(grid, "", gridFeature.Style{})
	if drawGridErr != nil {
		return derrors.Wrap(drawGridErr, derrors.CodeActionFailed, "failed to draw grid")
	}

	// Show overlay and switch mode
	a.manager.Show()
	a.manager.SwitchTo("grid")

	return nil
}

// DrawScrollHighlight draws a highlight for scroll mode.
func (a *Adapter) DrawScrollHighlight(
	ctx context.Context,
	rect image.Rectangle,
	_ string,
	_ int,
) error {
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	a.manager.DrawScrollHighlight(
		0,
		0,
		rect.Dx(),
		rect.Dy(),
	)

	// Show overlay and switch mode
	a.manager.Show()
	a.manager.SwitchTo("scroll")

	return nil
}

// DrawActionHighlight draws a highlight border for action mode.
func (a *Adapter) DrawActionHighlight(
	ctx context.Context,
	rect image.Rectangle,
	_ string,
	_ int,
) error {
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	// Use manager to draw action highlight (overlay is positioned at screen origin, so use local coords)
	a.manager.DrawActionHighlight(
		0,
		0,
		rect.Dx(),
		rect.Dy(),
	)

	return nil
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
	a.logger.Info("Overlay hidden")

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
	a.logger.Info("Overlay refreshed")

	return nil
}

// Health checks if the overlay manager is responsive.
func (a *Adapter) Health(_ context.Context) error {
	// For now, we assume if we can call methods, it's healthy.
	// Ideally, we'd ping the UI process.
	return nil
}

// Ensure Adapter implements ports.OverlayPort.
var _ ports.OverlayPort = (*Adapter)(nil)
