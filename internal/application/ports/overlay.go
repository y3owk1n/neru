package ports

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain/hint"
)

// OverlayPort defines the interface for managing UI overlays.
// Implementations handle the platform-specific rendering of hints and grids.
type OverlayPort interface {
	// ShowHints displays hint labels on the screen.
	ShowHints(ctx context.Context, hints []*hint.Hint) error

	// ShowGrid displays the grid overlay.
	ShowGrid(ctx context.Context, rows, cols int) error

	// DrawScrollHighlight draws a highlight for scroll mode.
	DrawScrollHighlight(ctx context.Context, rect image.Rectangle, color string, width int) error

	// DrawActionHighlight draws a highlight border for action mode.
	DrawActionHighlight(ctx context.Context, rect image.Rectangle, color string, width int) error

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
