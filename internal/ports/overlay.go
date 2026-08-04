package ports

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain/hint"
)

// MouseActionIndicatorStyle configures a transient mouse action indicator.
type MouseActionIndicatorStyle struct {
	Size              int
	BorderWidth       int
	BackgroundColor   string
	BorderColor       string
	Shape             string
	DurationMS        int
	StartScale        float64
	EndScale          float64
	StartOpacity      float64
	EndOpacity        float64
	Easing            string
	HideInScreenShare bool
}

// OverlayPort is the interface for managing UI overlays.
// Implementations handle the platform-specific rendering of hints and grids.
type OverlayPort interface {
	// Health returns nil if the component is healthy, or an error if it is not.
	Health(ctx context.Context) error

	// ShowHints displays hint labels on the screen.
	ShowHints(ctx context.Context, hints []*hint.Interface) error

	// ShowGrid displays the grid overlay.
	ShowGrid(ctx context.Context) error

	// Show shows the overlay.
	Show()

	// DrawModeIndicator draws a mode indicator at the specified position.
	DrawModeIndicator(x, y int)

	// DrawStickyModifiersIndicator draws the sticky modifiers indicator at the specified position.
	DrawStickyModifiersIndicator(x, y int, symbols string)

	// DrawMouseActionIndicator draws a transient indicator for a mouse action.
	DrawMouseActionIndicator(point image.Point, style MouseActionIndicatorStyle)

	// Hide hides the overlay.s from the screen.
	Hide(ctx context.Context) error

	// IsVisible returns true if any overlay is currently visible.
	IsVisible() bool

	// Refresh updates the overlay display (e.g., after screen changes).
	Refresh(ctx context.Context) error
}

// OverlayCapabilityReporter is an optional OverlayPort extension for managers
// that can report their own support state — the one subsystem whose
// availability is decided at runtime (Wayland without layer-shell, X11 without
// a display). OverlayPort.Health asks through this and reports
// CodeNotSupported with the manager's detail. Managers that do not implement
// it are treated as healthy.
type OverlayCapabilityReporter interface {
	// OverlayCapabilities reports whether this manager can currently render.
	OverlayCapabilities() FeatureCapability
}
