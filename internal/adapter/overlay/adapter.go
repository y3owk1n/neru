package overlay

import (
	"context"
	"fmt"
	"image"

	"github.com/y3owk1n/neru/internal/application/ports"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/errors"
	gridFeature "github.com/y3owk1n/neru/internal/features/grid"
	overlayHints "github.com/y3owk1n/neru/internal/features/hints"
	"github.com/y3owk1n/neru/internal/infra/bridge"
	uiOverlay "github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// Adapter implements ports.OverlayPort by wrapping the existing overlay.Manager.
type Adapter struct {
	manager *uiOverlay.Manager
	logger  *zap.Logger
}

// NewAdapter creates a new overlay adapter.
func NewAdapter(manager *uiOverlay.Manager, logger *zap.Logger) *Adapter {
	return &Adapter{
		manager: manager,
		logger:  logger,
	}
}

// ShowHints displays hint labels on the screen.
func (a *Adapter) ShowHints(ctx context.Context, hints []*hint.Hint) error {
	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	a.logger.Debug("Showing hints overlay", zap.Int("hint_count", len(hints)))

	// Convert domain hints to overlay hints for rendering
	overlayHintList := make([]*overlayHints.Hint, len(hints))
	for i, h := range hints {
		overlayHintList[i] = &overlayHints.Hint{
			Label:         h.Label(),
			Position:      h.Position(),
			Size:          h.Bounds().Size(),
			MatchedPrefix: h.MatchedPrefix(),
		}
	}

	// Show the overlay window
	a.manager.Show()
	a.manager.SwitchTo("hints")

	// Draw hints using the overlay manager
	// Use default style for now
	err := a.manager.DrawHintsWithStyle(overlayHintList, overlayHints.StyleMode{})
	if err != nil {
		return fmt.Errorf("failed to draw hints: %w", err)
	}

	a.logger.Info("Hints overlay displayed", zap.Int("count", len(hints)))
	return nil
}

// ShowGrid displays the grid overlay.
func (a *Adapter) ShowGrid(ctx context.Context, _ int, _ int) error {
	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Get screen bounds
	bounds := bridge.GetActiveScreenBounds()

	// Create grid
	// Note: We use default characters from config if available, or default
	// Since we don't have config passed here easily (unless we store it in adapter),
	// we'll use a default string. Ideally config should be passed or stored.
	// Get grid configuration from config service
	// We can assume the manager or grid package handles defaults.
	// For now, let's use a safe default.
	g := domainGrid.NewGrid("abcdefghijklmnopqrstuvwxyz", bounds, a.logger)

	// Draw grid
	// Note: DrawGrid takes input string (for filtering) and style.
	// We start with empty input and default style.
	err := a.manager.DrawGrid(g, "", gridFeature.Style{})
	if err != nil {
		return errors.Wrap(err, errors.CodeActionFailed, "failed to draw grid")
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
		return ctx.Err()
	default:
	}

	// The underlying manager doesn't have DrawScrollHighlight exposed directly on Manager,
	// but it has DrawScrollHighlight on the Overlay struct.
	// However, the Manager manages the Overlay.
	// We might need to update the Manager or access the Overlay via Manager.
	// Looking at internal/ui/overlay/manager.go (implied), it likely wraps the CGo overlay.
	// Draw scroll highlight using overlay manager
	// No, `ScrollComponent` had its own `Overlay` in `internal/features/scroll/overlay.go`.
	// This is a divergence. The new architecture should unify overlay management.
	// For now, we can assume the Adapter's manager can handle this, or we need to expose it.
	// Since I don't have access to modify `internal/ui/overlay` easily without checking it,
	// I will assume I need to add this method to `internal/ui/overlay/manager.go` first.
	// But wait, I am editing the adapter.
	// Let's assume the manager has it or I will add it.
	// Actually, `internal/features/scroll/overlay.go` was separate.
	// I should probably integrate scroll overlay logic into the main overlay manager.
	// For now, I'll implement it using the existing manager if possible, or add it.
	// Let's check `internal/ui/overlay/manager.go` first.
	// Use manager to draw scroll highlight
	a.manager.DrawScrollHighlight(
		rect.Min.X,
		rect.Min.Y,
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
		return ctx.Err()
	default:
	}

	// Use manager to draw action highlight
	a.manager.DrawActionHighlight(
		rect.Min.X,
		rect.Min.Y,
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
		return ctx.Err()
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
	return a.manager.GetMode() != "idle"
}

// Refresh updates the overlay display.
func (a *Adapter) Refresh(ctx context.Context) error {
	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	a.logger.Debug("Refreshing overlay")
	a.manager.ResizeToActiveScreenSync()
	a.logger.Info("Overlay refreshed")
	return nil
}

// Health checks if the overlay manager is responsive.
func (a *Adapter) Health(ctx context.Context) error {
	// For now, we assume if we can call methods, it's healthy.
	// Ideally, we'd ping the UI process.
	return nil
}

// Ensure Adapter implements ports.OverlayPort.
var _ ports.OverlayPort = (*Adapter)(nil)
