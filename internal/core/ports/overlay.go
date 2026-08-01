package ports

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/core/domain/hint"
)

// HintDisplay defines the interface for displaying hint overlays.
type HintDisplay interface {
	// ShowHints displays hint labels on the screen.
	ShowHints(ctx context.Context, hints []*hint.Interface) error
}

// GridDisplay defines the interface for displaying grid overlays.
type GridDisplay interface {
	// ShowGrid displays the grid overlay.
	ShowGrid(ctx context.Context) error
}

// HighlightDisplay defines the interface for displaying highlight overlays.
type HighlightDisplay interface {
	// DrawModeIndicator draws a mode indicator at the specified position.
	DrawModeIndicator(x, y int)
}

// OverlayVisibility defines the interface for overlay visibility management.
type OverlayVisibility interface {
	// Show shows the overlay.
	Show()

	// Hide hides the overlays from the screen.
	Hide(ctx context.Context) error

	// IsVisible returns true if any overlay is currently visible.
	IsVisible() bool

	// Refresh updates the overlay display (e.g., after screen changes).
	Refresh(ctx context.Context) error
}

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

// OverlayPort defines the interface for managing UI overlays.
// Implementations handle the platform-specific rendering of hints and grids.
type OverlayPort interface {
	HealthCheck

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

// GridConfig configures the grid overlay display.
type GridConfig struct {
	// Rows specifies the number of grid rows.
	Rows int

	// Columns specifies the number of grid columns.
	Columns int

	// ShowLabels determines whether to show cell labels.
	ShowLabels bool

	// HighlightedCell specifies which cell to highlight (-1 for none).
	HighlightedCell int
}

// OverlayCapabilityReporter is an optional OverlayPort extension for managers
// that can report their own support state.
//
// The overlay backend is the one subsystem whose availability is decided at
// runtime rather than at build time: a Wayland manager may come up without
// layer-shell, or an X11 manager without a usable display. OverlayPort.Health
// asks the manager through this interface and reports CodeNotSupported with the
// manager's own detail text.
//
// This used to be declared as overlay.CapabilityReporter inside the overlay
// package, which meant Neru had two capability vocabularies. It lives here now
// so the overlay entry in PlatformCapabilities and this reporter describe the
// same thing.
//
// Managers that do not implement it are treated as healthy.
type OverlayCapabilityReporter interface {
	// OverlayCapabilities reports whether this manager can currently render.
	OverlayCapabilities() FeatureCapability
}
