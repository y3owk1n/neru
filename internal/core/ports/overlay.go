package ports

import (
	"context"

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
