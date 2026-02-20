package ui

import (
	"image"

	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// OverlayRenderer manages rendering operations for all application overlays.
type OverlayRenderer struct {
	manager            overlay.ManagerInterface
	hintStyle          hints.StyleMode
	gridStyle          grid.Style
	recursiveGridStyle recursivegrid.Style
}

// NewOverlayRenderer initializes a new overlay renderer with the specified components.
func NewOverlayRenderer(
	manager overlay.ManagerInterface,
	hintStyle hints.StyleMode,
	gridStyle grid.Style,
	recursiveGridStyle recursivegrid.Style,
) *OverlayRenderer {
	return &OverlayRenderer{
		manager:            manager,
		hintStyle:          hintStyle,
		gridStyle:          gridStyle,
		recursiveGridStyle: recursiveGridStyle,
	}
}

// UpdateConfig updates the renderer with new configuration.
func (r *OverlayRenderer) UpdateConfig(
	hintStyle hints.StyleMode,
	gridStyle grid.Style,
	recursiveGridStyle recursivegrid.Style,
) {
	r.hintStyle = hintStyle
	r.gridStyle = gridStyle
	r.recursiveGridStyle = recursiveGridStyle
}

// DrawHints draws hints with the configured style.
func (r *OverlayRenderer) DrawHints(hs []*hints.Hint) error {
	return r.manager.DrawHintsWithStyle(hs, r.hintStyle)
}

// DrawGrid draws a grid with the configured style.
func (r *OverlayRenderer) DrawGrid(g *domainGrid.Grid, input string) error {
	return r.manager.DrawGrid(g, input, r.gridStyle)
}

// ShowSubgrid shows a subgrid for the specified cell.
func (r *OverlayRenderer) ShowSubgrid(
	cell *domainGrid.Cell,
) {
	r.manager.ShowSubgrid(cell, r.gridStyle)
}

// UpdateGridMatches updates the grid matches with the specified prefix.
func (r *OverlayRenderer) UpdateGridMatches(prefix string) {
	r.manager.UpdateGridMatches(prefix)
}

// SetHideUnmatched sets whether to hide unmatched cells.
func (r *OverlayRenderer) SetHideUnmatched(hide bool) {
	r.manager.SetHideUnmatched(hide)
}

// Show shows the overlay.
func (r *OverlayRenderer) Show() {
	r.manager.Show()
}

// Clear clears the overlay.
func (r *OverlayRenderer) Clear() {
	r.manager.Clear()
}

// ResizeActive resizes the overlay to the active screen.
func (r *OverlayRenderer) ResizeActive() {
	r.manager.ResizeToActiveScreen()
}

// DrawScrollIndicator draws a scroll indicator at the specified position.
func (r *OverlayRenderer) DrawScrollIndicator(x, y int) {
	r.manager.DrawScrollIndicator(x, y)
}

// DrawRecursiveGrid draws a recursive-grid with the current bounds and depth.
func (r *OverlayRenderer) DrawRecursiveGrid(
	bounds image.Rectangle,
	depth int,
	keys string,
	gridSize int,
) error {
	return r.manager.DrawRecursiveGrid(bounds, depth, keys, gridSize, r.recursiveGridStyle)
}
