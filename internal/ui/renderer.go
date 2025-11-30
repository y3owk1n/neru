package ui

import (
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// OverlayRenderer manages rendering operations for all application overlays.
type OverlayRenderer struct {
	manager   overlay.ManagerInterface
	hintStyle hints.StyleMode
	gridStyle grid.Style
}

// NewOverlayRenderer initializes a new overlay renderer with the specified components.
func NewOverlayRenderer(
	manager overlay.ManagerInterface,
	hintStyle hints.StyleMode,
	gridStyle grid.Style,
) *OverlayRenderer {
	return &OverlayRenderer{
		manager:   manager,
		hintStyle: hintStyle,
		gridStyle: gridStyle,
	}
}

// UpdateConfig updates the renderer with new configuration.
func (r *OverlayRenderer) UpdateConfig(hintStyle hints.StyleMode, gridStyle grid.Style) {
	r.hintStyle = hintStyle
	r.gridStyle = gridStyle
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

// DrawActionHighlight draws an action highlight border around the active screen.
func (r *OverlayRenderer) DrawActionHighlight(x, y, width, height int) {
	r.manager.DrawActionHighlight(x, y, width, height)
}

// DrawScrollHighlight draws a scroll highlight border around the active screen.
func (r *OverlayRenderer) DrawScrollHighlight(x, y, width, height int) {
	r.manager.DrawScrollHighlight(x, y, width, height)
}
